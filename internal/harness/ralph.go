package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"beacon-go/internal/core"
)

// Ralph loop (G-10 real integration; AC-043's multi-iteration shape
// applied to the ralph delegatee): the configured command is called
// again and again, one iteration per call, until it signals
// completion or the iteration ceiling is reached.
//
// COMPLETION CONVENTION (the contract, also in the p3c report): an
// iteration completes the loop iff the command exits 0 AND its
// stdout contains a line that is exactly RALPH_DONE (whitespace
// tolerated). A non-zero exit or a timeout ends the loop as FAILED —
// a failing iteration is not retried silently, it surfaces as a
// stage failure like any other. Reaching the ceiling without the
// marker returns Completed=false with ralph_max_iterations_reached;
// the loop NEVER reports done without the marker.
//
// Each iteration receives: its iteration number ($BEACON_ITERATION
// and in the iteration file paths), the per-iteration context file
// (prev-result.json — the previous iteration's AC-042 result, or an
// initial seed), and the loop's own state file path
// (.beacon-ralph/state.json) inside the artifact dir. Per-iteration
// stdout/stderr are captured to iteration-N.stdout.txt /
// iteration-N.stderr.txt in the artifact dir by the orchestrator
// side (ExecuteStage), which also checkpoints every iteration.
type RalphBackend struct {
	name    string
	argv    []string
	timeout time.Duration
	maxIter int
}

// DefaultRalphMaxIterations bounds the loop when unconfigured.
const DefaultRalphMaxIterations = 8

// NewRalphBackend builds the loop backend over one argv.
func NewRalphBackend(argv []string, timeout time.Duration, maxIterations int) *RalphBackend {
	name := "ralph"
	if len(argv) > 0 {
		name = "ralph:" + filepath.Base(argv[0])
	}
	if maxIterations <= 0 {
		maxIterations = DefaultRalphMaxIterations
	}
	if timeout <= 0 {
		timeout = DefaultIterationTimeout
	}
	return &RalphBackend{name: name, argv: argv, timeout: timeout, maxIter: maxIterations}
}

// Name implements AgentBackend.
func (b *RalphBackend) Name() string { return b.name }

// Argv returns the loop command (copied).
func (b *RalphBackend) Argv() []string { return append([]string{}, b.argv...) }

// MaxIterations returns the loop ceiling.
func (b *RalphBackend) MaxIterations() int { return b.maxIter }

// HealthCheck probes the loop command like the subprocess backend.
func (b *RalphBackend) HealthCheck(ctx context.Context) HealthStatus {
	sub := NewSubprocessBackend(b.argv, b.timeout)
	hs := sub.HealthCheck(ctx)
	hs.Backend = b.name
	return hs
}

// Abort: iterations are synchronous; cancellation kills the child.
func (b *RalphBackend) Abort(ctx context.Context) error { return nil }

// RalphRun is the loop's full outcome: every iteration's result, in
// order, plus the completion verdict.
type RalphRun struct {
	Completed  bool              `json:"completed"`
	ReasonCode string            `json:"reason_code,omitempty"`
	Iterations []IterationResult `json:"iterations"`
}

