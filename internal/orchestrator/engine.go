// Goal engine (P2a): the product-goal runtime of product_goal_runtime.py
// re-implemented over the global store. Every mutation writes the run
// snapshot (runs row) and appends to lifecycle_events; there is no
// second state file (charter rule 7).
package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Engine reason codes. Values reuse the source runtime's constants
// where they exist; the rest are documented in docs/p2a-implementation.md.
const (
	ReasonRunNotFound          = "product_goal_run_not_found"
	ReasonFeatureNotFound      = "feature_not_found"
	ReasonProjectNotRegistered = "project_not_registered"
	ReasonUpToDate             = "up_to_date"
	ReasonExecutorNotReg       = "stage_executor_not_registered"
	ReasonStageExecutionFailed = "stage_execution_failed"
	ReasonBlockedStreak        = "product_goal_blocked_streak"
	ReasonVerifierRequired     = "product_goal_complete_requires_verifier"
	ReasonQAGateNotPassed      = "qa_gate_not_passed"
	ReasonExecutorError        = "stage_executor_error"
)

// StageOutcome is what a StageExecutor reports for one stage attempt.
type StageOutcome struct {
	Status     string // "done" | "failed"
	Summary    string
	ReasonCode string // required when Status == "failed"
}

// StageExecutor executes one product stage of a goal run. P2a ships no
// real backend; implementations must not fake success — the engine
// blocks with stage_executor_not_registered when no executor is
// registered for a stage.
type StageExecutor interface {
	ID() string
	ExecuteStage(ctx context.Context, run RunView, stage string) (StageOutcome, error)
}

// Engine is the goal core engine over one store handle.
type Engine struct {
	store         *store.Store
	executors     map[string]StageExecutor
	laneExecutors map[string]StageExecutor
	stopWhen      core.StopWhenEvaluator
}

// NewEngine builds an engine with no executors registered.
func NewEngine(st *store.Store) *Engine {
	return &Engine{store: st, executors: map[string]StageExecutor{}, laneExecutors: map[string]StageExecutor{}}
}

// RegisterLaneExecutor binds an executor to one implement lane
// (G-22): the lane registry is how Tick / ExecuteLaneStage / the
// ralph delegation resolve who runs the implement stage.
func (e *Engine) RegisterLaneExecutor(lane string, ex StageExecutor) {
	e.laneExecutors[lane] = ex
}

// RegisterLaneExecutorForAllLanes binds one executor to every lane.
func (e *Engine) RegisterLaneExecutorForAllLanes(ex StageExecutor) {
	for _, l := range []string{LaneSingle, LaneTeam, LaneRalph} {
		e.laneExecutors[l] = ex
	}
}

// LaneExecutor returns the executor registered for a lane.
func (e *Engine) LaneExecutor(lane string) (StageExecutor, bool) {
	ex, ok := e.laneExecutors[lane]
	return ex, ok
}

// implementExecutor resolves who executes the implement stage: when a
// lane has been routed onto the run, the lane registry is the only
// path (a routed lane with no registered executor blocks, exactly
// like an unregistered stage executor in P2a); otherwise the stage
// registry applies.
func (e *Engine) implementExecutor(m *runMeta) (StageExecutor, bool) {
	if m.Implement != nil && m.Implement.Lane != "" {
		return e.LaneExecutor(m.Implement.Lane)
	}
	ex, ok := e.executors[StageImplement]
	return ex, ok
}

// RegisterExecutor binds an executor to one product stage.
func (e *Engine) RegisterExecutor(stage string, ex StageExecutor) {
	e.executors[stage] = ex
}

// RegisterExecutorForAllStages binds one executor to every stage.
func (e *Engine) RegisterExecutorForAllStages(ex StageExecutor) {
	for _, s := range []string{StagePlan, StageTruth, StageDesign, StageImplement, StageQA, StageRelease} {
		e.executors[s] = ex
	}
}

