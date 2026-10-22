package cli

import (
	"testing"

	"beacon-go/internal/core"
)

// P2b CLI chain over the seeded project: driver start (dual start
// refused) -> run-once -> run-loop to the release gate -> supervise
// attach/once -> longrun-doctor -> todos -> steering surfacing in
// status -> self-heal plan -> queue enqueue/consume -> mark-complete
// rejection -> driver stop.
func TestGoalP2BCLIChain(t *testing.T) {
	root, _ := seedGoalProject(t)

	out, err := runCLI(t, "goal", "run", "long-run the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	// Driver start + dual-start refusal.
	out, err = runCLI(t, "goal", "driver", "start", "--run-id", runID, "--max-turns", "10", "--json")
	if err != nil {
		t.Fatalf("driver start: %v (out %s)", err, out)
	}
	doc := parseDoc(t, out)
	if got := strField(t, doc, "state"); got != "arming" {
		t.Fatalf("driver state = %q", got)
	}
	out, err = runCLI(t, "goal", "driver", "start", "--run-id", runID, "--json")
	if err == nil {
		t.Fatal("second driver start must fail")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDriverDualStart {
		t.Errorf("dual start reason = %q (err %v, out %s)", rc, err, out)
	}

	// One turn, then the loop to the release gate (truth pre-frozen +
	// qa pass seeded by seedGoalProject? qa is not seeded there, so
	// seed it now via a second run path: the loop must stop on its own
	// either at qa block or release gate; assert it never completes).
	out, err = runCLI(t, "goal", "driver", "run-once", "--run-id", runID, "--executor", "recording", "--json")
	if err != nil {
		t.Fatalf("driver run-once: %v (out %s)", err, out)
	}
	doc = parseDoc(t, out)
	drv := doc["driver"].(map[string]any)
	if drv["turn_n"].(float64) != 1 {
		t.Errorf("turn_n = %v", drv["turn_n"])
	}

	out, err = runCLI(t, "goal", "driver", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("driver status: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "state"); got != "requeue" {
		t.Errorf("driver status state = %q", got)
	}

	// Supervise: attach (driver start already attached one; re-attach
	// is a fresh session) and one poll without a stall.
	out, err = runCLI(t, "goal", "supervise", "attach", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("supervise attach: %v", err)
	}
	if parseDoc(t, out)["attached"] != true {
		t.Errorf("attach doc = %s", out)
	}
	out, err = runCLI(t, "goal", "supervise", "once", "--run-id", runID, "--threshold-sec", "1800", "--json")
	if err != nil {
		t.Fatalf("supervise once: %v", err)
	}
	if parseDoc(t, out)["stall_detected"] != false {
		t.Errorf("fresh run must not stall: %s", out)
	}

	// Longrun doctor: one driver round so far.
	out, err = runCLI(t, "goal", "longrun-doctor", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("longrun-doctor: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["driver_rounds"].(float64) != 1 {
		t.Errorf("doctor rounds = %v", doc["driver_rounds"])
	}

	// Todos via CLI.
	out, err = runCLI(t, "goal", "todo", "add", "--run-id", runID, "--title", "write docs", "--json")
	if err != nil {
		t.Fatalf("todo add: %v", err)
	}
	if todo := parseDoc(t, out)["todo"].(map[string]any); todo["id"] != "todo-1" {
		t.Errorf("todo = %v", todo)
	}
	out, err = runCLI(t, "goal", "todo", "list", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("todo list: %v", err)
	}
	if todos := parseDoc(t, out)["todos"].([]any); len(todos) != 1 {
		t.Errorf("todos = %v", todos)
	}
	out, err = runCLI(t, "goal", "todo", "done", "--run-id", runID, "--id", "todo-1", "--json")
	if err != nil {
		t.Fatalf("todo done: %v", err)
	}
	if todos := parseDoc(t, out)["todos"].(map[string]any); todos["done"].(float64) != 1 {
		t.Errorf("status todos after done = %v", todos)
	}

	// Steering surfaces in goal status until acked.
	out, err = runCLI(t, "goal", "steer", "--run-id", runID, "--text", "pivot: docs first", "--json")
	if err != nil {
		t.Fatalf("steer: %v", err)
	}
	out, err = runCLI(t, "goal", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	ps := parseDoc(t, out)["pending_steering"].(map[string]any)
	if ps["text"] != "pivot: docs first" {
		t.Errorf("pending_steering = %v", ps)
	}
	out, err = runCLI(t, "goal", "steer", "--run-id", runID, "--ack", "--json")
	if err != nil {
		t.Fatalf("steer ack: %v", err)
	}
	if parseDoc(t, out)["pending_steering"] != nil {
		t.Errorf("steering still pending after ack: %s", out)
	}

	// Self-heal: plan only executes nothing.
	out, err = runCLI(t, "goal", "self-heal", "--run-id", runID, "--phase", "pre", "--plan", "--json")
	if err != nil {
		t.Fatalf("self-heal plan: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["executed"] != false || len(doc["steps"].([]any)) != 2 {
		t.Errorf("self-heal plan = %s", out)
	}
	out, err = runCLI(t, "goal", "self-heal", "--run-id", runID, "--phase", "post", "--json")
	if err != nil {
		t.Fatalf("self-heal post: %v", err)
	}
	steps := parseDoc(t, out)["steps"].([]any)
	if steps[0].(map[string]any)["status"] != "requires_confirmation" {
		t.Errorf("post step = %v", steps[0])
	}

	// Queue: enqueue -> consume-queue -> mark-complete rejection.
	out, err = runCLI(t, "goal", "queue", "enqueue", "discovered: flaky test",
		"--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("queue enqueue: %v", err)
	}
	item := parseDoc(t, out)["item"].(map[string]any)
	if item["status"] != "queued" || item["product_complete"] != false {
		t.Errorf("queue item = %v", item)
	}
	out, err = runCLI(t, "goal", "consume-queue", "--json")
	if err != nil {
		t.Fatalf("consume-queue: %v", err)
	}
	doc = parseDoc(t, out)
	if doc["status"] != "consumed" || doc["run"] == nil {
		t.Errorf("consume doc = %s", out)
	}
	out, err = runCLI(t, "goal", "consume-queue", "--json")
	if err != nil {
		t.Fatalf("consume-queue empty must not error: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "status"); got != "empty" {
		t.Errorf("second consume = %q", got)
	}
	out, err = runCLI(t, "goal", "queue", "mark-complete", "--json")
	if err == nil {
		t.Fatal("mark-complete must be rejected")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLoopCannotComplete {
		t.Errorf("mark-complete reason = %q", rc)
	}
	if parseDoc(t, out)["allowed"] != false {
		t.Errorf("mark-complete doc = %s", out)
	}

	// Driver run-loop + stop on the original run: the loop stops by
	// itself (qa exit blocks without evidence -> driver halts), then
	// stop records the user stop.
	out, err = runCLI(t, "goal", "driver", "run-loop", "--run-id", runID, "--executor", "recording", "--max-rounds", "6", "--json")
	if err != nil {
		t.Fatalf("driver run-loop: %v (out %s)", err, out)
	}
	doc = parseDoc(t, out)
	if doc["final_state"] == "requeue" || doc["final_state"] == "arming" {
		t.Errorf("run-loop final state = %v (must have halted)", doc["final_state"])
	}
	if doc["run"].(map[string]any)["status"] == "completed" {
		t.Error("driver loop completed the run — forbidden")
	}
	out, err = runCLI(t, "goal", "driver", "stop", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("driver stop: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "state"); got != "halt_terminal" {
		t.Errorf("stop state = %q", got)
	}
}
