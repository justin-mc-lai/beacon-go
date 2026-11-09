// Block / AttachVerifier / Complete (P2a, G-05): the blocker and
// closure surface of the goal engine.
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ---- Block / AttachVerifier / Complete (G-05) ----

// Block records a manual blocker (record_blocker semantics).
func (e *Engine) Block(runID, reasonCode string) (RunView, error) {
	if reasonCode == "" {
		reasonCode = "manual_block"
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if err := e.applyBlockBatched(run, m, reasonCode, "goal_blocked"); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// AttachVerifier records the independent checker for the run.
// Maker != checker: the verifier source must differ from the run's
// executor identity; equality is rejected with reason_code
// stop_when_unverified (the source's stop_when family for unverified
// execution — see p2a report for the alignment note).
func (e *Engine) AttachVerifier(runID string, refs []string, source, executorID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if source == "" {
		source = "beacon-eval-qa" // v1 default
	}
	executor := executorID
	if executor == "" {
		executor = m.ExecutorID
	}
	if len(refs) == 0 {
		v := viewOf(run, m, slug)
		return v, core.NewError(ReasonVerifierRequired, "goal.AttachVerifier",
			"verifier refs must be non-empty")
	}
	if executor != "" && source == executor {
		v := viewOf(run, m, slug)
		return v, core.NewError(core.ReasonStopWhenUnverified, "goal.AttachVerifier",
			"maker != checker: verifier source equals the executor identity "+executor)
	}
	m.Verifier = &VerifierInfo{RequiredForComplete: true, Present: true, Refs: refs, Source: source}
	wasBlocked := m.Blocked
	// AC-004: attaching a verifier records the independent checker. It
	// does not prove that the blocker's cause is gone, so it never
	// clears the block (v1 cleared it unconditionally, which let this
	// command bypass an unresolved product_goal_blocked_streak).
	// Recovery is the explicit Engine.Recover path.
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "verifier_attached", map[string]any{
			"refs": refs, "source": source, "cleared_block": false,
			"block_pending": wasBlocked,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// Recover explicitly clears a recorded blocker after new evidence has
// resolved its cause (AC-004). Continue refuses a hard block at
// streak >= 3; this is the one explicit path that resets the snapshot,
// and it records who asserted the cause is gone. Attaching a verifier
// is not this path: it cannot clear an unresolved block.
func (e *Engine) Recover(runID, by, note string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	by = strings.TrimSpace(by)
	if by == "" {
		return viewOf(run, m, slug), core.NewError(ReasonRecoveryIdentityRequired,
			"goal.Recover", "explicit recovery requires the identity asserting the cause is gone (--by)")
	}
	priorBlocked, priorStreak, priorReason := m.Blocked, m.BlockedStreak, m.LastBlockReason
	m.Blocked = false
	m.BlockedStreak = 0
	m.LastBlockReason = ""
	m.LastTickKey = ""
	if stage := m.currentStage(); stage != "" && m.CurrentStageIdx < len(m.StagePlan) {
		m.StagePlan[m.CurrentStageIdx].Status = "active"
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "goal_recovered", map[string]any{
			"by": by, "note": note, "stage": m.currentStage(),
			"prior_blocked": priorBlocked, "prior_blocked_streak": priorStreak,
			"prior_reason_code": priorReason,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// CompleteOptions carries the human-confirmation signal for Complete.
type CompleteOptions struct {
	HumanConfirmed bool
	ConfirmedBy    string
}

// Complete closes the run to done. ALL four conditions must hold —
// loop at release_gate, human confirmed, verifier attached, latest qa
// gate verdict = pass — and each missing one rejects with its own
// reason_code (stricter than the v1 runtime, per the frozen truth;
// see p2a report).
func (e *Engine) Complete(runID string, opts CompleteOptions) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	// Anti-impersonation (G-09, goal_queue.py's hard guard): a
	// discovery queue item is not a goal run and can never be marked
	// product complete, no matter which flags are passed.
	if run.RunKind == RunKindDiscoveryQueue {
		_ = e.appendEvent(run, "complete_rejected", map[string]any{
			"reason_code": core.ReasonLoopCannotComplete,
		})
		return viewOf(run, m, slug), RejectQueueComplete("goal.complete")
	}
	reject := func(code, detail string) (RunView, error) {
		_ = e.appendEvent(run, "complete_rejected", map[string]any{"reason_code": code})
		v := viewOf(run, m, slug)
		v.ReasonCode = code
		return v, core.NewError(code, "goal.Complete", detail)
	}
	if m.LoopStage != string(core.LoopReleaseGate) {
		return reject(core.ReasonNotAtReleaseGate, "current loop stage is "+m.LoopStage)
	}
	if opts.HumanConfirmed && !m.ReleaseHumanConfirmed {
		m.ReleaseHumanConfirmed = true
		m.HumanConfirmedBy = opts.ConfirmedBy
		if err := e.store.WithTx(func(tx *store.Tx) error {
			if err := e.appendEventOn(tx, run, "release_confirmed", map[string]any{"by": opts.ConfirmedBy}); err != nil {
				return err
			}
			return e.saveRunOn(tx, run, m)
		}); err != nil {
			return RunView{}, err
		}
	}
	if !m.ReleaseHumanConfirmed {
		return reject(core.ReasonReleaseRequiresHuman, "release requires explicit human confirmation")
	}
	if m.Verifier == nil || !m.Verifier.Present || len(m.Verifier.Refs) == 0 {
		return reject(ReasonVerifierRequired, "an independent verifier must be attached before complete")
	}
	qa, err := e.store.Gates().Latest(run.FeatureID, "qa")
	if err != nil || qa.Verdict != "pass" {
		return reject(ReasonQAGateNotPassed, "latest qa gate verdict is not pass")
	}
	ls := m.loopState()
	if err := ls.CheckReleaseGate(true); err != nil {
		if rc, ok := core.ReasonCodeOf(err); ok {
			return reject(rc, err.Error())
		}
		return RunView{}, err
	}
	if err := ls.Transition(core.LoopDone, true, e.stopWhen); err != nil {
		if rc, ok := core.ReasonCodeOf(err); ok {
			return reject(rc, err.Error())
		}
		return RunView{}, err
	}
	m.applyLoopState(ls)
	if idx := m.CurrentStageIdx; idx >= 0 && idx < len(m.StagePlan) {
		m.StagePlan[idx].Status = "done"
	}
	m.Blocked = false
	m.LastBlockReason = ""
	// Closure unit: completed event + final run state + lifecycle
	// snapshot commit together (the lifecycle write stays
	// best-effort inside the batch, as it was outside it).
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "goal_completed", map[string]any{
			"verifier_source": m.Verifier.Source, "human_confirmed_by": m.HumanConfirmedBy,
		}); err != nil {
			return err
		}
		if err := e.saveRunOn(tx, run, m); err != nil {
			return err
		}
		_ = updateLifecycleMonotonic(tx.Features(), store.Lifecycle{
			FeatureID: run.FeatureID, Stage: core.StageRelease, Status: core.StatusCompleted,
		})
		return nil
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}
