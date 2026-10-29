package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/orchestrator"
)

// TestExecutorsListBuiltinsHonestly (AC-006): the shell is offered the
// built-in agent backends and told which ones can actually start on
// this machine. An unavailable backend carries a reason instead of
// being silently offered and then failing at run time.
func TestExecutorsListBuiltinsHonestly(t *testing.T) {
	svc := newTestService(t)
	opts := svc.Executors()
	if len(opts) == 0 {
		t.Fatal("no built-in executors listed")
	}
	for _, o := range opts {
		if o.Name == "" {
			t.Fatalf("executor with an empty name: %+v", o)
		}
		if len(o.Argv) == 0 {
			t.Errorf("executor %s has no argv", o.Name)
		}
		if o.Detail == "" {
			t.Errorf("executor %s reports neither a resolved path nor a reason", o.Name)
		}
		if o.Available && !strings.Contains(o.Detail, "/") {
			t.Errorf("executor %s marked available but detail is not a path: %q", o.Name, o.Detail)
		}
	}
}

// TestUseExecutorRejectsUnknownProfile (AC-006): an unknown backend is
// an error. The shell must never fall back to some other executor
// because the requested one did not exist.
func TestUseExecutorRejectsUnknownProfile(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.UseExecutor("not-a-real-profile", nil); err == nil {
		t.Fatal("an unknown profile must be rejected")
	}
	id, err := svc.UseExecutor("codex-style", nil)
	if err != nil {
		t.Fatalf("codex-style must be selectable: %v", err)
	}
	if !strings.Contains(id, "codex") {
		t.Fatalf("executor id = %q, want it to name the selected profile", id)
	}
}

// TestAdvanceParksWithReasonWhenNoExecutor (AC-008): with nothing bound
// to execute a stage, the run parks as blocked with
// stage_executor_not_registered. A park is reported as a park — never
// as progress, and never as completion.
func TestAdvanceParksWithReasonWhenNoExecutor(t *testing.T) {
	svc := newTestService(t)
	view, err := svc.StartGoal("demo-feature", "add a demo feature", "tester")
	if err != nil {
		t.Fatalf("start goal: %v", err)
	}
	after, err := svc.Advance(view.RunID)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if !after.Blocked {
		t.Fatalf("with no executor the run must park blocked, got status=%q blocked=%v", after.Status, after.Blocked)
	}
	if after.ReasonCode != orchestrator.ReasonExecutorNotReg {
		t.Fatalf("park reason = %q, want %q", after.ReasonCode, orchestrator.ReasonExecutorNotReg)
	}
	if after.Status == "completed" {
		t.Fatal("a parked run must never report completed")
	}
}

// TestAdvanceWithBoundExecutorDoesNotParkOnMissingExecutor (AC-006):
// once a backend is bound, the run must no longer park with
// "no executor registered" — the binding took effect. The command is
// overridden with a local stub so this proves the wiring without
// spending a real model call.
func TestAdvanceWithBoundExecutorDoesNotParkOnMissingExecutor(t *testing.T) {
	svc := newTestService(t)
	view, err := svc.StartGoal("demo-feature", "add a demo feature", "tester")
	if err != nil {
		t.Fatalf("start goal: %v", err)
	}
	stub := filepath.Join(t.TempDir(), "stub.sh")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho stub ran\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}
	if err := os.Chmod(stub, 0o755); err != nil {
		t.Fatalf("chmod stub: %v", err)
	}
	if _, err := svc.UseExecutor("codex-style", []string{stub}); err != nil {
		t.Fatalf("bind executor: %v", err)
	}
	after, err := svc.Advance(view.RunID)
	if err != nil {
		if strings.Contains(err.Error(), orchestrator.ReasonExecutorNotReg) {
			t.Fatalf("binding did not take effect: %v", err)
		}
		// A later-stage failure (exit gate, evidence) is a different
		// question and is out of scope here.
		return
	}
	if after.ReasonCode == orchestrator.ReasonExecutorNotReg {
		t.Fatalf("after binding an executor the run still parks with %q", after.ReasonCode)
	}
}
