// Unified goal reason-code vocabulary (P2b, G-21): the Go port of
// beacon/utils/goal_reason_codes.py plus the reason constants of the
// goal guardrail family (goal_agent_driver.py, goal_durable_supervisor.py,
// goal_queue.py, goal_budget.py, goal_quota.py, goal_todos.py).
//
// Rule (charter): every goal-surface reason string lives here as a core
// constant. Orchestrator / CLI code must reference these constants, never
// re-spell the strings locally. Values are API-stable contract strings,
// copied verbatim from the v1 sources.
package core

import "strings"

// ---- Stage exit family (goal_reason_codes.py) ----

const (
	ReasonStageExitOK              = "stage_exit_ok"
	ReasonStageExitPlanIncomplete  = "stage_exit_plan_incomplete"
	ReasonStageExitTruthNotFrozen  = "stage_exit_truth_not_frozen"
	ReasonStageExitTruthMissing    = "stage_exit_truth_package_missing"
	ReasonStageExitDesignIncompl   = "stage_exit_design_incomplete"
	ReasonStageExitImplementBlock  = "stage_exit_implement_blocked"
	ReasonStageExitQAEvidenceMiss  = "stage_exit_qa_evidence_missing"
	ReasonStageExitBlocked         = "stage_exit_blocked"
	ReasonFreezeAckRequired        = "freeze_ack_required"
	ReasonGoalImplementFeatReq     = "goal_implement_feature_required"
	ReasonGoalImplementAdmissBlock = "goal_implement_admission_blocked"
	ReasonGoalImplementAdmissOK    = "goal_implement_admission_ok"
	ReasonMainImplementForbidden   = "main_implement_forbidden"
	ReasonMainDocsPolicyForbidden  = "main_docs_policy_forbidden"
	ReasonMainImplementExcAllowed  = "main_implement_exception_allowed"
)

// ReasonCatalog maps the stage-exit / admission family to the
// user-facing descriptions of goal_reason_codes.REASON_CATALOG.
var ReasonCatalog = map[string]string{
	ReasonStageExitOK:             "Stage exit verifier passed",
	ReasonStageExitPlanIncomplete: "Plan/journey incomplete; cannot leave plan",
	ReasonStageExitTruthNotFrozen: "Feature truth exists but status is not frozen",
	ReasonStageExitTruthMissing:   "Feature truth package missing; fill then freeze",
	ReasonStageExitDesignIncompl:  "Design stage lacks verifiable design evidence",
	ReasonStageExitImplementBlock: "Implement admission failed",
	ReasonStageExitQAEvidenceMiss: "QA lacks scorecard/evidence artifact",
	ReasonReleaseRequiresHuman:    "Release requires human confirmation",
	ReasonFreezeAckRequired:       "Freeze ack required before implement (fill ≠ self-sign)",
	ReasonMainImplementForbidden:  "Business path commit on main/master forbidden",
	ReasonMainDocsPolicyForbidden: "Docs path outside allowed main docs policy",
}

// ---- Driver family (goal_agent_driver.py) ----

const (
	ReasonDriverLockHeld      = "driver_lock_held"
	ReasonDriverDualStart     = "agent_driver_dual_start_forbidden"
	ReasonHostBridgeMissing   = "host_bridge_missing"
	ReasonDriverBudgetExhaust = "agent_driver_budget_exhausted"
	ReasonDriverUserStop      = "agent_driver_user_stop"
	ReasonDriverNotStarted    = "driver_not_started"
	ReasonDriverAwaitingHuman = "driver_awaiting_human"
	ReasonDriverGoalTerminal  = "driver_goal_terminal"
	ReasonDriverLeaseTakeover = "driver_lease_takeover"
	ReasonNotAGoalRun         = "not_a_goal_run"
)

// ---- Supervisor family (goal_durable_supervisor.py) ----

