package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGraphFile writes a single-repo GitScopeGraph JSON for root
// and returns its path (shared with the S2 flow CLI seed).
func writeGraphFile(t *testing.T, root string) string {
	t.Helper()
	graph := `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo",` +
		`"scopes":[{"scope_id":"primary","git_root":"` + root + `","role":"primary","dirty_policy":"fail_closed"}]}`
	gp := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(gp, []byte(graph), 0o644); err != nil {
		t.Fatal(err)
	}
	return gp
}

// TestGoalTopologyCLI walks the AC-171/173 CLI surface on the real
// command tree: project a graph file, show the recorded context,
// detect on a non-git root fails closed, and a malformed graph is
// refused with the frozen parity code.
func TestGoalTopologyCLI(t *testing.T) {
	root := seedP4AProject(t)
	truthCreate(t, root, "topo-feat")

	gp := writeGraphFile(t, root)
	out, err := runCLI(t, "goal", "topology", "project", "--project", root, "--graph", gp, "--by", "tester", "--json")
	if err != nil {
		t.Fatalf("topology project: %v (%s)", err, out)
	}
	doc := parseDoc(t, out)
	if doc["topology_kind"] != "single_repo" {
		t.Fatalf("projected doc = %v", doc)
	}
	ref, _ := doc["topology_gate_ref"].(string)
	if !strings.HasPrefix(ref, "bc_") {
		t.Fatalf("ref = %q, want bc_ prefix", ref)
	}
	if scopes, _ := doc["scopes"].([]any); len(scopes) != 1 {
		t.Fatalf("scopes = %v, want 1", doc["scopes"])
	}
	if doc["confirmed"] != true {
		t.Fatalf("a file projection is confirmed by the act of projecting: %v", doc)
	}

	// Show resolves the same recorded projection.
	out, err = runCLI(t, "goal", "topology", "show", "--project", root, "--json")
	if err != nil {
		t.Fatalf("topology show: %v (%s)", err, out)
	}
	if doc2 := parseDoc(t, out); doc2["topology_gate_ref"] != ref {
		t.Fatalf("show ref = %v, want %s", doc2["topology_gate_ref"], ref)
	}

	// A malformed graph fails closed with the frozen parity code.
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte(`{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "goal", "topology", "project", "--project", root, "--graph", bad); err == nil ||
		!strings.Contains(err.Error(), "gitscope_branch_context_parity_failed") {
		t.Fatalf("bad graph err = %v, want gitscope_branch_context_parity_failed", err)
	}

	// Detect on a non-git root without sub-repositories fails closed.
	if _, err := runCLI(t, "goal", "topology", "detect", "--project", root); err == nil ||
		!strings.Contains(err.Error(), "worktree_not_a_git_repository") {
		t.Fatalf("detect err = %v, want worktree_not_a_git_repository", err)
	}
}
