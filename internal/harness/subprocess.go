package harness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"beacon-go/internal/core"
)

// IterationRequest is one backend iteration: which run/stage, where
// to work, what iteration number, and the context handoff.
type IterationRequest struct {
	RunID       string
	FeatureID   string
	Stage       string
	Lane        string // set for lane-scoped execution ("" otherwise)
	Iteration   int
	Workdir     string
	ContextPath string // stage-context file path (profile layer)
	Timeout     time.Duration
}

// IterationOutput is the raw record of one subprocess execution.
type IterationOutput struct {
	ExitCode int
	TimedOut bool
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error // typed failure unrelated to the exit code (spawn etc.)
}

// RunCommand executes argv in dir with env, capturing stdout/stderr.
//
// Argv is executed directly (exec.CommandContext) — the command is
// NEVER routed through a shell, so no argument can be re-interpreted
// by one (the anti-injection rule of this slice). Command existence
// and executability are probed before spawning and fail closed with
// typed errors: a missing command is executor_command_missing, a
// non-executable one executor_command_not_executable.
func RunCommand(ctx context.Context, argv []string, dir string, env []string, timeout time.Duration) IterationOutput {
	started := time.Now()
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return IterationOutput{ExitCode: -1, Err: core.NewError(core.ReasonExecutorCommandMissing,
			"harness.RunCommand", "empty command argv")}
	}
	resolved, err := exec.LookPath(argv[0])
	if err != nil {
		// LookPath fails both for a missing command and for a file
		// that exists but lacks the execute bit; stat splits the two
		// cases so the reason codes stay truthful.
		if _, statErr := os.Stat(argv[0]); statErr == nil {
			return IterationOutput{ExitCode: -1, Err: core.NewError(core.ReasonExecutorCommandNotExec,
				"harness.RunCommand", argv[0]+" exists but is not executable")}
		}
		return IterationOutput{ExitCode: -1, Err: core.NewError(core.ReasonExecutorCommandMissing,
			"harness.RunCommand", "command not found: "+argv[0])}
	}
	if info, statErr := os.Stat(resolved); statErr == nil && info.IsDir() {
		return IterationOutput{ExitCode: -1, Err: core.NewError(core.ReasonExecutorCommandNotExec,
			"harness.RunCommand", resolved+" is a directory, not an executable")}
	}
	if timeout <= 0 {
		timeout = DefaultIterationTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, resolved, argv[1:]...)
	cmd.Dir = dir
	cmd.Env = env
	// Kill the whole process GROUP on cancel/timeout: killing only
	// the direct child leaves grandchildren (e.g. a `sleep` the agent
	// spawned) holding the stdout pipe open, and cmd.Run would block
	// on the pipe long past the timeout. Setpgid + group SIGKILL is
	// what "timeout kills the process" must mean.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	out := IterationOutput{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started), ExitCode: -1}
	if cmd.ProcessState != nil {
		out.ExitCode = cmd.ProcessState.ExitCode()
	}
	if runCtx.Err() == context.DeadlineExceeded {
		out.TimedOut = true
		if out.ExitCode < 0 {
			out.ExitCode = -1
		}
		return out
	}
	if runErr != nil {
		if _, ok := runErr.(*exec.ExitError); !ok {
			out.Err = runErr
		}
	}
	return out
}

// resultOf maps one subprocess execution onto the AC-042 schema.
// Token usage is 0: a subprocess exposes no token accounting, and
// fabricating a number would be a lie — 0 means "not reported".
func resultOf(agentName string, out IterationOutput) IterationResult {
	res := IterationResult{
		Success:   out.Err == nil && !out.TimedOut && out.ExitCode == 0,
		AgentName: agentName, ExitCode: out.ExitCode, TimedOut: out.TimedOut,
		StdoutText: out.Stdout, StderrText: out.Stderr,
	}
	res.Summary = iterationSummary(out)
	if res.Success {
		res.KeyChangesMade = []string{fmt.Sprintf("stage command exited 0 (%s)", agentName)}
	}
	return res
}

func iterationSummary(out IterationOutput) string {
	switch {
	case out.Err != nil:
		return "spawn failed: " + out.Err.Error()
	case out.TimedOut:
		return "iteration timed out and the process was killed"
	case out.ExitCode != 0:
		return fmt.Sprintf("command exited %d", out.ExitCode)
	default:
		// Prefer the command's last non-empty stdout line as the
		// human summary; fall back to a bare success statement.
		lines := strings.Split(strings.TrimRight(out.Stdout, "\n"), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if s := strings.TrimSpace(lines[i]); s != "" {
				return s
			}
		}
		return "command exited 0"
	}
}

