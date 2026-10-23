// Shared truth-markdown extractors for the R2a freeze-quality
// checks. Each function names the Python function it ports; the
// Python codebase carries several near-identical extractors with
// subtly different rules (review gate vs feature package), and the
// ports keep those differences instead of merging them.
package gate

import (
	"regexp"
	"strings"
)

// truthyValue ports the shared Python _truthy: 1/true/yes/y/on/required.
func truthyValue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on", "required":
		return true
	}
	return false
}

// ACDoc is one acceptance criterion as the review gate sees it.
type ACDoc struct {
	ID          string
	Description string
}

var acIDPatternRe = regexp.MustCompile(`(?i)\b(AC-[A-Z0-9]+(?:-[A-Z0-9]+)*)\b`)

// ExtractACDescriptions ports truth_review_gate.extract_acceptance_criteria:
// AC definition tables first (description = 2nd cell of a 2-column
// table, 3rd cell of a wider one). Multiple tables may mention the
// same AC (coverage matrix before the definition table): Python
// keeps the LONGEST description per id (add_description replaces
// only when longer, length in characters) — verified against the
// Python source and a live run. Falls back to a bare id scan with
// empty descriptions when no table yields rows.
func ExtractACDescriptions(truthMD string) []ACDoc {
	var order []string
	byID := map[string]string{}
	add := func(id, desc string) {
		cur, ok := byID[id]
		if !ok {
			order = append(order, id)
			byID[id] = desc
			return
		}
		if len([]rune(desc)) > len([]rune(cur)) {
			byID[id] = desc
		}
	}
	for _, block := range mdTableBlocks(truthMD) {
		if len(block) == 0 {
			continue
		}
		header := headerJoin(block[0])
		firstTok := ""
		if fields := strings.Fields(header); len(fields) > 0 {
			firstTok = fields[0]
		}
		qualifies := strings.Contains(header, "ac id") || strings.HasPrefix(header, "ac ") ||
			header == "ac description" || firstTok == "ac"
		if !qualifies && !strings.Contains(header, "ac") {
			continue
		}
		for _, row := range block[1:] {
			if len(row) == 0 {
				continue
			}
			m := acIDPatternRe.FindStringSubmatch(row[0])
			if m == nil {
				continue
			}
			desc := ""
			switch {
			case len(row) == 2:
				desc = row[1]
			case len(row) > 2:
				desc = row[2]
			case len(row) > 1:
				desc = row[1]
			}
			add(strings.ToUpper(m[1]), desc)
		}
	}
	if len(order) > 0 {
		out := make([]ACDoc, 0, len(order))
		for _, id := range order {
			out = append(out, ACDoc{ID: id, Description: byID[id]})
		}
		return out
	}
	seen := map[string]bool{}
	var acs []ACDoc
	for _, m := range acIDPatternRe.FindAllStringSubmatch(truthMD, -1) {
		id := strings.ToUpper(m[1])
		if seen[id] {
			continue
		}
		seen[id] = true
		acs = append(acs, ACDoc{ID: id})
	}
	return acs
}

// domainKind ports truth_review_gate._domain_kind: the explicit
// frontmatter kind wins; a truth-review-gate lifecycle package is
// detected heuristically; a domain_required package defaults to
// business, everything else is general.
var domainKindLifecycleRe = regexp.MustCompile(`(?i)lifecycle|package lifecycle|gate_a_b_c`)

func domainKind(scalars map[string]string, truthMD string) string {
	kind := strings.ToLower(strings.TrimSpace(scalars["domain_kind"]))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(scalars["package_kind"]))
	}
	if kind != "" {
		return kind
	}
	low := strings.ToLower(truthMD)
	if strings.Contains(low, "truth review gate") && strings.Contains(low, "domain_required: true") {
		if domainKindLifecycleRe.MatchString(truthMD) {
			return "lifecycle"
		}
	}
	if truthyValue(scalars["domain_required"]) {
		return "business"
	}
	return "general"
}

// businessDomain mirrors check_d_fill_depth's business_domain:
// domain_required and not a lifecycle/gate/platform/meta package.
func businessDomain(scalars map[string]string, truthMD string) bool {
	if !truthyValue(scalars["domain_required"]) {
		return false
	}
	switch domainKind(scalars, truthMD) {
	case "lifecycle", "gate", "platform", "meta":
		return false
	}
	return true
}

// ---- deferral ledger (M5) ----

// DeferralEntry is one row of the package's `## Deferral Ledger`.
// Python's _deferral_ledger generates the obligation; the Go
// package records the user's disposition in the truth document
// itself (the kernel's one source of requirement truth).
type DeferralEntry struct {
	Capability string
	Item       string
	Reason     string
	Status     string
	ApprovedBy string
	Line       int
}

// ParseDeferralLedger reads the `## Deferral Ledger` table.
func ParseDeferralLedger(truth string) []DeferralEntry {
	sec, ok := sectionText(truth, "deferral ledger")
	if !ok {
		return nil
	}
	headers, rows := tableOf(sec)
	var out []DeferralEntry
	for i, r := range rows {
		e := DeferralEntry{
			Capability: col(headers, r, "capability"),
			Item:       col(headers, r, "item", "deferred item"),
			Reason:     col(headers, r, "reason"),
			Status:     strings.ToLower(strings.TrimSpace(col(headers, r, "status"))),
			ApprovedBy: col(headers, r, "approved_by", "approved by", "approver", "批准人"),
			Line:       i + 1,
		}
		if e.Capability == "" && e.Item == "" {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Approved reports whether the entry is a user disposition:
// an approval-shaped status AND a named approver.
func (e DeferralEntry) Approved() bool {
	switch e.Status {
	case "approved", "accepted", "deferred", "approved_deferred", "批准", "已批准", "同意推迟":
		return !isPlaceholder(e.ApprovedBy)
	}
	return false
}

// deferralSignals are planner_review_runtime._deferral_ledger's
// trigger words (line 376), matched against a lowercased blob.
var deferralSignals = []string{"mvp", "defer", "later", "延期", "裁剪", "partial"}

func containsAnySignal(blob string, signals []string) bool {
	for _, s := range signals {
		if strings.Contains(blob, s) {
			return true
		}
	}
	return false
}
