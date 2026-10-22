// RalphExecutor (P3c, G-10 real integration): the ralph lane's
// StageExecutor over harness.RalphBackend. Unlike the P2c simulated
// path (a RecordingExecutor call labelled simulated everywhere),
// this executor REALLY loops: the configured command runs once per
// iteration until it prints the completion marker on its own line
// (exit 0 + RALPH_DONE line — the convention is stated in
// internal/harness/ralph.go and the p3c report), fails, times out,
// or hits the iteration ceiling.
//
// Per iteration it writes real evidence: stdout/stderr capture
// artifacts + a stage_executed event PER ITERATION (so the trail
// shows iteration n with its own exit code) + one store checkpoint
// (ralph-iteration-N). Only the loop's COMPLETED outcome counts as a
// done stage; max-iterations returns failed with
// ralph_max_iterations_reached so Tick routes it into the repair /
// budget machinery instead of pretending progress.
package orchestrator

import (
	"context"
	"strconv"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// RalphExecutor loops a configured command as the implement stage.
type RalphExecutor struct {
	eng     *Engine
	backend *harness.RalphBackend
}

// NewRalphExecutor builds the loop executor (maxIter <= 0 = default).
func NewRalphExecutor(eng *Engine, argv []string, timeoutSec, maxIter int) *RalphExecutor {
	return &RalphExecutor{eng: eng, backend: harness.NewRalphBackend(argv, harnessTimeout(timeoutSec), maxIter)}
}

// ID returns the loop backend identity.
func (x *RalphExecutor) ID() string { return x.backend.Name() }

// Backend exposes the loop (iteration ceiling etc.).
func (x *RalphExecutor) Backend() *harness.RalphBackend { return x.backend }

// ExecuteStage runs the loop for the implement stage.
func (x *RalphExecutor) ExecuteStage(ctx context.Context, run RunView, stage string) (StageOutcome, error) {
	full, m, _, err := x.eng.loadRun(run.RunID)
	if err != nil {
		return StageOutcome{}, err
	}
	workdir, err := x.eng.executorWorkdir(full, m)
	if err != nil {
		return StageOutcome{}, err
	}
	dir, err := x.eng.artifactDirFor(full)
	if err != nil {
		return StageOutcome{}, err
	}
	var lastErr error
	loop, err := x.backend.RunLoop(ctx, harness.IterationRequest{
		RunID: run.RunID, FeatureID: run.FeatureID, Stage: stage, Lane: LaneRalph, Workdir: workdir,
	}, dir, func(iteration int, res harness.IterationResult) {
		// Per-iteration evidence + checkpoint. A checkpoint here is a
		// RESUME POINT, not bookkeeping: crashing mid-loop resumes
		// after the last finished iteration.
		if _, evErr := x.eng.recordIterationEvidence(full, stage, res, "ralph"); evErr != nil && lastErr == nil {
			lastErr = evErr
		}
		if _, _, cpErr := x.eng.Checkpoint(run.RunID, "ralph_loop", "ralph-iteration-"+strconv.Itoa(iteration)); cpErr != nil && lastErr == nil {
			lastErr = cpErr
		}
	})
	if err != nil {
		return StageOutcome{}, err // typed fail-closed (unconfigured/missing command)
	}
	if lastErr != nil {
		return StageOutcome{}, lastErr
	}
	n := len(loop.Iterations)
	summary := "ralph loop finished"
	if n > 0 {
		summary = loop.Iterations[n-1].Summary
	}
	switch {
	case loop.Completed:
		return StageOutcome{Status: "done", Summary: "ralph completed in " + strconv.Itoa(n) + " iteration(s): " + summary}, nil
	case loop.ReasonCode == core.ReasonRalphMaxIterations:
		return StageOutcome{Status: "failed", ReasonCode: core.ReasonRalphMaxIterations,
			Summary: "ralph reached the iteration ceiling (" + strconv.Itoa(x.backend.MaxIterations()) + ") without the " + harness.DoneMarker + " completion marker"}, nil
	case loop.ReasonCode == core.ReasonExecutorTimedOut:
		return StageOutcome{Status: "failed", ReasonCode: core.ReasonExecutorTimedOut, Summary: summary}, nil
	default:
		reason := loop.ReasonCode
		if reason == "" {
			reason = core.ReasonExecutorExitNonZero
		}
		return StageOutcome{Status: "failed", ReasonCode: reason, Summary: summary}, nil
	}
}
