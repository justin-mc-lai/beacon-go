package cli

import (
	"strings"
	"testing"
)

// TestS2GoalFlowCLI walks the AC-138 flow + AC-142 bundle on the
// real command surface over a real git project: skip is refused,
// the eight steps advance in order, the bundle command records a
// passing bundle, and a repeated advance reports already_done.
func TestS2GoalFlowCLI(t *testing.T) {
	root := seedGitProject(t)
	truthCreate(t, root, "feat-flow")
	truthFreeze(t, root, "feat-flow")
	gitCLI(t, root, "add", "docs")
	gitCLI(t, root, "commit", "-m", "freeze feat-flow")
	out, err := runCLI(t, "truth", "baseline", "--project", root, "--feature", "feat-flow", "--by", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("truth baseline: %v (%s)", err, out)
	}
	// S3 seed (declared change): since AC-173 the flow's
	// workspace_admission step consumes the topology gate; project
	// this repo's single-repo topology through the real CLI first.
	graphPath := writeGraphFile(t, root)
	out, err = runCLI(t, "goal", "topology", "project", "--project", root, "--feature", "feat-flow", "--graph", graphPath, "--by", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("topology project: %v (%s)", err, out)
	}
	out, err = runCLI(t, "goal", "run", "build it", "--project", root, "--feature", "feat-flow", "--executor-id", "maker", "--json")
	if err != nil {
		t.Fatalf("goal run: %v (%s)", err, out)
	}
	runID, _ := parseDoc(t, out)["run_id"].(string)
	if runID == "" {
		t.Fatalf("no run_id in %s", out)
	}
	flow := func(args ...string) (string, error) {
		return runCLI(t, append([]string{"goal", "flow"}, args...)...)
	}
	// Status: nothing completed, advisory is next.
	out, err = flow("status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("flow status: %v (%s)", err, out)
	}
	doc := parseDoc(t, out)
	if doc["completed_count"] != float64(0) || doc["next_step"] != "support_advisory" {
		t.Fatalf("initial flow status = %v", doc)
	}
	// Skipping to workspace_admission is refused with the frozen code.
	out, err = flow("advance", "--run-id", runID, "--step", "workspace_admission", "--json")
	if err == nil || !strings.Contains(out+err.Error(), "standard_flow_order_violation") {
		t.Fatalf("skip advance must fail with standard_flow_order_violation: err=%v out=%s", err, out)
	}
	steps := []string{
		"support_advisory", "canonical_truth_authoring", "canonical_refreeze",
		"canonical_commit", "feature_worktree_reconcile", "governance_verify",
		"workspace_admission", "implement_resume",
	}
	for _, s := range steps {
		out, err = flow("advance", "--run-id", runID, "--step", s, "--json")
		if err != nil {
			t.Fatalf("advance %s: %v (%s)", s, err, out)
		}
		if doc := parseDoc(t, out); doc["advanced"] != true {
			t.Fatalf("advance %s doc = %v", s, doc)
		}
	}
	// Repeat: already_done, no error.
	out, err = flow("advance", "--run-id", runID, "--step", "implement_resume", "--json")
	if err != nil {
		t.Fatalf("repeat advance: %v (%s)", err, out)
	}
	if doc := parseDoc(t, out); doc["already_done"] != true || doc["complete"] != true {
		t.Fatalf("repeat advance doc = %v", doc)
	}
	// The bundle command: five items, allow_implement, gate row id.
	out, err = flow("bundle", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("flow bundle: %v (%s)", err, out)
	}
	b := parseDoc(t, out)
	if b["allow_implement"] != true || b["gate_result_id"] == "" || b["gate_result_id"] == nil {
		t.Fatalf("bundle doc = %v", b)
	}
	if items, _ := b["items"].([]any); len(items) != 5 {
		t.Fatalf("bundle items = %d, want 5", len(items))
	}
}
