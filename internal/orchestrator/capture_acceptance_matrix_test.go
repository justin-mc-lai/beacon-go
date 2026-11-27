package orchestrator

import (
	"testing"

	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// An explicit older run must not borrow the latest run's human approval.
func TestCaptureReleaseConfirmationRunIsolation(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	older := env.createRun(t)
	newer := env.createRun(t)
	row, _, _, err := env.eng.loadRun(newer.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.appendEvent(row, "release_confirmed", map[string]any{"by": "owner"}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"truth_freeze", "implement", "qa"} {
		if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: kind, Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	oldVerdict, err := env.eng.EvaluateGate(env.ctx, "release", older.RunID, "checker")
	if err != nil {
		t.Fatal(err)
	}
	if oldVerdict.Verdict != gate.VerdictBlocked {
		t.Fatalf("older run borrowed confirmation: %+v", oldVerdict)
	}
	newVerdict, err := env.eng.EvaluateGate(env.ctx, "release", newer.RunID, "checker")
	if err != nil {
		t.Fatal(err)
	}
	if newVerdict.Verdict != gate.VerdictPass {
		t.Fatalf("selected confirmation refused: %+v", newVerdict)
	}
}