const (
	ReasonHostInterrupted       = "host_interrupted"
	ReasonSupervisorNotAttached = "supervisor_not_attached"
	ReasonSupervisorStall       = "supervisor_stall_detected"
)

// ---- Queue family (goal_queue.py) ----

const (
	ReasonLoopCannotComplete = "loop_cannot_mark_product_complete"
	ReasonGoalQueueEmpty     = "goal_queue_empty"
)

// ---- Budget / quota family (goal_budget.py, goal_quota.py) ----

const (
	ReasonSpendWithoutWriteback  = "spend_without_writeback"
	ReasonSpendWithoutValidWB    = "spend_without_validated_writeback"
	ReasonQuietSkipNoSpend       = "quiet_skip_no_spend"
	ReasonSpendIdempotencyConf   = "spend_idempotency_conflict"
	ReasonSpendSourceNotAllowed  = "spend_source_not_allowed"
	ReasonProgramBudgetExhausted = "program_drive_budget_exhausted"
	ReasonQuotaExhausted         = "quota_exhausted"
	ReasonBudgetExhausted        = "budget_exhausted"
)

// ---- Todo family (goal_todos.py) ----

const (
	ReasonTodoMissing     = "todo_missing"
	ReasonTodoDoubleClaim = "double_claim"
	ReasonTodoDone        = "todo_done"
	ReasonTodoNotClaimed  = "todo_not_claimed"
	ReasonTodoNotOwner    = "not_owner"
	ReasonTodoSuccessorEx = "successor_exists"
)

// ---- Lane family (P2c, G-22: implement single/team/ralph) ----

const (
	ReasonLaneUnknown            = "lane_unknown"
	ReasonLaneStageMismatch      = "lane_stage_not_implement"
	ReasonTeamHandoffIncomplete  = "team_handoff_incomplete"
	ReasonLaneResultFailed       = "lane_result_failed"
	ReasonResultArtifactsMissing = "result_artifacts_missing"
	ReasonResultEvidenceMissing  = "result_evidence_missing"
	ReasonExecutorNotRegistered  = "stage_executor_not_registered"
	ReasonAdmissionWorktreeMiss  = "goal_implement_worktree_missing"
	ReasonAdmissionWorktreeMism  = "goal_implement_worktree_mismatch"
	ReasonSteeringUnacknowledged = "steering_unacknowledged"
)

// ---- Delegation family (P2c, G-10: delegate-ralph) ----

const (
	ReasonDelegationNotFound      = "delegation_not_found"
	ReasonDelegationNotActive     = "delegation_not_active"
	ReasonDelegationAlreadyActive = "delegation_already_active"
	ReasonDelegationStageMismatch = "delegation_stage_not_implement"
)

// ---- Parallel family (P2c, G-11: main-star / satellite) ----

const (
	ReasonParallelTouchConflict = "parallel_touch_conflict"
	ReasonSatelliteTruthWrite   = "satellite_truth_write_forbidden"
)

// ---- Design family (P2c, G-18: design primitive binding) ----

const (
	ReasonDesignPrimitiveNotBound = "design_primitive_not_bound"
	ReasonDesignStageNotRequired  = "design_stage_not_required"
	ReasonDesignPrimitiveSetReq   = "design_primitive_set_required"
)

// ---- Self-heal family (plan_goal_self_heal steps) ----

const (
	ReasonSelfHealNeedsHuman = "self_heal_requires_human_confirmation"
	ReasonSelfHealManualOnly = "self_heal_step_manual_only"
)

// ---- Program family (P2d, G-12: program graph / admission / freeze) ----
//
// Graph + freeze codes are ported from the program_* family
// (program_graph.py's feature_graph_invalid /
// feature_graph_structure_changed, program_admission.py's
// program_dependency_unclosed); the structural codes (duplicate /
// self / cycle / plan frozen / change note) are new spellings owned
// here — the v1 family spread the same conditions across payload
// error lists instead of one vocabulary.

