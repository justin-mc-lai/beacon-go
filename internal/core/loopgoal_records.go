// Loop-goal record functions + the planner-review routing decision
// (R2b, parity audit H5/M8): the pure half of loop_goal_state.py's
// record_* family and goal_gates.py's gate cards. Python keeps these
// records inside the loop state file's `outputs` dict; Go keeps them
// inside the run snapshot (runs.metadata_json) — the functions here
// are pure value transforms, persistence lives in the orchestrator.
//
// Ported semantics (loop_goal_state.py, line refs in the r2b report):
//   - record_intent_fork: append {stage, utterance_slice, decision,
//     reason_code, at}; the slice also becomes latest_utterance.
//   - record_user_decision: append {key, value, at} to a LIST cache;
//     a later hit on the same key is how "asked once, never again"
//     is judged (record_review_decision consults it). Appending
//     never rejects — rejection lives in goal_gates.resolve_gate.
//   - _ensure_human_stop_metrics / _bump_stop_reason: per-run human
//     stop counter + per-reason distribution + p1_continue_count +
//     pseudo_stall_flag. NOTE: pseudo_stall_flag is only ever
//     initialised False in the Python tree (state init and the
//     metrics default); no code path sets it — ported as-is.
//   - record_review_decision: fail-closed routing of a review
//     packet by severity / route / user-decision, in the source's
//     exact decision order (RouteReviewDecision below).
//   - goal_gates.py gate cards: build (cached answer wins),
//     resolve (re-resolving is already_resolved), assert
//     (unresolved is gate_unresolved).
package core

import "strings"

// PlannerReviewDecisionKey is the decision-cache key
// record_review_decision consults for its user-decision branch
// (verbatim from the source).
const PlannerReviewDecisionKey = "planner_review_user_decision"

// Review route vocabularies (loop_goal_state.py module constants,
// verbatim).
var (
	// BlockingReviewSeverities: only P0 blocks by severity alone
	// (R6: standalone P1 is advisory continue).
	BlockingReviewSeverities = map[string]bool{"P0": true}
	// DriftReviewRoutes route to drift_routing (truth change flow).
	DriftReviewRoutes = map[string]bool{
		"beacon-gen-truth": true, "beacon-gen-change": true, "beacon-gen-refreeze": true,
	}
	// ImplementReadyReviewRoutes may continue to
	// standard_execution_precheck.
	ImplementReadyReviewRoutes = map[string]bool{
		"": true, "continue_delivery": true, "beacon-gen-implement": true,
		"beacon-pln-friction": true, "beacon-eval-qa": true,
	}
	// AdvisoryContinueSeverities record a continue audit when the
	// route is implement-ready.
	AdvisoryContinueSeverities = map[string]bool{"P1": true, "P2": true, "P3": true}
	// P1HumanGateOverrides are the human_gate_overrides spellings
	// that elevate a standalone P1 to a human stop.
	P1HumanGateOverrides = map[string]bool{
		"p1": true, "planner_review_p1": true, "review_p1": true, "review": true,
	}
)

// ---- Intent forks (record_intent_fork) ----

// IntentFork is one routing-decision trace entry (R7).
type IntentFork struct {
	Stage          string `json:"stage"`
	UtteranceSlice string `json:"utterance_slice"`
	Decision       string `json:"decision"`
	ReasonCode     string `json:"reason_code,omitempty"`
	At             string `json:"at"`
}

// ---- User decision cache (record_user_decision + goal_gates) ----

// UserDecision is one cached user answer. The Python loop state
// keeps a list of {key, value, at}; goal_gates.py keeps the same
// cache as a dict — one list here serves both readers.
type UserDecision struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	At    string `json:"at"`
}

// DecisionCacheHit mirrors _decision_cache_hit: any entry with the
// key counts as answered (the same question is never asked twice).
func DecisionCacheHit(cache []UserDecision, key string) bool {
	for _, d := range cache {
		if d.Key == key {
			return true
		}
	}
	return false
}

// DecisionCacheValue returns the cached answer for a key.
func DecisionCacheValue(cache []UserDecision, key string) (string, bool) {
	for _, d := range cache {
		if d.Key == key {
			return d.Value, true
		}
	}
	return "", false
}

// ---- Human stop metrics (_ensure_human_stop_metrics) ----

// HumanStopMetrics is the per-run human-stop accounting of
// loop_goal_state._default_human_stop_metrics.
type HumanStopMetrics struct {
	HumanStopsPerRun int            `json:"human_stops_per_run"`
	StopReasonCounts map[string]int `json:"stop_reason_counts"`
	P1ContinueCount  int            `json:"p1_continue_count"`
	PseudoStallFlag  bool           `json:"pseudo_stall_flag"`
}

