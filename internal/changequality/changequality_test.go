package changequality

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func sampleDiff() CandidateDiff {
	return CandidateDiff{
		Baseline: "base-0",
		Files: []FileDiff{
			{Path: "internal/a.go", Hunks: []Hunk{{ID: "h1", Old: "old-a", New: "new-a"}}},
			{Path: "internal/b.go", Hunks: []Hunk{
				{ID: "h2", Old: "old-b", New: "new-b"},
				{ID: "h1", Old: "old-b1", New: "new-b1"},
			}},
		},
	}
}

func boundReceipt(t *testing.T, cand CandidateDiff) []byte {
	t.Helper()
	raw, err := json.Marshal(Receipt{DiffSHA: cand.SHA256(), Baseline: cand.Baseline})
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	return raw
}

// TC-001 / AC-001: an unset policy is off, and the vocabulary accepts
// exactly off, safe_fix and strict_receipt.
func TestChangeQualityPolicyDefaultsToOff(t *testing.T) {
	if got := NewPolicy("").Mode(); got != ModeOff {
		t.Fatalf("unset mode = %q, want off", got)
	}
	if got := NewPolicy("   ").Mode(); got != ModeOff {
		t.Fatalf("blank mode = %q, want off", got)
	}
	vocabulary := []Mode{ModeOff, ModeSafeFix, ModeStrictReceipt}
	for _, want := range vocabulary {
		p := NewPolicy(string(want))
		if p.Mode() != want {
			t.Fatalf("mode %q = %q, want %q", want, p.Mode(), want)
		}
		if p.ConfigError() != nil {
			t.Fatalf("mode %q reported configuration error: %v", want, p.ConfigError())
		}
	}
	if len(vocabulary) != 3 {
		t.Fatalf("vocabulary size = %d, want 3", len(vocabulary))
	}
	if got := NewPolicy("").ConfigError(); got != nil {
		t.Fatalf("unset mode reported configuration error: %v", got)
	}
}

// TC-002 / AC-001: an unrecognized value resolves to off and reports a
// configuration error instead of upgrading to a stricter mode.
func TestChangeQualityUnknownModeIsReportedAndNotUpgraded(t *testing.T) {
	for _, raw := range []string{"turbo", "SAFE_FIX", "safe-fix", "strict"} {
		p := NewPolicy(raw)
		if p.Mode() != ModeOff {
			t.Fatalf("unknown mode %q resolved to %q, want off", raw, p.Mode())
		}
		var cfg *ConfigError
		if !errors.As(p.ConfigError(), &cfg) {
			t.Fatalf("unknown mode %q error = %v, want ConfigError", raw, p.ConfigError())
		}
		if !errors.Is(p.ConfigError(), ErrInvalidMode) {
			t.Fatalf("unknown mode %q error is not ErrInvalidMode: %v", raw, p.ConfigError())
		}
		if cfg.Value != raw {
			t.Fatalf("ConfigError.Value = %q, want %q", cfg.Value, raw)
		}
		if p.Mode() == ModeSafeFix || p.Mode() == ModeStrictReceipt {
			t.Fatalf("unknown mode %q was upgraded to %q", raw, p.Mode())
		}
	}
}

// TC-003 / AC-002: a receipt binding the exact current diff SHA passes
// premerge, and equivalent diffs hash identically regardless of order.
func TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA(t *testing.T) {
	cand := sampleDiff()
	policy := NewPolicy(string(ModeStrictReceipt))
	decision := policy.EvaluatePremerge(cand, boundReceipt(t, cand))
	if !decision.Allowed {
		t.Fatalf("bound receipt refused: reason=%s detail=%s", decision.Reason, decision.Detail)
	}
	if decision.Mode != ModeStrictReceipt {
		t.Fatalf("decision mode = %q, want strict_receipt", decision.Mode)
	}

	reordered := CandidateDiff{
		Baseline: cand.Baseline,
		Files: []FileDiff{
			{Path: "internal/b.go", Hunks: []Hunk{
				{ID: "h1", Old: "old-b1", New: "new-b1"},
				{ID: "h2", Old: "old-b", New: "new-b"},
			}},
			{Path: "internal/a.go", Hunks: []Hunk{{ID: "h1", Old: "old-a", New: "new-a"}}},
		},
	}
	if cand.SHA256() != reordered.SHA256() {
		t.Fatalf("exact diff SHA depends on ordering: %s vs %s", cand.SHA256(), reordered.SHA256())
	}
	if again := policy.EvaluatePremerge(reordered, boundReceipt(t, cand)); !again.Allowed {
		t.Fatalf("reordered equivalent diff refused: %s", again.Reason)
	}

	changed := sampleDiff()
	changed.Files[0].Hunks[0].New = "tampered"
	if changed.SHA256() == cand.SHA256() {
		t.Fatal("content change did not change the exact diff SHA")
	}
}

