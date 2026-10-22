// Delegate-ralph REAL path (P3c, G-10): when a ralph command is
// configured, DelegateRalph runs the REAL loop instead of the P2c
// simulation, and its return is collected by the SAME
// ReceiveRalphResult harvest as any other return (artifact pointers
// + post-delegation implement evidence). The simulated path
// (DelegateRalph with simulate=true) is untouched and stays labelled
// simulated on every surface; the real path is labelled simulated=
// false everywhere and its delegation records carry the loop's real
// evidence (per-iteration stage_executed events from RalphExecutor).
package orchestrator

import (
	"context"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// DelegateRalphReal packs the implement stage exactly like
// DelegateRalph (same admission, same pack, same one-active rule)
// and then runs the configured ralph loop for real. It exists
// alongside DelegateRalph because the P2c entry point's contract
// (simulate flag semantics) is load-bearing for its tests; this is
// the P3c entry the CLI uses when --ralph-cmd is configured.
//
// Collection: on loop completion the loop's own capture artifacts
// (stdout/stderr of every iteration, registered by the executor)
// are handed to ReceiveRalphResult as the return's artifact set —
// the harvest then validates them like any external return. A loop
// that fails or exhausts its iterations marks the delegation failed
// with the loop's reason code; nothing is half-collected.
func (e *Engine) DelegateRalphReal(ctx context.Context, runID string, argv []string, timeoutSec, maxIter int) (DelegationView, error) {
	if len(argv) == 0 {
		return DelegationView{}, core.NewError(core.ReasonRalphConfigMissing, "goal.DelegateRalphReal",
			"no ralph command configured; the real path never falls back to simulation")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DelegationView{}, err
	}
	if m.currentStage() != StageImplement {
		return DelegationView{}, core.NewError(core.ReasonDelegationStageMismatch, "goal.DelegateRalphReal",
			"run "+runID+" is at stage "+m.currentStage()+", not implement")
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	if active := m.Implement.activeDelegation(); active != nil {
		return DelegationView{}, core.NewError(core.ReasonDelegationAlreadyActive, "goal.DelegateRalphReal",
			"delegation "+active.ID+" is still out")
	}
	bundle, err := e.admissionBundleForEntry(run, m)
	if err != nil {
		return DelegationView{}, err
	}
	adm := bundle.Admission
	if !bundle.AllowImplement {
		_ = e.appendEvent(run, "implement_delegation_blocked", map[string]any{
			"lane": LaneRalph, "reason_code": bundle.ReasonCode, "reason_codes": bundle.ReasonCodes, "real": true,
			"bundle_gate_result_id": bundle.GateResultID,
		})
		return DelegationView{}, core.NewError(bundle.ReasonCode, "goal.DelegateRalphReal",
			"implement admission bundle blocked the delegation (AC-142)")
	}
	dec := LaneDecision{Lane: LaneRalph, Source: LaneSourceExplicit,
		Reason: "delegate-ralph with a configured ralph command routes the implement stage to the real ralph loop"}
	if err := e.routeLane(run, m, dec); err != nil {
		return DelegationView{}, err
	}
	d := Delegation{
		ID: newWatchID("dlg-"), Lane: LaneRalph, Status: DelegationDelegated,
		Simulated: false, CreatedAt: nowISO(),
		Package: e.buildDelegationPackage(run, m, slug, adm),
	}
	m.Implement.Delegations = append(m.Implement.Delegations, d)
	if err := e.appendEvent(run, "implement_delegated", map[string]any{
		"delegation_id": d.ID, "lane": LaneRalph, "simulated": false, "real": true,
		"truth_path": d.Package.TruthPath, "worktree_ref": d.Package.WorktreeRef,
		"ralph_command": argv[0],
	}); err != nil {
		return DelegationView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return DelegationView{}, err
	}
	// Run the REAL loop as the ralph lane executor for this run.
	loop := NewRalphExecutor(e, argv, timeoutSec, maxIter)
	outcome, exErr := loop.ExecuteStage(ctx, viewOf(run, m, slug), StageImplement)
	if exErr != nil {
		return DelegationView{}, exErr
	}
	// Reload: the loop appended evidence and checkpoints.
	run, m, slug, err = e.loadRun(runID)
	if err != nil {
		return DelegationView{}, err
	}
	got := m.Implement.findDelegation(d.ID)
	if got == nil {
		return DelegationView{}, core.NewError(core.ReasonDelegationNotFound, "goal.DelegateRalphReal", "delegation vanished")
	}
	if outcome.Status != "done" {
		got.Status = DelegationFailed
		got.ReturnedAt = nowISO()
		got.ResultSummary = outcome.Summary
		if outcome.ReasonCode != "" {
			got.ReasonCodes = []string{outcome.ReasonCode}
		}
		_ = e.appendEvent(run, "implement_delegation_failed", map[string]any{
			"delegation_id": d.ID, "summary": outcome.Summary, "simulated": false,
			"reason_code": outcome.ReasonCode,
		})
		if err := e.saveRun(run, m); err != nil {
			return DelegationView{}, err
		}
		return DelegationView{RunID: runID, Delegation: *got, Run: viewOf(run, m, slug)}, nil
	}
	// Collect through the SHARED harvest. The loop's capture
	// artifacts are already in artifact_refs (the executor registered
	// them per iteration), so the return set adds no new pointers —
	// validateHarvest judges exactly those captures plus the
	// post-delegation implement evidence, like any external return.
	return e.ReceiveRalphResult(runID, d.ID, LaneResultInput{
		Status: "done", Summary: outcome.Summary,
	})
}

// RalphConfigured reports whether a ralph command is configured on
// the run's executor config (used by CLI surfaces to pick the real
// vs simulated delegate path — explicitly, never silently).
func RalphConfigured(cfg *ExecutorConfig) bool {
	return cfg != nil && len(cfg.RalphArgv) > 0
}

var _ = harness.DoneMarker // the completion convention lives in harness