// Ensure fills the zero fields with the source defaults (a present
// metrics record keeps its values; the counts map is normalised to
// non-nil), mirroring _ensure_human_stop_metrics.
func (m *HumanStopMetrics) Ensure() {
	if m.StopReasonCounts == nil {
		m.StopReasonCounts = map[string]int{}
	}
}

// BumpStopReason mirrors _bump_stop_reason: one more human stop,
// attributed to reasonCode. record_host_interrupt is the same bump
// with HOST_INTERRUPT_REASON (ReasonHostTurnInterrupted).
func (m *HumanStopMetrics) BumpStopReason(reasonCode string) {
	m.Ensure()
	m.HumanStopsPerRun++
	m.StopReasonCounts[reasonCode]++
}

// ---- Planner-review routing (record_review_decision) ----

// ReviewFinding is one review finding; severity is the P0..P3 grade
// the routing consumes.
type ReviewFinding struct {
	Severity   string `json:"severity"`
	ReasonCode string `json:"reason_code,omitempty"`
	Message    string `json:"message,omitempty"`
	Source     string `json:"source,omitempty"`
}

// ReviewPacket is the decision packet record_review_decision routes
// on: the Go form of the Python review artifact + synthesis dict.
// DeclaredFindingCount is a pointer because the source's
// completeness test distinguishes "finding_count present" from
// absent. SynthesisMarker records whether the synthesis carried ANY
// routing marker (highest_severity / finding_count /
// user_decision_required / recommended_route key present) — the
// second leg of the source's fail-closed completeness test.
type ReviewPacket struct {
	ReviewID             string
	ArtifactRef          string
	RecommendedRoute     string
	UserDecisionRequired bool
	Findings             []ReviewFinding
	DeclaredHighest      string
	DeclaredFindingCount *int
	SynthesisMarker      bool
}

// HighestReviewSeverity mirrors _highest_review_severity: a declared
// synthesis severity wins; otherwise the strongest finding grade
// (P0 > P1 > P2 > P3); "" when neither speaks.
func HighestReviewSeverity(findings []ReviewFinding, declared string) string {
	if d := strings.ToUpper(strings.TrimSpace(declared)); d != "" {
		return d
	}
	order := map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}
	highest := ""
	for _, f := range findings {
		sev := strings.ToUpper(strings.TrimSpace(f.Severity))
		if rank, ok := order[sev]; ok && (highest == "" || rank < order[highest]) {
			highest = sev
		}
	}
	return highest
}

// ReviewPacketIncomplete mirrors _review_packet_incomplete: an
// empty / placeholder packet cannot authorize implement continue.
func ReviewPacketIncomplete(p ReviewPacket) bool {
	hasRoute := p.RecommendedRoute != ""
	hasFindings := len(p.Findings) > 0
	if p.ReviewID == "" && p.ArtifactRef == "" && !hasRoute && !hasFindings {
		return true
	}
	if !hasRoute && !hasFindings && !p.SynthesisMarker {
		return true
	}
	return false
}

// ReviewRouteInput carries the run-side facts the routing consults
// beyond the packet itself.
type ReviewRouteInput struct {
	Packet             ReviewPacket
	DecisionCacheHit   bool // cache holds PlannerReviewDecisionKey
	AutoDecide         bool
	HumanGateOverrides []string
}

// ReviewRouteResult is the routing outcome: the next loop stage, the
// stop reason ("" = continue), and the continue-audit facts the
// caller folds into the human-stop metrics (P1Continue asks for a
// p1_continue_count increment; BumpReason asks for a
// _bump_stop_reason with ReasonCode).
type ReviewRouteResult struct {
	Blocked               bool
	ReasonCode            string
	NextStage             LoopStage
	HighestSeverity       string
	FindingCount          int
	ContinuedWithAdvisory bool
	AdvisoryOnly          bool
	P1Continue            bool
	BumpReason            bool
	NextSkill             string
}

