package gate

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// AC-006 / TC-009: a truth body with a UI/design chapter while the
// frontmatter says ux_required: false is a requirement-level mismatch.
// The default verdict is audit (AC-101's frozen-package caliber);
// block mode turns it into a blocking verdict.
func TestIntegrityFlagsUxMismatch(t *testing.T) {
	in := baseIntegrityInput()
	in.Pkg.Truth.Text = strings.Replace(goodTruth, "status: frozen",
		"status: frozen\nux_required: false", 1) +
		"\n## UI 规格说明书\n\n- 首页布局\n"

	rep := CheckIntegrity(in, IntegrityModeAuditOnly)
	var found *Finding
	for i := range rep.Findings {
		if rep.Findings[i].Kind == "truth_ux_requirement_mismatch" {
			found = &rep.Findings[i]
		}
	}
	if found == nil {
		t.Fatalf("ux mismatch finding missing: %+v", rep.Findings)
	}
	if found.ReasonCode != core.ReasonIntegrityUxMismatch {
		t.Fatalf("reason code = %s", found.ReasonCode)
	}
	if rep.Verdict != VerdictAudit {
		t.Fatalf("default verdict must be audit, got %s", rep.Verdict)
	}
	if !hasReasonCode(rep.ReasonCodes, core.ReasonIntegrityUxMismatch) {
		t.Fatalf("audit report must carry the ux reason code: %v", rep.ReasonCodes)
	}

	blocked := CheckIntegrity(in, IntegrityModeBlock)
	if blocked.Verdict != VerdictBlocked {
		t.Fatalf("strict/block mode must block the mismatch, got %s", blocked.Verdict)
	}

	// The declaration agreeing with the body is not a finding.
	ok := baseIntegrityInput()
	ok.Pkg.Truth.Text = strings.Replace(goodTruth, "status: frozen",
		"status: frozen\nux_required: true", 1) + "\n## UI 规格说明书\n\n- 首页布局\n"
	for _, f := range CheckIntegrity(ok, IntegrityModeBlock).Findings {
		if f.Kind == "truth_ux_requirement_mismatch" {
			t.Fatalf("ux_required: true must not be flagged: %+v", f)
		}
	}
}

func hasReasonCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}