// SetStopWhenEvaluator installs the stop_when judgement hook (core
// interface; P2a leaves concrete evaluators to P3 gate work).
func (e *Engine) SetStopWhenEvaluator(ev core.StopWhenEvaluator) { e.stopWhen = ev }

// RunView is the stable JSON surface of one goal run (AC-065 style:
// snake_case, one document).
type RunView struct {
	RunID                 string           `json:"run_id"`
	FeatureID             string           `json:"feature_id"`
	FeatureSlug           string           `json:"feature_slug"`
	Status                string           `json:"status"` // running | blocked | completed | error_recovery
	RunStatus             string           `json:"run_status"`
	Blocked               bool             `json:"blocked"`
	ReasonCode            string           `json:"reason_code,omitempty"`
	CurrentStage          string           `json:"current_stage,omitempty"`
	LoopStage             string           `json:"loop_stage"`
	NextSkill             string           `json:"next_skill,omitempty"`
	StagePlan             []StagePlanEntry `json:"stage_plan"`
	Verifier              *VerifierInfo    `json:"verifier,omitempty"`
	ReleaseHumanConfirmed bool             `json:"release_human_confirmed"`
	RepairLoopCount       int              `json:"repair_loop_count"`
	BlockedStreak         int              `json:"blocked_streak"`
	UpToDate              bool             `json:"up_to_date,omitempty"`
	LastHeartbeatAt       string           `json:"last_heartbeat_at,omitempty"`
	StartedAt             string           `json:"started_at"`
	// P2b long-run care surfaces (present only when the corresponding
	// subsystem has state on the run).
	Driver          *DriverState     `json:"driver,omitempty"`
	Supervisor      *SupervisorState `json:"supervisor,omitempty"`
	Budget          *BudgetState     `json:"budget,omitempty"`
	Todos           *TodoSummary     `json:"todos,omitempty"`
	PendingSteering *SteeringRecord  `json:"pending_steering,omitempty"`
	FromQueueItem   string           `json:"from_queue_item,omitempty"`
	// P2c delegation-and-parallel surfaces (present only when the
	// corresponding state exists on the run).
	Implement     *ImplementView `json:"implement,omitempty"`
	ParallelPlan  *ParallelPlan  `json:"parallel_plan,omitempty"`
	DesignBinding *DesignBinding `json:"design_binding,omitempty"`
	// P2d orchestration-extension surfaces (present only when the
	// corresponding state exists on the run).
	Program      *ProgramState     `json:"program,omitempty"`
	Hotfix       *HotfixPlan       `json:"hotfix,omitempty"`
	MultiVersion *MultiVersionPlan `json:"multi_version,omitempty"`
	MainAxis     *MainAxis         `json:"main_axis,omitempty"`
	// S3 (AC-173): the topology fact recorded at run creation.
	Topology *TopologyFact `json:"topology,omitempty"`
}

// ---- internal load/save helpers ----

func (e *Engine) loadRun(runID string) (store.Run, *runMeta, string, error) {
	run, err := e.store.Runs().GetByID(runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Run{}, nil, "", core.NewError(ReasonRunNotFound, "goal.loadRun", "no such goal run: "+runID)
		}
		return store.Run{}, nil, "", err
	}
	m := &runMeta{}
	if len(run.Metadata) > 0 {
		if err := json.Unmarshal(run.Metadata, m); err != nil {
			return store.Run{}, nil, "", err
		}
	}
	slug := ""
	if feat, err := e.store.Features().GetByID(run.FeatureID); err == nil {
		slug = feat.Slug
	}
	return run, m, slug, nil
}

// writeSet is the store surface the engine's write helpers run
// against: *store.Store in autocommit, or *store.Tx inside a
// WithTx batch. Multi-write engine operations batch through a Tx
// so one logical step pays one commit instead of one fsync per row
// (P6 write-path fix; see docs/p6-fix-write-perf.md).
type writeSet interface {
	Runs() *store.RunDao
	Features() *store.FeatureDao
	Gates() *store.GateDao
	Artifacts() *store.ArtifactDao
	AppendEvent(store.LifecycleEvent) (store.LifecycleEvent, error)
}

