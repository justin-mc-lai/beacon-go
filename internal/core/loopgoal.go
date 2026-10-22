// Loop-goal runtime axis (P2a, G-20): the 12-state machine of the v1
// Python beacon/utils/loop_goal_state.py, ported as pure functions over a
// value snapshot. This axis is SEPARATE from the lifecycle axis
// (WorkflowStage/StageStatus) and from the runs.status axis (RunStatus);
// the three axes must never be merged (see core.go).
//
// Semantics ported from loop_goal_state.py:
//   - transition(): to_stage must be one of the 12 states (else
//     blocked/invalid_stage); a pending stop_when blocks with
//     stopped/stop_when_satisfied; done is reachable only from
//     release_gate and only with release confirmation (mechanical human
//     gate, independent of any auto-decide flag).
//   - increment_repair_loop(): counts implement/qa repair loops; exceeding
//     MaxRepairLoops (3) routes the run to error_recovery.
//   - check_stop_when() evaluation is an injected hook: callers register a
//     StopWhenEvaluator; with no evaluator the machine only enforces the
//     mechanical rules above (P2a scope).
package core

// LoopStage is one state of the loop-goal runtime axis (12 states).
type LoopStage string

const (
	LoopIdle                      LoopStage = "idle"
	LoopGovernancePrecheck        LoopStage = "governance_precheck"
	LoopTruthRouting              LoopStage = "truth_routing"
	LoopReviewRouting             LoopStage = "review_routing"
	LoopStandardExecutionPrecheck LoopStage = "standard_execution_precheck"
	LoopImplementRouting          LoopStage = "implement_routing"
	LoopQaRouting                 LoopStage = "qa_routing"
	LoopQaDeepRouting             LoopStage = "qa_deep_routing"
	LoopDriftRouting              LoopStage = "drift_routing"
	LoopReleaseGate               LoopStage = "release_gate"
	LoopDone                      LoopStage = "done"
	LoopErrorRecovery             LoopStage = "error_recovery"
)

// MaxRepairLoops is the repair-loop ceiling from loop_goal_state.py:
// incrementing past it routes to error_recovery.
const MaxRepairLoops = 3

// Reason codes of the loop-goal axis (values mirror loop_goal_state.py).
const (
	ReasonInvalidLoopStage             = "invalid_stage"
	ReasonStopWhenSatisfied            = "stop_when_satisfied"
	ReasonReleaseRequiresHuman         = "release_requires_human_confirmation"
	ReasonNotAtReleaseGate             = "not_at_release_gate"
	ReasonMaxRepairLoopsExceeded       = "max_repair_loops_exceeded"
	ReasonLoopGoalAlreadyDone          = "loop_goal_already_done"
	ReasonLoopGoalResumeNoRuns         = "loop_goal_resume_no_runs"
	ReasonLoopGoalResumeNoopForbidden  = "loop_goal_resume_noop_forbidden"
	ReasonHostTurnInterrupted          = "host_turn_interrupted"
	ReasonStopWhenUnverified           = "stop_when_unverified"
	ReasonCompleteBlockedStopWhenUnver = "complete_blocked_stop_when_unverified_execution"
)

// Resume actions (goal_reason_codes.py RESUME_* family + build_resume).
const (
	ResumeNone              = "none"
	ResumeLoadNextSkill     = "load_next_skill"
	ResumeAwaitHumanConfirm = "await_human_confirmation"
	ResumeReplaySubstep     = "replay_substep"
	ResumeAwaitFreezeAck    = "await_freeze_ack"
	ResumeAwaitHumanRelease = "await_human_release"
)

// AllLoopStages returns the 12 loop stages in pipeline order.
func AllLoopStages() []LoopStage {
	return []LoopStage{
		LoopIdle, LoopGovernancePrecheck, LoopTruthRouting, LoopReviewRouting,
		LoopStandardExecutionPrecheck, LoopImplementRouting, LoopQaRouting,
		LoopQaDeepRouting, LoopDriftRouting, LoopReleaseGate, LoopDone,
		LoopErrorRecovery,
	}
}

func (s LoopStage) String() string { return string(s) }

// Valid reports whether s is one of the 12 loop stages.
func (s LoopStage) Valid() bool {
	for _, v := range AllLoopStages() {
		if s == v {
			return true
		}
	}
	return false
}

// ParseLoopStage parses a loop stage name, rejecting unknown values with
// reason_code=invalid_stage (loop_goal_state.py transition guard).
func ParseLoopStage(v string) (LoopStage, error) {
	s := LoopStage(v)
	if !s.Valid() {
		return "", NewError(ReasonInvalidLoopStage, "ParseLoopStage", "unknown loop stage "+v)
	}
	return s, nil
}

