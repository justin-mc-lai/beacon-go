// Review decision write-back (R2b, parity audit H5): the engine
// half of loop_goal_state.record_review_decision. A planner-review
// (or implementation-review, M15) conclusion is not just filed — it
// is ROUTED back into the loop state machine, fail-closed:
//
//   - P0 severity                -> error_recovery (implement stops)
//   - user decision required     -> error_recovery until the answer
//     is in the decision cache (M8), then a re-review may pass
//   - drift route (truth/change/refreeze) -> drift_routing, with
//     the truth-change flow named on the record as the way out
//   - P1 with the human gate forced -> error_recovery
//   - implement-ready route      -> standard_execution_precheck,
//     with an advisory-continue audit for P1/P2/P3 findings
//   - incomplete / unhandled packet -> error_recovery
//
// The pure routing lives in core.RouteReviewDecision; this file
// applies it to the run snapshot, the human-stop metrics and the
// event trail. Consumers: admission item 9 refuses implement while
// a routed block is unresolved, and Tick holds the run at
// error_recovery / drift_routing until a later decision routes
// onward.
package orchestrator

import (
	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ReviewDecisionRecord is the run's latest routed review decision
// (the Go form of the packet record_review_decision writes into the
// loop outputs). Resolved=false is the hold every consumer reads:
// it flips only when a later decision routes to
// standard_execution_precheck.
type ReviewDecisionRecord struct {
	ReviewID              string `json:"review_id,omitempty"`
	ArtifactRef           string `json:"artifact_ref,omitempty"`
	Source                string `json:"source,omitempty"` // planner_review | review_face
	RecommendedRoute      string `json:"recommended_route,omitempty"`
	UserDecisionRequired  bool   `json:"user_decision_required"`
	HighestSeverity       string `json:"highest_severity,omitempty"`
	FindingCount          int    `json:"finding_count"`
	NextStage             string `json:"next_stage"`
	ReasonCode            string `json:"reason_code,omitempty"`
	ContinuedWithAdvisory bool   `json:"continued_with_advisory,omitempty"`
	AdvisoryOnly          bool   `json:"advisory_only,omitempty"`
	Resolved              bool   `json:"resolved"`
	DriftHint             string `json:"drift_hint,omitempty"`
	At                    string `json:"at"`
}

// DriftRouteHint is the way out of a drift route, spelled for the
// human: the conclusion says the work drifted from the frozen
// truth, so the resolution is the truth change flow, not a bare
// continue.
const DriftRouteHint = "需求跑偏：走 truth 变更流程 — `beacon truth change` 登记变更、改包、`beacon truth refreeze` 重新冻结，再重新评审"

// RecordReviewDecision routes one review packet into the run's loop
// state (record_review_decision). It is the public door; the
// planner-review and review-face callers use recordReviewDecisionOn
// with their already-loaded snapshot.
func (e *Engine) RecordReviewDecision(runID string, packet core.ReviewPacket, source string) (ReviewDecisionRecord, RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return ReviewDecisionRecord{}, RunView{}, err
	}
	rec, err := e.recordReviewDecisionOn(run, m, packet, source)
	if err != nil {
		return ReviewDecisionRecord{}, RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ReviewDecisionRecord{}, RunView{}, err
	}
	return rec, viewOf(run, m, slug), nil
}

// recordReviewDecisionOn applies the routing to a loaded snapshot:
// metrics bumps, the packet record, the loop-stage move and the
// block bookkeeping. It appends the planner_review_decision event
// itself (autocommit); the caller's save persists the snapshot.
func (e *Engine) recordReviewDecisionOn(run store.Run, m *runMeta, packet core.ReviewPacket, source string) (ReviewDecisionRecord, error) {
	autoDecide := true
	if m.AutoDecide != nil {
		autoDecide = *m.AutoDecide
	}
	res := core.RouteReviewDecision(core.ReviewRouteInput{
		Packet:             packet,
		DecisionCacheHit:   core.DecisionCacheHit(m.Decisions, core.PlannerReviewDecisionKey),
		AutoDecide:         autoDecide,
		HumanGateOverrides: m.HumanGateOverrides,
	})
	metrics := ensureHumanStops(m)
	if res.P1Continue {
		metrics.P1ContinueCount++
	}
	if res.BumpReason {
		metrics.BumpStopReason(res.ReasonCode)
	}
	rec := ReviewDecisionRecord{
		ReviewID: packet.ReviewID, ArtifactRef: packet.ArtifactRef, Source: source,
		RecommendedRoute: packet.RecommendedRoute, UserDecisionRequired: packet.UserDecisionRequired,
		HighestSeverity: res.HighestSeverity, FindingCount: res.FindingCount,
		NextStage: string(res.NextStage), ReasonCode: res.ReasonCode,
		ContinuedWithAdvisory: res.ContinuedWithAdvisory, AdvisoryOnly: res.AdvisoryOnly,
		Resolved: !res.Blocked, At: nowISO(),
	}
	if res.NextStage == core.LoopDriftRouting {
		rec.DriftHint = DriftRouteHint
	}
	m.ReviewDecision = &rec
	// The loop stage moves directly, as the source writes it (no
	// transition table on this axis for review routing; the done
	// edge's human gate is untouched — review routing never routes
	// to done).
	ls := m.loopState()
	ls.CurrentStage = res.NextStage
	ls.StageHistory = append(ls.StageHistory, string(res.NextStage))
	m.applyLoopState(ls)
	if res.Blocked {
		m.Blocked = true
		m.LastBlockReason = res.ReasonCode
	} else if IsReviewBlockReason(m.LastBlockReason) {
		// A routed-onward decision clears a previous review block.
		m.Blocked = false
		m.LastBlockReason = ""
	}
	if err := e.appendEvent(run, "planner_review_decision", map[string]any{
		"reason_code": res.ReasonCode, "next_stage": string(res.NextStage),
		"recommended_route": packet.RecommendedRoute, "highest_severity": res.HighestSeverity,
		"finding_count": res.FindingCount, "continued_with_advisory": res.ContinuedWithAdvisory,
		"user_decision_required": packet.UserDecisionRequired, "source": source,
	}); err != nil {
		return ReviewDecisionRecord{}, err
	}
	return rec, nil
}
