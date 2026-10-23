// Implement admission (P2c, G-04): the checker itself.
//
// v1's ensure_goal_implement_admission delegates to ONE shared
// fail-closed pre-write invariant (ensure_implement_admission) so
// goal, direct, team and ralph can never drift into separate
// judgements. This is that one checker for the Go engine: Tick (when
// it is about to execute the implement stage), DelegateRalph (when it
// packs the stage) and the `goal admit` surface all call
// checkAdmission — there is no second implementation.
//
// Five items, each with its own reason code; any failure closes the
// gate and the headline reason is the first failing item's code:
//  1. truth_frozen      — the requirement truth must be frozen,
//     by the ONE shared determination (Engine.FrozenFact,
//     frozen_fact.go: pending change -> not frozen; the document's
//     frontmatter decides when it speaks; a formal truth_freeze
//     pass corroborates only when the document is silent; the
//     lifecycle snapshot never decides). fill != self-sign, so an
//     unfrozen package reports freeze_ack_required (v1's goal
//     surface code) and a pending change reports
//     truth_change_pending. S1 (AC-139) adds the baseline leg:
//     on a git project a frozen body that is not a COMMITTED
//     baseline on the canonical branch is not consumable and
//     reports canonical_refreeze_not_committed; non-git projects
//     keep the P4b caliber (frontmatter is the authority).
//  2. worktree_binding  — the run must be bound to the workstation
//     its parallel plan assigns (satellite path in pool mode); with
//     no plan the binding is the project root itself (single_worktree
//     default) and always exists.
//  3. design_bound      — only when the run requires design (UX):
//     the design primitives must be bound (G-18); unbound never
//     starts implementing (charter 3.4).
//  4. blocker_streak    — a hard-blocked run (streak >= 3) is not
//     admitted until a human deals with the blocker. A streak built
//     by the repair loop itself (stage failures being retried under
//     the MAX_REPAIR_LOOPS ceiling, P2a) is not "uncleared" in this
//     sense — the ceiling, not admission, governs those retries —
//     so the item only fires when no repair loop is in progress.
//  5. steering_ack      — an unacknowledged direction change (G-21)
//     blocks admission: the implementer must not start on a stale
//     objective.
//  6. baseline_merged   — S2 (AC-141): when the project is a git
//     repository and the run is bound to a real worktree/branch, the
//     bound HEAD must already contain the S1 truth_baseline_commit
//     (truth_baseline_commit_missing / truth_baseline_not_merged).
//     Non-git projects and planned-only bindings report
//     not_applicable and never block.
//  7. dirty_policy      — S2 (AC-141): the bound worktree must not
//     carry dirty state in the requirement-package directory or on
//     the P4c protected-path denylist (unscoped_dirty_state).
//     Ordinary code files dirty at the pre-implement point are not
//     a violation (DERIVED caliber, baseline_check.go).
//  8. topology_gate     — S3 (AC-173): loop/implement must consume
//     the topology gate. On a git project a recorded, confirmed
//     BranchContext (AC-171) is required and its live gate
//     (AC-172) must pass: topology_gate_missing /
//     topology_gate_blocked. Non-git projects are not_applicable.
//  9. review_decision   — R2b (H5): an unresolved routed review
//     decision (P0 / user-decision / drift) refuses implement with
//     the routing's own code, until a later review routes onward.
package orchestrator

import (
	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// CheckImplementAdmission runs the shared admission checker for one
// run (read-only judgement; the verdict is not persisted).
func (e *Engine) CheckImplementAdmission(runID string) (AdmissionResult, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return AdmissionResult{}, err
	}
	return e.checkAdmission(run, m)
}

