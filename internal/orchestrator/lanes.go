// Implement lanes (P2c, G-22): single / team / ralph under ONE
// admission and ONE collection rule.
//
// v1's implement package grew three write surfaces (direct/team/
// ralph in commands.py's 5,505 lines) that share the fail-closed
// admission of implement_admission.py "so goal, direct, team, and
// ralph share one fail-closed pre-write invariant". The Go port keeps
// exactly that shape: whichever lane runs the implement stage, entry
// is gated by checkAdmission (G-04) and the result is collected by
// validateHarvest — artifact pointers in artifact_refs plus a
// stage_executed evidence event, missing either one is not collected.
//
// Lane selection: v1 takes the mode explicitly (--mode, normalized by
// _normalize_mode; runner defaults then follow the mode) and has NO
// size-based rule. The priority landed here follows the task's
// fallback and is labelled DERIVED where it goes beyond v1:
// explicit ask first, then the task-scale field (large -> ralph, the
// self-checking loop suits the biggest jobs), then lake count
// (>1 -> team: several lakes mean several hands handing work over),
// and single as the default (v1's default runner is native-single).
package orchestrator

import (
	"context"
	"encoding/json"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// NormalizeLane mirrors _normalize_mode: one of the three lane names,
// or "" for anything else.
func NormalizeLane(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case LaneSingle, LaneTeam, LaneRalph:
		return strings.ToLower(strings.TrimSpace(s))
	}
	return ""
}

// RouteLane picks the implement lane (pure function). Priority:
// explicit > scale > lake count > default single.
func RouteLane(req LaneRequest, lakeCount int) (LaneDecision, error) {
	if req.Explicit != "" {
		lane := NormalizeLane(req.Explicit)
		if lane == "" {
			return LaneDecision{}, core.NewError(core.ReasonLaneUnknown, "goal.RouteLane",
				"unknown lane "+req.Explicit+" (want single|team|ralph)")
		}
		return LaneDecision{Lane: lane, Source: LaneSourceExplicit, Reason: "explicit lane requested"}, nil
	}
	// DERIVED (no v1 counterpart): the scale field routes the biggest
	// jobs to the ralph loop, which re-checks its own work each round.
	if strings.EqualFold(strings.TrimSpace(req.Scale), "large") {
		return LaneDecision{Lane: LaneRalph, Source: LaneSourceScale,
			Reason: "scale=large routes to the ralph self-checking loop (DERIVED rule)"}, nil
	}
	// DERIVED from the parallel-plan semantics: several lakes are
	// several workstations handing results to one hub — the team lane.
	if lakeCount > 1 {
		return LaneDecision{Lane: LaneTeam, Source: LaneSourceLakes,
			Reason: "multiple lakes route to the team handoff lane (DERIVED rule)"}, nil
	}
	return LaneDecision{Lane: LaneSingle, Source: LaneSourceDefault,
		Reason: "default lane (v1 default runner is native-single)"}, nil
}

// ValidateTeamHandoff is the team lane's handoff check (pure):
// from, to and at least one artifact ref are required; it returns
// the names of the missing fields (empty when the handoff is whole).
func ValidateTeamHandoff(h *TeamHandoff) []string {
	var missing []string
	if h == nil || strings.TrimSpace(h.From) == "" {
		missing = append(missing, "from")
	}
	if h == nil || strings.TrimSpace(h.To) == "" {
		missing = append(missing, "to")
	}
	refs := 0
	if h != nil {
		for _, r := range h.ArtifactRefs {
			if strings.TrimSpace(r) != "" {
				refs++
			}
		}
	}
	if refs == 0 {
		missing = append(missing, "artifact_refs")
	}
	return missing
}

// ImplementPlanView is the `goal implement plan` surface: the routing
// decision, the shared admission verdict, which lanes have a
// registered executor, and the collection rule every lane obeys.
type ImplementPlanView struct {
	RunID       string          `json:"run_id"`
	Decision    LaneDecision    `json:"decision"`
	Admission   AdmissionResult `json:"admission"`
	Executors   map[string]bool `json:"executors"`
	HarvestRule []string        `json:"harvest_rule"`
}

// HarvestRuleText states the shared collection rule on every surface
// that can produce a lane result, so no lane can quietly soften it.
var HarvestRuleText = []string{
	"artifact pointers must land in artifact_refs (absolute paths)",
	"a stage_executed evidence event for the implement stage must exist on the run",
	"team lane additionally requires a whole handoff (from + to + artifact refs)",
	"a collected result never completes the run by itself (release still needs human confirmation, verifier and qa pass)",
}

