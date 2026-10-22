package core

import (
	"errors"
	"testing"
)

// AC-026: stage/status parsing accepts every contract value and rejects
// unknown ones with a typed error carrying a reason code.
func TestParseWorkflowStageAndStatus(t *testing.T) {
	for _, s := range AllWorkflowStages() {
		got, err := ParseWorkflowStage(s.String())
		if err != nil || got != s {
			t.Errorf("ParseWorkflowStage(%q) = %v, %v", s, got, err)
		}
	}
	for _, s := range AllStageStatuses() {
		got, err := ParseStageStatus(s.String())
		if err != nil || got != s {
			t.Errorf("ParseStageStatus(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := ParseWorkflowStage("Nonsense"); err == nil {
		t.Error("ParseWorkflowStage(Nonsense) must fail")
	} else if rc, ok := ReasonCodeOf(err); !ok || rc != ReasonInvalidStage {
		t.Errorf("reason code = %q,%v want %q", rc, ok, ReasonInvalidStage)
	}
	if _, err := ParseStageStatus(""); err == nil {
		t.Error("ParseStageStatus(\"\") must fail")
	}
	if WorkflowStage("bogus").Valid() || StageStatus("bogus").Valid() {
		t.Error("bogus values must not be Valid")
	}
}

// AC-027 (derived transition table): immediate forward moves pass;
// skipping, going backwards, self-moves and moving past Archive fail with
// reason_code=illegal_stage_transition.
func TestStageTransitions(t *testing.T) {
	stages := AllWorkflowStages()
	for i := 0; i+1 < len(stages); i++ {
		if err := ValidateTransition(stages[i], stages[i+1]); err != nil {
			t.Errorf("ValidateTransition(%s->%s) must pass, got %v", stages[i], stages[i+1], err)
		}
		if !CanTransition(stages[i], stages[i+1]) {
			t.Errorf("CanTransition(%s->%s) must be true", stages[i], stages[i+1])
		}
	}
	illegal := [][2]WorkflowStage{
		{StageThink, StagePrd},         // skip one
		{StagePrd, StageThink},         // backwards
		{StageQa, StageQa},             // self
		{StageArchive, StageThink},     // past the end / wrap
		{StageImplement, StageRelease}, // skip Qa
	}
	for _, tc := range illegal {
		err := ValidateTransition(tc[0], tc[1])
		if err == nil {
			t.Errorf("ValidateTransition(%s->%s) must fail", tc[0], tc[1])
			continue
		}
		var ce *Error
		if !errors.As(err, &ce) {
			t.Fatalf("error must be *core.Error, got %T", err)
		}
		if ce.ReasonCode != ReasonIllegalStageTransition {
			t.Errorf("%s->%s reason = %q want %q", tc[0], tc[1], ce.ReasonCode, ReasonIllegalStageTransition)
		}
	}
	if _, ok := NextStage(StageArchive); ok {
		t.Error("Archive must have no successor")
	}
}

// AC-028: Blocked requires non-empty reason codes; blocking_gate optional.
func TestBlockedRequiresReasonCodes(t *testing.T) {
	if _, err := NewBlockedState(StageImplement, nil, "qa"); err == nil {
		t.Error("Blocked with no reason codes must fail")
	} else if rc, _ := ReasonCodeOf(err); rc != ReasonBlockedMissingReasons {
		t.Errorf("reason = %q want %q", rc, ReasonBlockedMissingReasons)
	}
	if _, err := NewBlockedState(StageImplement, []string{""}, ""); err == nil {
		t.Error("Blocked with only empty reason codes must fail")
	}
	st, err := NewBlockedState(StageImplement, []string{"truth_not_frozen"}, "truth_freeze")
	if err != nil {
		t.Fatalf("valid Blocked rejected: %v", err)
	}
	if st.BlockingGate != "truth_freeze" || len(st.ReasonCodes) != 1 {
		t.Errorf("unexpected state %+v", st)
	}
	// A plain InProgress state needs no reason codes.
	ok := StageState{Stage: StagePrd, Status: StatusInProgress}
	if err := ok.Validate(); err != nil {
		t.Errorf("InProgress must validate without reason codes: %v", err)
	}
}

// AC-029: Skipped requires a deferral id or a ledger reference.
func TestSkippedRequiresDeferralRef(t *testing.T) {
	if _, err := NewSkippedState(StagePrototype, "", ""); err == nil {
		t.Error("Skipped without any deferral ref must fail")
	} else if rc, _ := ReasonCodeOf(err); rc != ReasonSkippedMissingDeferral {
		t.Errorf("reason = %q want %q", rc, ReasonSkippedMissingDeferral)
	}
	if _, err := NewSkippedState(StagePrototype, "D-V2-001", ""); err != nil {
		t.Errorf("Skipped with deferral id must pass: %v", err)
	}
	if _, err := NewSkippedState(StagePrototype, "", "ledger://deferrals/12"); err != nil {
		t.Errorf("Skipped with ledger ref must pass: %v", err)
	}
}

// AC-030: exactly Crashed/TimedOut/Interrupted are recovery-class.
func TestNeedsRecoveryEvent(t *testing.T) {
	want := map[StageStatus]bool{
		StatusCrashed: true, StatusTimedOut: true, StatusInterrupted: true,
	}
	for _, s := range AllStageStatuses() {
		if got := NeedsRecoveryEvent(s); got != want[s] {
			t.Errorf("NeedsRecoveryEvent(%s) = %v want %v", s, got, want[s])
		}
	}
}

// AC-031 (core half): exactly 12 run statuses, all Valid, unknown rejected.
// The store half (CHECK set equality) is asserted in internal/store tests.
func TestRunStatusSet(t *testing.T) {
	if n := len(AllRunStatuses()); n != 12 {
		t.Fatalf("RunStatus count = %d want 12", n)
	}
	seen := map[RunStatus]bool{}
	for _, s := range AllRunStatuses() {
		if !s.Valid() {
			t.Errorf("%s must be Valid", s)
		}
		if seen[s] {
			t.Errorf("duplicate run status %s", s)
		}
		seen[s] = true
		got, err := ParseRunStatus(s.String())
		if err != nil || got != s {
			t.Errorf("ParseRunStatus(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := ParseRunStatus("Sleeping"); err == nil {
		t.Error("unknown run status must fail")
	} else if rc, _ := ReasonCodeOf(err); rc != ReasonInvalidRunStatus {
		t.Errorf("reason = %q want %q", rc, ReasonInvalidRunStatus)
	}
}

// AC-037/038: a chain built by NewEvidenceChain verifies; empty, truncated,
// reordered and tampered chains all fail closed with EVIDENCE_CHAIN_BROKEN.
func TestEvidenceChainVerify(t *testing.T) {
	refs := []string{"art-1", "art-2", "art-3"}
	chain := NewEvidenceChain(refs)
	if err := chain.Verify(); err != nil {
		t.Fatalf("fresh chain must verify: %v", err)
	}
	// Order matters: same refs in another order must produce another chain.
	if NewEvidenceChain([]string{"art-2", "art-1", "art-3"}).ManifestHash == chain.ManifestHash {
		t.Error("manifest hash must be order-sensitive")
	}

	broken := map[string]EvidenceChain{
		"empty":        {},
		"no refs":      {ManifestHash: "x"},
		"truncated":    {ManifestHash: chain.ManifestHash, SignatureChain: chain.SignatureChain[:2], ArtifactRefs: refs},
		"tampered sig": {ManifestHash: chain.ManifestHash, SignatureChain: []string{"deadbeef", chain.SignatureChain[1], chain.SignatureChain[2]}, ArtifactRefs: refs},
		"tampered ref": {ManifestHash: chain.ManifestHash, SignatureChain: chain.SignatureChain, ArtifactRefs: []string{"art-1", "art-X", "art-3"}},
		"bad manifest": {ManifestHash: "deadbeef", SignatureChain: chain.SignatureChain, ArtifactRefs: refs},
	}
	for name, c := range broken {
		err := c.Verify()
		if err == nil {
			t.Errorf("%s: Verify must fail", name)
			continue
		}
		if rc, _ := ReasonCodeOf(err); rc != ReasonEvidenceChainBroken {
			t.Errorf("%s: reason = %q want %q", name, rc, ReasonEvidenceChainBroken)
		}
	}
}