// checkAdmission is the single implementation behind the CLI
// surface, the tick path and the delegation packer.
func (e *Engine) checkAdmission(run store.Run, m *runMeta) (AdmissionResult, error) {
	slug := ""
	if feat, err := e.store.Features().GetByID(run.FeatureID); err == nil {
		slug = feat.Slug
	}
	res := AdmissionResult{
		Schema: "goal-implement-admission.v1",
		RunID:  run.ID, FeatureSlug: slug, Status: "pass",
		ExpectedWorktree: expectedWorktree(m, slug),
	}
	if m.Implement != nil {
		res.Lane, res.LaneSource = m.Implement.Lane, m.Implement.LaneSource
		res.WorktreeRef = m.Implement.WorktreeRef
	}
	fail := func(item AdmissionItem) {
		res.Items = append(res.Items, item)
		res.ReasonCodes = append(res.ReasonCodes, item.ReasonCode)
	}
	ok := func(item AdmissionItem) { res.Items = append(res.Items, item) }

	// 1. truth frozen (the shared FrozenFact determination, via
	// buildExitInput — no second derivation lives here).
	in, err := e.buildExitInput(run, m)
	if err != nil {
		return AdmissionResult{}, err
	}
	switch {
	case !in.TruthPackagePresent && in.HasFeature:
		fail(AdmissionItem{Check: "truth_frozen", Applicable: true,
			ReasonCode: core.ReasonStageExitTruthMissing,
			Detail:     "feature truth package missing; fill then freeze"})
	case in.TruthChangePending:
		fail(AdmissionItem{Check: "truth_frozen", Applicable: true,
			ReasonCode: core.ReasonTruthChangePending,
			Detail:     "a truth change is pending (unrefrozen); refreeze before implementing"})
	case in.TruthFrozen && in.TruthBaselineUncommitted:
		fail(AdmissionItem{Check: "truth_frozen", Applicable: true,
			ReasonCode: core.ReasonCanonicalRefreezeNotCommitted,
			Detail:     "truth is frozen but not committed on the canonical branch; commit it and run `beacon truth baseline` before implementing (AC-139)"})
	case !in.TruthFrozen:
		fail(AdmissionItem{Check: "truth_frozen", Applicable: true,
			ReasonCode: core.ReasonFreezeAckRequired,
			Detail:     "truth exists but is not frozen (fill != self-sign)"})
	default:
		ok(AdmissionItem{Check: "truth_frozen", OK: true, Applicable: true,
			Detail: "truth frozen (source: " + in.FrozenSource + ")"})
	}

	// 2. worktree binding: the run's bound ref must be the workstation
	// the plan assigns. No plan -> the project root itself (".") is
	// the binding by definition (single_worktree); with a plan, the
	// binding was recorded when the plan was applied, so an empty or
	// different ref is a real failure.
	bound := res.WorktreeRef
	if bound == "" && m.ParallelPlan == nil {
		bound = "."
		res.WorktreeRef = bound
	}
	switch {
	case bound == "":
		fail(AdmissionItem{Check: "worktree_binding", Applicable: true,
			ReasonCode: core.ReasonAdmissionWorktreeMiss,
			Detail:     "no worktree bound; expected " + res.ExpectedWorktree})
	case bound != res.ExpectedWorktree:
		fail(AdmissionItem{Check: "worktree_binding", Applicable: true,
			ReasonCode: core.ReasonAdmissionWorktreeMism,
			Detail:     "bound worktree " + bound + " != plan workstation " + res.ExpectedWorktree})
	default:
		ok(AdmissionItem{Check: "worktree_binding", OK: true, Applicable: true,
			Detail: "bound to " + bound})
	}

	// 3. design bound (only when the run requires design).
	if designRequired(m) {
		if designBound(m) {
			ok(AdmissionItem{Check: "design_bound", OK: true, Applicable: true,
				Detail: "primitives bound: " + m.DesignBinding.PrimitiveSet})
		} else {
			fail(AdmissionItem{Check: "design_bound", Applicable: true,
				ReasonCode: core.ReasonDesignPrimitiveNotBound,
				Detail:     "UX run has no design primitive binding; bind before implementing"})
		}
	} else {
		ok(AdmissionItem{Check: "design_bound", OK: true, Applicable: false,
			Detail: "run requires no design (no --ux)"})
	}

	// 4. blocker streak clear of the hard-block line (outside an
	// active repair loop — see the package comment).
	if m.BlockedStreak >= 3 && m.RepairLoopCount == 0 {
		fail(AdmissionItem{Check: "blocker_streak", Applicable: true,
			ReasonCode: ReasonBlockedStreak,
			Detail:     "run is hard-blocked (blocked streak >= 3, no repair loop in progress)"})
	} else {
		ok(AdmissionItem{Check: "blocker_streak", OK: true, Applicable: true, Detail: "no uncleared hard block"})
	}

	// 5. no unacknowledged steering.
	if p := m.pendingSteering(); p != nil {
		fail(AdmissionItem{Check: "steering_ack", Applicable: true,
			ReasonCode: core.ReasonSteeringUnacknowledged,
			Detail:     "unacknowledged steering " + p.ID + " (" + p.Kind + ") must be surfaced and acked first"})
	} else {
		ok(AdmissionItem{Check: "steering_ack", OK: true, Applicable: true, Detail: "no pending steering"})
	}

	// 6+7. S2 (AC-141): baseline-merged + dirty policy over the
	// bound worktree (baseline_check.go — the same judgement the
	// admission bundle's branch-governance and diff items reuse).
	merged, _, err := e.evalBaselineMerged(run, m, slug)
	if err != nil {
		return AdmissionResult{}, err
	}
	if merged.OK || !merged.Applicable {
		ok(merged)
	} else {
		fail(merged)
	}
	dirty, err := e.evalDirtyPolicy(run, m, slug)
	if err != nil {
		return AdmissionResult{}, err
	}
	if dirty.OK || !dirty.Applicable {
		ok(dirty)
	} else {
		fail(dirty)
	}

	// 8. topology gate (S3, AC-173): on a git project the run's
	// feature must have a recorded, confirmed BranchContext whose
	// live topology gate passes — missing ref reports
	// topology_gate_missing, a non-pass gate topology_gate_blocked.
	// Non-git projects with no projection are not_applicable. The
	// judgement itself is the shared evalTopologyGateItem (the
	// bundle's branch-governance item consumes the same function).
	topo, _, _, err := e.evalTopologyGateItem(run)
	if err != nil {
		return AdmissionResult{}, err
	}
	if topo.OK || !topo.Applicable {
		ok(topo)
	} else {
		fail(topo)
	}

	// 9. review decision clear (R2b, H5): an unresolved routed
	// review block (record_review_decision's error_recovery /
	// drift_routing outcome) refuses implement with the routing's
	// own reason code — a P0 the reviewers found must stop the
	// writing hand, not just sit in a report.
	if rd := m.ReviewDecision; rd != nil && !rd.Resolved {
		fail(AdmissionItem{Check: "review_decision", Applicable: true,
			ReasonCode: rd.ReasonCode,
			Detail:     "unresolved review decision (" + rd.NextStage + ") from " + rd.Source + "; resolve it before implementing"})
	} else {
		ok(AdmissionItem{Check: "review_decision", OK: true, Applicable: true, Detail: "no unresolved review decision"})
	}

	if len(res.ReasonCodes) > 0 {
		res.Status = "blocked"
		res.Blocked = true
		res.AllowImplement = false
		res.ReasonCode = res.ReasonCodes[0]
	} else {
		res.AllowImplement = true
		res.ReasonCode = core.ReasonGoalImplementAdmissOK
	}
	return res, nil
}
