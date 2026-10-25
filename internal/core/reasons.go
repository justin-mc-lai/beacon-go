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
	ReasonTruthDriftDetected     = "truth_drift_detected"
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

// ---- Freeze quality R2a family (Python v1.6 parity audit H1-H4 /
// M5-M7) ----
//
// Spellings are verbatim from the Python sources they port:
// ac_vague_disjunction is truth_review_gate.REASON_VAGUE,
// truth_scaffold_not_filled its REASON_SCAFFOLD, truth_fsm_invalid
// is truth_fsm.REASON_TRUTH_FSM_INVALID, materials_status_stale is
// evaluate_release_truth_materials_gate's headline code,
// feature_package_validation_failed is truth_integrity's validation
// code, the TRUTH_DETERMINISM_* pair are truth_integrity's TIG-DC
// codes (uppercase in the source), and the domain-FSM codes are
// domain_fsm.py's REASON_* constants. requirement_clarity_open is
// requirement_clarity.py's blocked-payload code.
// unapproved_scope_reduction is planner_review_runtime's
// _deferral_ledger capability name (the ledger row carries no
// reason_code field in Python; the capability string is its stable
// identifier). TWO codes are new in this slice and marked as such:
// scope_mode_invalid (Python only ever DERIVES scope_mode, so it
// never needed a code for a hand-written bad value; the Go intent
// command accepts an explicit override and must fail closed on it)
// — the deferral ledger's own code reuses the Python capability
// name instead of inventing a second spelling.

const (
	ReasonACVagueDisjunction        = "ac_vague_disjunction"
	ReasonTruthScaffoldNotFilled    = "truth_scaffold_not_filled"
	ReasonTruthFSMInvalid           = "truth_fsm_invalid"
	ReasonMaterialsStale            = "materials_status_stale"
	ReasonIntegrityPackageValid     = "feature_package_validation_failed"
	ReasonDeterminismPlaceholder    = "TRUTH_DETERMINISM_PLACEHOLDER"
	ReasonDeterminismHollowTemplate = "TRUTH_DETERMINISM_HOLLOW_TEMPLATE"
	ReasonRequirementClarityOpen    = "requirement_clarity_open"
	ReasonDeferralUnapproved        = "unapproved_scope_reduction"
	ReasonScopeModeInvalid          = "scope_mode_invalid" // NEW (R2a): see family comment
	// Domain FSM authority (domain_fsm.py REASON_* verbatim).
	ReasonDomainModelMissing      = "domain_model_missing"
	ReasonEntityPrecedenceMissing = "entity_precedence_missing"
	ReasonDomainFSMMissing        = "domain_fsm_missing"
	ReasonDomainFSMIncomplete     = "domain_fsm_incomplete"
	ReasonIllegalTransitionSlot   = "illegal_transition_slot_missing"
	ReasonOrphanACWithoutFSMPath  = "orphan_ac_without_fsm_path"
	ReasonDomainFSMSchemaInvalid  = "domain_fsm_schema_invalid"
	// Integrity extension scan kinds (truth_integrity.py
	// scan_integrity_extension_lines: the finding kind IS the code
	// in the Python payloads).
	ReasonIntegrityPlaceholderAC    = "placeholder_ac_language"
	ReasonIntegrityIllegalClosure   = "illegal_closure_path"
	ReasonIntegritySoftTestAssert   = "soft_test_assertion"
	ReasonIntegrityAlignmentMissing = "alignment_surface_missing"
	ReasonIntegrityUserIntentMiss   = "user_intent_missing"
)

// ---- Orchestration & release R2b family (Python v1.6 parity
// audit H5/H6/H7/M8/M9/M15/M16) ----
//
// Spellings are verbatim from the Python sources they port:
// planner_review_* are loop_goal_state.record_review_decision's
// routing codes (the drift-route code is BUILT by that function as
// planner_review_route_<route with '-' -> '_'>); health_blocked /
// operator_gate_open / evidence_wait / focus_wait are
// goal_quota.quota_should_run's four wait-state codes (its
// quota_exhausted already lives in the budget family above);
// already_resolved / gate_unresolved are goal_gates.py's gate-card
// codes; spend_idempotent_replay is goal_budget.commit_spend's
// replay code; release_profile_invalid is prod_ops_v168
// .select_release_profile's refusal, release_check_failed is
// release.py's lifecycle-trace code for a failed version check,
// and project_metadata_contract_blocked /
// requirement_package_contract_blocked are release.py check's two
// early-block codes; feature_package_not_resolved and
// machine_json_source_hash_stale are truth_map.verify_truth_map's
// diagnostic codes (the Go truth-map face reuses them for its
// resolver/fingerprint-staleness diagnostics — see the r2b
// report for the mapping). THREE codes are new in this slice and
// marked as such: release_check_hard_block /
// release_check_downgraded_warning (the task-level distinction the
// version check must surface per item; Python's check only had
// pass/warn/fail labels) and review_spec_drift_detected (Python's
// drift findings bind to its prototype/runtime-projection
// machinery, which has no Go counterpart; the Go review face
// judges package drift vs the freeze record instead).

