package core

import "testing"

// loop_goal_state.py: transition to an unknown stage is blocked with
// reason_code=invalid_stage.
func TestLoopTransitionRejectsInvalidStage(t *testing.T) {
	st := NewLoopState()
	err := st.Transition(LoopStage("nope"), false, nil)
	if rc, _ := ReasonCodeOf(err); rc != ReasonInvalidLoopStage {
		t.Fatalf("reason = %q, err = %v", rc, err)
	}
	if st.CurrentStage != LoopIdle {
		t.Errorf("state moved on failed transition: %s", st.CurrentStage)
	}
	if _, err := ParseLoopStage("bogus"); err == nil {
		t.Error("ParseLoopStage must reject unknown stages")
	}
}

// check_release_gate semantics: done only from release_gate and only with
// human confirmation — an unconfirmed release can never reach done.
func TestLoopDoneRequiresReleaseGateAndHuman(t *testing.T) {
	st := NewLoopState()
	// Not at release_gate yet.
	if err := st.Transition(LoopDone, true, nil); err == nil {
		t.Fatal("done from idle must be rejected")
	} else if rc, _ := ReasonCodeOf(err); rc != ReasonNotAtReleaseGate {
		t.Errorf("reason = %q want %q", rc, ReasonNotAtReleaseGate)
	}
	// At release_gate but unconfirmed.
	st.CurrentStage = LoopReleaseGate
	if err := st.Transition(LoopDone, false, nil); err == nil {
		t.Fatal("done without human confirmation must be rejected")
	} else if rc, _ := ReasonCodeOf(err); rc != ReasonReleaseRequiresHuman {
		t.Errorf("reason = %q want %q", rc, ReasonReleaseRequiresHuman)
	}
	if err := st.CheckReleaseGate(false); err == nil {
		t.Fatal("CheckReleaseGate(false) must fail")
	}
	// Confirmed: allowed.
	if err := st.Transition(LoopDone, true, nil); err != nil {
		t.Fatalf("confirmed release -> done: %v", err)
	}
	if st.CurrentStage != LoopDone || !st.ReleaseConfirmed {
		t.Errorf("state = %+v", st)
	}
}

// A pending stop_when blocks any further transition
// (stopped/stop_when_satisfied in the source).
func TestLoopTransitionBlockedByStopWhen(t *testing.T) {
	st := NewLoopState()
	st.StopWhen = "qa_failed_twice"
	err := st.Transition(LoopGovernancePrecheck, false, nil)
	if rc, _ := ReasonCodeOf(err); rc != ReasonStopWhenSatisfied {
		t.Errorf("reason = %q, err = %v", rc, err)
	}
}

// increment_repair_loop: loops 1..3 are tolerated; the 4th exceeds
// MAX_REPAIR_LOOPS=3 and routes to error_recovery.
func TestLoopRepairLoopCeiling(t *testing.T) {
	st := NewLoopState()
	st.CurrentStage = LoopImplementRouting
	for i := 1; i <= MaxRepairLoops; i++ {
		if err := st.IncrementRepairLoop(); err != nil {
			t.Fatalf("repair loop %d must be tolerated: %v", i, err)
		}
		if st.CurrentStage != LoopImplementRouting {
			t.Fatalf("stage changed early at loop %d", i)
		}
	}
	err := st.IncrementRepairLoop()
	if rc, _ := ReasonCodeOf(err); rc != ReasonMaxRepairLoopsExceeded {
		t.Fatalf("4th repair loop reason = %q, err = %v", rc, err)
	}
	if st.CurrentStage != LoopErrorRecovery {
		t.Errorf("stage = %s want error_recovery", st.CurrentStage)
	}
}

// Frozen AC-035 short continue words.
func TestShortContinueUtterances(t *testing.T) {
	for _, u := range []string{"继续", "continue", "resume", "go"} {
		if !IsShortContinueUtterance(u) {
			t.Errorf("%q must be a short continue utterance", u)
		}
	}
	if IsShortContinueUtterance("please continue the whole thing now") {
		t.Error("long utterance must not map to the short-continue fast path")
	}
}

// The 12 states and the stage->skill map mirror the source tables.
func TestLoopStagesAndSkills(t *testing.T) {
	if got := len(AllLoopStages()); got != 12 {
		t.Fatalf("loop stages = %d want 12", got)
	}
	if got := LoopSkillForStage(LoopTruthRouting); got != "beacon-gen-truth" {
		t.Errorf("truth skill = %q", got)
	}
	if got := LoopSkillForStage(LoopDone); got != "" {
		t.Errorf("done skill = %q want empty", got)
	}
}
