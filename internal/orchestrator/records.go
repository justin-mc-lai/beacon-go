// Orchestration records (R2b, parity audit M8): the engine half of
// loop_goal_state.py's record_* family — intent forks, the user
// decision cache + gate cards, human-stop metrics plumbing, and the
// ONE route into error_recovery every failure path converges on.
// All four live on the run snapshot (runs.metadata_json) and in the
// lifecycle event trail; no new tables, no side files (charter
// rule 7). The pure shapes and rules live in core
// (loopgoal_records.go); this file is load/mutate/persist.
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ensureHumanStops returns the run's metrics record, creating and
// default-filling it on first use (_ensure_human_stop_metrics).
func ensureHumanStops(m *runMeta) *core.HumanStopMetrics {
	if m.HumanStops == nil {
		m.HumanStops = &core.HumanStopMetrics{}
	}
	m.HumanStops.Ensure()
	return m.HumanStops
}

// RecordIntentFork appends an intent-fork trace at a routing
// decision point (record_intent_fork, R7): the utterance slice that
// caused the fork, the decision taken and its reason code. A
// non-empty slice also becomes the run's latest utterance, and the
// fork's stage becomes the last substep (the source sets both).
func (e *Engine) RecordIntentFork(runID, stage, utteranceSlice, decision, reasonCode string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	fork := core.IntentFork{
		Stage: stage, UtteranceSlice: utteranceSlice,
		Decision: decision, ReasonCode: reasonCode, At: nowISO(),
	}
	m.IntentForks = append(m.IntentForks, fork)
	if utteranceSlice != "" {
		m.LatestUtterance = utteranceSlice
	}
	m.LastSubstep = stage
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "intent_fork", map[string]any{
			"stage": stage, "decision": decision, "reason_code": reasonCode,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// RecordUserDecision appends one user answer to the run's decision
// cache (record_user_decision): the same key answered once is never
// asked again — the planner-review routing consults the cache for
// its user-decision branch, and the gate cards below read it too.
// Appending never rejects (the source appends); the rejecting
// variant is ResolveGate.
func (e *Engine) RecordUserDecision(runID, key, value string) (RunView, error) {
	if key == "" {
		return RunView{}, core.NewError("decision_key_required", "goal.RecordUserDecision", "decision key must be non-empty")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	m.Decisions = append(m.Decisions, core.UserDecision{Key: key, Value: value, At: nowISO()})
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "user_decision_recorded", map[string]any{
			"decision_key": key,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// DecisionCacheHit reports whether the run's cache already holds an
// answer for key (read-only; _decision_cache_hit).
func (e *Engine) DecisionCacheHit(runID, key string) (bool, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return false, err
	}
	return core.DecisionCacheHit(m.Decisions, key), nil
}

// BuildGateCard builds the concrete gate card for one decision
// (goal_gates.build_gate_card): when the cache already holds the
// key, the cached answer comes back instead of a fresh question.
// Read-only — building a card writes nothing.
func (e *Engine) BuildGateCard(runID, key, question, context, resumeAction string) (core.GateCardResult, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return core.GateCardResult{}, err
	}
	return core.BuildGateCard(m.Decisions, key, question, context, resumeAction), nil
}

// ResolveGate records the user's answer for one gate card
// (goal_gates.resolve_gate): re-resolving the same key is refused
// with already_resolved.
func (e *Engine) ResolveGate(runID, key, answer string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	cache, ok, code := core.ResolveGate(m.Decisions, key, answer, nowISO())
	if !ok {
		return RunView{}, core.NewError(code, "goal.ResolveGate",
			"decision "+key+" was already resolved for run "+runID+"; a cached answer is never asked or answered twice")
	}
	m.Decisions = cache
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "gate_resolved", map[string]any{
			"decision_key": key,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// AssertGateResolved is the illegal-transition guard
// (goal_gates.assert_gate_resolved): an unresolved gate blocks the
// move to the next stage with gate_unresolved. Read-only.
func (e *Engine) AssertGateResolved(runID, key string) (bool, string, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return false, "", err
	}
	ok, code := core.AssertGateResolved(m.Decisions, key)
	return ok, code, nil
}

// RouteToErrorRecovery is the unified failure exit
// (route_to_error_recovery): any caller — a stage failure, a review
// route, a supervisor stall, a self-heal dead end — sends the run
// to error_recovery through THIS door, with the source's
// beacon-gov-debug recommendation on the trail. The loop stage is
// set directly, as the source's transition writes it; the run is
// marked blocked with the caller's reason code.
func (e *Engine) RouteToErrorRecovery(runID, reasonCode string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	e.routeToErrorRecovery(run, m, reasonCode)
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "error_recovery", map[string]any{
			"reason_code": reasonCode, "recommended_route": "beacon-gov-debug",
			"via": "route_to_error_recovery",
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// routeToErrorRecovery applies the state mutation only (the caller
// batches its own event + save). Shared by RouteToErrorRecovery and
// the repair-loop ceiling path in tick.go.
func (e *Engine) routeToErrorRecovery(run store.Run, m *runMeta, reasonCode string) {
	ls := m.loopState()
	ls.CurrentStage = core.LoopErrorRecovery
	ls.StageHistory = append(ls.StageHistory, string(core.LoopErrorRecovery))
	m.applyLoopState(ls)
	m.Blocked = true
	if reasonCode != "" {
		m.LastBlockReason = reasonCode
	}
}

// IsReviewBlockReason reports whether a block reason came from the
// review routing family (planner_review_*): such a block is cleared
// only by a later review decision that routes onward, never by a
// bare continue.
func IsReviewBlockReason(code string) bool {
	return strings.HasPrefix(code, "planner_review_")
}