// RouteReviewDecision is record_review_decision's routing core, in
// the source's exact order (fail-closed):
//
//  1. incomplete packet            -> error_recovery / planner_review_packet_incomplete
//  2. drift route                   -> drift_routing / planner_review_route_<route>
//  3. user decision required+uncached -> error_recovery / planner_review_user_decision_required
//  4. P0 severity                   -> error_recovery / planner_review_blocking_finding
//  5. P1 + human gate forced + implement-ready route
//     -> error_recovery / planner_review_human_gate_p1
//  6. implement-ready route        -> standard_execution_precheck (advisory audit for P1/P2/P3)
//  7. anything else                 -> error_recovery / planner_review_route_unhandled
func RouteReviewDecision(in ReviewRouteInput) ReviewRouteResult {
	p := in.Packet
	highest := HighestReviewSeverity(p.Findings, p.DeclaredHighest)
	count := len(p.Findings)
	if p.DeclaredFindingCount != nil {
		count = *p.DeclaredFindingCount
	}
	res := ReviewRouteResult{HighestSeverity: highest, FindingCount: count}
	route := strings.TrimSpace(p.RecommendedRoute)

	forceP1 := !in.AutoDecide
	if !forceP1 {
		for _, o := range in.HumanGateOverrides {
			if P1HumanGateOverrides[strings.ToLower(strings.TrimSpace(o))] {
				forceP1 = true
				break
			}
		}
	}

	switch {
	case ReviewPacketIncomplete(p):
		res.NextStage, res.ReasonCode = LoopErrorRecovery, ReasonPlannerReviewPacketIncompl
	case DriftReviewRoutes[route]:
		res.NextStage, res.ReasonCode = LoopDriftRouting, PlannerReviewRouteCode(route)
	case p.UserDecisionRequired && !in.DecisionCacheHit:
		res.NextStage, res.ReasonCode = LoopErrorRecovery, ReasonPlannerReviewUserDecisionReq
	case BlockingReviewSeverities[highest]:
		res.NextStage, res.ReasonCode = LoopErrorRecovery, ReasonPlannerReviewBlockingFinding
	case highest == "P1" && forceP1 && ImplementReadyReviewRoutes[route]:
		res.NextStage, res.ReasonCode = LoopErrorRecovery, ReasonPlannerReviewHumanGateP1
	case ImplementReadyReviewRoutes[route]:
		res.NextStage = LoopStandardExecutionPrecheck
		if AdvisoryContinueSeverities[highest] {
			res.ContinuedWithAdvisory = true
			if highest == "P1" {
				res.P1Continue = true
			} else {
				res.AdvisoryOnly = true
			}
		}
	default:
		res.NextStage, res.ReasonCode = LoopErrorRecovery, ReasonPlannerReviewRouteUnhandled
	}
	res.Blocked = res.ReasonCode != ""
	res.BumpReason = res.Blocked
	res.NextSkill = LoopSkillForStage(res.NextStage)
	return res
}

// ---- Gate cards (goal_gates.py) ----

// GateCardSchema is goal_gates.GATE_SCHEMA.
const GateCardSchema = "goal-user-gate.v1"

// GateCard is one concrete user gate card: question + context + the
// resume action that unblocks the run.
type GateCard struct {
	Schema       string `json:"schema"`
	DecisionKey  string `json:"decision_key"`
	Question     string `json:"question"`
	Context      string `json:"context,omitempty"`
	ResumeAction string `json:"resume_action"`
	Status       string `json:"status"` // open
}

// GateCardResult is build_gate_card's outcome: a cached decision
// short-circuits the card (duplicate=true, card=nil).
type GateCardResult struct {
	Schema       string    `json:"schema"`
	Duplicate    bool      `json:"duplicate"`
	CachedAnswer string    `json:"cached_answer,omitempty"`
	Card         *GateCard `json:"card,omitempty"`
}

// BuildGateCard mirrors build_gate_card: when the decision cache
// already holds the key, the cached answer is returned instead of a
// new question (AC-003: never ask twice).
func BuildGateCard(cache []UserDecision, decisionKey, question, context, resumeAction string) GateCardResult {
	if resumeAction == "" {
		resumeAction = ResumeAwaitHumanConfirm
	}
	if v, hit := DecisionCacheValue(cache, decisionKey); hit {
		return GateCardResult{Schema: GateCardSchema, Duplicate: true, CachedAnswer: v}
	}
	return GateCardResult{Schema: GateCardSchema, Card: &GateCard{
		Schema: GateCardSchema, DecisionKey: decisionKey, Question: question,
		Context: context, ResumeAction: resumeAction, Status: "open",
	}}
}

// ResolveGate mirrors resolve_gate: recording the answer appends it
// to the cache; re-resolving the same key is rejected with
// already_resolved and the cache is returned unchanged.
func ResolveGate(cache []UserDecision, decisionKey, answer, at string) ([]UserDecision, bool, string) {
	if DecisionCacheHit(cache, decisionKey) {
		return cache, false, ReasonGateAlreadyResolved
	}
	out := append(append([]UserDecision{}, cache...), UserDecision{Key: decisionKey, Value: answer, At: at})
	return out, true, ""
}

// AssertGateResolved mirrors assert_gate_resolved: an unresolved
// gate blocks the transition to the next stage (gate_unresolved).
func AssertGateResolved(cache []UserDecision, decisionKey string) (bool, string) {
	if DecisionCacheHit(cache, decisionKey) {
		return true, ""
	}
	return false, ReasonGateUnresolved
}
