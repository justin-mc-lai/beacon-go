package harness

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
)

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	rc, ok := core.ReasonCodeOf(err)
	if !ok {
		t.Fatalf("error %v carries no reason code", err)
	}
	return rc
}

// AC-044: the subprocess backend runs ONE real iteration and returns
// the AC-042 schema (success + summary + agent name).
func TestSubprocessBackendRealIteration(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "agent.sh", "#!/bin/sh\necho did-the-work\nexit 0\n")
	b := NewSubprocessBackend([]string{script}, 0)
	res, err := b.Iterate(context.Background(), IterationRequest{RunID: "r1", Stage: "plan", Workdir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.ExitCode != 0 || res.AgentName != b.Name() {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(res.StdoutText, "did-the-work") || res.Summary != "did-the-work" {
		t.Errorf("stdout/summary = %q / %q", res.StdoutText, res.Summary)
	}
	if hs := b.HealthCheck(context.Background()); !hs.OK {
		t.Errorf("health = %+v", hs)
	}
}

// Non-zero exit is a failed iteration with the REAL exit code.
func TestSubprocessBackendNonZero(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", "#!/bin/sh\necho nope >&2\nexit 3\n")
	b := NewSubprocessBackend([]string{script}, 0)
	res, err := b.Iterate(context.Background(), IterationRequest{Workdir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || res.ExitCode != 3 {
		t.Errorf("res = %+v, want failure with exit 3", res)
	}
}

// Timeout kills the child and marks the iteration timed out.
func TestSubprocessBackendTimeout(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "slow.sh", "#!/bin/sh\nsleep 5\n")
	b2 := NewSubprocessBackend([]string{script}, 300*time.Millisecond)
	res, err := b2.Iterate(context.Background(), IterationRequest{Workdir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if res.Success || !res.TimedOut {
		t.Errorf("res = %+v, want timed-out failure", res)
	}
}

// Missing command: typed fail-closed, executor_command_missing.
func TestSubprocessBackendMissingCommand(t *testing.T) {
	b := NewSubprocessBackend([]string{"/nonexistent/beacon-nope-xyz"}, 0)
	_, err := b.Iterate(context.Background(), IterationRequest{Workdir: t.TempDir()})
	if got := reasonOf(t, err); got != core.ReasonExecutorCommandMissing {
		t.Errorf("reason = %s", got)
	}
	if hs := b.HealthCheck(context.Background()); hs.OK {
		t.Error("health must fail for a missing command")
	}
}

// Non-executable file: executor_command_not_executable.
func TestSubprocessBackendNotExecutable(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notexec.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := NewSubprocessBackend([]string{p}, 0)
	_, err := b.Iterate(context.Background(), IterationRequest{Workdir: dir})
	if got := reasonOf(t, err); got != core.ReasonExecutorCommandNotExec {
		t.Errorf("reason = %s", got)
	}
}

// No argv at all: executor_not_configured.
func TestSubprocessBackendUnconfigured(t *testing.T) {
	b := NewSubprocessBackend(nil, 0)
	_, err := b.Iterate(context.Background(), IterationRequest{Workdir: t.TempDir()})
	if got := reasonOf(t, err); got != core.ReasonExecutorNotConfigured {
		t.Errorf("reason = %s", got)
	}
}

// The ralph completion marker must be a line of its own.
func TestContainsDoneMarker(t *testing.T) {
	if !ContainsDoneMarker("work\nRALPH_DONE\n") {
		t.Error("bare line must match")
	}
	if !ContainsDoneMarker("  RALPH_DONE  \n") {
		t.Error("whitespace-padded line must match")
	}
	if ContainsDoneMarker("RALPH_DONE-ish\n") || ContainsDoneMarker("echo RALPH_DONE\n") {
		t.Error("substring must not match")
	}
}

// Profile layer: the template's {context} resolves to the stage
// context file and the profile command really runs with it.
func TestProfileBackendResolvesContext(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "prof.sh", "#!/bin/sh\ntest -f \"$1\" || exit 9\ngrep -q '\"stage\": \"implement\"' \"$1\" || exit 8\necho profile-ran\nexit 0\n")
	ctxPath, err := WriteStageContext(dir, StageContext{RunID: "r1", Stage: "implement", Workdir: dir, Prompt: "go"})
	if err != nil {
		t.Fatal(err)
	}
	prof := Profile{Name: "test-style", Argv: []string{script, "{context}"}}
	b := NewProfileBackend(prof, ctxPath, "go", 0)
	res, err := b.Iterate(context.Background(), IterationRequest{RunID: "r1", Stage: "implement", Workdir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || res.AgentName != "subprocess:test-style" {
		t.Errorf("res = %+v", res)
	}
	// A template needing {context} with none built refuses.
	b2 := NewProfileBackend(prof, "", "go", 0)
	_, err = b2.Iterate(context.Background(), IterationRequest{Workdir: dir})
	if got := reasonOf(t, err); got != core.ReasonExecutorProfileIncomplete {
		t.Errorf("reason = %s", got)
	}
}

// ACP probe: a target that answers garbage gets Responded=false,
// not a crash and not a pass.
func TestACPProbeGarbageTarget(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "noise.sh", "#!/bin/sh\necho hello-not-jsonrpc\nsleep 0.2\n")
	res, err := ProbeACP(context.Background(), []string{script}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Responded {
		t.Errorf("probe = %+v, want no response", res)
	}
}

// ACP probe, two-step (P4c): a target that answers initialize but
// refuses session/new is reported as stopped at session/new — the
// initialize answer is not dressed up as session support.
func TestACPProbeSessionNewRefused(t *testing.T) {
	dir := t.TempDir()
	agent := writeScript(t, dir, "fake-acp1.py", `#!/usr/bin/env python3
import sys, json
for line in sys.stdin:
    req = json.loads(line)
    method = req.get("method")
    if method == "initialize":
        resp = {"jsonrpc": "2.0", "id": req["id"], "result": {"protocolVersion": 1, "agentCapabilities": {}, "authMethods": []}}
    elif method == "session/new":
        resp = {"jsonrpc": "2.0", "id": req["id"], "error": {"code": -32601, "message": "method not found"}}
    else:
        continue
    sys.stdout.write(json.dumps(resp) + "\n")
    sys.stdout.flush()
`)
	res, err := ProbeACP(context.Background(), []string{agent}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Responded || res.SessionCreated || res.StoppedAt != "session/new" {
		t.Errorf("probe = %+v, want initialize answered, session refused, stopped_at=session/new", res)
	}
	if res.ReasonCode != core.ReasonACPProbeFailed {
		t.Errorf("reason = %q, want %q", res.ReasonCode, core.ReasonACPProbeFailed)
	}
}
