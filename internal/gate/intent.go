// Intent gate (R1: ask before sign). The freeze gate's intent half:
// before a package can be signed, the author must have recorded the
// user's intent, closed every open question, traced every AC to an
// intent source, and produced the domain FSM / parity materials the
// package's own frontmatter requires.
//
// The interaction (one question at a time) lives in the truth
// ask/answer CLI; this file is the pure, deterministic judgement over
// the resulting markdown. No prompts, no model calls — the same bytes
// always produce the same findings, so the gate is auditable.
package gate

import (
	"regexp"
	"strings"

	"beacon-go/internal/core"
)

// IntentSnapshot is the recorded user-intent block.
type IntentSnapshot struct {
	Utterance   string `json:"utterance"`
	Goal        string `json:"goal"`
	NonGoals    string `json:"non_goals"`
	Constraints string `json:"constraints"`
	ScopeMode   string `json:"scope_mode"`
}

// OpenQuestion is one row of the open-question ledger.
type OpenQuestion struct {
	ID             string `json:"id"`
	Question       string `json:"question"`
	Recommendation string `json:"recommendation,omitempty"`
	Status         string `json:"status"`
	Answer         string `json:"answer,omitempty"`
	Line           int    `json:"line,omitempty"`
}

// IntentCoverageRow traces one AC to the intent/evidence it serves.
type IntentCoverageRow struct {
	AC       string `json:"ac"`
	Source   string `json:"source"`
	Evidence string `json:"evidence"`
	Line     int    `json:"line,omitempty"`
}

// FSMRow is one row of the domain state machine.
type FSMRow struct {
	State  string `json:"state"`
	Event  string `json:"event"`
	Guard  string `json:"guard"`
	To     string `json:"to"`
	Action string `json:"action"`
	Line   int    `json:"line,omitempty"`
}

var placeholderPrefixes = []string{"(fill", "（fill", "(todo", "（todo", "待补", "待填", "tbd", "todo"}

func isPlaceholder(s string) bool {
	t := strings.ToLower(strings.TrimSpace(s))
	if t == "" || t == "-" || t == "n/a" {
		return true
	}
	for _, p := range placeholderPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// sectionText returns the body of the first `## ` section whose
// heading contains name (case-insensitive), without the heading line.
func sectionText(text, name string) (string, bool) {
	name = strings.ToLower(name)
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") && strings.Contains(strings.ToLower(line), name) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func splitRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// tableOf returns the first markdown table in a section: lowercased
// headers plus rows (separator rows skipped).
func tableOf(section string) (headers []string, rows [][]string) {
	lines := strings.Split(section, "\n")
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") {
			if in && t != "" {
				break
			}
			continue
		}
		cells := splitRow(t)
		if !in {
			in = true
			for _, c := range cells {
				headers = append(headers, strings.ToLower(strings.TrimSpace(c)))
			}
			continue
		}
		if isSeparatorRow(cells) {
			continue
		}
		rows = append(rows, cells)
	}
	return headers, rows
}

func col(headers []string, row []string, names ...string) string {
	for i, h := range headers {
		for _, n := range names {
			if h == n {
				if i < len(row) {
					return strings.TrimSpace(row[i])
				}
				return ""
			}
		}
	}
	return ""
}

func yamlBlock(section string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(section, "\n")
	in := false
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			in = !in
			continue
		}
		if !in || t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	// Tolerate key: value lines without a fenced block.
	if len(out) == 0 {
		for _, line := range lines {
			t := strings.TrimSpace(line)
			k, v, ok := strings.Cut(t, ":")
			if !ok || strings.Contains(k, "|") {
				continue
			}
			out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return out
}

// ParseIntentSnapshot reads the `## User Intent Snapshot` block.
func ParseIntentSnapshot(truth string) IntentSnapshot {
	sec, _ := sectionText(truth, "user intent snapshot")
	m := yamlBlock(sec)
	return IntentSnapshot{
		Utterance: m["utterance"], Goal: m["goal"],
		NonGoals: m["non_goals"], Constraints: m["constraints"],
		ScopeMode: m["scope_mode"],
	}
}

func questionStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "open"
	}
	return s
}

func isClosedStatus(s string) bool {
	switch questionStatus(s) {
	case "answered", "dropped", "closed", "resolved", "done", "n/a":
		return true
	}
	return false
}

// ParseOpenQuestions reads the `## Open Questions` ledger.
func ParseOpenQuestions(truth string) []OpenQuestion {
	sec, _ := sectionText(truth, "open questions")
	headers, rows := tableOf(sec)
	out := []OpenQuestion{}
	for i, r := range rows {
		q := OpenQuestion{
			ID:             col(headers, r, "q id", "id", "question id"),
			Question:       col(headers, r, "question"),
			Recommendation: col(headers, r, "recommendation"),
			Status:         questionStatus(col(headers, r, "status")),
			Answer:         col(headers, r, "answer"),
			Line:           i + 1,
		}
		if q.ID == "" && q.Question == "" {
			continue
		}
		out = append(out, q)
	}
	return out
}

// ParseIntentCoverage reads the `## Intent Coverage` matrix.
func ParseIntentCoverage(truth string) []IntentCoverageRow {
	sec, _ := sectionText(truth, "intent coverage")
	headers, rows := tableOf(sec)
	out := []IntentCoverageRow{}
	for i, r := range rows {
		ac := strings.ToUpper(col(headers, r, "ac id", "ac", "requirement"))
		if !acRefRe.MatchString(ac) {
			continue
		}
		out = append(out, IntentCoverageRow{
			AC: ac, Source: col(headers, r, "intent source", "source"),
			Evidence: col(headers, r, "evidence", "evidence ref"), Line: i + 1,
		})
	}
	return out
}

