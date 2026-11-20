// Program-level drive (issue #7): consume ProgramRoute's topological
// next batch and push one ready lake's own goal run to its release
// gate per call. UntilDone keeps going across lakes, treating a lake
// at its release gate as release-ready so the whole sea can be
// confirmed once at the end (issue: program-sea-complete).
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"beacon-go/internal/core"
)

type ProgramDriveOptions struct {
	// Executor is the stage executor selector ("" / recording /
	// subprocess / subprocess:<profile> / ralph). Empty parks the
	// lake run with stage_executor_not_registered, which is honest.
	Executor string
	Exec     *ExecutorConfig
	MaxTurns int
	// UntilDone advances ready lakes until none is startable or one
	// blocks. Completed and release-ready (awaiting_release) lakes
	// are skipped; dependents may start once their dependency is at
	// its release gate (release-ready closure).
	UntilDone bool
	// Context bounds the whole drive (host interrupt / deadline).
	// Nil keeps the legacy context.Background behavior, so existing
	// callers are unchanged.
	Context context.Context
	// MaxDurationSec bounds continuous drive wall-clock time (<=0
	// unlimited). It is the program-level counterpart of the driver's
	// --max-duration-sec.
	MaxDurationSec int
	// TurnTimeoutSec bounds one lake's drive step (<=0 no per-step
	// deadline). It is the program-level counterpart of the driver's
	// --turn-timeout-sec.
	TurnTimeoutSec int
}

func (o ProgramDriveOptions) ctx() context.Context {
	if o.Context != nil {
		return o.Context
	}
	return context.Background()
}

type ProgramDriveView struct {
	RunID         string           `json:"run_id"`
	ProgramSlug   string           `json:"program_slug"`
	ProgramStatus string           `json:"program_status"`
	AdvancedLake  string           `json:"advanced_lake,omitempty"`
	LakeRunID     string           `json:"lake_run_id,omitempty"`
	LakeStatus    string           `json:"lake_status,omitempty"`
	ReasonCode    string           `json:"reason_code,omitempty"`
	Route         ProgramRouteView `json:"route"`
}

var programFlowSteps = []string{
	"support_advisory", "canonical_truth_authoring", "canonical_refreeze",
	"canonical_commit", "feature_worktree_reconcile", "governance_verify",
	"workspace_admission", "implement_resume",
}

// advanceFlowForDrive runs the deterministic standard-flow steps. A
// blocked step returns its reason code; already-done steps are no-ops.
func (e *Engine) advanceFlowForDrive(runID string) (string, error) {
	for _, step := range programFlowSteps {
		v, err := e.FlowAdvance(runID, step)
		if err != nil {
			// A failed flow step is a blocked lake, not a crash:
			// record the reason and stop this drive.
			return driveReasonCode(err), nil
		}
		if v.Blocked {
			return v.ReasonCode, nil
		}
	}
	return "", nil
}