const (
	ReasonProgramLakesMissing      = "program_lakes_missing"
	ReasonProgramLakeDuplicate     = "program_lake_duplicate"
	ReasonProgramLakeSlugRequired  = "program_lake_slug_required"
	ReasonProgramDependencyMissing = "program_dependency_missing"
	ReasonProgramDependencySelf    = "program_dependency_self_reference"
	ReasonProgramDependencyCycle   = "program_dependency_cycle"
	ReasonProgramDependencyUnclos  = "program_dependency_unclosed"
	ReasonProgramPlanMissing       = "program_plan_missing"
	ReasonProgramPlanFrozen        = "program_plan_frozen"
	ReasonProgramChangeNoteReq     = "program_change_note_required"
	ReasonProgramLakeFeatureMiss   = "program_lake_feature_missing"
	ReasonProgramLakeNotInPlan     = "program_lake_not_in_plan"
	ReasonProgramTruthNotFrozen    = "program_truth_not_frozen"
	ReasonProgramGraphInvalid      = "feature_graph_invalid"
)

// ---- Hotfix family (P2d, G-13: prod_ops_v168.py) ----
//
// The first three codes are verbatim from prod_ops_v168.py
// (REASON_HF_MISSING_BASELINE / REASON_HF_FORWARD_PORT /
// REASON_HF_ROLLBACK). The rest are new: the v1 builder defaulted
// missing fields instead of refusing, so it needed no codes for
// them (see the p2d report's difference list).

const (
	ReasonHotfixBaselineMissing = "hotfix_baseline_missing"
	ReasonHotfixForwardPortReq  = "hotfix_forward_port_required"
	ReasonHotfixRollbackReq     = "hotfix_rollback_notes_required"
	ReasonHotfixFixPointsReq    = "hotfix_fix_points_required"
	ReasonHotfixPlanMissing     = "hotfix_plan_missing"
	ReasonHotfixRollbackNotExec = "hotfix_rollback_not_executable"
	ReasonHotfixDescriptionReq  = "hotfix_description_required"
)

// ---- Multi-version family (P2d, G-14: multi_version_intent.py) ----

const (
	ReasonMultiVersionSegmentsEmpty  = "multi_version_segments_empty"
	ReasonMultiVersionInvalid        = "multi_version_version_invalid"
	ReasonMultiVersionNotMonotonic   = "multi_version_version_not_monotonic"
	ReasonMultiVersionCarryMissing   = "multi_version_carry_missing"
	ReasonMultiVersionSegmentUnknown = "multi_version_segment_unknown"
	ReasonMultiVersionDepInvalid     = "multi_version_segment_dependency_invalid"
)

// ---- Memory family (P2d, G-15: goal memory, support_advisory only) ----

const (
	ReasonMemoryQueryEmpty = "memory_query_empty"
)

// ---- Axis family (P2d, G-16: goalrun_main_axis.py) ----
//
// main_axis_missing mirrors the source's main_axis_artifact_missing;
// the drift code is the Go surface's headline for a failed validate
// (the per-item drift kinds ride in the validation view).

const (
	ReasonMainAxisMissing = "main_axis_artifact_missing"
	ReasonMainAxisDrift   = "main_axis_validation_failed"
)

// ---- Gate evaluation family (P3a: formal gate write-back) ----
//
// Codes for the beacon-gate evaluation surface: the evaluator identity
// obligation, prerequisite composition of the release gate, and the
// design binding's version consistency. Per-item codes of the four
// gates themselves reuse the families that already own the underlying
// judgement (stage-exit, admission, design, QA evidence) — one
// vocabulary, no re-spelling.

