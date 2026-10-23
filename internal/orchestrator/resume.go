// Heartbeat / Checkpoint / Resume / Trace (P2a, G-08 + G-17 minimum
// surface): liveness, resume points and the read-only event bundle.
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ---- Heartbeat / Checkpoint / Resume (G-08) ----

// Heartbeat records liveness: updates last_heartbeat_at (run metadata —
// the runs table has no heartbeat column; choice documented in the
// p2a report) and appends a goal_heartbeat event.
func (e *Engine) Heartbeat(runID, summary, substep string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	m.LastHeartbeatAt = time.Now().UTC().Format(time.RFC3339Nano)
	m.HeartbeatSummary = summary
	if substep != "" {
		m.LastSubstep = substep
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "goal_heartbeat", map[string]any{
			"summary": summary, "substep": substep, "at": m.LastHeartbeatAt,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// Checkpoint persists a resume point through the store checkpoints
// table (iteration strictly increases, AC-075) plus a goal_checkpoint
// event, mirroring goal_checkpoint's pack contents. The three writes
// (checkpoint row, event, run state) commit as ONE transaction
// (P6 write-path fix).
func (e *Engine) Checkpoint(runID, nextAction, substep string) (store.Checkpoint, RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return store.Checkpoint{}, RunView{}, err
	}
	var ckpt store.Checkpoint
	err = e.store.WithTx(func(tx *store.Tx) error {
		c, err := e.checkpointOn(tx, run, m, nextAction, substep)
		if err != nil {
			return err
		}
		ckpt = c
		return nil
	})
	if err != nil {
		return store.Checkpoint{}, RunView{}, err
	}
	return ckpt, viewOf(run, m, slug), nil
}

// checkpointOn is Checkpoint's write unit against a caller-provided
// write set, so DriverRunOnce can fold the turn-end checkpoint into
// the turn's own batch. It mutates m (LastSubstep /
// LastCheckpointAction) and persists it via the final saveRunOn.
func (e *Engine) checkpointOn(w writeSet, run store.Run, m *runMeta, nextAction, substep string) (store.Checkpoint, error) {
	iteration := 1
	if latest, err := w.Runs().LatestCheckpoint(run.ID); err == nil {
		iteration = latest.Iteration + 1
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.Checkpoint{}, err
	}
	if substep != "" {
		m.LastSubstep = substep
	}
	m.LastCheckpointAction = nextAction
	openBlockers := []string{}
	if m.Blocked && m.LastBlockReason != "" {
		openBlockers = []string{m.LastBlockReason}
	}
	state, err := json.Marshal(map[string]any{
		"run_id": run.ID, "loop_stage": m.LoopStage, "current_stage": m.currentStage(),
		"next_action": nextAction, "last_substep": m.LastSubstep,
		"open_blockers": openBlockers, "last_heartbeat_at": m.LastHeartbeatAt,
	})
	if err != nil {
		return store.Checkpoint{}, err
	}
	ckpt, err := w.Runs().SaveCheckpoint(store.Checkpoint{
		RunID: run.ID, Iteration: iteration, State: state,
	})
	if err != nil {
		return store.Checkpoint{}, err
	}
	if err := e.appendEventOn(w, run, "goal_checkpoint", map[string]any{
		"iteration": iteration, "next_action": nextAction, "substep": substep,
	}); err != nil {
		return store.Checkpoint{}, err
	}
	if err := e.saveRunOn(w, run, m); err != nil {
		return store.Checkpoint{}, err
	}
	return ckpt, nil
}

// RecordInterrupt records a host-turn interruption (AC-034): the next
// Resume will answer replay_substep for the interrupted substep.
func (e *Engine) RecordInterrupt(runID, substep string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if substep == "" {
		substep = m.LastSubstep
	}
	m.InterruptPending = true
	m.InterruptSubstep = substep
	m.InterruptCount++
	if substep != "" {
		m.LastSubstep = substep
	}
	// M8 (R2b): record_host_interrupt also feeds the human-stop
	// metrics (one more stop, attributed to host_turn_interrupted) —
	// the metrics, not just the bare count, are what build_resume
	// reports.
	ensureHumanStops(m).BumpStopReason(core.ReasonHostTurnInterrupted)
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "host_interrupt", map[string]any{
			"reason_code": core.ReasonHostTurnInterrupted, "substep": substep,
			"interrupt_count": m.InterruptCount,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// ResumeView is the resume decision (build_resume semantics).
type ResumeView struct {
	RunID                       string          `json:"run_id"`
	Status                      string          `json:"status"` // ok | blocked
	ReasonCode                  string          `json:"reason_code,omitempty"`
	CurrentStage                string          `json:"current_stage,omitempty"`
	LoopStage                   string          `json:"loop_stage"`
	NextSkill                   string          `json:"next_skill,omitempty"`
	NextAction                  string          `json:"next_action,omitempty"`
	ResumeAction                string          `json:"resume_action"`
	SameSessionContinueRequired bool            `json:"same_session_continue_required"`
	PendingSteering             *SteeringRecord `json:"pending_steering,omitempty"`
	// HumanStops is the run's human-stop accounting (M8, R2b —
	// build_resume includes human_stop_metrics in its output).
	HumanStops *core.HumanStopMetrics `json:"human_stop_metrics,omitempty"`
}

// decideResume is the pure half of Resume (loop_goal_state
// .build_resume): done -> none; a pending host interrupt ->
// replay_substep (AC-034); release_gate -> await_human_confirmation;
// a current stage with a next skill -> load_next_skill (short
// continue utterances 继续 / continue / resume / go take the same
// path, frozen AC-035); a run with nothing to load refuses a no-op
// resume (loop_goal_resume_noop_forbidden). It never mutates the
// snapshot — Resume applies the side effects (soft-block clear,
// interrupt consumption) around it, and ResumePack (G-08) reuses
// the very same decision read-only.
func decideResume(runID string, m *runMeta) ResumeView {
	rv := ResumeView{RunID: runID, Status: "ok", LoopStage: m.LoopStage, CurrentStage: m.currentStage(),
		PendingSteering: m.pendingSteering(), HumanStops: m.HumanStops}
	switch {
	case m.LoopStage == string(core.LoopDone):
		rv.ReasonCode = core.ReasonLoopGoalAlreadyDone
		rv.ResumeAction = core.ResumeNone
		rv.NextAction = "none"
		return rv
	case m.InterruptPending:
		rv.ResumeAction = core.ResumeReplaySubstep
		rv.NextAction = "replay_substep:" + m.InterruptSubstep
		rv.NextSkill = core.LoopSkillForStage(core.LoopStage(m.LoopStage))
		return rv
	case m.LoopStage == string(core.LoopReleaseGate):
		rv.ResumeAction = core.ResumeAwaitHumanConfirm
		rv.NextAction = "await_human_confirmation"
		rv.NextSkill = core.LoopSkillForStage(core.LoopReleaseGate)
		return rv
	}
	stage := m.currentStage()
	nextSkill := ""
	if stage != "" {
		if skills := ProgressiveSkills[stage]; len(skills) > 0 {
			nextSkill = skills[0]
		}
	}
	if nextSkill == "" {
		rv.Status = "blocked"
		rv.ReasonCode = core.ReasonLoopGoalResumeNoopForbidden
		rv.ResumeAction = core.ResumeNone
		rv.NextAction = "none"
		return rv
	}
	rv.ResumeAction = core.ResumeLoadNextSkill
	rv.NextSkill = nextSkill
	rv.NextAction = "load_next_skill:" + nextSkill
	rv.SameSessionContinueRequired = true
	return rv
}

// Resume decides how a run continues and applies the decision's
// side effects: a soft block (streak < 3) is cleared on resume,
// mirroring the v1 goal resume command, and a replayed interrupt
// is consumed (goal_resumed event + flag cleared) exactly once.
func (e *Engine) Resume(runID, utterance string) (ResumeView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ResumeView{}, err
	}
	// Short continue utterances (AC-035) and explicit resumes both
	// map to load_next_skill; a long utterance is advisory only.
	_ = utterance
	// A review-routed block (H5) is NOT a soft block: it clears only
	// when a later review decision routes onward, never by resume.
	reviewHeld := m.ReviewDecision != nil && !m.ReviewDecision.Resolved && IsReviewBlockReason(m.LastBlockReason)
	if m.Blocked && m.BlockedStreak < 3 && !reviewHeld {
		m.Blocked = false
		m.LastTickKey = ""
		_ = e.saveRun(run, m)
	}
	rv := decideResume(runID, m)
	if rv.ResumeAction == core.ResumeReplaySubstep && m.InterruptPending {
		m.InterruptPending = false
		_ = e.appendEvent(run, "goal_resumed", map[string]any{"resume_action": core.ResumeReplaySubstep})
		_ = e.saveRun(run, m)
	}
	return rv, nil
}

// ---- Trace (G-17, minimum surface) ----

// TraceBundle aggregates a run's full event trail in time order.
type TraceBundle struct {
	RunID       string                 `json:"run_id"`
	FeatureID   string                 `json:"feature_id"`
	FeatureSlug string                 `json:"feature_slug"`
	RunKind     string                 `json:"run_kind"`
	Status      string                 `json:"status"`
	LoopStage   string                 `json:"loop_stage"`
	EventCount  int                    `json:"event_count"`
	Events      []store.LifecycleEvent `json:"events"`
}

// Trace returns the run bundle: run metadata + all lifecycle_events of
// the run ordered by (created_at, insertion order). Read-only.
func (e *Engine) Trace(runID string) (TraceBundle, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return TraceBundle{}, err
	}
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return TraceBundle{}, err
	}
	if events == nil {
		events = []store.LifecycleEvent{}
	}
	return TraceBundle{
		RunID: run.ID, FeatureID: run.FeatureID, FeatureSlug: slug,
		RunKind: run.RunKind, Status: viewOf(run, m, slug).Status,
		LoopStage: m.LoopStage, EventCount: len(events), Events: events,
	}, nil
}