const (
	ReasonPlannerReviewPacketIncompl   = "planner_review_packet_incomplete"
	ReasonPlannerReviewBlockingFinding = "planner_review_blocking_finding"
	ReasonPlannerReviewUserDecisionReq = "planner_review_user_decision_required"
	ReasonPlannerReviewHumanGateP1     = "planner_review_human_gate_p1"
	ReasonPlannerReviewRouteUnhandled  = "planner_review_route_unhandled"
	ReasonHealthBlocked                = "health_blocked"
	ReasonOperatorGateOpen             = "operator_gate_open"
	ReasonEvidenceWait                 = "evidence_wait"
	ReasonFocusWait                    = "focus_wait"
	ReasonGateAlreadyResolved          = "already_resolved"
	ReasonGateUnresolved               = "gate_unresolved"
	ReasonSpendIdempotentReplay        = "spend_idempotent_replay"
	ReasonReleaseProfileInvalid        = "release_profile_invalid"
	ReasonReleaseCheckFailed           = "release_check_failed"
	ReasonProjectMetadataBlocked       = "project_metadata_contract_blocked"
	ReasonRequirementContractBlocked   = "requirement_package_contract_blocked"
	ReasonTruthMapNotResolved          = "feature_package_not_resolved"
	ReasonTruthMapStale                = "machine_json_source_hash_stale"
	ReasonReleaseCheckHardBlock        = "release_check_hard_block"         // NEW (R2b): see family comment
	ReasonReleaseCheckDowngraded       = "release_check_downgraded_warning" // NEW (R2b): see family comment
	ReasonReviewSpecDrift              = "review_spec_drift_detected"       // NEW (R2b): see family comment
)

// ---- Exec & host R2c family (Python v1.6 parity audit
// H8/H9/H10/M10/M11/M12) ----
//
// Spellings are verbatim from the Python sources they port:
// qa9_command_missing_auto_case and auto_test_layer_failed are
// auto_test.py's per-TC codes; the hook_* / invalid_* /
// manual_confirmation_required / hidden_auto_run_not_allowed /
// post_release_followup_failed set is project_hooks.py's
// admission + run vocabulary (its diagnostic codes double as
// reason codes); archive_governance_not_supported is
// commands/archive.py's blocked payload code; invalid_project_mode
// is commands/init.py's refusal code. Codes marked NEW have no
// Python spelling because Python never needed one (its auto-test
// ran through a shell, its skill install never conflict-checked,
// its state lived in files so it never exported/imported).

const (
	ReasonAutoTestCommandMissing  = "qa9_command_missing_auto_case"
	ReasonAutoTestLayerFailed     = "auto_test_layer_failed"
	ReasonAutoTestTestsDocMissing = "autotest_tests_document_missing"   // NEW (R2c)
	ReasonAutoTestNoExecutable    = "autotest_no_executable_tests"      // NEW (R2c)
	ReasonAutoTestUnparseable     = "autotest_command_unparseable"      // NEW (R2c): shell-free argv split failed
	ReasonAutoTestShellSyntax     = "autotest_shell_syntax_unsupported" // NEW (R2c): shell operators refused, never silently misrun
	ReasonAutoTestFailed          = "autotest_failed"                   // NEW (R2c): aggregate headline when any TC fails/blocks
	ReasonAutoTestDeclaredBlocked = "tc_declared_blocked"               // NEW (R2c): tests.md's own BLOCKED: escape, reported not executed
	ReasonSkillInstallConflict    = "skill_install_conflict"            // NEW (R2c)
	ReasonSkillInstallHostUnknown = "skill_install_host_unknown"        // NEW (R2c)
	ReasonStateSnapshotMissing    = "state_snapshot_missing"            // NEW (R2c)
	ReasonStateSnapshotInvalid    = "state_snapshot_invalid"            // NEW (R2c)
	ReasonArchiveStatusInvalid    = "archive_status_invalid"            // NEW (R2c): Python normalizes silently; the Go face refuses a bad status instead of guessing
	ReasonArchiveFeatureNotFound  = "archive_feature_not_found"         // NEW (R2c)
	ReasonArchiveNotSupported     = "archive_governance_not_supported"
	ReasonProjectModeInvalid      = "invalid_project_mode"
	ReasonHookAdmissionBlocked    = "hook_admission_blocked"
	ReasonHookDryRunOnly          = "hook_dry_run_only"
	ReasonHookManualConfirm       = "manual_confirmation_required"
	ReasonHookHiddenAutoRun       = "hidden_auto_run_not_allowed"
	ReasonHookFollowupFailed      = "post_release_followup_failed"
	ReasonHookFileMissing         = "hook_file_missing"
	ReasonHookCommandFailed       = "hook_command_failed" // NEW (R2c): Python never executed command_template; the Go face does (via harness) and needs the code
)

