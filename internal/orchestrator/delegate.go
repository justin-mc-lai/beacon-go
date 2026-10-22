// Delegate-ralph (P2c, G-10): the implement stage, packed whole and
// handed to the ralph loop; goal only dispatches and collects.
//
// v1's delegate_implement_to_ralph merely annotated the run
// (implement_delegate = "ralph") and let the host drive the loop.
// The Go surface keeps that division of labour — the ralph executor
// itself is a P3 real integration — but makes the pack and the
// collection explicit and checkable:
//
//   - DelegateRalph packs the stage: requirement-package pointers,
//     the stage plan, the shared admission verdict (the packer calls
//     the same checkAdmission as Tick — a stage that may not start
//     may not be delegated either), the acceptance rule the return
//     will be collected by, and the rollback requirements. The pack
//     is recorded on the run (metadata + implement_delegated event);
//     the run is NOT advanced or completed by delegating.
//   - ReceiveRalphResult collects the return under the shared
//     harvest rule (lanes.go): artifact pointers plus implement
//     evidence produced AFTER the delegation went out. An unwhole
//     return is rejected with reason codes and the delegation stays
//     out; a returned delegation never completes the run by itself.
//
// Simulation honesty: with Simulate=true the engine runs the ralph
// lane executor (or a RecordingExecutor) so the flow can be exercised
// end to end; the delegation, its events and every output carry
// simulated=true, and the executor remains a placeholder that
// performs no real work.
package orchestrator

import (
	"context"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// AcceptanceRuleText is packed into every delegation: the rule the
// return will be collected by, stated up front.
var AcceptanceRuleText = []string{
	"artifact pointers must land in artifact_refs (absolute paths)",
	"a stage_executed evidence event for the implement stage must be produced after the delegation",
	"the qa stage exit still requires a passing qa gate or artifact evidence (P2a exit gates)",
	"returning a delegation never completes the run: release needs human confirmation, an attached verifier and a passing qa gate",
}

// RollbackRuleText is packed into every delegation.
var RollbackRuleText = []string{
	"work only inside the bound worktree; never write truth on the main line from a satellite",
	"on failure, stop and report a reason_code — never mark the stage done",
	"leave the workspace clean: no half-applied changes outside the bound worktree",
}

// featureContextOf resolves the project/version/feature context of a
// run's feature from the store (for package paths).
func (e *Engine) featureContextOf(featureID string) (store.FeatureContext, bool, error) {
	ctxs, err := e.store.ListFeatureContexts("")
	if err != nil {
		return store.FeatureContext{}, false, err
	}
	for _, c := range ctxs {
		if c.Feature.ID == featureID {
			return c, true, nil
		}
	}
	return store.FeatureContext{}, false, nil
}

// buildDelegationPackage assembles the pack for one run.
func (e *Engine) buildDelegationPackage(run store.Run, m *runMeta, slug string, adm AdmissionResult) DelegationPackage {
	pkg := DelegationPackage{
		FeatureSlug: slug, Lane: LaneRalph, StagePlan: m.StagePlan,
		WorktreeRef: adm.WorktreeRef, Admission: adm,
		Acceptance: AcceptanceRuleText, Rollback: RollbackRuleText,
	}
	if fc, ok, err := e.featureContextOf(run.FeatureID); err == nil && ok {
		pkg.PackageRoot = filepath.Join(fc.Project.RootPath, fc.Version.DocsRoot, "features", fc.Feature.Slug)
		pkg.TruthPath = joinPkgPath(pkg.PackageRoot, fc.Feature.TruthPath)
		pkg.TestsPath = joinPkgPath(pkg.PackageRoot, fc.Feature.TestsPath)
	}
	return pkg
}

// joinPkgPath resolves a feature-package-relative path against the
// package root (already-absolute paths pass through).
func joinPkgPath(root, p string) string {
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}

// DelegateRalph packs the run's implement stage and records the
// delegation. The run must be AT the implement stage, admission must
// pass, and no other delegation may be out. With simulate=true the
// ralph executor runs immediately (placeholder; labelled simulated).
func (e *Engine) DelegateRalph(ctx context.Context, runID string, simulate bool) (DelegationView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DelegationView{}, err
	}
	if m.currentStage() != StageImplement {
		return DelegationView{}, core.NewError(core.ReasonDelegationStageMismatch, "goal.DelegateRalph",
			"run "+runID+" is at stage "+m.currentStage()+", not implement")
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	if active := m.Implement.activeDelegation(); active != nil {
		return DelegationView{}, core.NewError(core.ReasonDelegationAlreadyActive, "goal.DelegateRalph",
			"delegation "+active.ID+" is still out")
	}
	bundle, err := e.admissionBundleForEntry(run, m)
	if err != nil {
		return DelegationView{}, err
	}
	adm := bundle.Admission
	if !bundle.AllowImplement {
		_ = e.appendEvent(run, "implement_delegation_blocked", map[string]any{
			"lane": LaneRalph, "reason_code": bundle.ReasonCode, "reason_codes": bundle.ReasonCodes,
			"bundle_gate_result_id": bundle.GateResultID,
		})
		return DelegationView{}, core.NewError(bundle.ReasonCode, "goal.DelegateRalph",
			"implement admission bundle blocked the delegation (AC-142)")
	}
	dec := LaneDecision{Lane: LaneRalph, Source: LaneSourceExplicit,
		Reason: "delegate-ralph explicitly routes the implement stage to the ralph lane"}
	if err := e.routeLane(run, m, dec); err != nil {
		return DelegationView{}, err
	}
	d := Delegation{
		ID: newWatchID("dlg-"), Lane: LaneRalph, Status: DelegationDelegated,
		Simulated: simulate, CreatedAt: nowISO(),
		Package: e.buildDelegationPackage(run, m, slug, adm),
	}
	m.Implement.Delegations = append(m.Implement.Delegations, d)
	if err := e.appendEvent(run, "implement_delegated", map[string]any{
		"delegation_id": d.ID, "lane": LaneRalph, "simulated": simulate,
		"truth_path": d.Package.TruthPath, "worktree_ref": d.Package.WorktreeRef,
	}); err != nil {
		return DelegationView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return DelegationView{}, err
	}
	if simulate {
		if err := e.simulateDelegationExecution(ctx, runID, d.ID); err != nil {
			return DelegationView{}, err
		}
	}
	// Reload so the view reflects any simulated execution outcome.
	run, m, slug, err = e.loadRun(runID)
	if err != nil {
		return DelegationView{}, err
	}
	got := m.Implement.findDelegation(d.ID)
	if got == nil {
		return DelegationView{}, core.NewError(core.ReasonDelegationNotFound, "goal.DelegateRalph", "delegation vanished")
	}
	return DelegationView{RunID: runID, Delegation: *got, Run: viewOf(run, m, slug)}, nil
}

// simulateDelegationExecution runs the ralph lane executor (or a
// recording placeholder) for the delegated stage and stores the
// outcome on the delegation as a pending simulated result. It never
// advances the run.
func (e *Engine) simulateDelegationExecution(ctx context.Context, runID, delegationID string) error {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return err
	}
	ex, ok := e.LaneExecutor(LaneRalph)
	if !ok {
		ex = NewRecordingExecutor(e.store, "ralph-simulated")
	}
	outcome, exErr := ex.ExecuteStage(ctx, viewOf(run, m, slug), StageImplement)
	if exErr != nil {
		outcome = StageOutcome{Status: "failed", ReasonCode: ReasonExecutorError, Summary: exErr.Error()}
	}
	d := m.Implement.findDelegation(delegationID)
	if d == nil {
		return core.NewError(core.ReasonDelegationNotFound, "goal.simulateDelegationExecution", delegationID)
	}
	d.ResultSummary = "simulated: " + outcome.Summary
	if outcome.Status != "done" {
		d.Status = DelegationFailed
		d.ReasonCodes = []string{outcome.ReasonCode}
	}
	_ = e.appendEvent(run, "implement_delegation_simulated", map[string]any{
		"delegation_id": delegationID, "executor": ex.ID(), "simulated": true,
		"status": outcome.Status, "reason_code": outcome.ReasonCode,
	})
	return e.saveRun(run, m)
}