var (
	_ writeSet = (*store.Store)(nil)
	_ writeSet = (*store.Tx)(nil)
)

func (e *Engine) saveRun(run store.Run, m *runMeta) error {
	return e.saveRunOn(e.store, run, m)
}

func (e *Engine) saveRunOn(w writeSet, run store.Run, m *runMeta) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	status := runStatusFor(m)
	var ended *string
	if m.LoopStage == string(core.LoopDone) {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		ended = &now
	}
	return w.Runs().UpdateRunState(run.ID, status, raw, ended)
}

// runStatusFor maps engine state onto the runs.status axis (AC-031).
// DERIVED mapping: Running while active, Paused while blocked,
// Repairing in error_recovery, Succeeded when done.
func runStatusFor(m *runMeta) core.RunStatus {
	switch {
	case m.LoopStage == string(core.LoopDone):
		return core.RunSucceeded
	case m.LoopStage == string(core.LoopErrorRecovery):
		return core.RunRepairing
	case m.Blocked:
		return core.RunPaused
	default:
		return core.RunRunning
	}
}

func (e *Engine) appendEvent(run store.Run, eventType string, payload map[string]any) error {
	return e.appendEventOn(e.store, run, eventType, payload)
}

func (e *Engine) appendEventOn(w writeSet, run store.Run, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = w.AppendEvent(store.LifecycleEvent{
		FeatureID: run.FeatureID,
		RunID:     sql.NullString{String: run.ID, Valid: true},
		EventType: eventType,
		Payload:   raw,
	})
	return err
}

func viewOf(run store.Run, m *runMeta, slug string) RunView {
	v := RunView{
		RunID: run.ID, FeatureID: run.FeatureID, FeatureSlug: slug,
		RunStatus: string(runStatusFor(m)), Blocked: m.Blocked,
		LoopStage: m.LoopStage, StagePlan: m.StagePlan,
		Verifier: m.Verifier, ReleaseHumanConfirmed: m.ReleaseHumanConfirmed,
		RepairLoopCount: m.RepairLoopCount, BlockedStreak: m.BlockedStreak,
		LastHeartbeatAt: m.LastHeartbeatAt, StartedAt: run.StartedAt,
		CurrentStage: m.currentStage(),
		Driver:       m.Driver, Supervisor: m.Supervisor, Budget: m.Budget,
		Todos: m.todoSummary(), PendingSteering: m.pendingSteering(),
		FromQueueItem: m.FromQueueItem,
		ParallelPlan:  m.ParallelPlan, DesignBinding: m.DesignBinding,
		Program: m.Program, Hotfix: m.Hotfix,
		MultiVersion: m.MultiVersion, MainAxis: m.MainAxis,
		Topology: m.Topology,
	}
	if m.Implement != nil {
		v.Implement = m.Implement.view()
	}
	switch {
	case m.LoopStage == string(core.LoopDone):
		v.Status = "completed"
	case m.LoopStage == string(core.LoopErrorRecovery):
		v.Status = "error_recovery"
	case m.Blocked:
		v.Status = "blocked"
	default:
		v.Status = "running"
	}
	if m.LastBlockReason != "" && m.Blocked {
		v.ReasonCode = m.LastBlockReason
	}
	if v.CurrentStage != "" {
		if skills := ProgressiveSkills[v.CurrentStage]; len(skills) > 0 {
			v.NextSkill = skills[0]
		}
	}
	return v
}