// LoopSkillForStage maps a loop stage to the skill the host should load,
// mirroring loop_goal_state.get_skill_for_stage. Empty means no skill.
func LoopSkillForStage(s LoopStage) string {
	switch s {
	case LoopGovernancePrecheck:
		return "CLI:beacon-plan-start"
	case LoopTruthRouting:
		return "beacon-gen-truth"
	case LoopReviewRouting:
		return "beacon-pln-review"
	case LoopStandardExecutionPrecheck:
		return "CLI:beacon-status-standard-execution-precheck"
	case LoopImplementRouting:
		return "beacon-gen-implement"
	case LoopQaRouting:
		return "beacon-eval-qa"
	case LoopQaDeepRouting:
		return "beacon-eval-qa-deep"
	case LoopDriftRouting:
		return "beacon-gen-change"
	case LoopReleaseGate:
		return "beacon-eval-release"
	case LoopErrorRecovery:
		return "beacon-gov-debug"
	default: // idle, done
		return ""
	}
}

// LoopState is the runtime snapshot of one goal run on the loop axis.
// It is a pure value; persistence lives in the orchestrator (run metadata
// in the store), never in this package.
type LoopState struct {
	CurrentStage     LoopStage `json:"current_stage"`
	StageHistory     []string  `json:"stage_history,omitempty"`
	StopWhen         string    `json:"stop_when,omitempty"`
	ReleaseConfirmed bool      `json:"release_confirmed"`
	RepairLoopCount  int       `json:"repair_loop_count"`
	LastSubstep      string    `json:"last_substep,omitempty"`
}

// NewLoopState starts a run at idle.
func NewLoopState() LoopState {
	return LoopState{CurrentStage: LoopIdle, StageHistory: []string{string(LoopIdle)}}
}

// StopWhenEvaluator is the stop_when judgement hook (P2a: interface only;
// concrete evaluators land with the gate work in P3). It returns a
// non-empty reason_code when the stop condition is satisfied.
type StopWhenEvaluator interface {
	EvaluateStopWhen(st LoopState) (reasonCode string, satisfied bool)
}

// Transition moves the state to toStage following loop_goal_state.py
// transition() semantics. On success the state is updated in place; on
// failure a typed error carries the source reason_code and the state is
// left unchanged.
func (st *LoopState) Transition(to LoopStage, releaseConfirmed bool, ev StopWhenEvaluator) error {
	if !to.Valid() {
		return NewError(ReasonInvalidLoopStage, "LoopState.Transition", "unknown loop stage "+string(to))
	}
	if ev != nil {
		if code, satisfied := ev.EvaluateStopWhen(*st); satisfied {
			if code == "" {
				code = ReasonStopWhenSatisfied
			}
			return NewError(code, "LoopState.Transition", "stop_when already satisfied")
		}
	}
	if st.StopWhen != "" {
		return NewError(ReasonStopWhenSatisfied, "LoopState.Transition",
			"pending stop_when from a previous step: "+st.StopWhen)
	}
	if to == LoopDone {
		if st.CurrentStage != LoopReleaseGate {
			return NewError(ReasonNotAtReleaseGate, "LoopState.Transition",
				"done is only reachable from release_gate, current is "+string(st.CurrentStage))
		}
		if !releaseConfirmed {
			return NewError(ReasonReleaseRequiresHuman, "LoopState.Transition",
				"release requires explicit human confirmation")
		}
	}
	st.CurrentStage = to
	st.StageHistory = append(st.StageHistory, string(to))
	if releaseConfirmed {
		st.ReleaseConfirmed = true
	}
	return nil
}

// CheckReleaseGate mirrors loop_goal_state.check_release_gate: only at
// release_gate and only with human confirmation may the run enter done.
func (st *LoopState) CheckReleaseGate(humanConfirmed bool) error {
	if st.CurrentStage != LoopReleaseGate {
		return NewError(ReasonNotAtReleaseGate, "LoopState.CheckReleaseGate",
			"current stage is "+string(st.CurrentStage))
	}
	if !humanConfirmed {
		return NewError(ReasonReleaseRequiresHuman, "LoopState.CheckReleaseGate",
			"release requires explicit human confirmation")
	}
	return nil
}

// IncrementRepairLoop counts one implement/qa repair loop
// (loop_goal_state.increment_repair_loop). When the count exceeds
// MaxRepairLoops the state is routed to error_recovery and a typed error
// with reason_code=max_repair_loops_exceeded is returned.
func (st *LoopState) IncrementRepairLoop() error {
	st.RepairLoopCount++
	if st.RepairLoopCount > MaxRepairLoops {
		st.CurrentStage = LoopErrorRecovery
		st.StageHistory = append(st.StageHistory, string(LoopErrorRecovery))
		return NewError(ReasonMaxRepairLoopsExceeded, "LoopState.IncrementRepairLoop",
			"repair loops exceeded; routed to error_recovery (recommended_route=beacon-gov-debug)")
	}
	return nil
}

// IsShortContinueUtterance reports whether an utterance is one of the
// short continue words (frozen AC-035: 继续/continue/resume/go).
func IsShortContinueUtterance(u string) bool {
	switch u {
	case "继续", "continue", "resume", "go":
		return true
	default:
		return false
	}
}
