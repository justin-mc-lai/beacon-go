package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// SDDReviewFingerprint binds authored truth and task definitions, excluding
// only kernel signing bookkeeping. Execution receipts live outside these files.
func SDDReviewFingerprint(pkg PackageFiles) string {
	decl := ParseSurfaceDeclaration(pkg.Truth.Text)
	normalize := func(text string) string {
		lines := strings.Split(text, "\n")
		out := lines[:0]
		inFront := false
		for i, line := range lines {
			if i == 0 && line == "---" {
				inFront = true
				out = append(out, line)
				continue
			}
			if inFront && line == "---" {
				inFront = false
				out = append(out, line)
				continue
			}
			key, _, ok := strings.Cut(line, ":")
			if inFront && ok && key == "surface_strict" {
				continue
			}
			if inFront && ok && (strings.HasPrefix(key, "sdd_review_") || strings.HasPrefix(key, "intent_confirmation_")) {
				continue
			}
			if inFront && ok && (key == "status" || key == "materials_status" || key == "intent_confirmed") {
				line = key + ": <bookkeeping>"
			}
			out = append(out, line)
		}
		return strings.Join(out, "\n")
	}
	h := sha256.New()
	h.Write([]byte("sdd-review-v1\x00"))
	h.Write([]byte(normalize(pkg.Truth.Text)))
	h.Write([]byte("\x00surface-strict\x00"))
	if decl.EffectiveStrict() {
		h.Write([]byte("true"))
	} else {
		h.Write([]byte("false"))
	}
	h.Write([]byte("\x00tasks\x00"))
	h.Write([]byte(normalize(pkg.Tasks.Text)))
	return hex.EncodeToString(h.Sum(nil))
}

// SDDReviewFindings checks an external review attestation, not the semantic
// correctness of the prose. Reviewer identity is caller-attested, not authenticated.
func SDDReviewFindings(pkg PackageFiles, requireConfirmation bool) []Finding {
	if pkg.Contract != "truth-tasks" {
		return nil
	}
	scalars, _, _ := ParseFrontmatter(pkg.Truth.Text)
	var out []Finding
	add := func(code, detail string) {
		out = append(out, Finding{Kind: code, ReasonCode: code, File: pkg.Truth.Path, Detail: detail, RepairRoute: DefaultRepairRoute})
	}
	author := strings.TrimSpace(scalars["package_author"])
	if author == "" {
		add("sdd_author_missing", "declare package_author so independent review can be checked")
	}
	grounding, ok := sectionText(pkg.Truth.Text, "grounding")
	if !ok {
		add("sdd_grounding_missing", "record grounding references and what they support")
	} else {
		_, rows := tableOf(grounding)
		valid := false
		for _, row := range rows {
			if len(row) >= 3 && !isPlaceholder(row[0]) && !isPlaceholder(row[1]) && !isPlaceholder(row[2]) {
				valid = true
			}
		}
		if !valid {
			add("sdd_grounding_missing", "Grounding needs a source, reference and supported claim")
		}
	}
	assumptions, ok := sectionText(pkg.Truth.Text, "assumptions")
	if !ok || isPlaceholder(assumptions) || strings.Contains(assumptions, "记录需要确认的假设") {
		add("sdd_assumptions_missing", "state confirmed assumptions, or explicitly state none")
	}
	current := SDDReviewFingerprint(pkg)
	reviewer := strings.TrimSpace(scalars["sdd_review_by"])
	if reviewer == "" || scalars["sdd_review_verdict"] != "pass" || scalars["sdd_review_at"] == "" {
		add("sdd_review_required", "record an independent passing semantic review with no blocking findings")
	}
	if reviewer != "" && reviewer == author {
		add("sdd_review_self", "the package author cannot certify semantic review")
	}
	if scalars["sdd_review_fingerprint"] != current {
		add("sdd_review_stale", "semantic review does not bind the current truth and task definitions")
	}
	if requireConfirmation {
		if scalars["intent_confirmation_fingerprint"] != current {
			add("sdd_confirmation_stale", "confirm the exact reviewed truth and task definitions")
		}
		if scalars["intent_confirmation_by"] == "" {
			add("sdd_confirmation_missing", "confirmation actor is missing")
		}
	}
	return out
}
