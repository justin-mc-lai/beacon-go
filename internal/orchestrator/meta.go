// Goal run metadata + static stage tables (P2a).
//
// ALL goal runtime state lives in the global store (charter rule 7,
// frozen AC-016): runs.status / runs.metadata_json hold the run
// snapshot, lifecycle_events is the append-only trail, checkpoints the
// resume points. This package never writes its own JSON state files.
//
// The stage order mirrors product_goal_runtime.build_stage_order and
// the per-stage skill lists mirror its PROGRESSIVE_SKILLS table
// (progressive disclosure: the first entry is the stage's root skill).
package orchestrator

import "beacon-go/internal/core"

// Product stages of a goal run (product_goal_runtime stage axis). These
// are the plan stages; the loop axis (core.LoopStage) tracks routing.
const (
	StagePlan      = "plan"
	StageTruth     = "truth"
	StageDesign    = "design"
	StageImplement = "implement"
	StageQA        = "qa"
	StageRelease   = "release"
)

// BuildStageOrder returns the stage plan order: the base pipeline
// plan/truth/implement/qa/release, with design inserted after truth
// when UX work is required (build_stage_order semantics). UX detection
// is an explicit caller input in P2a — no heuristic guessing.
func BuildStageOrder(uxRequired bool) []string {
	stages := []string{StagePlan, StageTruth}
	if uxRequired {
		stages = append(stages, StageDesign)
	}
	return append(stages, StageImplement, StageQA, StageRelease)
}

// ProgressiveSkills is the static per-stage skill table, ported from
// product_goal_runtime.PROGRESSIVE_SKILLS.
var ProgressiveSkills = map[string][]string{
	StagePlan:      {"beacon-plan", "mode:pln-program", "mode:pln-program-auto", "mode:pln-program-interactive", "mode:pln-review", "mode:pln-friction", "mode:pln-brainstorm", "mode:pln-source"},
	StageTruth:     {"beacon-truth", "mode:gen-truth", "mode:gen-truth-init", "mode:gen-change", "mode:gen-refreeze"},
	StageDesign:    {"beacon-design", "mode:gen-design", "mode:gen-design-init", "mode:gen-design-system", "mode:gen-design-explore", "mode:gen-design-polish", "mode:gen-prototype"},
	StageImplement: {"beacon-implement", "mode:gen-implement"},
	StageQA:        {"beacon-qa", "mode:eval-qa", "mode:eval-qa-deep", "mode:eval-real-project", "mode:eval-agent"},
	StageRelease:   {"beacon-release", "mode:eval-release"},
	"goal":         {"beacon-goal", "mode:loop-goal"},
}

// LoopStageForProductStage maps a product stage to its loop-axis
// routing state. DERIVED: loop_goal_state.py has no design routing
// state; design work is reviewed at review_routing.
func LoopStageForProductStage(stage string) core.LoopStage {
	switch stage {
	case StagePlan:
		return core.LoopGovernancePrecheck
	case StageTruth:
		return core.LoopTruthRouting
	case StageDesign:
		return core.LoopReviewRouting
	case StageImplement:
		return core.LoopImplementRouting
	case StageQA:
		return core.LoopQaRouting
	case StageRelease:
		return core.LoopReleaseGate
	default:
		return core.LoopIdle
	}
}

// workflowStageForProductStage maps a product stage onto the lifecycle
// axis for lifecycle snapshot writes (engine is the lifecycle writer
// while a goal run drives the feature). DERIVED mapping.
func workflowStageForProductStage(stage string) core.WorkflowStage {
	switch stage {
	case StagePlan:
		return core.StagePrd
	case StageTruth:
		return core.StageFreeze
	case StageDesign:
		return core.StagePrototype
	case StageImplement:
		return core.StageImplement
	case StageQA:
		return core.StageQa
	case StageRelease:
		return core.StageRelease
	default:
		return core.StageThink
	}
}

// StagePlanEntry is one planned stage of a run.
type StagePlanEntry struct {
	Stage     string   `json:"stage"`
	Status    string   `json:"status"` // pending | active | done | blocked
	Skills    []string `json:"skills"`
	LoopStage string   `json:"loop_stage"`
}

// VerifierInfo is the attached independent checker (attach_verifier).
type VerifierInfo struct {
	RequiredForComplete bool     `json:"required_for_complete"`
	Present             bool     `json:"present"`
	Refs                []string `json:"refs,omitempty"`
	Source              string   `json:"source,omitempty"`
}

