package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func TestPlanDriftAgentsMDRealRepositoryAligned(t *testing.T) {
	t.Setenv(store.EnvStorePath, filepath.Join(t.TempDir(), "absent.db"))
	project, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "plan", "drift", "--project", project, "--agents-md", "--json")
	if err != nil {
		t.Fatalf("real entry must compare nine canonical contracts: %s %v", out, err)
	}
	doc := parseDoc(t, out)
	if doc["blocked"] != float64(0) || doc["evaluated"] != float64(9) || len(doc["changed"].([]any)) != 0 || len(doc["items"].([]any)) != 9 {
		t.Fatalf("nine-rule canonical alignment: %v", doc)
	}
}

func TestPlanDriftAgentsMDNumberedCLI(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	if err := os.MkdirAll(filepath.Join(proj, "docs"), 0755); err != nil {
		t.Fatal(err)
	}
	// Read the canonical field used by this fixture; the mapping stores no text.
	n, err := intenttree.ReadNode(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree"), "t1")
	if err != nil {
		t.Fatal(err)
	}
	agents := "## 唯一真源（最高口径）\n1. " + n.Title + "\n## Process\nold process\n"
	mapPath := filepath.Join(proj, "docs", "agents-md-drift-map.json")
	mapping := `{"schema":"beacon-agents-md-map-v2","section":"唯一真源（最高口径）","requirements":[{"item":1,"targets":[{"node_id":"t1","field":"/title"}]}],"governance":["Process"]}`
	write := func(path, text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(proj, "AGENTS.md"), agents)
	write(mapPath, mapping)
	before, err := os.ReadFile(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "nodes", "t1.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc := parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--agents-md", "--json"))
	if doc["evaluated"] != float64(1) || doc["blocked"] != float64(0) || len(doc["changed"].([]any)) != 0 {
		t.Fatalf("bare fixture: %v", doc)
	}
	write(filepath.Join(proj, "AGENTS.md"), strings.Replace(agents, "old process", "new process", 1))
	doc = parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--agents-md", "--agents-map", mapPath, "--json"))
	if len(doc["changed"].([]any)) != 0 {
		t.Fatalf("governance: %v", doc)
	}
	write(filepath.Join(proj, "AGENTS.md"), strings.Replace(agents, "1. "+n.Title, "1. reverse "+n.Title, 1))
	doc = parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--agents-md", "--agents-map", "docs/agents-md-drift-map.json", "--json"))
	if len(doc["changed"].([]any)) != 1 {
		t.Fatalf("reverse product: %v", doc)
	}
	after, err := os.ReadFile(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "nodes", "t1.json"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("comparison mutated truth: %v", err)
	}
	for _, args := range [][]string{
		{"--agents-md", "--snapshot", "latest"},
		{"--agents-map", mapPath},
		{"--agents-md", "--agents-map", ""},
		{"--agents-md", "--agents-map", " "},
		{"--agents-md", "--node", "t2"},
	} {
		if _, err := runCLI(t, append([]string{"plan", "drift", "--project", proj}, args...)...); err == nil {
			t.Fatalf("must reject: %v", args)
		}
	}
	write(mapPath, strings.Replace(mapping, `"targets":`, `"gap":"missing contract","targets":`, 1))
	out, err := runCLI(t, "plan", "drift", "--project", proj, "--agents-md", "--json")
	if err == nil || !strings.Contains(err.Error(), "agents_md_drift_blocked") || parseDoc(t, out)["blocked"] != float64(1) {
		t.Fatalf("gap must return diagnostic and error: %s %v", out, err)
	}
}

