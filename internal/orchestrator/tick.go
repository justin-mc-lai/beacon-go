// Tick / Continue (P2a, G-02): one-step advancement of a goal run.
package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ---- Tick / Continue (G-02) ----

// tickKey fingerprints the inputs a tick decision consumes: run, loop
// stage, product stage and the latest gate facts. Same fingerprint as
// the last consumed tick means there is nothing new to do
// (idempotency, frozen AC-143). Engine-owned writes (the lifecycle
// Blocked snapshot a blocked tick itself records) are deliberately NOT
// part of the fingerprint; new external evidence (a fresh gate result)
// changes it and re-arms the tick.
func (e *Engine) tickKey(run store.Run, m *runMeta) string {
	parts := []string{run.ID, m.LoopStage, m.currentStage(), fmt.Sprint(m.CurrentStageIdx)}
	gates, err := e.store.Gates().LatestByType(run.FeatureID)
	if err == nil {
		for _, g := range gates {
			parts = append(parts, g.GateType+"="+g.Verdict+"@"+g.EvaluatedAt)
		}
	}
	return strings.Join(parts, "|")
}

// applyBlock records a blocker: streak bookkeeping (record_blocker:
// streak >= 3 hardens to product_goal_blocked_streak), lifecycle
// snapshot to Blocked with the reason code, and a goal_blocked event.
// applyBlockOn runs the writes against the caller's write set;
// applyBlockBatched commits them as ONE transaction (the lifecycle
// update stays best-effort inside the batch, exactly as it was
// outside it — a failed snapshot never masked the block itself).
func (e *Engine) applyBlock(run store.Run, m *runMeta, reasonCode, eventType string) error {
	return e.applyBlockOn(e.store, run, m, reasonCode, eventType)
}

func (e *Engine) applyBlockBatched(run store.Run, m *runMeta, reasonCode, eventType string) error {
	return e.store.WithTx(func(tx *store.Tx) error {
		return e.applyBlockOn(tx, run, m, reasonCode, eventType)
	})
}

func (e *Engine) applyBlockOn(w writeSet, run store.Run, m *runMeta, reasonCode, eventType string) error {
	m.Blocked = true
	m.BlockedStreak++
	m.LastBlockReason = reasonCode
	if m.BlockedStreak >= 3 {
		m.LastBlockReason = ReasonBlockedStreak
	}
	if stage := m.currentStage(); stage != "" {
		_ = w.Features().UpdateLifecycle(store.Lifecycle{
			FeatureID: run.FeatureID, Stage: workflowStageForProductStage(stage),
			Status: core.StatusBlocked, ReasonCodes: []string{m.LastBlockReason},
		})
		if m.CurrentStageIdx < len(m.StagePlan) {
			m.StagePlan[m.CurrentStageIdx].Status = "blocked"
		}
	}
	if err := e.appendEventOn(w, run, eventType, map[string]any{
		"reason_code": m.LastBlockReason, "stage": m.currentStage(),
		"blocked_streak": m.BlockedStreak,
	}); err != nil {
		return err
	}
	return e.saveRunOn(w, run, m)
}