// ParseFSM reads the `## Domain FSM` table.
func ParseFSM(truth string) []FSMRow {
	sec, _ := sectionText(truth, "domain fsm")
	headers, rows := tableOf(sec)
	out := []FSMRow{}
	for i, r := range rows {
		out = append(out, FSMRow{
			State: col(headers, r, "state"), Event: col(headers, r, "event"),
			Guard: col(headers, r, "guard"), To: col(headers, r, "to"),
			Action: col(headers, r, "action"), Line: i + 1,
		})
	}
	return out
}

var fsmHangRe = regexp.MustCompile(`(?i)hang|挂起|挂死`)
var fsmIllegalRe = regexp.MustCompile(`(?i)illegal|invalid|非法`)

// CheckIntentGate is the pure intent judgement over one package.
func CheckIntentGate(pkg PackageFiles) []Finding {
	if !pkg.Truth.Present || strings.TrimSpace(pkg.Truth.Text) == "" {
		return nil // completeness owns the missing-file finding
	}
	file := pkg.Truth.Path
	scalars, _, _ := ParseFrontmatter(pkg.Truth.Text)
	var out []Finding
	add := func(kind, code, detail string) {
		out = append(out, Finding{Kind: kind, ReasonCode: code, File: file, Detail: detail, RepairRoute: DefaultRepairRoute})
	}

	if strings.ToLower(strings.TrimSpace(scalars["intent_confirmed"])) != "true" {
		add("intent_not_confirmed", core.ReasonIntentNotConfirmed,
			"intent_confirmed is not true; run `beacon truth confirm` after recording intent and closing open questions")
	}

	snap := ParseIntentSnapshot(pkg.Truth.Text)
	missing := []string{}
	for _, f := range []struct{ name, val string }{
		{"utterance", snap.Utterance}, {"goal", snap.Goal},
		{"non_goals", snap.NonGoals}, {"constraints", snap.Constraints},
		{"scope_mode", snap.ScopeMode},
	} {
		if isPlaceholder(f.val) {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		add("intent_snapshot_incomplete", core.ReasonIntentSnapshotIncomple,
			"User Intent Snapshot is missing: "+strings.Join(missing, ", ")+
				" (one answer per unknown; `beacon truth intent` records the fields)")
	}

	for _, q := range ParseOpenQuestions(pkg.Truth.Text) {
		if !isClosedStatus(q.Status) {
			add("intent_question_open", core.ReasonIntentQuestionOpen,
				q.ID+" is still open: "+q.Question)
		}
	}

	defined := DefinedACs(pkg.Truth.Text)
	covered := map[string]bool{}
	for _, row := range ParseIntentCoverage(pkg.Truth.Text) {
		covered[row.AC] = true
		if !defined[row.AC] {
			add("intent_coverage_unknown_ac", core.ReasonIntentCoverageUnknown,
				"Intent Coverage names "+row.AC+" which truth does not define")
			continue
		}
		if isPlaceholder(row.Source) || isPlaceholder(row.Evidence) {
			add("intent_coverage_untraced", core.ReasonIntentCoverageUntraced,
				row.AC+" has no traced intent source/evidence")
		}
	}
	var acs []string
	for id := range defined {
		acs = append(acs, id)
	}
	sortStrings(acs)
	for _, id := range acs {
		if !covered[id] {
			add("intent_coverage_missing", core.ReasonIntentCoverageMissing,
				id+" is defined in truth but absent from the Intent Coverage matrix")
		}
	}

	if strings.EqualFold(strings.TrimSpace(scalars["domain_required"]), "true") {
		sec, _ := sectionText(pkg.Truth.Text, "domain fsm")
		rows := ParseFSM(pkg.Truth.Text)
		if len(rows) == 0 {
			add("intent_fsm_missing", core.ReasonIntentFSMMissing,
				"domain_required is true but no Domain FSM table is present")
		} else {
			for _, r := range rows {
				if isPlaceholder(r.State) || isPlaceholder(r.Event) || isPlaceholder(r.Guard) ||
					isPlaceholder(r.To) || isPlaceholder(r.Action) {
					add("intent_fsm_incomplete", core.ReasonIntentFSMIncomplete,
						"Domain FSM row is missing State/Event/Guard/To/Action")
					break
				}
			}
			if !fsmHangRe.MatchString(sec) || !fsmIllegalRe.MatchString(sec) {
				add("intent_fsm_incomplete", core.ReasonIntentFSMIncomplete,
					"Domain FSM must name at least one hang state and one illegal transition")
			}
		}
	}

	maturity := strings.ToLower(strings.TrimSpace(scalars["package_maturity"]))
	if maturity == "" || maturity == "scaffold" {
		add("intent_package_scaffold", core.ReasonIntentPackageScaffold,
			"package_maturity is scaffold/empty; fill the package before signing")
	}
	if human, ok := sectionText(pkg.Truth.Text, "人话"); !ok || isPlaceholder(human) ||
		strings.Contains(human, "一句话说清这个需求") || strings.Contains(human, "冻结前由作者补齐") {
		add("intent_package_scaffold", core.ReasonIntentPackageScaffold,
			"## 人话 still carries the skeleton placeholder; write the one-sentence human summary")
	}

	if strings.EqualFold(strings.TrimSpace(snap.ScopeMode), "full_parity") {
		sec, ok := sectionText(pkg.Truth.Text, "parity matrix")
		if !ok || len(strings.TrimSpace(sec)) == 0 {
			add("intent_parity_missing", core.ReasonIntentParityMissing,
				"scope_mode is full_parity but no Parity Matrix is present")
		}
	}
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