const (
	ReasonGateEvaluatorRequired    = "gate_evaluator_required"
	ReasonGateTypeUnknown          = "gate_type_unknown"
	ReasonGateRunRequired          = "gate_run_required"
	ReasonGateRunMismatch          = "gate_run_feature_mismatch"
	ReasonGateNotEvaluated         = "gate_not_evaluated"
	ReasonGatePrereqNotPassed      = "gate_prerequisite_not_passed"
	ReasonDesignBindingVersionMiss = "design_binding_version_missing"
	ReasonDesignBindingVersionMism = "design_binding_version_mismatch"
	ReasonTruthChangePending       = "truth_change_pending"
	ReasonQAStageNotExecuted       = "qa_stage_not_executed"
	ReasonQAArtifactStale          = "qa_artifact_stale"
	ReasonReleaseAwaitingComplete  = "release_awaiting_complete"
)

// ---- Truth Integrity family (P3a, AC-101: TIG) ----
//
// One code per fracture kind of the Truth Integrity Gate. The v1 TIG
// (beacon/utils/truth_integrity.py) spread the same kinds across
// finding kinds + per-finding reason strings; the Go gate binds each
// fracture kind to one stable code here.

const (
	ReasonIntegrityPackageIncomplete = "truth_integrity_package_incomplete"
	ReasonIntegrityTestsUnknownAC    = "truth_integrity_tests_unknown_ac"
	ReasonIntegrityACWithoutTC       = "truth_integrity_ac_without_tc"
	ReasonIntegritySoftTC            = "truth_integrity_soft_tc"
	ReasonIntegrityDocsOnlyTC        = "truth_integrity_docs_only_tc"
	ReasonIntegrityDeferralPending   = "truth_integrity_deferral_pending"
	ReasonIntegrityFakeDelivery      = "truth_integrity_fake_delivery"
	ReasonIntegrityEvidenceUnreg     = "truth_integrity_evidence_artifact_unregistered"
	ReasonIntegrityEvidenceMissing   = "truth_integrity_evidence_artifact_missing"
	ReasonIntegritySlugMismatch      = "truth_integrity_slug_mismatch"
	ReasonIntegrityAuditOnly         = "truth_integrity_audit_only"
)

// ---- Intent gate family (R1: ask before sign) ----
//
// The freeze gate's intent half: a package cannot be signed until the
// author has recorded the user's intent, closed every open question,
// traced every AC to an intent source, declared maturity, and written
// the domain FSM/parity materials the package's own frontmatter
// requires. These checks are structural and deterministic; the
// interaction (one question at a time) is the truth ask/answer flow.
const (
	ReasonIntentNotConfirmed     = "truth_intent_not_confirmed"
	ReasonIntentSnapshotIncomple = "truth_intent_snapshot_incomplete"
	ReasonIntentQuestionOpen     = "truth_intent_question_open"
	ReasonIntentCoverageMissing  = "truth_intent_coverage_missing"
	ReasonIntentCoverageUnknown  = "truth_intent_coverage_unknown_ac"
	ReasonIntentCoverageUntraced = "truth_intent_coverage_untraced"
	ReasonIntentFSMMissing       = "truth_intent_fsm_missing"
	ReasonIntentFSMIncomplete    = "truth_intent_fsm_incomplete"
	ReasonIntentPackageScaffold  = "truth_intent_package_scaffold"
	ReasonIntentParityMissing    = "truth_intent_parity_missing"
	ReasonTruthBaseDiverged      = "truth_base_diverged"
	ReasonTruthRebaseNotAllowed  = "truth_rebase_not_allowed"
	ReasonTruthDriftNeverFrozen  = "truth_drift_never_frozen"
)

// ---- Real-integration family (P3c: subprocess / profile / ralph /
// worktree / planner review / ACP probe) ----
//
// Codes for the P3c true-integration surfaces: the subprocess stage
// executor, agent profiles, the ralph loop's completion convention,
// real git worktrees, planner review aggregation and the ACP probe.
// They live here under the same rule as every other family: one
// vocabulary, owned by core, never re-spelled at call sites.

