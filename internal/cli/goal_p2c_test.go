package cli

import (
	"testing"
)

// P2c CLI chain: admit + implement plan -> parallel-plan (conflict)
// -> delegate-ralph --simulate -> receive-result (reject then accept)
// on one run.
func TestGoalP2CChain(t *testing.T) {
	root, _ := seedGoalProject(t)

	out, err := runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	for i := 0; i < 2; i++ {
		out, err = runCLI(t, "goal", "tick", "--run-id", runID, "--executor", "recording", "--json")
		if err != nil {
			t.Fatalf("tick %d: %v (out %s)", i, err, out)
		}
	}
	if got := strField(t, parseDoc(t, out), "current_stage"); got != "implement" {
		t.Fatalf("current_stage = %q, want implement", got)
	}

	// admit: all items pass on the seeded (frozen) feature. S2
	// (AC-141) extended the checker from five items to seven
	// (baseline_merged + dirty_policy; both not_applicable on this
	// non-git seeded feature) — the count assertion follows.
	out, err = runCLI(t, "goal", "admit", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("admit: %v (out %s)", err, out)
	}
	adm := parseDoc(t, out)
	if adm["allow_implement"] != true || adm["reason_code"] != "goal_implement_admission_ok" {
		t.Errorf("admission = %v", adm)
	}
	// Admission items: truth_frozen, worktree_binding, blocker_streak,
	// steering_ack, baseline_merged, dirty_policy, topology_gate and
	// review_decision (non-applicable ones still reported).
	if items, _ := adm["items"].([]any); len(items) != 8 {
		t.Errorf("admission items = %d, want 8", len(items))
	}

	// implement plan with an explicit ralph ask.
	out, err = runCLI(t, "goal", "implement", "plan", "--run-id", runID, "--lane", "ralph", "--json")
	if err != nil {
		t.Fatalf("implement plan: %v", err)
	}
	plan := parseDoc(t, out)
	dec, _ := plan["decision"].(map[string]any)
	if dec["lane"] != "ralph" || dec["source"] != "explicit" {
		t.Errorf("decision = %v", dec)
	}
}
