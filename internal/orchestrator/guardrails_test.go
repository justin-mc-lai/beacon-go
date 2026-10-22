package orchestrator

import (
	"context"
	"testing"

	"beacon-go/internal/core"
)

// Steering: an objective change is recorded, supersedes the run's
// objective, and is surfaced by tick AND resume until acknowledged —
// never silently swallowed.
func TestSteeringSurfacedUntilAcked(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	v, err := env.eng.Steer(run.RunID, SteeringObjective, "pivot: mobile first")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	if v.PendingSteering == nil || v.PendingSteering.Text != "pivot: mobile first" {
		t.Fatalf("steer view = %+v", v.PendingSteering)
	}
	// Objective superseded on the snapshot (in-package check).
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil || m.Utterance != "pivot: mobile first" {
		t.Errorf("utterance after steer = %q", m.Utterance)
	}
	// Tick surfaces it.
	v, err = env.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.PendingSteering == nil {
		t.Error("tick swallowed the pending steering")
	}
	// Resume surfaces it.
	rv, err := env.eng.Resume(run.RunID, "继续")
	if err != nil {
		t.Fatal(err)
	}
	if rv.PendingSteering == nil || rv.PendingSteering.Text != "pivot: mobile first" {
		t.Errorf("resume steering = %+v", rv.PendingSteering)
	}
	// After ack, surfaces are clean.
	v, err = env.eng.AckSteering(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.PendingSteering != nil {
		t.Errorf("steering still pending after ack: %+v", v.PendingSteering)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.PendingSteering != nil {
		t.Errorf("status after ack = %+v", st.PendingSteering)
	}
	// Bad inputs are refused.
	if _, err := env.eng.Steer(run.RunID, "bogus", "x"); err == nil {
		t.Error("bogus steering kind must fail")
	}
	if _, err := env.eng.Steer(run.RunID, SteeringNote, ""); err == nil {
		t.Error("empty steering text must fail")
	}
}

// Todos: add/claim/done/remove with the source's claim rules, and the
// status summary always agrees with the list.
func TestTodosAndStatusSummary(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	item1, _, err := env.eng.TodoAdd(run.RunID, "write the tests")
	if err != nil {
		t.Fatal(err)
	}
	item2, _, err := env.eng.TodoAdd(run.RunID, "ship it")
	if err != nil {
		t.Fatal(err)
	}
	if item1.ID != "todo-1" || item2.ID != "todo-2" {
		t.Errorf("todo ids = %q %q", item1.ID, item2.ID)
	}
	// Claim rules: one owner only.
	if _, err := env.eng.TodoClaim(run.RunID, "todo-1", "agent-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.TodoClaim(run.RunID, "todo-1", "agent-2"); err == nil {
		t.Error("double claim must be rejected")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTodoDoubleClaim {
		t.Errorf("double claim reason = %q", rc)
	}
	if _, err := env.eng.TodoDone(run.RunID, "todo-1"); err != nil {
		t.Fatal(err)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Todos == nil || st.Todos.Total != 2 || st.Todos.Done != 1 || st.Todos.Open != 1 || st.Todos.Claimed != 0 {
		t.Errorf("status todos = %+v", st.Todos)
	}
	list, err := env.eng.TodoList(run.RunID)
	if err != nil || len(list) != 2 || list[0].Status != TodoDone || list[1].Status != TodoOpen {
		t.Errorf("todo list = %+v %v", list, err)
	}
	// Re-done and missing todos are refused with the source codes.
	if _, err := env.eng.TodoDone(run.RunID, "todo-1"); err == nil {
		t.Error("re-done must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTodoDone {
		t.Errorf("re-done reason = %q", rc)
	}
	if _, err := env.eng.TodoDone(run.RunID, "todo-99"); err == nil {
		t.Error("missing todo must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTodoMissing {
		t.Errorf("missing reason = %q", rc)
	}
	// Remove: list and summary shrink together; ids never recycle.
	if _, err := env.eng.TodoRemove(run.RunID, "todo-2"); err != nil {
		t.Fatal(err)
	}
	item3, _, err := env.eng.TodoAdd(run.RunID, "third")
	if err != nil || item3.ID != "todo-3" {
		t.Errorf("post-remove add = %+v %v", item3, err)
	}
	st, err = env.eng.Status(run.RunID)
	if err != nil || st.Todos.Total != 2 || st.Todos.Done != 1 || st.Todos.Open != 1 {
		t.Errorf("status after remove/add = %+v", st.Todos)
	}
}

// Budget configuration: limits land on the run view; configuring a
// limit at/below the spent amount flags exhaustion immediately.
func TestConfigureBudget(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	v, err := env.eng.ConfigureBudget(run.RunID, 5, 600)
	if err != nil {
		t.Fatal(err)
	}
	if v.Budget == nil || v.Budget.MaxRounds != 5 || v.Budget.MaxDurationSec != 600 || v.Budget.Exhausted {
		t.Errorf("budget view = %+v", v.Budget)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{MaxTurns: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DriverRunOnce(context.Background(), run.RunID); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.ConfigureBudget(run.RunID, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Budget.Exhausted || v.Budget.ExhaustedReason != core.ReasonDriverBudgetExhaust {
		t.Errorf("budget after shrink = %+v", v.Budget)
	}
}

// The unified vocabulary: source strings are preserved verbatim and
// the catalog recognises the stage-exit family.
func TestReasonVocabularyVerbatim(t *testing.T) {
	pairs := map[string]string{
		core.ReasonDriverDualStart:        "agent_driver_dual_start_forbidden",
		core.ReasonDriverBudgetExhaust:    "agent_driver_budget_exhausted",
		core.ReasonDriverUserStop:         "agent_driver_user_stop",
		core.ReasonHostBridgeMissing:      "host_bridge_missing",
		core.ReasonSupervisorNotAttached:  "supervisor_not_attached",
		core.ReasonSupervisorStall:        "supervisor_stall_detected",
		core.ReasonLoopCannotComplete:     "loop_cannot_mark_product_complete",
		core.ReasonGoalQueueEmpty:         "goal_queue_empty",
		core.ReasonProgramBudgetExhausted: "program_drive_budget_exhausted",
		core.ReasonSpendWithoutWriteback:  "spend_without_writeback",
		core.ReasonTodoDoubleClaim:        "double_claim",
		core.ReasonStageExitTruthMissing:  "stage_exit_truth_package_missing",
		core.ReasonFreezeAckRequired:      "freeze_ack_required",
	}
	for got, want := range pairs {
		if got != want {
			t.Errorf("constant = %q want %q", got, want)
		}
	}
	if !core.IsKnownReasonCode("stage_exit_ok") || !core.IsKnownReasonCode(core.ReasonStageExitQAEvidenceMiss) {
		t.Error("catalog must know the stage-exit family")
	}
	if core.IsKnownReasonCode("totally_made_up") {
		t.Error("catalog must not know invented codes")
	}
}