// ---- Intelligence R2d family (Python v1.6 parity audit
// M1/M2/M3/M4/M13/M14) ----
//
// Spellings are verbatim from the Python sources they port:
// boundary_forbidden_phrase / route_probe_missing /
// route_probe_mismatch are agent_eval_grader.py's verdict codes;
// agent_eval_live_runner_not_admitted / agent_eval_admission_missing
// / prompt_fixture_missing / catalog_harness_missing /
// live_skill_matrix_mismatch are agent_eval_kit.py's admission and
// catalog vocabulary. Codes marked NEW have no Python spelling:
// Python's scorecard simply reports status blocked below the 70
// composite threshold (agent_eval_kit.REGRESSION_COMPOSITE_THRESHOLD)
// and never wires that verdict into executor profile selection —
// the Go face does, and needs names for the dispositions.

const (
	ReasonBoundaryForbiddenPhrase   = "boundary_forbidden_phrase"
	ReasonRouteProbeMissing         = "route_probe_missing"
	ReasonRouteProbeMismatch        = "route_probe_mismatch"
	ReasonAgentEvalRunnerNotAdmit   = "agent_eval_live_runner_not_admitted"
	ReasonAgentEvalAdmissionMissing = "agent_eval_admission_missing"
	ReasonPromptFixtureMissing      = "prompt_fixture_missing"
	ReasonCatalogHarnessMissing     = "catalog_harness_missing"
	ReasonLiveSkillMatrixMismatch   = "live_skill_matrix_mismatch"
	ReasonAgentEvalBelowThreshold   = "agent_eval_below_threshold" // NEW (R2d): profile selection refuses an agent whose latest scorecard is blocked (<70 composite, Python threshold)
	ReasonAgentEvalNoRecord         = "agent_eval_no_record"       // NEW (R2d): warning disposition — no eval record exists for the profile; selection proceeds
	ReasonBrainstormModeInvalid     = "brainstorm_mode_invalid"    // NEW (R2d): Python raises ValueError("unsupported brainstorm mode"); the Go face names it
	ReasonDecomposeFrozen           = "decompose_frozen_package"   // NEW (R2d): decomposition materializes files; a frozen body is never rewritten by it
	ReasonDecomposeNoIntents        = "decompose_no_intents"       // NEW (R2d): the utterance produced no clause of substance
	ReasonCode2TruthTargetMissing   = "code2truth_target_missing"  // NEW (R2d): the intake target directory does not exist
	ReasonTruthReopenNotFrozen      = "truth_reopen_not_frozen"    // NEW (R2d): reopen opens a new revision of a frozen/active package; a draft is simply edited
)

// PlannerReviewRouteCode builds record_review_decision's drift
// routing code for one drift route (beacon-gen-truth /
// beacon-gen-change / beacon-gen-refreeze): the source spells it
// planner_review_route_<route> with dashes folded to underscores.
func PlannerReviewRouteCode(route string) string {
	return "planner_review_route_" + strings.ReplaceAll(route, "-", "_")
}

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
	for _, prefix := range []string{"stage_exit_", "goal_implement_", "main_", "freeze_", "release_", "program_", "hotfix_", "multi_version_", "main_axis_", "gate_", "truth_integrity_", "design_binding_", "truth_change_", "truth_canonical_", "truth_", "canonical_", "empty_", "revision_", "stale_", "qa_", "executor_", "ralph_", "worktree_", "planner_", "acp_", "skill_", "standard_flow_", "truth_baseline_", "workspace_admission_", "unscoped_", "git_diff_", "topology_gate_", "topology_scope_", "gitscope_", "satellite_", "multi_git_", "monorepo_", "ac_vague_", "materials_", "feature_package_", "requirement_clarity_", "unapproved_", "scope_mode_", "domain_", "entity_", "illegal_", "orphan_", "placeholder_", "alignment_", "user_intent_", "TRUTH_DETERMINISM_", "health_", "operator_gate_", "evidence_wait", "focus_wait", "gate_unresolved", "already_resolved", "release_", "feature_package_", "machine_json_", "review_", "autotest_", "qa9_", "auto_test_", "tc_declared_", "skill_install_", "state_snapshot_", "archive_", "hook_", "invalid_project_mode", "agent_eval_", "boundary_", "route_probe_", "prompt_fixture_", "catalog_", "live_skill_", "brainstorm_", "decompose_", "code2truth_", "qa_strategy_"} {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	return false
}
