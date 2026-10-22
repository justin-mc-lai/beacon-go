// Standard flow S2 (AC-138): the Standard Execution Flow state
// machine — the THIRD small state machine of the kernel, kept
// deliberately separate from the loop-goal 12-state axis
// (core/loopgoal.go) and the lifecycle axis (core/fsm.go): those
// axes answer "where is the run" and "where is the feature";
// this one answers "how far has the canonical-truth-to-feature
// standard procedure been walked for this run". The axes never
// write each other; the flow only READS facts the other surfaces
// produced (stage plan, package files, freeze trail, S1 baseline,
// integrity gate record, AC-142 bundle, resume decision).
//
// The step order is frozen (R11 / AC-138), fixed in the explicit
// table below:
//
//	support_advisory -> canonical_truth_authoring ->
//	canonical_refreeze -> canonical_commit ->
//	feature_worktree_reconcile -> governance_verify ->
//	workspace_admission -> implement_resume
//
// FlowAdvance is the only way forward: it may complete exactly
// the next pending step, and only when that step's criterion —
// bound to real facts, per step, in flowCriterion — holds.
// Skipping a step, or advancing one whose facts are missing,
// fails closed (standard_flow_order_violation for order breaches;
// the step's own fact code for a missing criterion). Repeating a
// completed step returns already_done and records NOTHING again
// (idempotent, AC-143's flow face): no duplicate event, no new
// revision. Progress lives in run metadata (FlowState) plus one
// flow_step_completed event per completion — no new table.
package orchestrator