// PlanImplement computes the lane routing + admission for a run
// without persisting anything (the plan surface is read-only; the
// routing is recorded when a delegation is packed or a lane executes
// under Tick's routed lane).
func (e *Engine) PlanImplement(runID string, req LaneRequest) (ImplementPlanView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ImplementPlanView{}, err
	}
	if req.Scale == "" {
		req.Scale = m.TaskScale
	}
	dec, err := RouteLane(req, len(m.Lakes))
	if err != nil {
		return ImplementPlanView{}, err
	}
	adm, err := e.checkAdmission(run, m)
	if err != nil {
		return ImplementPlanView{}, err
	}
	adm.Lane, adm.LaneSource = dec.Lane, dec.Source
	executors := map[string]bool{}
	for _, l := range []string{LaneSingle, LaneTeam, LaneRalph} {
		_, ok := e.laneExecutors[l]
		executors[l] = ok
	}
	return ImplementPlanView{
		RunID: runID, Decision: dec, Admission: adm,
		Executors: executors, HarvestRule: HarvestRuleText,
	}, nil
}

// RouteImplementLane persists a routing decision on the run (used by
// the delegation packer; Tick consults the recorded lane).
func (e *Engine) routeLane(run store.Run, m *runMeta, dec LaneDecision) error {
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	m.Implement.Lane = dec.Lane
	m.Implement.LaneSource = dec.Source
	return e.appendEvent(run, "implement_lane_routed", map[string]any{
		"lane": dec.Lane, "source": dec.Source, "reason": dec.Reason,
	})
}

// ExecuteLaneStage runs the implement stage through one lane's
// registered executor: admission first (the shared gate), then the
// lane registry — an unregistered lane blocks with
// stage_executor_not_registered, never a faked execution. The
// executor itself writes the stage_executed evidence event.
func (e *Engine) ExecuteLaneStage(ctx context.Context, runID, lane string) (StageOutcome, error) {
	lane = NormalizeLane(lane)
	if lane == "" {
		return StageOutcome{}, core.NewError(core.ReasonLaneUnknown, "goal.ExecuteLaneStage", "unknown lane")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return StageOutcome{}, err
	}
	if m.currentStage() != StageImplement {
		return StageOutcome{}, core.NewError(core.ReasonLaneStageMismatch, "goal.ExecuteLaneStage",
			"run "+runID+" is at stage "+m.currentStage()+", not implement")
	}
	adm, err := e.checkAdmission(run, m)
	if err != nil {
		return StageOutcome{}, err
	}
	if !adm.AllowImplement {
		_ = e.appendEvent(run, "implement_lane_blocked", map[string]any{
			"lane": lane, "reason_code": adm.ReasonCode, "reason_codes": adm.ReasonCodes,
		})
		return StageOutcome{}, core.NewError(adm.ReasonCode, "goal.ExecuteLaneStage",
			"implement admission blocked lane "+lane)
	}
	ex, ok := e.LaneExecutor(lane)
	if !ok {
		return StageOutcome{}, core.NewError(core.ReasonExecutorNotRegistered, "goal.ExecuteLaneStage",
			"no executor registered for lane "+lane)
	}
	outcome, exErr := ex.ExecuteStage(ctx, viewOf(run, m, slug), StageImplement)
	if exErr != nil {
		outcome = StageOutcome{Status: "failed", ReasonCode: ReasonExecutorError, Summary: exErr.Error()}
	}
	_ = e.appendEvent(run, "implement_lane_executed", map[string]any{
		"lane": lane, "executor": ex.ID(), "status": outcome.Status,
		"reason_code": outcome.ReasonCode,
	})
	return outcome, nil
}

// ---- shared harvest (collection) machinery ----

// registerArtifacts lands result artifact pointers in artifact_refs.
func (e *Engine) registerArtifacts(runID string, inputs []ArtifactInput) ([]string, error) {
	var ids []string
	for _, a := range inputs {
		kind := a.Kind
		if kind == "" {
			kind = "implement_artifact"
		}
		ref, err := e.store.Artifacts().Add(store.ArtifactRef{
			RunID: runID, Kind: kind, AbsPath: a.AbsPath,
			ContentHash: a.ContentHash, SizeBytes: a.SizeBytes,
		})
		if err != nil {
			return ids, err
		}
		ids = append(ids, ref.ID)
	}
	return ids, nil
}