// RunLoop executes the loop against req in artifactDir. onIteration
// (optional) fires after every iteration with the iteration number
// and its result — the orchestrator uses it to checkpoint and to
// append trail events per iteration. The loop state file
// (<artifactDir>/.beacon-ralph/state.json) is refreshed after every
// iteration with the latest summary, so the command itself can also
// read its own progress.
func (b *RalphBackend) RunLoop(ctx context.Context, req IterationRequest, artifactDir string, onIteration func(iteration int, res IterationResult)) (RalphRun, error) {
	if len(b.argv) == 0 {
		return RalphRun{}, core.NewError(core.ReasonRalphConfigMissing,
			"harness.RalphBackend.RunLoop", "no ralph command configured")
	}
	stateDir := filepath.Join(artifactDir, ".beacon-ralph")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return RalphRun{}, err
	}
	run := RalphRun{}
	prevPath := filepath.Join(stateDir, "prev-result.json")
	seed := []byte(`{"iteration":0,"success":false,"summary":"loop start (no previous iteration)"}` + "\n")
	if err := os.WriteFile(prevPath, seed, 0o644); err != nil {
		return RalphRun{}, err
	}
	for i := 1; i <= b.maxIter; i++ {
		iterReq := req
		iterReq.Iteration = i
		iterReq.ContextPath = prevPath
		res, err := b.Iterate(ctx, iterReq)
		if err != nil {
			return run, err
		}
		res.KeyLearnings = append(res.KeyLearnings, fmt.Sprintf("iteration %d of max %d", i, b.maxIter))
		run.Iterations = append(run.Iterations, res)
		if onIteration != nil {
			onIteration(i, res)
		}
		// Refresh the loop's own state + the next iteration's
		// prev-result input.
		raw := marshalIteration(i, res)
		_ = os.WriteFile(prevPath, raw, 0o644)
		_ = os.WriteFile(filepath.Join(stateDir, "state.json"), raw, 0o644)
		switch {
		case res.TimedOut:
			run.ReasonCode = core.ReasonExecutorTimedOut
			return run, nil
		case !res.Success:
			run.ReasonCode = core.ReasonExecutorExitNonZero
			return run, nil
		case ContainsDoneMarker(res.StdoutText):
			run.Completed = true
			res := run.Iterations[len(run.Iterations)-1]
			res.StopSignal = "done_marker:" + DoneMarker
			run.Iterations[len(run.Iterations)-1] = res
			return run, nil
		}
	}
	run.ReasonCode = core.ReasonRalphMaxIterations
	return run, nil
}

// Iterate runs ONE loop iteration (AgentBackend contract). Callers
// wanting the loop use RunLoop; Iterate exists so the ralph backend
// can also serve as a plain one-shot executor.
func (b *RalphBackend) Iterate(ctx context.Context, req IterationRequest) (IterationResult, error) {
	if len(b.argv) == 0 {
		return IterationResult{}, core.NewError(core.ReasonRalphConfigMissing,
			"harness.RalphBackend.Iterate", "no ralph command configured")
	}
	sub := NewSubprocessBackend(b.argv, b.timeout)
	sub.backendName = b.name
	res, err := sub.Iterate(ctx, req)
	if err == nil {
		res.AgentName = b.name
	}
	return res, err
}

// marshalIteration renders one iteration result for the loop state
// files. Hand-rolled field list (no reflection): the state file is
// an input to the next iteration, so its shape is a contract —
// iteration, success, summary, exit_code, timed_out.
func marshalIteration(iteration int, res IterationResult) []byte {
	summary := strings.ReplaceAll(res.Summary, `"`, `'`)
	var b strings.Builder
	b.WriteString("{\n")
	fmt.Fprintf(&b, "  \"iteration\": %d,\n", iteration)
	fmt.Fprintf(&b, "  \"success\": %v,\n", res.Success)
	fmt.Fprintf(&b, "  \"summary\": \"%s\",\n", summary)
	fmt.Fprintf(&b, "  \"agent_name\": \"%s\",\n", res.AgentName)
	fmt.Fprintf(&b, "  \"exit_code\": %d,\n", res.ExitCode)
	fmt.Fprintf(&b, "  \"timed_out\": %v,\n", res.TimedOut)
	fmt.Fprintf(&b, "  \"stop_signal\": \"%s\",\n", res.StopSignal)
	b.WriteString("  \"done_marker\": \"" + DoneMarker + "\"\n}\n")
	return []byte(b.String())
}

// ParseArgvFlag splits a --exec-cmd style value. The value is a
// SINGLE program path, never a shell string: arguments travel as
// separate --exec-arg flags. This is the one supported config shape
// so no caller can smuggle a shell pipeline into an executor.
func ParseArgvFlag(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	return []string{cmd}
}

// EnvInt reads a positive-int override (used for the ralph ceiling in
// tests); empty/invalid values return def.
func EnvInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
