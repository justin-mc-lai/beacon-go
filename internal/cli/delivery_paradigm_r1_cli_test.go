package cli

import (
	"strings"
	"testing"
)

// B4 (K1 original path): `goal run` with neither --executor nor
// --exec-cmd must not silently create an undriven run. It defaults to
// manual, prints the tick contract at creation, and the first tick says
// stage_manual_tick_required instead of stage_executor_not_registered.
func TestGoalRunWithoutExecutorDefaultsToManual(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "feat-gate",
		"--executor-id", "renlink-dev-room")
	if err != nil {
		t.Fatalf("goal run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "defaulted to executor manual") || !strings.Contains(out, "--by <identity>") {
		t.Fatalf("creation output must state the default + tick contract:\n%s", out)
	}
	id := runIDFromHuman(t, out)
	statusOut, err := runCLI(t, "goal", "status", "--run-id", id)
	if err != nil {
		t.Fatalf("status: %v\n%s", err, statusOut)
	}
	if !strings.Contains(statusOut, "executor: manual") || !strings.Contains(statusOut, "--by <identity>") {
		t.Fatalf("status must repeat the manual contract:\n%s", statusOut)
	}
	tickOut, tickErr := runCLI(t, "goal", "tick", "--run-id", id)
	if tickErr == nil {
		t.Fatalf("tick without --by must park:\n%s", tickOut)
	}
	if strings.Contains(tickOut, "stage_executor_not_registered") {
		t.Fatalf("original K1 path must be gone:\n%s", tickOut)
	}
	if !strings.Contains(tickOut, "stage_manual_tick_required") {
		t.Fatalf("park must name the manual tick contract:\n%s", tickOut)
	}
}