// stageExecutedAfter reports whether a stage_executed evidence event
// for the stage exists on the run strictly after the event at index
// anchorIdx (anchorIdx < 0 scans the whole trail). Event order is
// the store's append order (ListEventsByRun).
func (e *Engine) stageExecutedAfter(runID, stage string, anchorIdx int) (bool, error) {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return false, err
	}
	for i, ev := range events {
		if i <= anchorIdx || ev.EventType != "stage_executed" {
			continue
		}
		var p struct {
			Stage string `json:"stage"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err == nil && p.Stage == stage {
			return true, nil
		}
	}
	return false, nil
}

// delegationAnchor finds the trail index of a delegation's
// implement_delegated event (-1 when absent).
func (e *Engine) delegationAnchor(runID, delegationID string) (int, error) {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return -1, err
	}
	for i, ev := range events {
		if ev.EventType != "implement_delegated" {
			continue
		}
		var p struct {
			DelegationID string `json:"delegation_id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err == nil && p.DelegationID == delegationID {
			return i, nil
		}
	}
	return -1, nil
}

// validateHarvest is THE collection rule for all three lanes and for
// delegation returns: artifact pointers present on the run, and
// implement-stage evidence after the anchor. It reports what is
// missing instead of guessing.
func (e *Engine) validateHarvest(runID string, anchorIdx int) (evidence bool, codes []string, err error) {
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return false, nil, err
	}
	if len(arts) == 0 {
		codes = append(codes, core.ReasonResultArtifactsMissing)
	}
	evidence, err = e.stageExecutedAfter(runID, StageImplement, anchorIdx)
	if err != nil {
		return false, nil, err
	}
	if !evidence {
		codes = append(codes, core.ReasonResultEvidenceMissing)
	}
	return evidence, codes, nil
}

// HarvestLaneResult collects one lane's returned result under the
// shared rule. A failed result is recorded as failed (no harvest
// demanded of a failure); a done result is accepted only when the
// harvest is whole — for the team lane the handoff must be whole
// first. Rejection leaves a lane_result_rejected event and a typed
// error carrying the first missing rule's reason code.
func (e *Engine) HarvestLaneResult(runID, lane string, in LaneResultInput) (HarvestView, error) {
	lane = NormalizeLane(lane)
	if lane == "" {
		return HarvestView{}, core.NewError(core.ReasonLaneUnknown, "goal.HarvestLaneResult", "unknown lane")
	}
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return HarvestView{}, err
	}
	v := HarvestView{RunID: runID, Lane: lane}
	if in.Status == "failed" {
		v.ReasonCode = core.ReasonLaneResultFailed
		v.ReasonCodes = []string{core.ReasonLaneResultFailed}
		_ = e.appendEvent(run, "lane_result_failed", map[string]any{
			"lane": lane, "summary": in.Summary,
		})
		return v, nil
	}
	if lane == LaneTeam {
		if missing := ValidateTeamHandoff(in.Handoff); len(missing) > 0 {
			v.Missing = missing
			v.ReasonCode = core.ReasonTeamHandoffIncomplete
			v.ReasonCodes = []string{core.ReasonTeamHandoffIncomplete}
			_ = e.appendEvent(run, "lane_result_rejected", map[string]any{
				"lane": lane, "reason_code": v.ReasonCode, "missing": missing,
			})
			return v, core.NewError(core.ReasonTeamHandoffIncomplete, "goal.HarvestLaneResult",
				"team handoff missing: "+strings.Join(missing, ", "))
		}
	}
	ids, err := e.registerArtifacts(runID, in.Artifacts)
	if err != nil {
		return v, err
	}
	v.ArtifactIDs = ids
	evidence, codes, err := e.validateHarvest(runID, -1)
	if err != nil {
		return v, err
	}
	v.EvidenceFound = evidence
	if len(codes) > 0 {
		v.ReasonCodes = codes
		v.ReasonCode = codes[0]
		_ = e.appendEvent(run, "lane_result_rejected", map[string]any{
			"lane": lane, "reason_codes": codes,
		})
		return v, core.NewError(codes[0], "goal.HarvestLaneResult",
			"lane result harvest incomplete: "+strings.Join(codes, ", "))
	}
	v.Accepted = true
	payload := map[string]any{"lane": lane, "artifact_ids": ids, "summary": in.Summary}
	if in.Handoff != nil {
		payload["handoff_from"] = in.Handoff.From
		payload["handoff_to"] = in.Handoff.To
	}
	_ = e.appendEvent(run, "lane_result_harvested", payload)
	return v, nil
}
