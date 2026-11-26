// Executor resolution. The kernel runs no agent (spec #15 / ADR-0002):
//
//	"" / "none"  -> no executor; ticks block with stage_executor_not_registered
//	"recording"  -> a no-work placeholder, labelled simulated in its evidence
//	anything else -> refused with stage_executor_not_registered
//
// Real work is done by the host agent and recorded with `goal tick --by`.
// The exec configuration (--exec-cmd / --ralph-cmd / --profile) is still
// merged and stored on the run as host-facing metadata; it never selects
// an executable backend here.
package orchestrator

import (
	"beacon-go/internal/core"
)

// mergeExecConfig overlays the invocation config on the run record:
// non-empty invocation fields win.
func mergeExecConfig(inv, recorded *ExecutorConfig) *ExecutorConfig {
	out := &ExecutorConfig{}
	if recorded != nil {
		*out = *recorded
	}
	if inv != nil {
		if len(inv.Argv) > 0 {
			out.Argv = inv.Argv
		}
		if inv.TimeoutSec > 0 {
			out.TimeoutSec = inv.TimeoutSec
		}
		if len(inv.RalphArgv) > 0 {
			out.RalphArgv = inv.RalphArgv
		}
		if inv.RalphMaxIter > 0 {
			out.RalphMaxIter = inv.RalphMaxIter
		}
		if inv.Profile != "" {
			out.Profile = inv.Profile
		}
		if len(inv.ProfileArgv) > 0 {
			out.ProfileArgv = inv.ProfileArgv
		}
		if inv.ReviewerProfile != "" {
			out.ReviewerProfile = inv.ReviewerProfile
		}
		if len(inv.ReviewerArgv) > 0 {
			out.ReviewerArgv = inv.ReviewerArgv
		}
	}
	return out
}

// ExecutorConfigFor returns the run's merged executor configuration
// (invocation over record) without resolving any executor.
func (e *Engine) ExecutorConfigFor(runID string, inv *ExecutorConfig) (*ExecutorConfig, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return nil, err
	}
	return mergeExecConfig(inv, m.Exec), nil
}

// ResolveExecutorFor resolves the selector against the run's merged
// configuration. A nil executor with nil error means "none
// registered" — by design (see the grammar above). The special lane
// selector "ralph" resolves the run's ralph loop as a lane executor
// for ExecuteStage use; DelegateRalphReal is the delegation path
// and does not go through here.
func (e *Engine) ResolveExecutorFor(runID, selector string, inv *ExecutorConfig) (StageExecutor, error) {
	if selector == ExecutorModeRecording {
		return NewRecordingExecutor(e.store, ExecutorModeRecording), nil
	}
	return nil, core.NewError(ReasonExecutorNotReg, "goal.ResolveExecutorFor",
		"built-in executors were removed; use the manual/external lane and `goal tick --by`")
}
