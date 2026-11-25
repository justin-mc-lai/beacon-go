package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestTruthNodeLinkAndSyncStatus(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	truthDir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "features", "feature")
	if err := os.MkdirAll(truthDir, 0o755); err != nil {
		t.Fatal(err)
	}
	truthPath := filepath.Join(truthDir, "truth.md")
	if err := os.WriteFile(truthPath, []byte("---\nslug: feature\nstatus: frozen\n---\n# Truth\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Only feature nodes carry the package link.
	if _, err := runCLI(t, "truth", "node", "link", "--project", proj, "--node", "root", "--truth", "docs/beacon/v0.1.0/features/feature/truth.md", "--json"); err == nil || !strings.Contains(err.Error(), "tree_truth_path_invalid") {
		t.Fatalf("non-feature link must be refused: %v", err)
	}
	out := mustRun(t, "truth", "node", "link", "--project", proj, "--node", "f", "--truth", "docs/beacon/v0.1.0/features/feature/truth.md", "--json")
	if parseDoc(t, out)["truth_path"] != "docs/beacon/v0.1.0/features/feature/truth.md" {
		t.Fatalf("link output: %s", out)
	}

	// sync-status is read-only and reports draft node vs frozen package.
	before, err := os.ReadFile(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "nodes", "f.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc := parseDoc(t, mustRun(t, "truth", "node", "sync-status", "--project", proj, "--json"))
	nodes := doc["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["match"] != false {
		t.Fatalf("sync report: %s", doc)
	}
	after, _ := os.ReadFile(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "nodes", "f.json"))
	if string(before) != string(after) {
		t.Fatal("sync-status must not write the node")
	}

	// Freeze the leaf pair and the feature; package frozen then matches.
	for _, id := range []string{"t1", "t2", "f"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	doc = parseDoc(t, mustRun(t, "truth", "node", "sync-status", "--project", proj, "--json"))
	if doc["ok"] != true {
		t.Fatalf("frozen feature should match its frozen package: %s", doc)
	}
	node, err := intenttree.ReadNode(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree"), "f")
	if err != nil {
		t.Fatal(err)
	}
	if node.EffectiveTruthPath() == "" {
		t.Fatal("effective truth path is empty")
	}
}

func TestTruthNodeLinkSlugMismatch(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "features", "feature")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "truth.md"), []byte("---\nslug: other\nstatus: draft\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCLI(t, "truth", "node", "link", "--project", proj, "--node", "f", "--truth", "docs/beacon/v0.1.0/features/feature/truth.md", "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_truth_slug_mismatch") {
		t.Fatalf("slug mismatch must fail: %v", err)
	}
}
