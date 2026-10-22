// Hotfix fast lane (P2d, G-13): prod_ops_v168.py's build_hotfix_plan
// + validate_hotfix_for_release, folded onto a goal run.
//
// Differences from the source, deliberately (see the p2d report):
// the v1 builder auto-defaults a missing baseline (pins the current
// runtime release line) and a missing forward port (main) and still
// emits a plan dict with blocked=true. The Go surface refuses to
// emit a plan at all when a required piece is missing — a plan that
// cannot be executed is not a plan — and it never guesses a
// baseline: the caller names the version to fix from. The rollback
// is required at build time, not just at release validation.
//
// The fast lane never skips the human release: the plan carries
// requires_human_release=true structurally and Engine.Complete is
// untouched by it.
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
)

// BuildHotfixPlan validates the request and records the plan on the
// run (metadata + hotfix_plan_created event). Missing rollback,
// baseline, forward target or fix points refuse the build, each
// with the family's reason code.
func (e *Engine) BuildHotfixPlan(runID string, in HotfixPlanInput) (HotfixPlan, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return HotfixPlan{}, err
	}
	refuse := func(code, detail string) (HotfixPlan, error) {
		return HotfixPlan{}, core.NewError(code, "goal.BuildHotfixPlan", detail)
	}
	switch {
	case strings.TrimSpace(in.Description) == "":
		return refuse(core.ReasonHotfixDescriptionReq, "a hotfix plan needs the incident description")
	case strings.TrimSpace(in.FromVersion) == "":
		return refuse(core.ReasonHotfixBaselineMissing,
			"hotfix without baseline: name the version to fix from (I-HF-001)")
	case strings.TrimSpace(in.TargetVersion) == "":
		return refuse(core.ReasonHotfixForwardPortReq,
			"hotfix without a forward-port target: name where the fix must also land (I-HF-002)")
	case len(in.FixPoints) == 0:
		return refuse(core.ReasonHotfixFixPointsReq, "a hotfix plan needs at least one fix point")
	case strings.TrimSpace(in.RollbackPlan) == "":
		return refuse(core.ReasonHotfixRollbackReq,
			"hotfix without rollback notes: write how the fix is rolled back before planning it")
	}
	plan := HotfixPlan{
		Schema: "hotfix-plan.v1", RunID: runID,
		Description: in.Description, FromVersion: in.FromVersion,
		BaselineRef: in.FromVersion, TargetVersion: in.TargetVersion,
		ForwardPortTarget: in.TargetVersion, FixPoints: in.FixPoints,
		RollbackPlan:               in.RollbackPlan,
		RollbackDeclaredExecutable: in.RollbackDeclaredExecutable,
		ReleaseProfile:             "hotfix",
		RequiredGates:              HotfixRequiredGates,
		RequiresHumanRelease:       true,
		CreatedAt:                  nowRFC(),
	}
	m.Hotfix = &plan
	if err := e.appendEvent(run, "hotfix_plan_created", map[string]any{
		"from_version": plan.FromVersion, "target_version": plan.TargetVersion,
		"fix_points": plan.FixPoints, "requires_human_release": true,
	}); err != nil {
		return HotfixPlan{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return HotfixPlan{}, err
	}
	return plan, nil
}

// ValidateHotfixForRelease checks the run's recorded hotfix plan
// for release: plan completeness (baseline / forward target /
// rollback present — validate_hotfix_for_release's three checks),
// the rollback declared executable, and the run's feature qa gate
// at pass (the minimal_qa required gate, judged by the same
// gate_results the rest of the engine reads). The human_release
// gate is NOT checked off here: requires_human_release stays true
// and Engine.Complete still demands the human confirmation.
func (e *Engine) ValidateHotfixForRelease(runID string) (HotfixValidation, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return HotfixValidation{}, err
	}
	v := HotfixValidation{
		RunID: runID, ReleaseProfile: "hotfix", RequiresHumanRelease: true,
	}
	if m.Hotfix == nil {
		v.ReasonCodes = []string{core.ReasonHotfixPlanMissing}
		v.Blocked = true
		return v, nil
	}
	plan := m.Hotfix
	if strings.TrimSpace(plan.BaselineRef) == "" {
		v.ReasonCodes = append(v.ReasonCodes, core.ReasonHotfixBaselineMissing)
	}
	if strings.TrimSpace(plan.ForwardPortTarget) == "" {
		v.ReasonCodes = append(v.ReasonCodes, core.ReasonHotfixForwardPortReq)
	}
	if strings.TrimSpace(plan.RollbackPlan) == "" {
		v.ReasonCodes = append(v.ReasonCodes, core.ReasonHotfixRollbackReq)
	}
	if !plan.RollbackDeclaredExecutable {
		v.ReasonCodes = append(v.ReasonCodes, core.ReasonHotfixRollbackNotExec)
	}
	if g, err := e.store.Gates().Latest(run.FeatureID, "qa"); err == nil {
		v.QAGateVerdict = g.Verdict
	}
	if v.QAGateVerdict != "pass" {
		v.ReasonCodes = append(v.ReasonCodes, ReasonQAGateNotPassed)
	}
	v.Blocked = len(v.ReasonCodes) > 0
	v.OK = !v.Blocked
	return v, nil
}