// ReceiveRalphResult collects a delegation's return under the shared
// harvest rule. A failed return is recorded as failed without a
// harvest; a done return must bring artifact pointers AND implement
// evidence produced after the delegation went out — otherwise it is
// rejected (delegation stays out) with the missing rules' codes.
// Receiving never advances or completes the run.
func (e *Engine) ReceiveRalphResult(runID, delegationID string, in LaneResultInput) (DelegationView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DelegationView{}, err
	}
	d := m.Implement.findDelegation(delegationID)
	if d == nil {
		return DelegationView{}, core.NewError(core.ReasonDelegationNotFound, "goal.ReceiveRalphResult",
			"no delegation "+delegationID+" on run "+runID)
	}
	view := func() DelegationView {
		return DelegationView{RunID: runID, Delegation: *d, Run: viewOf(run, m, slug)}
	}
	if d.Status != DelegationDelegated {
		return view(), core.NewError(core.ReasonDelegationNotActive, "goal.ReceiveRalphResult",
			"delegation "+delegationID+" is "+d.Status+", not delegated")
	}
	if in.Status == "failed" {
		d.Status = DelegationFailed
		d.ReturnedAt = nowISO()
		d.ResultSummary = in.Summary
		_ = e.appendEvent(run, "implement_delegation_failed", map[string]any{
			"delegation_id": delegationID, "summary": in.Summary, "simulated": d.Simulated,
		})
		if err := e.saveRun(run, m); err != nil {
			return DelegationView{}, err
		}
		return view(), nil
	}
	ids, err := e.registerArtifacts(runID, in.Artifacts)
	if err != nil {
		return DelegationView{}, err
	}
	anchor, err := e.delegationAnchor(runID, delegationID)
	if err != nil {
		return DelegationView{}, err
	}
	_, codes, err := e.validateHarvest(runID, anchor)
	if err != nil {
		return DelegationView{}, err
	}
	if len(codes) > 0 {
		_ = e.appendEvent(run, "implement_result_rejected", map[string]any{
			"delegation_id": delegationID, "reason_codes": codes, "simulated": d.Simulated,
		})
		return view(), core.NewError(codes[0], "goal.ReceiveRalphResult",
			"delegation return harvest incomplete: "+strings.Join(codes, ", "))
	}
	d.Status = DelegationReturned
	d.ReturnedAt = nowISO()
	d.ResultSummary = in.Summary
	_ = e.appendEvent(run, "implement_delegation_returned", map[string]any{
		"delegation_id": delegationID, "artifact_ids": ids, "simulated": d.Simulated,
		"summary": in.Summary,
	})
	if err := e.saveRun(run, m); err != nil {
		return DelegationView{}, err
	}
	return view(), nil
}
