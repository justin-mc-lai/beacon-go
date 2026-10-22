// Durable supervisor (P2b, G-07): the opt-in watchdog of
// goal_durable_supervisor.py. Attach is explicit (never from run
// creation, source parity). SuperviseOnce performs ONE check: if the
// run's heartbeat is older than the stall threshold while the run is
// in a work stage, it records the stall (supervisor_stall_detected
// event) and marks the run host-interrupted through the SAME P2a
// interrupt representation (InterruptPending via RecordInterrupt), so
// the next Resume answers replay_substep (AC-034) — no second
// interrupt dialect is invented.
//
// One deliberate divergence from the source: supervise_once there
// writes a supervisor heartbeat right after detecting a stall, which
// immediately re-arms the detector and hides the stall until another
// full interval passes. Here the stall stays visible until real work
// heartbeats again; the poll itself only records last_poll_at.
package orchestrator

import (
	"encoding/json"
	"time"

	"beacon-go/internal/core"
)

// SuperviseView is the result of one supervisor operation.
type SuperviseView struct {
	RunID           string           `json:"run_id"`
	Attached        bool             `json:"attached"`
	ReasonCode      string           `json:"reason_code,omitempty"`
	StallDetected   bool             `json:"stall_detected"`
	ThresholdSec    int              `json:"threshold_sec,omitempty"`
	HeartbeatAgeSec int64            `json:"heartbeat_age_sec"` // -1 when no heartbeat exists
	Actions         []string         `json:"actions,omitempty"`
	WorkerPlan      string           `json:"worker_plan,omitempty"`
	Supervisor      *SupervisorState `json:"supervisor,omitempty"`
	Run             RunView          `json:"run"`
}

// AttachSupervisor opts the run into supervision (attach_supervisor:
// a fresh session each attach, interval normalized to >=30s, default
// 120s). Also records the run-level heartbeat interval used by stall
// detection, like the source payload.
func (e *Engine) AttachSupervisor(runID string, intervalSec int) (SuperviseView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return SuperviseView{}, err
	}
	interval := normalizeIntervalSec(intervalSec)
	m.Supervisor = &SupervisorState{
		Attached: true, SessionID: newWatchID("sup-"), State: "active",
		IntervalSec: interval, AttachedAt: nowISO(),
		StallCount: m.StallCount,
	}
	m.HeartbeatIntervalSec = interval
	if err := e.appendEvent(run, "supervise_attach", map[string]any{
		"session_id": m.Supervisor.SessionID, "interval_sec": interval,
	}); err != nil {
		return SuperviseView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return SuperviseView{}, err
	}
	return SuperviseView{
		RunID: runID, Attached: true, Supervisor: m.Supervisor,
		HeartbeatAgeSec: heartbeatAgeSec(m, run.StartedAt, time.Now()),
		Run:             viewOf(run, m, slug),
	}, nil
}

// DetachSupervisor stops supervision (detach_supervisor).
func (e *Engine) DetachSupervisor(runID string) (SuperviseView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return SuperviseView{}, err
	}
	if m.Supervisor == nil {
		m.Supervisor = &SupervisorState{IntervalSec: DefaultHeartbeatIntervalSec}
	}
	m.Supervisor.Attached = false
	m.Supervisor.State = "stopped"
	if err := e.appendEvent(run, "supervise_detach", map[string]any{
		"session_id": m.Supervisor.SessionID,
	}); err != nil {
		return SuperviseView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return SuperviseView{}, err
	}
	return SuperviseView{
		RunID: runID, Attached: false, Supervisor: m.Supervisor,
		HeartbeatAgeSec: heartbeatAgeSec(m, run.StartedAt, time.Now()),
		Run:             viewOf(run, m, slug),
	}, nil
}

// heartbeatAgeSec returns the age of the newest liveness anchor
// (last heartbeat, falling back to the run start) in seconds, or -1
// when no anchor parses.
func heartbeatAgeSec(m *runMeta, startedAt string, now time.Time) int64 {
	anchor := m.LastHeartbeatAt
	if anchor == "" {
		anchor = startedAt
	}
	t, err := time.Parse(time.RFC3339Nano, anchor)
	if err != nil {
		return -1
	}
	age := int64(now.Sub(t).Seconds())
	if age < 0 {
		return 0
	}
	return age
}

