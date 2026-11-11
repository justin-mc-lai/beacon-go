// kernel-recovery-qa-bridge AC-004/AC-005: the negative guards and the
// contract this fix must leave untouched.
package orchestrator

import (
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// AC-004: recover still refuses a human gate and mutates nothing; the
// three recording-shaped stage_executed variants still never count as
// qa evidence.
func TestKernelRecoveryNegativeGuards(t *testing.T) {
	t.Run("human gate recover refused without state change", func(t *testing.T) {
		env := newGateEnv(t, frozenTruthDoc)
		run := env.createRun(t)
		if _, err := env.eng.Block(run.RunID, core.ReasonReleaseRequiresHuman); err != nil {
			t.Fatal(err)
		}
		beforeRun, _, _, err := env.eng.loadRun(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		beforeLC, err := env.st.Features().GetLifecycle(env.feature.ID)
		if err != nil {
			t.Fatal(err)
		}
		v, err := env.eng.Recover(run.RunID, "intruder", "bypass attempt")
		if err == nil {
			t.Fatal("recover on a human gate must be refused")
		}
		if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonRecoveryHumanGateForbidden {
			t.Fatalf("recover reason = %v, want %s", err, core.ReasonRecoveryHumanGateForbidden)
		}
		if !v.Blocked || v.LastBlockReason != core.ReasonReleaseRequiresHuman {
			t.Fatalf("refused view = %+v, want the human gate still reported", v)
		}
		afterRun, _, _, err := env.eng.loadRun(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if string(beforeRun.Metadata) != string(afterRun.Metadata) {
			t.Fatal("refused recover changed the run snapshot")
		}
		afterLC, err := env.st.Features().GetLifecycle(env.feature.ID)
		if err != nil {
			t.Fatal(err)
		}
		if beforeLC.Stage != afterLC.Stage || beforeLC.Status != afterLC.Status ||
			len(beforeLC.ReasonCodes) != len(afterLC.ReasonCodes) {
			t.Fatalf("refused recover changed the lifecycle: before=%+v after=%+v", beforeLC, afterLC)
		}
	})

	for _, tc := range []struct {
		name    string
		payload string
	}{
		{"executor marker true", `{"stage":"qa","outcome":"passed","placeholder":true,"recording":false}`},
		{"recording true", `{"stage":"qa","outcome":"passed","placeholder":false,"recording":true}`},
		{"marker missing", `{"stage":"qa","outcome":"passed"}`},
	} {
		t.Run("stage_executed rejected: "+tc.name, func(t *testing.T) {
			env := newGateEnv(t, frozenTruthDoc)
			run := env.createRun(t)
			if _, err := env.st.AppendEvent(store.LifecycleEvent{
				FeatureID: env.feature.ID, RunID: nullRunID(run.RunID),
				EventType: "stage_executed", Payload: []byte(tc.payload),
			}); err != nil {
				t.Fatal(err)
			}
			ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
			if err != nil {
				t.Fatal(err)
			}
			if c := qaCheck(ev, "qa_stage_executed"); c.OK {
				t.Fatalf("%s must not count as qa stage evidence: %+v", tc.name, c)
			}
		})
	}

	t.Run("real stage_executed still counts", func(t *testing.T) {
		env := newGateEnv(t, frozenTruthDoc)
		run := env.createRun(t)
		if _, err := env.st.AppendEvent(store.LifecycleEvent{
			FeatureID: env.feature.ID, RunID: nullRunID(run.RunID),
			EventType: "stage_executed",
			Payload:   []byte(`{"stage":"qa","outcome":"passed","placeholder":false,"recording":false}`),
		}); err != nil {
			t.Fatal(err)
		}
		ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
		if err != nil {
			t.Fatal(err)
		}
		if c := qaCheck(ev, "qa_stage_executed"); !c.OK {
			t.Fatalf("a real stage_executed must count: %+v", c)
		}
	})
}

// AC-005: no reason code was added or renamed, and the qa exit stays
// FORMAL-FIRST inside the run: a run formal verdict decides both ways,
// and the derived artifact judgement only runs when no verdict exists.
func TestKernelRecoveryContractStable(t *testing.T) {
	for want, got := range map[string]string{
		"stage_exit_ok":                       core.ReasonStageExitOK,
		"stage_exit_implement_blocked":        core.ReasonStageExitImplementBlock,
		"stage_exit_qa_evidence_missing":      core.ReasonStageExitQAEvidenceMiss,
		"qa_stage_not_executed":               core.ReasonQAStageNotExecuted,
		"goal_recovery_human_gate_forbidden":  core.ReasonRecoveryHumanGateForbidden,
		"release_requires_human_confirmation": core.ReasonReleaseRequiresHuman,
	} {
		if got != want {
			t.Fatalf("reason code renamed: got %q, want %q", got, want)
		}
	}
	if ReasonStageExitImplementBlk != core.ReasonStageExitImplementBlock ||
		ReasonStageExitQAEvidence != core.ReasonStageExitQAEvidenceMiss {
		t.Fatal("the stage-exit aliases drifted from the core vocabulary")
	}

	// A run formal qa pass opens the exit even with zero derived evidence.
	pass := EvaluateStageExit(ExitInput{
		Stage: StageQA, HasFeature: true,
		LatestGates: map[string]GateFact{"qa": {Verdict: "pass", Source: "run_formal_gate"}},
	})
	if !pass.Allowed || pass.Derived || pass.FormalVerdict != "pass" || pass.FormalSource != "run_formal_gate" {
		t.Fatalf("a run formal pass must decide the qa exit: %+v", pass)
	}

	// A run formal blocked verdict closes it even when artifacts exist.
	blocked := EvaluateStageExit(ExitInput{
		Stage: StageQA, HasFeature: true, ArtifactCount: 3,
		LatestGates: map[string]GateFact{"qa": {
			Verdict: "blocked", ReasonCodes: []string{core.ReasonQAStageNotExecuted}, Source: "run_formal_gate",
		}},
	})
	if blocked.Allowed || blocked.Derived || blocked.ReasonCode != core.ReasonQAStageNotExecuted {
		t.Fatalf("a run formal blocked verdict must decide the qa exit: %+v", blocked)
	}

	// No verdict at all: the derived artifact judgement is the fallback.
	derived := EvaluateStageExit(ExitInput{Stage: StageQA, HasFeature: true, ArtifactCount: 1})
	if !derived.Allowed || !derived.Derived {
		t.Fatalf("the derived judgement must only run without a verdict: %+v", derived)
	}

	// The fix must not have opened a second route out of release.
	release := EvaluateStageExit(ExitInput{Stage: StageRelease, HasFeature: true})
	if release.Allowed || release.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("release must stay humans-only: %+v", release)
	}

}
