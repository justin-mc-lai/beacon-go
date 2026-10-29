package orchestrator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
)

// rewriteRunMeta applies fn to the run's stored metadata map and saves
// it back (test-only time travel for heartbeats / leases).
func rewriteRunMeta(t *testing.T, env testEnv, runID string, fn func(map[string]any)) {
	t.Helper()
	row, err := env.st.Runs().GetByID(runID)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(row.Metadata, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.st.Runs().UpdateRunState(runID, row.Status, raw, nil); err != nil {
		t.Fatal(err)
	}
}

func traceEventTypes(t *testing.T, env testEnv, runID string) []string {
	t.Helper()
	bundle, err := env.eng.Trace(runID)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, ev := range bundle.Events {
		out = append(out, ev.EventType)
	}
	return out
}

func hasEvent(types []string, want string) bool {
	for _, ty := range types {
		if ty == want {
			return true
		}
	}
	return false
}

// Dual start: a second driver start while the first lease is live is
// refused (agent_driver_dual_start_forbidden); after stop, a fresh
// start succeeds with a new driver id.
func TestDriverDuplicateStartRejected(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	dv, err := env.eng.DriverStart(run.RunID, DriverStartOptions{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if dv.State != DriverArming || dv.Driver == nil || dv.Driver.DriverID == "" {
		t.Fatalf("start view = %+v", dv)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err == nil {
		t.Fatal("second start must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDriverDualStart {
		t.Errorf("reason = %q", rc)
	}
	if _, err := env.eng.DriverStop(run.RunID); err != nil {
		t.Fatal(err)
	}
	dv2, err := env.eng.DriverStart(run.RunID, DriverStartOptions{})
	if err != nil {
		t.Fatalf("restart after stop: %v", err)
	}
	if dv2.Driver.DriverID == dv.Driver.DriverID {
		t.Error("restart must mint a new driver id")
	}
}

// An expired lease is taken over by a new start (the store CAS makes
// the swap atomic); the takeover is on the event trail.
func TestDriverLeaseTakeoverAfterExpiry(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	dv, err := env.eng.DriverStart(run.RunID, DriverStartOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rewriteRunMeta(t, env, run.RunID, func(m map[string]any) {
		m["driver"].(map[string]any)["lease_expires_at"] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)
	})
	dv2, err := env.eng.DriverStart(run.RunID, DriverStartOptions{})
	if err != nil {
		t.Fatalf("takeover start: %v", err)
	}
	if dv2.Driver.DriverID == dv.Driver.DriverID {
		t.Error("takeover must mint a new driver id")
	}
	if !hasEvent(traceEventTypes(t, env, run.RunID), "driver_lease_takeover") {
		t.Error("takeover event missing from trace")
	}
}

// The loop drives plan -> truth -> implement -> qa and then STOPS at
// the release gate in awaiting_human: the run is at release_gate and
// NOT completed, and no goal_completed event exists.
func TestDriverRunLoopStopsAtReleaseGate(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.seedQAPass(t)
	run := env.createRun(t)
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{MaxTurns: 10}); err != nil {
		t.Fatal(err)
	}
	sleeps := 0
	loop, err := env.eng.DriverRunLoop(context.Background(), run.RunID, 10, func() { sleeps++ })
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if loop.FinalState != DriverAwaitingHuman {
		t.Fatalf("final driver state = %q (loop %+v)", loop.FinalState, loop)
	}
	if loop.Rounds != 4 {
		t.Errorf("rounds = %d want 4 (plan, truth, implement, qa)", loop.Rounds)
	}
	if loop.AutoRelease {
		t.Error("driver must never auto-release")
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.LoopStage != string(core.LoopReleaseGate) || st.Status == "completed" {
		t.Errorf("run after loop = %+v", st)
	}
	if hasEvent(traceEventTypes(t, env, run.RunID), "goal_completed") {
		t.Error("driver completed the run — forbidden")
	}
	if sleeps == 0 {
		t.Error("injected sleep was never called between rounds")
	}
}

// Budget: with max 2 turns the driver halts with halt_budget after
// exactly 2 spent rounds, and budget_exhausted is on the trail.
func TestDriverBudgetExhaustedStopsLoop(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{MaxTurns: 2}); err != nil {
		t.Fatal(err)
	}
	loop, err := env.eng.DriverRunLoop(context.Background(), run.RunID, 10, nil)
	if err != nil {
		t.Fatalf("run loop: %v", err)
	}
	if loop.FinalState != DriverHaltBudget {
		t.Fatalf("final state = %q want halt_budget (loop %+v)", loop.FinalState, loop)
	}
	if loop.Rounds != 2 {
		t.Errorf("rounds = %d want 2", loop.Rounds)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Budget == nil || !st.Budget.Exhausted || st.Budget.UsedRounds != 2 {
		t.Errorf("budget = %+v", st.Budget)
	}
	if !hasEvent(traceEventTypes(t, env, run.RunID), "budget_exhausted") {
		t.Error("budget_exhausted event missing")
	}
}

// One driver turn: advances one stage, spends one round, writes a
// checkpoint (substep driver-turn-1) and prepares the next prompt
// digest naming the next skill.
func TestDriverRunOnceTurnBookkeeping(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// Run-once before start is refused.
	if _, err := env.eng.DriverRunOnce(context.Background(), run.RunID); err == nil {
		t.Fatal("run-once without driver must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDriverNotStarted {
		t.Errorf("reason = %q", rc)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err != nil {
		t.Fatal(err)
	}
	dv, err := env.eng.DriverRunOnce(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("run-once: %v", err)
	}
	if dv.Driver.TurnN != 1 || dv.State != DriverRequeue {
		t.Errorf("after turn 1: %+v", dv.Driver)
	}
	if dv.Driver.NextPrompt == "" || !strings.Contains(dv.Driver.NextPrompt, "beacon-truth") {
		t.Errorf("next prompt digest = %q", dv.Driver.NextPrompt)
	}
	ckpt, err := env.st.Runs().LatestCheckpoint(run.RunID)
	if err != nil {
		t.Fatalf("checkpoint after driver turn: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(ckpt.State, &state); err != nil {
		t.Fatal(err)
	}
	if state["last_substep"] != "driver-turn-1" {
		t.Errorf("checkpoint state = %v", state)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.LastHeartbeatAt == "" {
		t.Errorf("heartbeat after turn: %+v %v", st, err)
	}
}

// Stop: halt_terminal with agent_driver_user_stop; a later run-once
// returns the halted view without ticking (executor untouched).
func TestDriverStopHalts(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err != nil {
		t.Fatal(err)
	}
	dv, err := env.eng.DriverStop(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if dv.State != DriverHaltTerminal || dv.Driver.HaltReason != core.ReasonDriverUserStop {
		t.Errorf("stop view = %+v", dv)
	}
	if dv.LeaseActive {
		t.Error("lease must be released on stop")
	}
	calls := env.rec.CallCount()
	dv, err = env.eng.DriverRunOnce(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if dv.State != DriverHaltTerminal || env.rec.CallCount() != calls {
		t.Errorf("run-once after stop ticked: %+v calls %d -> %d", dv, calls, env.rec.CallCount())
	}
}

// AC-105: an interrupt (cancelled context) stops the loop between turns
// with host_interrupted and leaves the run resumable — it is not failed,
// not completed, and no release is confirmed.
func TestDriverRunLoopStopsOnInterrupt(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{MaxTurns: 10}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel before the first turn: the next loop iteration must stop.
	cancel()
	loop, err := env.eng.DriverRunLoop(ctx, run.RunID, 10, nil)
	if err != nil {
		t.Fatalf("interrupted loop must return a view, not an error: %v", err)
	}
	if loop.ReasonCode != core.ReasonHostInterrupted {
		t.Errorf("reason = %q want %q", loop.ReasonCode, core.ReasonHostInterrupted)
	}
	if loop.Rounds != 0 {
		t.Errorf("rounds = %d want 0 after a pre-turn interrupt", loop.Rounds)
	}
	if loop.AutoRelease {
		t.Error("interrupt must never release")
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status == "completed" {
		t.Errorf("interrupted run must stay resumable, got status %q", st.Status)
	}
}
