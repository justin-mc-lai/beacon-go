package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func treeTestEnv(t *testing.T) (projRoot string) {
	t.Helper()
	base := t.TempDir()
	t.Setenv(store.EnvStorePath, filepath.Join(base, "store.db"))
	projRoot = filepath.Join(base, "proj")
	if err := os.MkdirAll(projRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "projects", "register", "--path", projRoot, "--version", "v0.1.0", "--json")
	return projRoot
}

func mustRun(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runCLI(t, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out
}

func TestTreeMigrateReadAuditAndIdempotency(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")

	out := mustRun(t, "migrate", "tree", "--project", proj, "--version", "v0.1.0", "--json")
	rep := parseDoc(t, out)
	if rep["version"] != "v0.1.0" {
		t.Fatalf("migration report: %v", rep)
	}
	if got := int(rep["node_count"].(float64)); got != 3 {
		t.Fatalf("node_count = %d, want root+module+feature (3)", got)
	}
	rootID := rep["root_id"].(string)

	out = mustRun(t, "plan", "tree", "--project", proj, "--json")
	doc := parseDoc(t, out)
	if got := int(doc["node_count"].(float64)); got != 3 {
		t.Fatalf("plan tree node_count = %d", got)
	}
	nodes := doc["nodes"].([]any)
	if len(nodes) != 3 {
		t.Fatalf("nodes = %d", len(nodes))
	}

	out = mustRun(t, "truth", "tree", "--project", proj, "--node", rootID, "--json")
	node := parseDoc(t, out)
	if node["kind"] != "intent" {
		t.Fatalf("root kind = %v", node["kind"])
	}

	indexPath := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "index.json")
	before, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	out = mustRun(t, "migrate", "tree", "--project", proj, "--version", "v0.1.0", "--json")
	if parseDoc(t, out)["already"] != true {
		t.Fatal("second migrate tree must be idempotent (already=true)")
	}
	after, _ := os.ReadFile(indexPath)
	if string(before) != string(after) {
		t.Fatal("idempotent migration rewrote the index")
	}

	// verify audit is read-only: index bytes, tree file set and DB bytes unchanged.
	beforeHash := hashTreeFiles(t, filepath.Dir(indexPath))
	dbBefore, _ := os.ReadFile(storePathFromEnv(t))
	out = mustRun(t, "verify", "audit", "--project", proj, "--json")
	audit := parseDoc(t, out)
	if audit["ok"] != true || audit["reads_only"] != true {
		t.Fatalf("audit report: %v", audit)
	}
	afterHash := hashTreeFiles(t, filepath.Dir(indexPath))
	if beforeHash != afterHash {
		t.Fatal("verify audit wrote to the tree")
	}
	dbAfter, _ := os.ReadFile(storePathFromEnv(t))
	if string(dbBefore) != string(dbAfter) {
		t.Fatal("verify audit wrote to the database")
	}
}

func TestTreeExpandSchemaDAG(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	rep := parseDoc(t, mustRun(t, "migrate", "tree", "--project", proj, "--json"))
	rootID := rep["root_id"].(string)
	base := t.TempDir()

	badPath := filepath.Join(base, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{"children":[{"kind":"feature","title":"jump"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCLI(t, "plan", "expand", "--project", proj, "--node", rootID, "--children", badPath, "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_invalid_child_kind") {
		t.Fatalf("intent->feature must be rejected, got %v", err)
	}

	goodPath := filepath.Join(base, "good.json")
	if err := os.WriteFile(goodPath, []byte(`{"children":[{"kind":"module","title":"auth"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := mustRun(t, "plan", "expand", "--project", proj, "--node", rootID, "--children", goodPath, "--json")
	if parseDoc(t, out)["allowed"] != true {
		t.Fatalf("intent->module must be allowed: %s", out)
	}

	tooMany := filepath.Join(base, "many.json")
	kids := make([]map[string]string, 0, 8)
	for i := range 8 {
		kids = append(kids, map[string]string{"kind": "module", "title": fmt.Sprintf("m%d", i)})
	}
	raw, _ := json.Marshal(map[string]any{"children": kids})
	_ = os.WriteFile(tooMany, raw, 0o644)
	_, err = runCLI(t, "plan", "expand", "--project", proj, "--node", rootID, "--children", tooMany, "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_child_limit_exceeded") {
		t.Fatalf("8 children must hit the cap, got %v", err)
	}
}

func TestMigrateTreeRejectsUnknownSchema(t *testing.T) {
	proj := treeTestEnv(t)
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"schema":"bogus","root_id":"x","nodes":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCLI(t, "migrate", "tree", "--project", proj, "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_schema_unknown") {
		t.Fatalf("unknown schema must fail closed, got %v", err)
	}
}

func hashTreeFiles(t *testing.T, dir string) string {
	t.Helper()
	sum := sha256.New()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		fmt.Fprintf(sum, "%s:%x\n", path, sha256.Sum256(raw))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", sum.Sum(nil))
}

func TestTreeRebuildAndMarkdownRender(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	mustRun(t, "migrate", "tree", "--project", proj, "--json")

	// Cache loss: only the derived rows are removed; repo JSON and flat
	// feature packages stay untouched.
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`DELETE FROM tree_nodes`); err != nil {
		t.Fatal(err)
	}
	st.Close()

	out := mustRun(t, "doctor", "rebuild-tree", "--project", proj, "--json")
	if parseDoc(t, out)["rebuilt"] != float64(3) {
		t.Fatalf("rebuild report: %s", out)
	}

	out = mustRun(t, "plan", "tree", "--project", proj, "--format", "md")
	mdPath := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree", "tree.md")
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("markdown render missing: %v (%s)", err, out)
	}

	out = mustRun(t, "plan", "tree", "--project", proj, "--format", "html")
	if !strings.Contains(out, "index.html") {
		t.Fatalf("html path not printed: %s", out)
	}
}

func storePathFromEnv(t *testing.T) string {
	t.Helper()
	p := os.Getenv(store.EnvStorePath)
	if p == "" {
		t.Fatal("store env not set")
	}
	return p
}

func TestTreeRollbackAndRerun(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	mustRun(t, "migrate", "tree", "--project", proj, "--json")

	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	pr, err := st.Projects().GetByRootPath(store.ResolveRootPath(proj))
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	vers, err := st.Projects().ListVersions(pr.ID)
	if err != nil || len(vers) == 0 {
		st.Close()
		t.Fatalf("version lookup: %v", err)
	}
	if err := st.DeleteTree(vers[0].ID); err != nil {
		st.Close()
		t.Fatal(err)
	}
	// Old flat data must survive the cache rollback.
	if feats, err := st.Features().ListAll(); err != nil || len(feats) == 0 {
		st.Close()
		t.Fatalf("feature packages lost after rollback: %v", err)
	}
	st.Close()

	out := mustRun(t, "migrate", "tree", "--project", proj, "--json")
	if parseDoc(t, out)["node_count"] != float64(3) {
		t.Fatalf("rerun after rollback: %s", out)
	}
	mustRun(t, "plan", "tree", "--project", proj, "--json")
}