// stallThresholdSec resolves the stall threshold: explicit parameter
// first, then the run's recorded heartbeat interval (source parity:
// the attached supervisor interval, default 120s), and only when no
// interval was ever recorded, the 30-minute fallback.
func stallThresholdSec(m *runMeta, explicit int) int {
	if explicit > 0 {
		return explicit
	}
	if m.HeartbeatIntervalSec > 0 {
		return m.HeartbeatIntervalSec
	}
	if m.Supervisor != nil && m.Supervisor.IntervalSec > 0 {
		return m.Supervisor.IntervalSec
	}
	return DefaultStallThresholdSec
}

// stallApplies mirrors detect_stall's stage/status gate: terminal or
// blocked runs never stall, and neither does the release stage (it is
// waiting on a human, not on work).
func stallApplies(m *runMeta) bool {
	if m.Blocked || m.LoopStage == string(core.LoopDone) || m.LoopStage == string(core.LoopErrorRecovery) {
		return false
	}
	switch m.currentStage() {
	case StagePlan, StageTruth, StageDesign, StageImplement, StageQA:
		return true
	}
	return false
}

// workerPlanFor is the advisory dispatch hint of
// plan_tick_execute, condensed: it never executes anything itself.
func workerPlanFor(m *runMeta) string {
	switch m.currentStage() {
	case StageImplement:
		return "delegate_implement_lane (P2c)"
	case StageQA:
		return "run_qa_evaluation"
	case StageDesign:
		return "run_design_stage"
	case StageRelease:
		return "human_release_gate_only"
	default:
		return "record_only_load_next_skill"
	}
}

// SuperviseOnce runs one supervision poll. thresholdSec <= 0 resolves
// via stallThresholdSec. A detected stall appends
// supervisor_stall_detected and routes the run through RecordInterrupt
// so Resume replays the interrupted substep.
func (e *Engine) SuperviseOnce(runID string, thresholdSec int) (SuperviseView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return SuperviseView{}, err
	}
	sv := SuperviseView{
		RunID: runID, HeartbeatAgeSec: heartbeatAgeSec(m, run.StartedAt, time.Now()),
		Supervisor: m.Supervisor, Run: viewOf(run, m, slug),
	}
	if m.Supervisor == nil || !m.Supervisor.Attached {
		sv.ReasonCode = core.ReasonSupervisorNotAttached
		return sv, nil
	}
	sv.Attached = true
	threshold := stallThresholdSec(m, thresholdSec)
	sv.ThresholdSec = threshold
	now := time.Now()
	age := heartbeatAgeSec(m, run.StartedAt, now)
	sv.HeartbeatAgeSec = age
	stalled := stallApplies(m) && age >= 0 && age >= int64(threshold)
	m.Supervisor.State = "polling"
	m.Supervisor.LastPollAt = nowISO()
	if !stalled {
		if err := e.saveRun(run, m); err != nil {
			return SuperviseView{}, err
		}
		sv.Supervisor = m.Supervisor
		sv.Run = viewOf(run, m, slug)
		return sv, nil
	}
	// Stall: record it on the supervisor + run counters, then mark
	// the host interruption through the P2a path (single interrupt
	// representation).
	sv.StallDetected = true
	sv.ReasonCode = core.ReasonSupervisorStall
	sv.Actions = []string{"stall_detected", "host_interrupted"}
	sv.WorkerPlan = workerPlanFor(m)
	m.StallCount++
	m.LastStallAt = nowISO()
	m.Supervisor.StallCount = m.StallCount
	if err := e.appendEvent(run, "supervisor_stall_detected", map[string]any{
		"reason_code": core.ReasonSupervisorStall, "threshold_sec": threshold,
		"heartbeat_age_sec": age, "stage": m.currentStage(), "stall_count": m.StallCount,
	}); err != nil {
		return SuperviseView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return SuperviseView{}, err
	}
	substep := m.LastSubstep
	if substep == "" {
		substep = m.currentStage()
	}
	if _, err := e.RecordInterrupt(runID, substep); err != nil {
		return SuperviseView{}, err
	}
	run, m, slug, err = e.loadRun(runID)
	if err != nil {
		return SuperviseView{}, err
	}
	sv.Supervisor = m.Supervisor
	sv.Run = viewOf(run, m, slug)
	return sv, nil
}

// ---- Longrun doctor (G-07 read surface) ----