// runMeta is the goal snapshot persisted in runs.metadata_json.
type runMeta struct {
	Utterance             string           `json:"utterance,omitempty"`
	UXRequired            bool             `json:"ux_required"`
	LoopStage             string           `json:"loop_stage"`
	StageHistory          []string         `json:"stage_history,omitempty"`
	CurrentStageIdx       int              `json:"current_stage_idx"`
	StagePlan             []StagePlanEntry `json:"stage_plan"`
	ExecutorID            string           `json:"executor_id,omitempty"`
	Verifier              *VerifierInfo    `json:"verifier,omitempty"`
	ReleaseHumanConfirmed bool             `json:"release_human_confirmed"`
	HumanConfirmedBy      string           `json:"human_confirmed_by,omitempty"`
	Blocked               bool             `json:"blocked"`
	BlockedStreak         int              `json:"blocked_streak"`
	LastBlockReason       string           `json:"last_block_reason,omitempty"`
	RepairLoopCount       int              `json:"repair_loop_count"`
	StopWhen              string           `json:"stop_when,omitempty"`
	LastHeartbeatAt       string           `json:"last_heartbeat_at,omitempty"`
	HeartbeatSummary      string           `json:"heartbeat_summary,omitempty"`
	LastSubstep           string           `json:"last_substep,omitempty"`
	LastCheckpointAction  string           `json:"last_checkpoint_next_action,omitempty"`
	InterruptPending      bool             `json:"interrupt_pending"`
	InterruptSubstep      string           `json:"interrupt_substep,omitempty"`
	InterruptCount        int              `json:"interrupt_count"`
	LastTickKey           string           `json:"last_tick_key,omitempty"`
	// P2b long-run care fields (driver / supervisor / guardrails /
	// queue provenance). All still live in runs.metadata_json — the
	// single store snapshot; no side files (charter rule 7).
	Driver               *DriverState     `json:"driver,omitempty"`
	Supervisor           *SupervisorState `json:"supervisor,omitempty"`
	HeartbeatIntervalSec int              `json:"heartbeat_interval_sec,omitempty"`
	StallCount           int              `json:"stall_count"`
	LastStallAt          string           `json:"last_stall_at,omitempty"`
	Budget               *BudgetState     `json:"budget,omitempty"`
	Steerings            []SteeringRecord `json:"steerings,omitempty"`
	Todos                []TodoItem       `json:"todos,omitempty"`
	TodoSeq              int              `json:"todo_seq"`
	FromQueueItem        string           `json:"from_queue_item,omitempty"`
	DiscoverySource      string           `json:"discovery_source,omitempty"`
	// P2c delegation-and-parallel fields (lanes / delegation /
	// parallel plan / design binding / routing inputs). Same rule:
	// runs.metadata_json is the only snapshot; no side files.
	Lakes         []string        `json:"lakes,omitempty"`
	TaskScale     string          `json:"task_scale,omitempty"`
	Implement     *ImplementState `json:"implement,omitempty"`
	ParallelPlan  *ParallelPlan   `json:"parallel_plan,omitempty"`
	DesignBinding *DesignBinding  `json:"design_binding,omitempty"`
	// P2d orchestration-extension fields (program / hotfix /
	// multi-version / memory / axis). Same rule: runs.metadata_json
	// is the only snapshot; no side files, no extra tables.
	Program      *ProgramState     `json:"program,omitempty"`
	Hotfix       *HotfixPlan       `json:"hotfix,omitempty"`
	MultiVersion *MultiVersionPlan `json:"multi_version,omitempty"`
	Memory       *MemoryIndex      `json:"memory,omitempty"`
	MainAxis     *MainAxis         `json:"main_axis,omitempty"`
	// P3c real-integration field: the subprocess-family executor
	// configuration recorded at run creation (argv + profile +
	// reviewer identity). CLI layers may override it per invocation;
	// the run's copy is the default the invocation falls back to.
	Exec *ExecutorConfig `json:"exec,omitempty"`
	// S2 (AC-138): the standard execution flow's progress on this
	// run — the third, separate small state machine (stdflow.go).
	// Completed steps only; the step table itself is static in core
	// orchestrator code, never persisted.
	Flow *FlowState `json:"flow,omitempty"`
	// S3 (AC-173): the topology fact at run creation — status/ref/
	// kind of the project's topology gate when the loop started.
	// A fact, never a creation-time block: the implement admission
	// and the bundle consume the gate (topology_record.go).
	Topology *TopologyFact `json:"topology,omitempty"`
}

// pendingSteering returns the newest unacknowledged steering record,
// or nil. Tick/Resume/Status surface it — an unacknowledged direction
// change must never be silently swallowed (G-21).
func (m *runMeta) pendingSteering() *SteeringRecord {
	for i := len(m.Steerings) - 1; i >= 0; i-- {
		if !m.Steerings[i].Acknowledged {
			s := m.Steerings[i]
			return &s
		}
	}
	return nil
}

// todoSummary aggregates the run's todo list for status surfaces.
func (m *runMeta) todoSummary() *TodoSummary {
	if len(m.Todos) == 0 {
		return nil
	}
	s := &TodoSummary{Total: len(m.Todos)}
	for _, t := range m.Todos {
		switch t.Status {
		case TodoOpen:
			s.Open++
		case TodoClaimed:
			s.Claimed++
		case TodoDone:
			s.Done++
		}
	}
	return s
}

// currentStage returns the active product stage name, or "" when the
// plan is exhausted (run at/past the final stage boundary).
func (m *runMeta) currentStage() string {
	if m.CurrentStageIdx < 0 || m.CurrentStageIdx >= len(m.StagePlan) {
		return ""
	}
	return m.StagePlan[m.CurrentStageIdx].Stage
}

// loopState rebuilds the pure core loop state from the snapshot.
func (m *runMeta) loopState() core.LoopState {
	return core.LoopState{
		CurrentStage:     core.LoopStage(m.LoopStage),
		StageHistory:     m.StageHistory,
		StopWhen:         m.StopWhen,
		ReleaseConfirmed: m.ReleaseHumanConfirmed,
		RepairLoopCount:  m.RepairLoopCount,
		LastSubstep:      m.LastSubstep,
	}
}

// applyLoopState writes a core loop state back into the snapshot.
func (m *runMeta) applyLoopState(st core.LoopState) {
	m.LoopStage = string(st.CurrentStage)
	m.StageHistory = st.StageHistory
	m.RepairLoopCount = st.RepairLoopCount
	if st.ReleaseConfirmed {
		m.ReleaseHumanConfirmed = true
	}
}