// ProgramDrive advances at most one ready lake, or every ready lake
// when UntilDone is set.
func (e *Engine) ProgramDrive(hostRunID string, opts ProgramDriveOptions) (ProgramDriveView, error) {
	_, m, _, err := e.loadRun(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	if m.Program != nil && m.Program.Mode == ProgramModeIntegrated {
		unlock, err := e.lockProgram(hostRunID)
		if err != nil {
			return ProgramDriveView{}, err
		}
		defer unlock()
	}
	if opts.UntilDone {
		return e.programDriveUntilDone(hostRunID, opts)
	}
	route, err := e.ProgramRoute(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	for _, slug := range route.NextBatch {
		v, err := e.driveOneLake(hostRunID, slug, opts)
		if err != nil {
			return ProgramDriveView{}, err
		}
		if v.AdvancedLake != "" {
			return v, nil
		}
	}
	return ProgramDriveView{RunID: hostRunID, ProgramStatus: route.ProgramStatus, Route: route}, nil
}

func (e *Engine) programDriveUntilDone(hostRunID string, opts ProgramDriveOptions) (ProgramDriveView, error) {
	ctx := opts.ctx()
	if opts.MaxDurationSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.MaxDurationSec)*time.Second)
		defer cancel()
	}
	var lastLake, lastStatus, lastRollup string
	for {
		if rc := driveContextReason(ctx); rc != "" {
			route, _ := e.ProgramRoute(hostRunID)
			return ProgramDriveView{
				RunID: hostRunID, ProgramStatus: route.ProgramStatus,
				Route: route, ReasonCode: rc,
			}, nil
		}
		order, route, err := e.driveOrder(hostRunID)
		if err != nil {
			return ProgramDriveView{}, err
		}
		if len(order) == 0 {
			return ProgramDriveView{
				RunID: hostRunID, ProgramSlug: route.ProgramSlug,
				ProgramStatus: route.ProgramStatus, Route: route,
			}, nil
		}
		opts.Context = ctx
		v, err := e.driveOneLake(hostRunID, order[0], opts)
		if err != nil {
			return ProgramDriveView{}, err
		}
		// A deadline that fired during this round wins over any
		// reason the round reported: the round was cut short.
		if rc := driveContextReason(ctx); rc != "" {
			v.ReasonCode = rc
			return v, nil
		}
		if v.LakeStatus == "blocked" {
			return v, nil
		}
		// A lake that keeps reporting "running" without moving the
		// route is a stalled loop, not progress: stop and say so
		// instead of spinning until the deadline.
		rollup := fmt.Sprintf("%d/%d/%d/%d", v.Route.Rollup.Completed,
			v.Route.Rollup.AwaitingRelease, v.Route.Rollup.Blocked, v.Route.Rollup.Pending)
		cur := driveProgress{lake: v.AdvancedLake, status: v.LakeStatus, rollup: rollup}
		if driveStalled(driveProgress{lake: lastLake, status: lastStatus, rollup: lastRollup}, cur) {
			v.ReasonCode = core.ReasonProgramDriveNoProgress
			return v, nil
		}
		lastLake, lastStatus, lastRollup = cur.lake, cur.status, cur.rollup
		// completed / awaiting_release / running: keep the sea moving.
	}
}

// driveProgress is one loop round's fingerprint. A round that keeps
// the same lake, status and rollup has made no progress.
type driveProgress struct {
	lake, status, rollup string
}

// driveStalled reports the loop's stop rule: a "running" round that is
// identical to the previous one is a stall, not progress.
func driveStalled(prev, cur driveProgress) bool {
	return prev.lake != "" && cur.status == "running" &&
		cur.lake == prev.lake && cur.status == prev.status && cur.rollup == prev.rollup
}

// driveContextReason maps a finished context to the drive's stable
// reason code ("" while the context is still live).
func driveContextReason(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	switch ctx.Err() {
	case context.DeadlineExceeded:
		return core.ReasonProgramDriveTimeout
	case context.Canceled:
		return core.ReasonHostInterrupted
	}
	return ""
}

// driveOrder returns the startable lakes in topological order. A
// dependency counts as closed when it is completed OR at its release
// gate (release-ready): the final program-level signature releases
// them together, so dependents may work in parallel with the wait.
func (e *Engine) driveOrder(hostRunID string) ([]string, ProgramRouteView, error) {
	route, err := e.ProgramRoute(hostRunID)
	if err != nil {
		return nil, ProgramRouteView{}, err
	}
	bySlug := map[string]LakeRoute{}
	for _, row := range route.Lakes {
		bySlug[row.Lake] = row
	}
	var out []string
	for _, row := range route.Lakes { // topological order
		if row.ProgramStatus == "completed" || row.ProgramStatus == "awaiting_release" {
			continue
		}
		if row.Status == LakeBlocked {
			continue
		}
		// Integrated delivery: the shared start judgement (verified
		// upstream handoff) decides readiness, not lake completion.
		if route.Integrated {
			if row.Status == LakeReady || row.Status == LakeAdmitted {
				out = append(out, row.Lake)
			}
			continue
		}
		depsReady := true
		for _, dep := range row.DependsOn {
			d := bySlug[dep]
			if d.ProgramStatus != "completed" && d.ProgramStatus != "awaiting_release" {
				depsReady = false
			}
		}
		if depsReady {
			out = append(out, row.Lake)
		}
	}
	return out, route, nil
}

