// Executor resolution (P3c): one place turns an --executor selector
// string plus a configuration into a StageExecutor — or refuses
// honestly. Selector grammar:
//
//	"" / "none"            -> no executor (nil, nil): the engine blocks
//	                          with stage_executor_not_registered, exactly
//	                          like P2a — an absent executor is never
//	                          quietly replaced by a placeholder.
//	"recording"            -> the RecordingExecutor placeholder (labelled
//	                          simulated in its own evidence; kept from P2a).
//	"subprocess"           -> the real subprocess backend over cfg.Argv;
//	                          no argv configured -> executor_not_configured
//	                          (NEVER a fallback to recording).
//	"subprocess:<profile>" -> the real profile (harness) backend over
//	                          the named profile (codex-style |
//	                          claude-style, or cfg.Profile's name), with
//	                          cfg.ProfileArgv / cfg.Argv as its command.
//
// The cfg argument is per-invocation (CLI flags); when it carries no
// command, the run's recorded Exec (m.Exec) fills the gap, so a run
// created with --exec-cmd can be ticked later without repeating the
// command. Configuration precedence: invocation > run record >
// profile default binary name (which then fails honestly at
// selection when the binary is absent).
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