func TestPlanDriftSeparatesRequirementBodyFromRecomputedFields(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		mustRun(t, "truth", "node", "freeze", "--project", proj, "--node", id, "--json")
	}
	snap := parseDoc(t, mustRun(t, "truth", "snapshot", "create", "--project", proj, "--json"))
	if snap["snapshot_id"] == "" {
		t.Fatalf("snapshot id missing: %v", snap)
	}
	snapID := snap["snapshot_id"].(string)
	explicit := parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--node", "f", "--snapshot", snapID, "--json"))
	if explicit["snapshot_id"] != snapID {
		t.Fatalf("explicit snapshot key ignored: %v", explicit)
	}

	// Status and fingerprint recomputation alone are not requirement drift.
	node := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--node", "t1", "--json"))
	mustRun(t, "change", "request", "--project", proj, "--node", "t1",
		"--summary", "recompute only", "--by", "tester",
		"--base-fingerprint", node["fingerprint"].(string), "--json")

	out := mustRun(t, "plan", "drift", "--project", proj, "--node", "f", "--json")
	doc := parseDoc(t, out)
	if doc["snapshot_id"] == "" || doc["node_id"] != "f" {
		t.Fatalf("drift binding missing: %s", out)
	}
	if len(doc["changed"].([]any)) != 0 || len(doc["added"].([]any)) != 0 || len(doc["removed"].([]any)) != 0 {
		t.Fatalf("status/fingerprint churn must not be requirement drift: %s", out)
	}

	// A requirement-body edit is drift, and only the edited node is listed.
	patch := writeJSON(t, map[string]any{"intent": "新的提交意图"})
	mustRun(t, "truth", "node", "update", "--project", proj, "--node", "t1", "--file", patch, "--json")
	doc = parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--node", "f", "--json"))
	changed := doc["changed"].([]any)
	if len(changed) != 1 {
		t.Fatalf("changed = %d, want 1: %v", len(changed), doc)
	}
	entry := changed[0].(map[string]any)
	if entry["id"] != "t1" {
		t.Fatalf("changed entry = %v, want t1", entry)
	}
	if len(doc["added"].([]any)) != 0 || len(doc["removed"].([]any)) != 0 {
		t.Fatalf("body edit must not add/remove nodes: %v", doc)
	}
}

func TestPlanDriftClassifiesAddedAndRemoved(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		mustRun(t, "truth", "node", "freeze", "--project", proj, "--node", id, "--json")
	}
	mustRun(t, "truth", "snapshot", "create", "--project", proj, "--json")

	// Open the feature for edits; the change request's own snapshot is the baseline.
	f := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--node", "f", "--json"))
	mustRun(t, "change", "request", "--project", proj, "--node", "f",
		"--summary", "add leaf", "--by", "tester",
		"--base-fingerprint", f["fingerprint"].(string), "--json")
	proposal := writeJSON(t, map[string]any{"children": []any{map[string]any{
		"kind": "touchpoint", "title": "Three", "slug": "three", "intent": "third",
		"acs":          []any{map[string]any{"id": "t3-1", "text": "可追踪"}},
		"verification": map[string]any{"type": "test", "spec": "go test ./..."},
	}}})
	mustRun(t, "plan", "apply", "--project", proj, "--node", "f", "--proposal", proposal, "--json")

	// There is no physical-delete CLI; dropping the index row is enough to
	// exercise the removed half of the same public drift seam.
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	kept := idx.Nodes[:0]
	for _, e := range idx.Nodes {
		if e.ID != "t2" {
			kept = append(kept, e)
		}
	}
	idx.Nodes = kept
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}

	doc := parseDoc(t, mustRun(t, "plan", "drift", "--project", proj, "--node", "f", "--json"))
	assertDriftID(t, doc, "added", "m/f/three")
	assertDriftID(t, doc, "removed", "m/f/t2")
}

func assertDriftID(t *testing.T, doc map[string]any, field, path string) {
	t.Helper()
	for _, raw := range doc[field].([]any) {
		entry := raw.(map[string]any)
		if entry["path"] == path {
			return
		}
	}
	t.Fatalf("%s does not contain %s: %v", field, path, doc[field])
}
