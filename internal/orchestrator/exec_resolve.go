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
	"context"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
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
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return nil, err
	}
	cfg := mergeExecConfig(inv, m.Exec)
	switch {
	case selector == "" || selector == "none":
		return nil, nil
	case selector == "recording":
		return NewRecordingExecutor(e.store, "recording"), nil
	case selector == "ralph":
		if len(cfg.RalphArgv) == 0 {
			return nil, core.NewError(core.ReasonRalphConfigMissing, "goal.ResolveExecutorFor",
				"selector ralph needs a configured ralph command (--ralph-cmd or goal run --ralph-cmd)")
		}
		return NewRalphExecutor(e, cfg.RalphArgv, cfg.TimeoutSec, cfg.RalphMaxIter), nil
	case selector == "subprocess":
		if len(cfg.Argv) == 0 {
			return nil, core.NewError(core.ReasonExecutorNotConfigured, "goal.ResolveExecutorFor",
				"subprocess executor selected but no command configured (--exec-cmd); refusing to fall back to a placeholder")
		}
		return NewSubprocessExecutor(e, cfg.Argv, cfg.TimeoutSec), nil
	case strings.HasPrefix(selector, "subprocess:"):
		name := strings.TrimPrefix(selector, "subprocess:")
		if name == "" {
			name = cfg.Profile
		}
		if name == "" {
			return nil, core.NewError(core.ReasonExecutorProfileUnknown, "goal.ResolveExecutorFor",
				"subprocess: needs a profile name (codex-style|claude-style)")
		}
		prof, ok := harness.LookupProfile(name)
		if !ok {
			return nil, core.NewError(core.ReasonExecutorProfileUnknown, "goal.ResolveExecutorFor",
				"unknown profile "+name+" (built-ins: codex-style, claude-style)")
		}
		argv := cfg.ProfileArgv
		if len(argv) == 0 {
			argv = cfg.Argv
		}
		prof = prof.WithCommand(argv)
		if len(argv) == 0 {
			// No explicit command anywhere: only proceed when the
			// profile's canonical binary actually exists; otherwise
			// the machine is simply not configured for this profile.
			probe := harness.NewSubprocessBackend([]string{prof.Argv[0]}, 0)
			if hs := probe.HealthCheck(context.Background()); !hs.OK {
				return nil, core.NewError(core.ReasonExecutorNotConfigured, "goal.ResolveExecutorFor",
					"profile "+name+" selected but its command "+prof.Argv[0]+" is not available and no --exec-cmd override was given")
			}
		}
		return NewProfileExecutor(e, prof, cfg.TimeoutSec), nil
	default:
		return nil, core.NewError(core.ReasonExecutorProfileUnknown, "goal.ResolveExecutorFor",
			"unknown executor selector "+selector+" (want recording|subprocess|subprocess:<profile>|ralph)")
	}
}

// ExecuteProfileIteration runs ONE profile iteration outside the
// stage machine — the AC-100 integration surface: cwd is the run's
// bound ACTIVE real worktree when one exists (harness-in-worktree),
// the stage-context file is built and registered, evidence lands
// like a stage execution. stage is recorded as "implement" so the
// harvest rule sees it as implement evidence.
func (e *Engine) ExecuteProfileIteration(ctx context.Context, runID, profileName string, inv *ExecutorConfig) (StageOutcome, harness.Profile, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return StageOutcome{}, harness.Profile{}, err
	}
	cfg := mergeExecConfig(inv, m.Exec)
	if profileName == "" {
		profileName = cfg.Profile
	}
	if profileName == "" {
		profileName = "claude-style"
	}
	prof, ok := harness.LookupProfile(profileName)
	if !ok {
		return StageOutcome{}, harness.Profile{}, core.NewError(core.ReasonExecutorProfileUnknown,
			"goal.ExecuteProfileIteration", "unknown profile "+profileName)
	}
	argv := cfg.ProfileArgv
	if len(argv) == 0 {
		argv = cfg.Argv
	}
	prof = prof.WithCommand(argv)
	ex := NewProfileExecutor(e, prof, cfg.TimeoutSec)
	outcome, err := ex.ExecuteStage(ctx, RunView{RunID: runID, FeatureID: featureIDOf(e, runID)}, StageImplement)
	return outcome, prof, err
}

func featureIDOf(e *Engine, runID string) string {
	run, err := e.store.Runs().GetByID(runID)
	if err != nil {
		return ""
	}
	return run.FeatureID
}