import (
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Standard flow step names (frozen R11 order, verbatim).
const (
	FlowStepAdvisory   = "support_advisory"
	FlowStepAuthoring  = "canonical_truth_authoring"
	FlowStepRefreeze   = "canonical_refreeze"
	FlowStepCommit     = "canonical_commit"
	FlowStepReconcile  = "feature_worktree_reconcile"
	FlowStepGovernance = "governance_verify"
	FlowStepAdmission  = "workspace_admission"
	FlowStepResume     = "implement_resume"
)

// FlowSteps is the explicit ordered step table (AC-138). It is
// the single source of the order: status, advance and the CLI
// all read it, nothing re-spells the sequence.
var FlowSteps = []string{
	FlowStepAdvisory, FlowStepAuthoring, FlowStepRefreeze, FlowStepCommit,
	FlowStepReconcile, FlowStepGovernance, FlowStepAdmission, FlowStepResume,
}

// EventFlowStepCompleted is appended once per completed step.
const EventFlowStepCompleted = "flow_step_completed"

// FlowStepRecord is one completed step in the run snapshot.
type FlowStepRecord struct {
	Step   string `json:"step"`
	At     string `json:"at"`
	Detail string `json:"detail,omitempty"`
}

// FlowState is the run's standard-flow progress (metadata only).
type FlowState struct {
	Completed []FlowStepRecord `json:"completed,omitempty"`
	UpdatedAt string           `json:"updated_at,omitempty"`
}

func (s *FlowState) done(step string) (FlowStepRecord, bool) {
	if s == nil {
		return FlowStepRecord{}, false
	}
	for _, r := range s.Completed {
		if r.Step == step {
			return r, true
		}
	}
	return FlowStepRecord{}, false
}

// FlowStepView is one step's status in a FlowView.
type FlowStepView struct {
	Step        string `json:"step"`
	Status      string `json:"status"` // done | pending
	Detail      string `json:"detail,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// FlowView is the flow surface of one run (status + advance share
// the shape; the advance outcome fields are set only by advance).
type FlowView struct {
	RunID          string         `json:"run_id"`
	FeatureSlug    string         `json:"feature_slug"`
	Steps          []FlowStepView `json:"steps"`
	NextStep       string         `json:"next_step,omitempty"`
	CompletedCount int            `json:"completed_count"`
	Complete       bool           `json:"complete"`
	// Advance outcome (FlowAdvance only):
	Attempted   string        `json:"attempted,omitempty"`
	Advanced    bool          `json:"advanced,omitempty"`
	AlreadyDone bool          `json:"already_done,omitempty"`
	Blocked     bool          `json:"blocked,omitempty"`
	ReasonCode  string        `json:"reason_code,omitempty"`
	Detail      string        `json:"detail,omitempty"`
	Bundle      *BundleResult `json:"bundle,omitempty"`
}

func flowIndex(step string) int {
	for i, s := range FlowSteps {
		if s == step {
			return i
		}
	}
	return -1
}

func (e *Engine) flowView(run store.Run, m *runMeta, slug string) FlowView {
	v := FlowView{RunID: run.ID, FeatureSlug: slug}
	for _, s := range FlowSteps {
		sv := FlowStepView{Step: s, Status: "pending"}
		if rec, ok := m.Flow.done(s); ok {
			sv.Status, sv.Detail, sv.CompletedAt = "done", rec.Detail, rec.At
			v.CompletedCount++
		} else if v.NextStep == "" {
			v.NextStep = s
		}
		v.Steps = append(v.Steps, sv)
	}
	v.Complete = v.CompletedCount == len(FlowSteps)
	return v
}

// FlowStatus reports the run's standard-flow progress (read-only:
// no criterion is evaluated, no event is written).
func (e *Engine) FlowStatus(runID string) (FlowView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return FlowView{}, err
	}
	return e.flowView(run, m, slug), nil
}

// flowCriterion judges ONE step against live facts. It returns
// ok=false with the step's fact reason code when the criterion
// does not hold. Judging never writes; FlowAdvance owns the
// writes. bundleOut carries the AC-142 bundle when the admission
// step is judged (judged or not, the evaluation is persisted as
// entry evidence — see the step's case).
func (e *Engine) flowCriterion(run store.Run, m *runMeta, slug, step string) (ok bool, code, detail string, bundleOut *BundleResult, err error) {
	fc, hasFC, ferr := e.featureContextOf(run.FeatureID)
	if ferr != nil {
		return false, "", "", nil, ferr
	}
	switch step {
	case FlowStepAdvisory:
		// Fact: a consultation/plan record exists — the run's
		// stage plan IS the plan record (goal run creation writes
		// it from the advisory/planning surface).
		if len(m.StagePlan) == 0 {
			return false, core.ReasonStageExitPlanIncomplete, "no stage plan on the run; support_advisory has produced no plan record yet", nil, nil
		}
		return true, "", "stage plan present (" + strconv.Itoa(len(m.StagePlan)) + " stages) — advisory/planning record on the run", nil, nil
	case FlowStepAuthoring:
		// Fact: the four-piece package is drafted on the canonical
		// branch (S1 branch fact + the truth surface's files).
		if !hasFC {
			return false, core.ReasonStageExitTruthMissing, "feature is not registered; no package can be authored", nil, nil
		}
		pkg := e.packageFiles(fc)
		var missing []string
		for _, f := range []struct {
			name string
			p    bool
		}{{"truth", pkg.Truth.Present}, {"tests", pkg.Tests.Present}, {"tasks", pkg.Tasks.Present}, {"evidence", pkg.Evidence.Present}} {
			if !f.p {
				missing = append(missing, f.name)
			}
		}
		if len(missing) > 0 {
			return false, core.ReasonStageExitTruthMissing, "package four-piece set is not drafted on disk; missing: " + strings.Join(missing, ", "), nil, nil
		}
		bi := e.branchInfo(fc.Project.RootPath)
		if bi.IsGit && bi.BranchCheck != BranchCheckOK {
			return false, core.ReasonTruthCanonicalBranchMismatch, "authoring must happen on the canonical branch: current " +
				bi.CurrentBranch + " != truth_canonical_branch " + bi.CanonicalBranch + " (AC-136)", nil, nil
		}
		where := "branch_check=" + bi.BranchCheck
		if bi.IsGit {
			where = "canonical branch " + bi.CanonicalBranch
		}
		return true, "", "four-piece package drafted on the canonical branch (" + where + ")", nil, nil
	case FlowStepRefreeze:
		// Fact: a freeze/refreeze event is on the trail AND the
		// shared frozen determination says frozen.
		if !hasFC {
			return false, core.ReasonStageExitTruthNotFrozen, "feature is not registered; nothing can be frozen", nil, nil
		}
		recs, rerr := e.trailRecords(fc.Feature.ID)
		if rerr != nil {
			return false, "", "", nil, rerr
		}
		if recs.freeze == nil {
			return false, core.ReasonStageExitTruthNotFrozen, "no freeze/refreeze event on the feature trail; canonical_refreeze has not happened", nil, nil
		}
		ff, ferr := e.FrozenFact(fc)
		if ferr != nil {
			return false, "", "", nil, ferr
		}
		if !ff.Frozen {
			return false, core.ReasonStageExitTruthNotFrozen, "the shared frozen determination says not frozen (source: " + ff.Source + ")", nil, nil
		}
		return true, "", "freeze/refreeze event on record and the package is frozen (source: " + ff.Source + ")", nil, nil
	case FlowStepCommit:
		// Fact: the S1 baseline — frozen body committed on the
		// canonical branch, fingerprint baseline on record.
		if !hasFC {
			return false, core.ReasonCanonicalRefreezeNotCommitted, "feature is not registered; no baseline can exist", nil, nil
		}
		ff, ferr := e.FrozenFact(fc)
		if ferr != nil {
			return false, "", "", nil, ferr
		}
		switch ff.BaselineStatus {
		case BaselineCommitted:
			return true, "", "baseline_status=committed (truth_baseline_commit " + shortSHA(ff.BaselineCommit) + ")", nil, nil
		case BaselineNotGit:
			// S1 caliber: in a non-git project the frontmatter is
			// the authority and no commit baseline applies — the
			// step is satisfied by the freeze itself.
			return true, "", "not a git repository: no commit baseline applies (frontmatter is the authority, S1 caliber)", nil, nil
		default:
			d := ff.BaselineDetail
			if d == "" {
				d = "baseline_status=" + ff.BaselineStatus
			}
			return false, core.ReasonCanonicalRefreezeNotCommitted, "the refrozen body is not a committed baseline yet: " + d, nil, nil
		}
	case FlowStepReconcile:
		// Fact: the S1 quarantine scan over the run's worktrees
		// finds no stale feature-local refreeze (a fresh scan —
		// findings are recorded before the refusal, exactly as at
		// merge time).
		root, rerr := e.runProjectRoot(run)
		if rerr != nil {
			return false, "", "", nil, rerr
		}
		found := e.scanWorktreeQuarantines(run, m, root)
		if len(found) > 0 {
			if _, rerr := e.recordQuarantines(run, m, found); rerr != nil {
				return false, "", "", nil, rerr
			}
			return false, core.ReasonStaleFeatureLocalRefreeze, "stale feature-local refreeze in worktree " +
				found[0].WorktreePath + " (" + found[0].Reason + "); it is quarantined and must be reconciled on the canonical branch", nil, nil
		}
		return true, "", "worktree reconcile scan clean: no stale feature-local refreeze (AC-140 scan)", nil, nil
	case FlowStepGovernance:
		// Fact: a Truth Integrity evaluation is on record and it
		// does not block (pass, or audit_only for frozen history).
		gates, gerr := e.store.Gates().LatestByType(run.FeatureID)
		if gerr != nil {
			return false, "", "", nil, gerr
		}
		for _, g := range gates {
			if g.GateType != string(gate.GateTruthIntegrity) {
				continue
			}
			switch g.Verdict {
			case "pass", "audit":
				return true, "", "integrity evaluation on record with verdict " + g.Verdict + " (no block)", nil, nil
			default:
				return false, core.ReasonStandardFlowVerificationIncm, "integrity evaluation on record BLOCKS (verdict " +
					g.Verdict + ", codes " + strings.Join(g.ReasonCodes, ", ") + "); governance_verify cannot pass", nil, nil
			}
		}
		return false, core.ReasonStandardFlowVerificationIncm, "no truth_integrity evaluation on record; run `beacon gate integrity` first (governance_verify)", nil, nil
	case FlowStepAdmission:
		// Fact: the AC-142 admission bundle passes at the
		// implement entry (the bundle is the superset of the
		// admission five-plus-two; evaluating it here persists
		// the entry evidence, pass or blocked alike).
		bundle, berr := e.admissionBundleForEntry(run, m)
		if berr != nil {
			return false, "", "", nil, berr
		}
		if !bundle.AllowImplement {
			return false, bundle.ReasonCode, "admission bundle blocked at workspace_admission: " + bundle.ReasonCode, &bundle, nil
		}
		return true, "", "admission bundle passes (five verification classes; gate_result " + bundle.GateResultID + ")", &bundle, nil
	case FlowStepResume:
		// Fact: the shared resume decision hands out a
		// load_next_skill conclusion (P2a decideResume).
		rv := decideResume(run.ID, m)
		if rv.ResumeAction == core.ResumeLoadNextSkill {
			return true, "", "resume decision: " + rv.NextAction, nil, nil
		}
		code := rv.ReasonCode
		if code == "" {
			code = core.ReasonStandardFlowOrderViolation
		}
		return false, code, "resume decision is " + rv.ResumeAction + ", not load_next_skill; implement_resume cannot be entered", nil, nil
	}
	return false, core.ReasonStandardFlowOrderViolation, "unknown standard flow step " + step, nil, nil
}

// FlowAdvance advances the run's standard flow by exactly one
// named step — which must be the next pending step, with its
// criterion holding. Outcomes: advanced (event + metadata),
// already_done (idempotent repeat: no writes), or blocked (the
// view carries the code; an error with the same code is returned
// so CLI callers exit non-zero, mirroring goal admit).
func (e *Engine) FlowAdvance(runID, step string) (FlowView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return FlowView{}, err
	}
	view := e.flowView(run, m, slug)
	view.Attempted = step
	if flowIndex(step) < 0 {
		view.Blocked, view.ReasonCode = true, core.ReasonStandardFlowOrderViolation
		view.Detail = "unknown standard flow step " + step + " (table: " + strings.Join(FlowSteps, " -> ") + ")"
		return view, core.NewError(core.ReasonStandardFlowOrderViolation, "goal.FlowAdvance", view.Detail)
	}
	if _, done := m.Flow.done(step); done {
		view.AlreadyDone = true
		view.Detail = "step " + step + " already completed; nothing re-recorded (idempotent)"
		return view, nil
	}
	if view.NextStep != step {
		view.Blocked, view.ReasonCode = true, core.ReasonStandardFlowOrderViolation
		view.Detail = "standard flow order violation: next pending step is " + view.NextStep +
			"; " + step + " may not skip it (AC-138 fail-closed)"
		return view, core.NewError(core.ReasonStandardFlowOrderViolation, "goal.FlowAdvance", view.Detail)
	}
	ok, code, detail, bundle, err := e.flowCriterion(run, m, slug, step)
	if err != nil {
		return FlowView{}, err
	}
	if bundle != nil {
		view.Bundle = bundle
	}
	if !ok {
		view.Blocked, view.ReasonCode, view.Detail = true, code, detail
		return view, core.NewError(code, "goal.FlowAdvance", detail)
	}
	if m.Flow == nil {
		m.Flow = &FlowState{}
	}
	m.Flow.Completed = append(m.Flow.Completed, FlowStepRecord{Step: step, At: nowISO(), Detail: detail})
	m.Flow.UpdatedAt = nowISO()
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, EventFlowStepCompleted, map[string]any{
			"step": step, "detail": detail, "completed_count": len(m.Flow.Completed),
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return FlowView{}, err
	}
	view = e.flowView(run, m, slug)
	view.Attempted, view.Advanced, view.Detail = step, true, detail
	if bundle != nil {
		view.Bundle = bundle
	}
	return view, nil
}