// LongrunDoctorReport is the per-run long-run health summary: driver
// rounds, heartbeat age, stall count (counted from the append-only
// event trail, the authority), blocker streak and repair count. It is
// read-only. Note: the v1 `longrun-doctor` command reported install /
// bridge readiness instead; the charter assigns G-07 a per-run health
// entry, so this report is per-run (bridge readiness belongs to the
// doctor surface, out of P2b scope — see p2b report).
type LongrunDoctorReport struct {
	RunID              string       `json:"run_id"`
	Status             string       `json:"status"`
	LoopStage          string       `json:"loop_stage"`
	CurrentStage       string       `json:"current_stage,omitempty"`
	DriverState        string       `json:"driver_state"`
	DriverRounds       int          `json:"driver_rounds"`
	HeartbeatAgeSec    int64        `json:"heartbeat_age_sec"`
	StallCount         int          `json:"stall_count"`
	InterruptCount     int          `json:"interrupt_count"`
	BlockedStreak      int          `json:"blocked_streak"`
	RepairLoopCount    int          `json:"repair_loop_count"`
	SupervisorAttached bool         `json:"supervisor_attached"`
	Budget             *BudgetState `json:"budget,omitempty"`
	EventCount         int          `json:"event_count"`
	Healthy            bool         `json:"healthy"`
	Issues             []string     `json:"issues,omitempty"`
	// FailureModes explains the run's observed reason codes in
	// the governed failure-mode catalog's terms (P4c; v1.6.5
	// catalog consumption): symptom/cause/mitigation for each
	// matched mode. Explanatory only — never a verdict.
	FailureModes []core.FailureMode `json:"failure_modes,omitempty"`
}

// LongrunDoctor builds the health report for one run (read-only).
func (e *Engine) LongrunDoctor(runID string) (LongrunDoctorReport, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return LongrunDoctorReport{}, err
	}
	_ = slug
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return LongrunDoctorReport{}, err
	}
	stalls := 0
	for _, ev := range events {
		if ev.EventType == "supervisor_stall_detected" {
			stalls++
		}
	}
	v := viewOf(run, m, slug)
	rep := LongrunDoctorReport{
		RunID: runID, Status: v.Status, LoopStage: m.LoopStage, CurrentStage: m.currentStage(),
		DriverState: DriverIdle, HeartbeatAgeSec: heartbeatAgeSec(m, run.StartedAt, time.Now()),
		StallCount: stalls, InterruptCount: m.InterruptCount,
		BlockedStreak: m.BlockedStreak, RepairLoopCount: m.RepairLoopCount,
		Budget: m.Budget, EventCount: len(events), Healthy: true,
	}
	if m.Driver != nil {
		rep.DriverState = m.Driver.State
		rep.DriverRounds = m.Driver.TurnN
	}
	if m.Supervisor != nil {
		rep.SupervisorAttached = m.Supervisor.Attached
	}
	threshold := int64(stallThresholdSec(m, 0))
	if rep.HeartbeatAgeSec >= 0 && rep.HeartbeatAgeSec > threshold && stallApplies(m) {
		rep.Issues = append(rep.Issues, "heartbeat_stale")
	}
	if stalls > 0 {
		rep.Issues = append(rep.Issues, "stall_detected_previously")
	}
	if m.BlockedStreak >= 3 {
		rep.Issues = append(rep.Issues, "blocked_streak_hard")
	}
	if m.LoopStage == string(core.LoopErrorRecovery) {
		rep.Issues = append(rep.Issues, "error_recovery")
	}
	if m.Driver == nil && v.Status == "running" {
		rep.Issues = append(rep.Issues, "driver_not_started")
	}
	if exhausted, _ := budgetExhausted(m.Budget); exhausted {
		rep.Issues = append(rep.Issues, core.ReasonBudgetExhausted)
	}
	rep.Healthy = len(rep.Issues) == 0
	// Catalog explanation: gather every reason code this run has
	// actually produced (last block, event trail, doctor issues)
	// and map them to governed failure modes.
	codes := []string{}
	if m.LastBlockReason != "" {
		codes = append(codes, m.LastBlockReason)
	}
	for _, ev := range events {
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err == nil {
			if rc, ok := p["reason_code"].(string); ok && rc != "" {
				codes = append(codes, rc)
			}
		}
	}
	codes = append(codes, rep.Issues...)
	rep.FailureModes = core.FailureModesForReasons(codes)
	return rep, nil
}