const (
	// Subprocess executor (AC-044).
	ReasonExecutorCommandMissing    = "executor_command_missing"
	ReasonExecutorCommandNotExec    = "executor_command_not_executable"
	ReasonExecutorNotConfigured     = "executor_not_configured"
	ReasonExecutorTimedOut          = "executor_timed_out"
	ReasonExecutorExitNonZero       = "executor_exit_nonzero"
	ReasonExecutorProfileUnknown    = "executor_profile_unknown"
	ReasonExecutorProfileIncomplete = "executor_profile_incomplete"
	ReasonExecutorWorkdirMissing    = "executor_workdir_missing"
	// Ralph loop (G-10 real integration).
	ReasonRalphMaxIterations = "ralph_max_iterations_reached"
	ReasonRalphConfigMissing = "ralph_command_not_configured"
	// Worktree (AC-099).
	ReasonWorktreeNotGitRepo     = "worktree_not_a_git_repository"
	ReasonWorktreeMainDirty      = "worktree_main_dirty"
	ReasonWorktreeExists         = "worktree_already_exists"
	ReasonWorktreeMissing        = "worktree_missing"
	ReasonWorktreeMergeDirection = "worktree_merge_direction_forbidden"
	ReasonWorktreeMergeConflict  = "worktree_merge_conflict"
	ReasonWorktreeUnmerged       = "worktree_unmerged_changes"
	ReasonWorktreePlanRequired   = "worktree_plan_required"
	ReasonWorktreeInvalidCombo   = "worktree_invalid_combination"
	ReasonWorktreeModeUnknown    = "worktree_mode_unknown"
	// Planner review (AC-102).
	ReasonPlannerReviewBlocked     = "planner_review_blocked"
	ReasonPlannerReviewNotRun      = "planner_review_not_run"
	ReasonPlannerReviewerInvalid   = "planner_reviewer_invalid"
	ReasonPlannerReviewerUnconfig  = "planner_reviewer_not_configured"
	ReasonPlannerReviewInputMiss   = "planner_review_input_missing"
	ReasonPlannerReviewSummaryReq  = "planner_review_summary_required"
	ReasonPlannerReviewGateBlocked = "planner_review_gate_blocked"
	// ACP probe (AC-046).
	ReasonACPProbeFailed  = "acp_probe_failed"
	ReasonACPNotSupported = "acp_full_backend_not_supported"
	// Skill score family (P4b, AC-151..156 / AC-169 / AC-170). The
	// first two spellings are frozen-verbatim (AC-169 fail-closed,
	// AC-170 auto-apply prohibition); v1 loom_skill_score.py carries
	// the same corpus code.
	ReasonSkillScoreCorpusMissing = "skill_score_corpus_missing"
	ReasonSkillAutoEditForbidden  = "skill_auto_edit_forbidden"
	ReasonSkillScoreEvaluatorReq  = "skill_score_evaluator_required"
	ReasonSkillTargetNotFound     = "skill_target_not_found"
	ReasonSkillTargetInvalid      = "skill_target_invalid"
	ReasonSkillHumanScoreInvalid  = "skill_human_score_invalid"
	// P4c leftover family: protected-path denylist (v1.6.5
	// cross-version principle, global-boundaries §6.1 — the code
	// spelling is frozen-verbatim from the v1 truth) and the loop
	// kill switch (v1.6.5 loop-token-budget AC-006, verbatim).
	ReasonProtectedPathDenylist = "protected_path_denylist"
	ReasonLoopKillSwitchActive  = "loop_kill_switch_active"
)

// ---- Standard flow S1 family (AC-136/137/139/140) ----
//
// Spellings are frozen-verbatim: truth_canonical_branch_mismatch
// and empty_refreeze_reason_only / revision_authority_drift come
// from the R10 Truth Canonical Branch Contract (frozen truth.md),
// canonical_refreeze_not_committed from TC-139 and
// stale_feature_local_refreeze from TC-140.

