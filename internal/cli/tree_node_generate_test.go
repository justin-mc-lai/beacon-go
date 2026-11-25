package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestGenerateTestsAndDoc(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	patch := map[string]any{
		"acs": []map[string]string{{"id": "f-1", "text": "can move to frozen"}},
		"domain_fsm": map[string]any{
			"states":      []string{"draft", "frozen"},
			"transitions": []map[string]string{{"from": "draft", "to": "frozen", "event": "freeze"}},
			"illegal":     []map[string]string{{"from": "frozen", "to": "draft"}},
		},
	}
	raw, _ := json.Marshal(patch)
	f := filepath.Join(t.TempDir(), "patch.json")
	if err := os.WriteFile(f, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "truth", "node", "update", "--project", proj, "--node", "f", "--file", f, "--json")

	out := parseDoc(t, mustRun(t, "truth", "node", "generate-tests", "--project", proj, "--node", "f", "--json"))
	if len(out["test_cases"].([]any)) != 3 || out["applied"] != false {
		t.Fatalf("dry-run generate-tests: %v", out)
	}
	mustRun(t, "truth", "node", "generate-tests", "--project", proj, "--node", "f", "--apply", "--json")
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	n, err := intenttree.ReadNode(dir, "f")
	if err != nil {
		t.Fatal(err)
	}
	if len(n.TestCases) != 3 {
		t.Fatalf("applied test cases = %d, want 3", len(n.TestCases))
	}
	mustRun(t, "truth", "node", "generate-doc", "--project", proj, "--node", "f", "--apply", "--json")
	n, _ = intenttree.ReadNode(dir, "f")
	if !strings.Contains(n.Doc, "## 状态迁移") || !strings.Contains(n.Doc, "frozen -> draft") {
		t.Fatalf("generated doc: %s", n.Doc)
	}

	for _, id := range []string{"t1", "t2", "f"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	if _, err := runCLI(t, "truth", "node", "generate-doc", "--project", proj, "--node", "f", "--apply", "--json"); err == nil {
		t.Fatal("frozen node must refuse --apply without --change")
	}
	mustRun(t, "truth", "node", "generate-doc", "--project", proj, "--node", "f", "--apply", "--change", "--summary", "refresh doc", "--by", "qa", "--json")
	n, _ = intenttree.ReadNode(dir, "f")
	if n.Status != intenttree.StatusDraft || !strings.Contains(n.Doc, "## 状态迁移") {
		t.Fatalf("change+generate doc: status=%s doc=%s", n.Status, n.Doc)
	}
}