// Tick advances the run by one step: execute the current stage via its
// registered executor, judge the stage exit gate from store state, and
// either advance (transition + event) or stop with a recorded blocker.
// Repeating a tick over an unchanged state returns UpToDate without
// re-executing or re-advancing (AC-143).
func (e *Engine) Tick(ctx context.Context, runID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if m.LoopStage == string(core.LoopDone) {
		v := viewOf(run, m, slug)
		v.UpToDate = true
		v.ReasonCode = core.ReasonLoopGoalAlreadyDone
		return v, nil
	}
	// Review hold (H5): a run routed to error_recovery / drift_routing
	// by an unresolved review decision does not execute stages. The
	// hold clears only when a later review decision routes onward
	// (recordReviewDecisionOn flips Resolved), so this guard keys on
	// the record, not on the stage alone — a repair-loop
	// error_recovery without a review hold keeps its retry semantics.
	if m.ReviewDecision != nil && !m.ReviewDecision.Resolved &&
		(m.LoopStage == string(core.LoopErrorRecovery) || m.LoopStage == string(core.LoopDriftRouting)) {
		v := viewOf(run, m, slug)
		v.ReasonCode = m.ReviewDecision.ReasonCode
		return v, nil
	}
	key := e.tickKey(run, m)
	if m.LastTickKey != "" && m.LastTickKey == key {
		v := viewOf(run, m, slug)
		v.UpToDate = true
		if !m.Blocked {
			v.ReasonCode = ReasonUpToDate
		}
		return v, nil
	}
	stage := m.currentStage()
	if stage == "" {
		v := viewOf(run, m, slug)
		v.UpToDate = true
		v.ReasonCode = ReasonUpToDate
		return v, nil
	}
	// Release never executes or exits on tick (human gate).
	if stage != StageRelease {
		// Implement entry is gated by the AC-142 admission bundle
		// (S2): the superset verdict whose fifth item IS the one
		// shared admission checker (G-04) — the same bundle
		// DelegateRalph packs under. The evaluation is persisted
		// (gate_type=admission_bundle + admission_bundle_evaluated
		// event) as the entry evidence. A blocked bundle blocks
		// the tick before any executor runs — no lane gets a
		// second judgement.
		if stage == StageImplement {
			bundle, admErr := e.admissionBundleForEntry(run, m)
			if admErr != nil {
				return RunView{}, admErr
			}
			if !bundle.AllowImplement {
				m.LastTickKey = key
				// applyBlock already persists m (with LastTickKey);
				// the extra saveRun this branch used to do rewrote
				// the identical snapshot — dropped in the batching
				// pass, final state unchanged.
				if err := e.applyBlockBatched(run, m, bundle.ReasonCode, "implement_admission_blocked"); err != nil {
					return RunView{}, err
				}
				return viewOf(run, m, slug), nil
			}
		}
		var ex StageExecutor
		var ok bool
		if stage == StageImplement {
			ex, ok = e.implementExecutor(m)
		} else {
			ex, ok = e.executors[stage]
		}
		if !ok {
			// LastTickKey is set before the block so the batched
			// save inside applyBlock persists it (previously a
			// second saveRun after applyBlock did that).
			m.LastTickKey = key
			if err := e.applyBlockBatched(run, m, ReasonExecutorNotReg, "goal_blocked"); err != nil {
				return RunView{}, err
			}
			return viewOf(run, m, slug), nil
		}
		outcome, exErr := ex.ExecuteStage(ctx, viewOf(run, m, slug), stage)
		if exErr != nil {
			outcome = StageOutcome{Status: "failed", ReasonCode: ReasonExecutorError, Summary: exErr.Error()}
		}
		if outcome.Status != "done" {
			reason := outcome.ReasonCode
			if reason == "" {
				reason = ReasonStageExecutionFailed
			}
			return e.handleStageFailure(run, m, slug, stage, reason, key)
		}
	}
	// Exit gate judgement over store state.
	in, err := e.buildExitInput(run, m)
	if err != nil {
		return RunView{}, err
	}
	verdict := EvaluateStageExit(in)
	if !verdict.Allowed {
		m.LastTickKey = key
		// As in the admission branch, applyBlock's own save
		// persists LastTickKey; the old extra saveRun was a
		// duplicate write of the same snapshot.
		if err := e.applyBlockBatched(run, m, verdict.ReasonCode, "stage_exit_blocked"); err != nil {
			return RunView{}, err
		}
		return viewOf(run, m, slug), nil
	}
	// Advance one stage. The three writes (lifecycle snapshot,
	// stage_advanced event, run state) commit as ONE transaction.
	m.StagePlan[m.CurrentStageIdx].Status = "done"
	m.CurrentStageIdx++
	m.Blocked = false
	m.BlockedStreak = 0
	m.LastBlockReason = ""
	m.LastTickKey = key
	nextStage := m.currentStage()
	ls := m.loopState()
	if nextStage != "" {
		m.StagePlan[m.CurrentStageIdx].Status = "active"
		target := LoopStageForProductStage(nextStage)
		if ls.CurrentStage != target {
			if err := ls.Transition(target, false, e.stopWhen); err != nil {
				return RunView{}, err
			}
			m.applyLoopState(ls)
		}
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		_ = tx.Features().UpdateLifecycle(store.Lifecycle{
			FeatureID: run.FeatureID, Stage: workflowStageForProductStage(stage),
			Status: core.StatusCompleted,
		})
		if err := e.appendEventOn(tx, run, "stage_advanced", map[string]any{
			"from_stage": stage, "to_stage": nextStage, "loop_stage": m.LoopStage,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// handleStageFailure routes an execution failure: implement/qa failures
// count against the repair-loop ceiling (core machine); exceeding it
// lands the run in error_recovery.
func (e *Engine) handleStageFailure(run store.Run, m *runMeta, slug, stage, reason, key string) (RunView, error) {
	if stage == StageImplement || stage == StageQA {
		ls := m.loopState()
		if err := ls.IncrementRepairLoop(); err != nil {
			m.applyLoopState(ls) // routed to error_recovery by the machine
			m.Blocked = true
			m.LastBlockReason = core.ReasonMaxRepairLoopsExceeded
			if evErr := e.store.WithTx(func(tx *store.Tx) error {
				if err := e.appendEventOn(tx, run, "error_recovery", map[string]any{
					"reason_code": core.ReasonMaxRepairLoopsExceeded, "stage": stage,
					"repair_loop_count": m.RepairLoopCount, "recommended_route": "beacon-gov-debug",
				}); err != nil {
					return err
				}
				return e.saveRunOn(tx, run, m)
			}); evErr != nil {
				return RunView{}, evErr
			}
			return viewOf(run, m, slug), nil
		}
		m.applyLoopState(ls)
		// The repair_loop event and the block it leads to are one
		// unit: a single batched write.
		if err := e.store.WithTx(func(tx *store.Tx) error {
			if err := e.appendEventOn(tx, run, "repair_loop", map[string]any{
				"reason_code": reason, "stage": stage, "repair_loop_count": m.RepairLoopCount,
			}); err != nil {
				return err
			}
			return e.applyBlockOn(tx, run, m, reason, "stage_failed")
		}); err != nil {
			return RunView{}, err
		}
		return viewOf(run, m, slug), nil
	}
	// The failed attempt is consumed: a later Continue clears the block
	// and re-runs the stage (repair loop).
	if err := e.applyBlockBatched(run, m, reason, "stage_failed"); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// Continue mirrors goal_continue_run + the v1 resume unblock: a blocked
// run with streak < 3 is unblocked and ticked; a hard-blocked run
// (streak >= 3) stays blocked with product_goal_blocked_streak.
func (e *Engine) Continue(ctx context.Context, runID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	// A review-routed block (H5) is not cleared by continue: the run
	// waits for its resolution path (a user decision + re-review, or
	// the truth change flow for a drift route).
	if m.ReviewDecision != nil && !m.ReviewDecision.Resolved && IsReviewBlockReason(m.LastBlockReason) {
		v := viewOf(run, m, slug)
		v.ReasonCode = m.ReviewDecision.ReasonCode
		return v, nil
	}
	if m.Blocked {
		if m.BlockedStreak >= 3 {
			v := viewOf(run, m, slug)
			v.ReasonCode = ReasonBlockedStreak
			return v, nil
		}
		m.Blocked = false
		m.LastTickKey = ""
		if stage := m.currentStage(); stage != "" && m.CurrentStageIdx < len(m.StagePlan) {
			m.StagePlan[m.CurrentStageIdx].Status = "active"
		}
		if err := e.store.WithTx(func(tx *store.Tx) error {
			if err := e.appendEventOn(tx, run, "goal_continued", map[string]any{
				"stage": m.currentStage(), "blocked_streak": m.BlockedStreak,
			}); err != nil {
				return err
			}
			return e.saveRunOn(tx, run, m)
		}); err != nil {
			return RunView{}, err
		}
	}
	return e.Tick(ctx, runID)
}
