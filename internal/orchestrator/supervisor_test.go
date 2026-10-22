package orchestrator

import (
	"context"
	"testing"
	"time"

	"beacon-go/internal/core"
)

// Threshold resolution: explicit wins; then the recorded heartbeat
// interval (source parity); the 30-minute fallback only when no
// interval was ever recorded.
func TestStallThresholdResolution(t *testing.T) {
	m := &runMeta{}
	if got := stallThresholdSec(m, 0); got != DefaultStallThresholdSec {
		t.Errorf("fallback threshold = %d want %d (30 min)", got, DefaultStallThresholdSec)
	}
	if got := stallThresholdSec(m, 45); got != 45 {
		t.Errorf("explicit threshold = %d", got)
	}
	m.HeartbeatIntervalSec = 120
	if got := stallThresholdSec(m, 0); got != 120 {
		t.Errorf("recorded interval threshold = %d want 120", got)
	}
}

// Stall: heartbeat 2h old + threshold 30min -> stall detected, event
// on the trail, and Resume answers replay_substep (the P2a interrupt
// representation — no second dialect).
func TestSuperviseStallDetectedAndResumeReplays(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.AttachSupervisor(run.RunID, 0); err != nil {
		t.Fatal(err)
	}
	rewriteRunMeta(t, env, run.RunID, func(m map[string]any) {
		m["last_heartbeat_at"] = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	})
	sv, err := env.eng.SuperviseOnce(run.RunID, 1800)
	if err != nil {
		t.Fatal(err)
	}
	if !sv.StallDetected || sv.ReasonCode != core.ReasonSupervisorStall {
		t.Fatalf("supervise = %+v", sv)
	}
	if sv.HeartbeatAgeSec < 7000 {
		t.Errorf("heartbeat age = %d", sv.HeartbeatAgeSec)
	}
	if !hasEvent(traceEventTypes(t, env, run.RunID), "supervisor_stall_detected") {
		t.Error("supervisor_stall_detected event missing")
	}
	rv, err := env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rv.ResumeAction != core.ResumeReplaySubstep {
		t.Errorf("resume after stall = %+v want replay_substep", rv)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.Supervisor == nil || st.Supervisor.StallCount != 1 {
		t.Errorf("supervisor state = %+v %v", st.Supervisor, err)
	}
}

// No false stall: a fresh heartbeat under the threshold is fine, and
// a detached run reports supervisor_not_attached instead of judging.
func TestSuperviseNoFalseStall(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// Detached: no judgement.
	sv, err := env.eng.SuperviseOnce(run.RunID, 1800)
	if err != nil {
		t.Fatal(err)
	}
	if sv.Attached || sv.ReasonCode != core.ReasonSupervisorNotAttached || sv.StallDetected {
		t.Errorf("detached supervise = %+v", sv)
	}
	if _, err := env.eng.AttachSupervisor(run.RunID, 0); err != nil {
		t.Fatal(err)
	}
	sv, err = env.eng.SuperviseOnce(run.RunID, 1800)
	if err != nil {
		t.Fatal(err)
	}
	if !sv.Attached || sv.StallDetected {
		t.Errorf("fresh supervise = %+v", sv)
	}
	if hasEvent(traceEventTypes(t, env, run.RunID), "supervisor_stall_detected") {
		t.Error("false stall event recorded")
	}
	// Detach stops supervision again.
	dv, err := env.eng.DetachSupervisor(run.RunID)
	if err != nil || dv.Attached {
		t.Errorf("detach = %+v %v", dv, err)
	}
	sv, err = env.eng.SuperviseOnce(run.RunID, 1800)
	if err != nil || sv.Attached {
		t.Errorf("supervise after detach = %+v %v", sv, err)
	}
}

// Longrun doctor: rounds from the driver, stall count from the event
// trail, heartbeat age, streak + repair counters, health issues.
func TestLongrunDoctorReport(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{AttachSupervisor: false}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DriverRunOnce(context.Background(), run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachSupervisor(run.RunID, 0); err != nil {
		t.Fatal(err)
	}
	rewriteRunMeta(t, env, run.RunID, func(m map[string]any) {
		m["last_heartbeat_at"] = time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	})
	if _, err := env.eng.SuperviseOnce(run.RunID, 1800); err != nil {
		t.Fatal(err)
	}
	rep, err := env.eng.LongrunDoctor(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.DriverRounds != 1 {
		t.Errorf("doctor rounds = %d want 1", rep.DriverRounds)
	}
	if rep.StallCount != 1 {
		t.Errorf("doctor stall count = %d want 1", rep.StallCount)
	}
	if rep.InterruptCount < 1 {
		t.Errorf("doctor interrupt count = %d", rep.InterruptCount)
	}
	if rep.HeartbeatAgeSec < 7000 {
		t.Errorf("doctor heartbeat age = %d", rep.HeartbeatAgeSec)
	}
	if !rep.SupervisorAttached || rep.DriverState == DriverIdle {
		t.Errorf("doctor = %+v", rep)
	}
	if rep.Healthy || len(rep.Issues) == 0 {
		t.Errorf("doctor must flag issues after a stall: %+v", rep)
	}
	// Unknown run -> typed not-found.
	if _, err := env.eng.LongrunDoctor("run_nope"); err == nil {
		t.Error("doctor of unknown run must fail")
	}
}