// ContainsDoneMarker reports whether stdout carries the ralph
// completion marker as a line of its own (whitespace tolerated).
func ContainsDoneMarker(stdout string) bool {
	for _, line := range strings.Split(stdout, "\n") {
		if strings.TrimSpace(line) == DoneMarker {
			return true
		}
	}
	return false
}

// SubprocessBackend is the plain subprocess backend (AC-044): one
// configured argv, one iteration per call, AC-042 result out.
type SubprocessBackend struct {
	backendName string
	argv        []string
	timeout     time.Duration
}

// NewSubprocessBackend builds the backend; argv is validated lazily
// at iterate time so construction never masks a configuration error
// as a runtime one. name defaults to the argv's base name.
func NewSubprocessBackend(argv []string, timeout time.Duration) *SubprocessBackend {
	name := "subprocess"
	if len(argv) > 0 {
		name = "subprocess:" + filepath.Base(argv[0])
	}
	return &SubprocessBackend{backendName: name, argv: argv, timeout: timeout}
}

// Name implements AgentBackend.
func (b *SubprocessBackend) Name() string { return b.backendName }

// Argv returns the configured command (copied).
func (b *SubprocessBackend) Argv() []string { return append([]string{}, b.argv...) }

// Timeout returns the per-iteration timeout.
func (b *SubprocessBackend) Timeout() time.Duration { return b.timeout }

// HealthCheck probes the configured command: configured, resolvable,
// not a directory. It never spawns the command.
func (b *SubprocessBackend) HealthCheck(ctx context.Context) HealthStatus {
	if len(b.argv) == 0 {
		return HealthStatus{Backend: b.backendName, OK: false, Detail: core.ReasonExecutorNotConfigured}
	}
	resolved, err := exec.LookPath(b.argv[0])
	if err != nil {
		return HealthStatus{Backend: b.backendName, OK: false, Detail: err.Error()}
	}
	if info, statErr := os.Stat(resolved); statErr == nil && info.IsDir() {
		return HealthStatus{Backend: b.backendName, OK: false, Detail: resolved + " is a directory"}
	}
	return HealthStatus{Backend: b.backendName, OK: true, Detail: resolved}
}

// Abort is a no-op for the subprocess backend: iterations are
// synchronous and context cancellation (including the timeout)
// already kills the child via exec.CommandContext.
func (b *SubprocessBackend) Abort(ctx context.Context) error { return nil }

// Iterate runs the argv once in req.Workdir and maps the outcome
// onto the AC-042 result schema.
func (b *SubprocessBackend) Iterate(ctx context.Context, req IterationRequest) (IterationResult, error) {
	if len(b.argv) == 0 {
		return IterationResult{}, core.NewError(core.ReasonExecutorNotConfigured,
			"harness.SubprocessBackend.Iterate", "no command configured for the subprocess backend")
	}
	env := IterationEnv(req)
	out := RunCommand(ctx, b.argv, req.Workdir, env, b.timeout)
	if out.Err != nil {
		return IterationResult{}, out.Err
	}
	return resultOf(b.backendName, out), nil
}

// DefaultIterationTimeout bounds one iteration when nothing else is
// configured (one hour, mirroring the driver default turn timeout).
const DefaultIterationTimeout = time.Hour

// IterationEnv builds the environment an iteration runs with: the
// ambient environment plus the BEACON_* handoff variables. The stage
// context travels by file (ContextPath), not by env dump — env only
// carries pointers and scalars.
func IterationEnv(req IterationRequest) []string {
	env := os.Environ()
	set := func(k, v string) {
		env = append(env, k+"="+v)
	}
	set("BEACON_RUN_ID", req.RunID)
	set("BEACON_FEATURE_ID", req.FeatureID)
	set("BEACON_STAGE", req.Stage)
	set("BEACON_ITERATION", strconv.Itoa(req.Iteration))
	if req.Lane != "" {
		set("BEACON_LANE", req.Lane)
	}
	if req.ContextPath != "" {
		set("BEACON_CONTEXT_FILE", req.ContextPath)
	}
	return env
}