// driveOneLake pushes one lake to its release gate. It returns an
// empty AdvancedLake when the lake is already terminal or
// release-ready, so callers can move on.
func (e *Engine) driveOneLake(hostRunID, slug string, opts ProgramDriveOptions) (ProgramDriveView, error) {
	ctx := opts.ctx()
	if opts.TurnTimeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(opts.TurnTimeoutSec)*time.Second)
		defer cancel()
	}
	route, err := e.ProgramRoute(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	run, m, _, err := e.loadRun(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	if m.Program == nil {
		return ProgramDriveView{}, core.NewError(core.ReasonProgramPlanMissing, "goal.ProgramDrive",
			"run "+hostRunID+" has no program plan")
	}
	view := ProgramDriveView{RunID: hostRunID, ProgramSlug: m.Program.Slug, Route: route}
	lake := m.Program.lake(slug)
	if lake == nil {
		return view, nil
	}
	if lake.ProgramStatus == "completed" || lake.ProgramStatus == "blocked" || lake.ProgramStatus == "awaiting_release" {
		return view, nil
	}

	finish := func(lake *ProgramLake, lakeRunID, status, reason string) (ProgramDriveView, error) {
		if lake != nil {
			lake.ProgramStatus = status
			lake.ReasonCode = reason
			lake.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if lakeRunID != "" {
				lake.RunID = lakeRunID
			}
		}
		m.Program.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		if err := e.saveRun(run, m); err != nil {
			return ProgramDriveView{}, err
		}
		fresh, ferr := e.ProgramRoute(hostRunID)
		if ferr == nil {
			view.Route = fresh
			view.ProgramStatus = fresh.ProgramStatus
		}
		if lake != nil {
			view.AdvancedLake = lake.Slug
			view.LakeRunID = lake.RunID
			view.LakeStatus = status
		}
		view.ReasonCode = reason
		return view, nil
	}

	// AC-001: single-step drive shares the route's start judgement.
	if m.Program.Mode == ProgramModeIntegrated {
		live := e.livePlan(ctx, run, m.Program)
		if block := integratedStartBlock(live, slug); block != "" {
			return finish(lake, lake.RunID, "blocked", block)
		}
	}

	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	if !ok {
		return ProgramDriveView{}, core.NewError(ReasonProjectNotRegistered, "goal.ProgramDrive",
			"no feature context for host run "+hostRunID)
	}

	lakeRunID := lake.RunID
	if lakeRunID == "" {
		// Reuse an already running lake run before creating one.
		feat, ferr := e.lakeFeature(run, slug)
		if ferr != nil {
			return finish(lake, "", "blocked", core.ReasonProgramLakeFeatureMiss)
		}
		runs, _ := e.store.Runs().ListByFeature(feat.ID)
		for _, r := range runs {
			if r.Status == core.RunRunning || r.Status == core.RunCreated {
				lakeRunID = r.ID
			}
		}
		if lakeRunID == "" {
			created, cerr := e.CreateRun(CreateRunOptions{
				ProjectRoot: fc.Project.RootPath, Version: fc.Version.Version,
				FeatureSlug: slug, Utterance: "program drive: " + slug,
				ExecutorID: m.ExecutorID, Exec: opts.Exec,
			})
			if cerr != nil {
				return finish(lake, "", "blocked", driveReasonCode(cerr))
			}
			lakeRunID = created.RunID
		}
		lake.RunID = lakeRunID
	}

	// Admission is the shared per-lake judgement; one lake at a
	// time so a blocked dependency cannot leak progress.
	adm, aerr := e.ProgramAdmission(hostRunID, slug)
	if aerr != nil {
		return ProgramDriveView{}, aerr
	}
	// ProgramAdmission saved its own snapshot; reload ours.
	run, m, _, err = e.loadRun(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	lake = m.Program.lake(slug)
	if lake == nil {
		return view, nil
	}
	lake.RunID = lakeRunID
	if len(adm.Lakes) == 0 || !adm.Lakes[0].Admitted {
		reason := core.ReasonProgramTruthNotFrozen
		if len(adm.Lakes) > 0 && len(adm.Lakes[0].ReasonCodes) > 0 {
			reason = adm.Lakes[0].ReasonCodes[0]
		}
		return finish(lake, lakeRunID, "blocked", reason)
	}

	// A lake already at its release gate (or done) needs no
	// further flow/driver work; report the state and return.
	if stage, _ := e.lakeRunStage(lakeRunID); stage == string(core.LoopReleaseGate) {
		_ = e.ensureVerifier(lakeRunID)
		return finish(lake, lakeRunID, "awaiting_release", "")
	} else if stage == string(core.LoopDone) {
		return finish(lake, lakeRunID, "completed", "")
	}

	// Resolve and register the executor for the lake run.
	ex, xerr := e.ResolveExecutorFor(lakeRunID, opts.Executor, opts.Exec)
	if xerr != nil {
		return finish(lake, lakeRunID, "blocked", driveReasonCode(xerr))
	}
	if ex != nil {
		e.RegisterExecutorForAllStages(ex)
	}
	if blockReason, ferr := e.advanceFlowForDrive(lakeRunID); ferr != nil {
		return ProgramDriveView{}, ferr
	} else if blockReason != "" {
		return finish(lake, lakeRunID, "blocked", blockReason)
	}

	// Arm the driver when it is not already running, then push.
	if st, serr := e.DriverStatus(lakeRunID); serr == nil {
		if st.Driver == nil || st.Driver.terminal() {
			if _, derr := e.DriverStart(lakeRunID, DriverStartOptions{MaxTurns: opts.MaxTurns}); derr != nil {
				if rc, _ := core.ReasonCodeOf(derr); rc != core.ReasonDriverDualStart {
					return finish(lake, lakeRunID, "blocked", driveReasonCode(derr))
				}
			}
		}
	}
	if _, derr := e.DriverRunLoop(ctx, lakeRunID, opts.MaxTurns, nil); derr != nil {
		// A deadline/cancel is not a lake block: report it as the
		// drive's reason so UntilDone stops with a stable code.
		if rc := driveContextReason(ctx); rc != "" {
			view.ReasonCode = rc
			view.AdvancedLake = slug
			view.LakeRunID = lakeRunID
			view.LakeStatus = "running"
			return view, nil
		}
		return finish(lake, lakeRunID, "blocked", driveReasonCode(derr))
	}

	_, lm, _, lerr := e.loadRun(lakeRunID)
	if lerr != nil {
		return ProgramDriveView{}, lerr
	}
	switch lm.LoopStage {
	case string(core.LoopReleaseGate):
		_ = e.ensureVerifier(lakeRunID)
		return finish(lake, lakeRunID, "awaiting_release", "")
	case string(core.LoopDone):
		return finish(lake, lakeRunID, "completed", "")
	default:
		if lm.Blocked {
			return finish(lake, lakeRunID, "blocked", lm.LastBlockReason)
		}
		return finish(lake, lakeRunID, "running", "")
	}
}

// ensureVerifier attaches a verifier backed by the lake run's newest
// QA artifact when none is present. It is best-effort during drive;
// ProgramComplete refuses when it cannot find one.
func (e *Engine) ensureVerifier(runID string) error {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return err
	}
	if m.Verifier != nil && m.Verifier.Present && len(m.Verifier.Refs) > 0 {
		return nil
	}
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return err
	}
	ref := ""
	for _, a := range arts {
		if qaKind(a.Kind) {
			ref = a.AbsPath
		}
	}
	if ref == "" {
		return core.NewError(ReasonVerifierRequired, "program.ensureVerifier",
			"lake run "+runID+" has no QA artifact to verify against")
	}
	_, err = e.AttachVerifier(runID, []string{ref}, "qa-bot", m.ExecutorID)
	return err
}

// driveReasonCode extracts the stable reason code, falling back to
// the machine-readable text.
func driveReasonCode(err error) string {
	if err == nil {
		return ""
	}
	if rc, ok := core.ReasonCodeOf(err); ok {
		return rc
	}
	msg := err.Error()
	if i := strings.Index(msg, ":"); i > 0 {
		return strings.TrimSpace(msg[:i])
	}
	return msg
}