const (
	ReasonTruthCanonicalBranchMismatch  = "truth_canonical_branch_mismatch"
	ReasonCanonicalRefreezeNotCommitted = "canonical_refreeze_not_committed"
	ReasonEmptyRefreezeReasonOnly       = "empty_refreeze_reason_only"
	ReasonRevisionAuthorityDrift        = "revision_authority_drift"
	ReasonStaleFeatureLocalRefreeze     = "stale_feature_local_refreeze"
)

// ---- Standard flow S2 family (AC-138/141/142) ----
//
// Spellings are frozen-verbatim from the R11 Standard Execution
// Flow Contract's failure-code list (frozen truth.md) and its TC
// table: standard_flow_order_violation (also TC-N21),
// truth_baseline_commit_missing, truth_baseline_not_merged (also
// R10 / TC-141 / TC-N19), workspace_admission_missing,
// unscoped_dirty_state and standard_flow_verification_incomplete
// (TC-142 / TC-N20). ONE code is new in this slice and marked as
// such: git_diff_check_failed — the frozen text names the
// `git diff --check` verification (R11 governance_verify) but
// assigns it no code of its own; the bundle item needed one.

const (
	ReasonStandardFlowOrderViolation   = "standard_flow_order_violation"
	ReasonTruthBaselineCommitMissing   = "truth_baseline_commit_missing"
	ReasonTruthBaselineNotMerged       = "truth_baseline_not_merged"
	ReasonWorkspaceAdmissionMissing    = "workspace_admission_missing"
	ReasonUnscopedDirtyState           = "unscoped_dirty_state"
	ReasonStandardFlowVerificationIncm = "standard_flow_verification_incomplete"
	ReasonGitDiffCheckFailed           = "git_diff_check_failed" // NEW (S2): see family comment
)

// ---- Topology S3 family (AC-171/172/173) ----
//
// Spellings are frozen-verbatim: gitscope_branch_context_parity_failed
// (AC-171), topology_gate_missing / topology_gate_blocked (AC-173),
// satellite_worktree_dirty and multi_git_primary_missing and the
// dynamic monorepo_service_dirty:<service_key> family from the v1.6.7
// GitScopeGraph implementation (beacon/utils/git_scope_graph.py).
// ONE code is new in this slice and marked as such:
// topology_scope_unreadable — a scope whose worktree cannot be
// inspected at all (missing directory / not a git repository); the
// frozen text assigns it no code, and the gate must not guess pass.

const (
	ReasonGitScopeParityFailed    = "gitscope_branch_context_parity_failed"
	ReasonTopologyGateMissing     = "topology_gate_missing"
	ReasonTopologyGateBlocked     = "topology_gate_blocked"
	ReasonSatelliteWorktreeDirty  = "satellite_worktree_dirty"
	ReasonMultiGitPrimaryMissing  = "multi_git_primary_missing"
	ReasonTopologyScopeUnreadable = "topology_scope_unreadable" // NEW (S3): see family comment
)

// ReasonMonorepoServiceDirty is the v1.6.7 dynamic scope code for a
// dirty writable monorepo service scope: monorepo_service_dirty:<key>.
func ReasonMonorepoServiceDirty(serviceKey string) string {
	return "monorepo_service_dirty:" + serviceKey
}

// IsKnownReasonCode mirrors goal_reason_codes.is_known_reason: catalog
// membership, or one of the stable family prefixes.
func IsKnownReasonCode(code string) bool {
	if _, ok := ReasonCatalog[code]; ok {
		return true
	}
	for _, prefix := range []string{"stage_exit_", "goal_implement_", "main_", "freeze_", "release_", "program_", "hotfix_", "multi_version_", "main_axis_", "gate_", "truth_integrity_", "design_binding_", "truth_change_", "truth_canonical_", "canonical_", "empty_", "revision_", "stale_", "qa_", "executor_", "ralph_", "worktree_", "planner_", "acp_", "skill_", "standard_flow_", "truth_baseline_", "workspace_admission_", "unscoped_", "git_diff_", "topology_gate_", "topology_scope_", "gitscope_", "satellite_", "multi_git_", "monorepo_"} {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	return false
}