// Status returns the current view of a run (read-only).
func (e *Engine) Status(runID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// ---- CreateRun (G-01) ----

// CreateRunOptions carries the explicit create inputs. UXRequired is an
// explicit signal (flag/field) — P2a does no utterance heuristics.
type CreateRunOptions struct {
	ProjectRoot string
	Version     string
	FeatureSlug string
	Utterance   string
	UXRequired  bool
	ExecutorID  string
	// Lakes are the run's lake slugs (v1 goal run records them; the
	// parallel plan and lane routing consult the count). Scale is the
	// task-size field the lane router consults ("" = unset).
	Lakes []string
	Scale string
	// Exec optionally records the run's subprocess-family executor
	// configuration (P3c): later tick/driver/delegate invocations
	// fall back to it when they carry no command of their own.
	Exec *ExecutorConfig
}

// CreateRun resolves the feature, creates the run row, writes the
// goal_run_created event and parks the loop at governance_precheck
// (create_goal_run + the first routing transition).
func (e *Engine) CreateRun(opts CreateRunOptions) (RunView, error) {
	root := store.ResolveRootPath(opts.ProjectRoot)
	ctx, err := e.store.ResolveFeature(root, opts.FeatureSlug, opts.Version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if opts.FeatureSlug != "" {
				return RunView{}, core.NewError(ReasonFeatureNotFound, "goal.CreateRun",
					"feature "+opts.FeatureSlug+" not found under "+root)
			}
			return RunView{}, core.NewError(ReasonProjectNotRegistered, "goal.CreateRun",
				"no features registered under project root "+root)
		}
		return RunView{}, err
	}
	stages := BuildStageOrder(opts.UXRequired)
	m := &runMeta{
		Utterance:       opts.Utterance,
		UXRequired:      opts.UXRequired,
		CurrentStageIdx: 0,
		ExecutorID:      opts.ExecutorID,
		Verifier:        &VerifierInfo{RequiredForComplete: true, Present: false},
		LastHeartbeatAt: time.Now().UTC().Format(time.RFC3339Nano),
		Lakes:           opts.Lakes,
		TaskScale:       opts.Scale,
		Exec:            opts.Exec,
		// S3 (AC-173): the topology fact is part of the run's fact
		// layer from creation — recorded openly (missing included),
		// never a creation-time block.
		Topology: e.topologyFactFor(ctx),
	}
	ls := core.NewLoopState()
	if err := ls.Transition(core.LoopGovernancePrecheck, false, e.stopWhen); err != nil {
		return RunView{}, err
	}
	m.applyLoopState(ls)
	for i, s := range stages {
		status := "pending"
		if i == 0 {
			status = "active"
		}
		m.StagePlan = append(m.StagePlan, StagePlanEntry{
			Stage: s, Status: status, Skills: ProgressiveSkills[s],
			LoopStage: string(LoopStageForProductStage(s)),
		})
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return RunView{}, err
	}
	// One transaction for the whole creation unit: run row +
	// created event + initial state save + lifecycle snapshot.
	var run store.Run
	err = e.store.WithTx(func(tx *store.Tx) error {
		run, err = tx.Runs().CreateRun(store.Run{
			FeatureID: ctx.Feature.ID, RunKind: "goal",
			Status: core.RunCreated, Metadata: raw,
		})
		if err != nil {
			return err
		}
		if err := e.appendEventOn(tx, run, "goal_run_created", map[string]any{
			"utterance": opts.Utterance, "stages": stages, "ux_required": opts.UXRequired,
			"feature_slug": ctx.Feature.Slug, "loop_stage": m.LoopStage,
		}); err != nil {
			return err
		}
		if err := e.saveRunOn(tx, run, m); err != nil {
			return err
		}
		return tx.Features().UpdateLifecycle(store.Lifecycle{
			FeatureID: ctx.Feature.ID, Stage: workflowStageForProductStage(stages[0]),
			Status: core.StatusInProgress,
		})
	})
	if err != nil {
		return RunView{}, err
	}
	run, err = e.store.Runs().GetByID(run.ID)
	if err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, ctx.Feature.Slug), nil
}