// TC-004 / AC-002: a stale, missing, malformed or baseline-mismatched
// receipt is a terminal premerge refusal, never a downgrade to off.
func TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA(t *testing.T) {
	cand := sampleDiff()
	policy := NewPolicy(string(ModeStrictReceipt))

	cases := []struct {
		name       string
		receipt    []byte
		wantReason string
	}{
		{
			name:       "stale sha",
			receipt:    []byte(`{"diff_sha":"deadbeef","baseline":"base-0"}`),
			wantReason: ReasonStaleReceipt,
		},
		{
			name:       "empty receipt",
			receipt:    nil,
			wantReason: ReasonReceiptMissing,
		},
		{
			name:       "blank receipt",
			receipt:    []byte("   \n\t"),
			wantReason: ReasonReceiptMissing,
		},
		{
			name:       "missing sha field",
			receipt:    []byte(`{"baseline":"base-0"}`),
			wantReason: ReasonReceiptMissing,
		},
		{
			name:       "malformed json",
			receipt:    []byte("{not json"),
			wantReason: ReasonReceiptMalformed,
		},
		{
			name:       "baseline mismatch",
			receipt:    []byte(`{"diff_sha":"` + cand.SHA256() + `","baseline":"other-base"}`),
			wantReason: ReasonBaselineMismatch,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := policy.EvaluatePremerge(cand, tc.receipt)
			if decision.Allowed {
				t.Fatalf("receipt %q was accepted", tc.name)
			}
			if decision.Reason != tc.wantReason {
				t.Fatalf("reason = %q, want %q (detail: %s)", decision.Reason, tc.wantReason, decision.Detail)
			}
			if decision.Mode != ModeStrictReceipt {
				t.Fatalf("refusal downgraded mode to %q, want strict_receipt", decision.Mode)
			}
		})
	}
}

// TC-005 / AC-003: one repair whose files and hunks are all inside the
// caller's authorization is applied.
func TestSafeFixAllowsOneBoundedRepairWithinAuthorization(t *testing.T) {
	tree := map[string]string{
		"internal/a.go": "alpha old-a omega\n",
		"internal/b.go": "beta old-b gamma\n",
	}
	auth := Authorization{Files: map[string][]string{
		"internal/a.go": {"h1"},
	}}
	gate, err := NewSafeFixGate(NewPolicy(string(ModeSafeFix)), auth, tree)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	res, err := gate.Request(Repair{Files: []FileRepair{
		{Path: "internal/a.go", Hunks: []RepairHunk{{ID: "h1", Old: "old-a", New: "new-a"}}},
	}})
	if err != nil {
		t.Fatalf("bounded repair refused: %v", err)
	}
	if !res.Applied {
		t.Fatal("bounded repair not applied")
	}
	if res.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", res.Attempts)
	}
	if len(res.FilesChanged) != 1 || res.FilesChanged[0] != "internal/a.go" {
		t.Fatalf("files changed = %v, want [internal/a.go]", res.FilesChanged)
	}
	if got := gate.Tree()["internal/a.go"]; !strings.Contains(got, "new-a") || strings.Contains(got, "old-a") {
		t.Fatalf("file after repair = %q", got)
	}
	if got := gate.Tree()["internal/b.go"]; got != "beta old-b gamma\n" {
		t.Fatalf("unauthorized file changed: %q", got)
	}
	if got := tree["internal/a.go"]; got != "alpha old-a omega\n" {
		t.Fatalf("caller tree mutated: %q", got)
	}
}

// TC-006 / AC-003: a second attempt is refused, and a repair that leaves
// the authorization is fail-closed with no partial change.
func TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed(t *testing.T) {
	tree := map[string]string{
		"internal/a.go": "alpha old-a omega\n",
		"internal/b.go": "beta old-b gamma\n",
	}
	auth := Authorization{Files: map[string][]string{
		"internal/a.go": {"h1"},
	}}
	gate, err := NewSafeFixGate(NewPolicy(string(ModeSafeFix)), auth, tree)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	if _, err := gate.Request(Repair{Files: []FileRepair{
		{Path: "internal/a.go", Hunks: []RepairHunk{{ID: "h1", Old: "old-a", New: "new-a"}}},
	}}); err != nil {
		t.Fatalf("first repair: %v", err)
	}
	afterFirst := gate.Tree()["internal/a.go"]

	res, err := gate.Request(Repair{Files: []FileRepair{
		{Path: "internal/a.go", Hunks: []RepairHunk{{ID: "h1", Old: "new-a", New: "again"}}},
	}})
	if !errors.Is(err, ErrRepairBudgetExhausted) {
		t.Fatalf("second attempt error = %v, want ErrRepairBudgetExhausted", err)
	}
	if res.Applied {
		t.Fatal("second attempt reported applied")
	}
	if got := gate.Tree()["internal/a.go"]; got != afterFirst {
		t.Fatalf("second attempt changed the tree: %q -> %q", afterFirst, got)
	}
	if gate.Attempts() != 1 {
		t.Fatalf("attempts = %d, want 1", gate.Attempts())
	}

	fresh, err := NewSafeFixGate(NewPolicy(string(ModeSafeFix)), auth, tree)
	if err != nil {
		t.Fatalf("new gate: %v", err)
	}
	res, err = fresh.Request(Repair{Files: []FileRepair{
		{Path: "internal/a.go", Hunks: []RepairHunk{{ID: "h1", Old: "old-a", New: "new-a"}}},
		{Path: "internal/b.go", Hunks: []RepairHunk{{ID: "h9", Old: "old-b", New: "new-b"}}},
	}})
	if !errors.Is(err, ErrOutOfAuthorization) {
		t.Fatalf("out-of-authorization error = %v, want ErrOutOfAuthorization", err)
	}
	if res.Applied {
		t.Fatal("out-of-authorization repair reported applied")
	}
	if fresh.Attempts() != 0 {
		t.Fatalf("refused repair consumed the attempt budget: %d", fresh.Attempts())
	}
	if got := fresh.Tree()["internal/a.go"]; got != "alpha old-a omega\n" {
		t.Fatalf("authorized hunk leaked a partial change: %q", got)
	}
	if got := fresh.Tree()["internal/b.go"]; got != "beta old-b gamma\n" {
		t.Fatalf("unauthorized file changed: %q", got)
	}

	if _, err := NewSafeFixGate(NewPolicy(string(ModeOff)), auth, tree); !errors.Is(err, ErrModeDoesNotAllow) {
		t.Fatalf("off-mode gate error = %v, want ErrModeDoesNotAllow", err)
	}
}
