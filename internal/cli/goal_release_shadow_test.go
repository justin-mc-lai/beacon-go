// CLI surface for issue #8's shadow step: `goal run --release-policy
// auto_shadow` then `goal release-shadow`, and `goal status` exposing
// both the policy and the last shadow verdict.
package cli

import "testing"

func TestGoalReleaseShadowCLI(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "goal", "run", "shadow preview", "--project", root,
		"--feature", "feat-gate", "--executor-id", "host",
		"--release-policy", "auto_shadow", "--json")
	if err != nil {
		t.Fatalf("goal run: %v\n%s", err, out)
	}
	runID := strField(t, parseDoc(t, out), "run_id")
	if runID == "" {
		t.Fatalf("no run_id: %s", out)
	}

	out, err = runCLI(t, "goal", "release-shadow", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("release-shadow: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "policy") != "auto_shadow" {
		t.Fatalf("policy = %v", doc["policy"])
	}
	if strField(t, doc, "verdict") == "" {
		t.Fatalf("shadow must report a verdict, got %v", doc)
	}

	out, err = runCLI(t, "goal", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("status: %v\n%s", err, out)
	}
	st := parseDoc(t, out)
	if strField(t, st, "release_policy") != "auto_shadow" {
		t.Fatalf("status must expose release_policy, got %v", st["release_policy"])
	}
	if _, ok := st["release_shadow"].(map[string]any); !ok {
		t.Fatalf("status must expose the shadow verdict, got %v", st["release_shadow"])
	}
}
