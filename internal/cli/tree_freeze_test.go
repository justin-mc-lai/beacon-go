package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func seedFreezeTree(t *testing.T, proj string, withUnverified bool) map[string]string {
	t.Helper()
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	pr, err := st.Projects().GetByRootPath(store.ResolveRootPath(proj))
	if err != nil {
		t.Fatal(err)
	}
	vers, err := st.Projects().ListVersions(pr.ID)
	if err != nil || len(vers) == 0 {
		t.Fatalf("version: %v", err)
	}
	ver := vers[0]
	dir := intenttree.TreeDir(pr.RootPath, ver.DocsRoot)

	ac := func(id, text string) []intenttree.AC { return []intenttree.AC{{ID: id, Text: text}} }
	verif := &intenttree.Verification{Type: "test", Spec: "当用户提交时返回明确结果"}
	nodes := []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Title: "root", Path: "root", Status: intenttree.StatusDraft},
		{ID: "m", Kind: intenttree.KindModule, ParentID: "root", Title: "module", Path: "m", Status: intenttree.StatusDraft},
		{ID: "f", Kind: intenttree.KindFeature, ParentID: "m", Title: "feature", Path: "m/f", Status: intenttree.StatusDraft},
		{ID: "t1", Kind: intenttree.KindTouchpoint, ParentID: "f", Title: "one", Path: "m/f/t1", Status: intenttree.StatusDraft, ACs: ac("t1-1", "可提交"), Verification: verif},
		{ID: "t2", Kind: intenttree.KindTouchpoint, ParentID: "f", Title: "two", Path: "m/f/t2", Status: intenttree.StatusDraft, ACs: ac("t2-1", "可取消"), Verification: verif},
	}
	if withUnverified {
		nodes = append(nodes,
			intenttree.Node{ID: "m2", Kind: intenttree.KindModule, ParentID: "root", Title: "empty", Path: "m2", Status: intenttree.StatusDraft},
			intenttree.Node{ID: "t3", Kind: intenttree.KindTouchpoint, ParentID: "f", Title: "unverified", Path: "m/f/t3", Status: intenttree.StatusDraft, ACs: ac("t3-1", "无验证"), Verification: nil})
	}
	idx := intenttree.Index{Schema: intenttree.Schema, Version: ver.Version, RootID: "root"}
	now := time.Now().UTC()
	var cache []store.TreeNode
	for _, n := range nodes {
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{
			ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status, Title: n.Title,
			Path: n.Path, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
		})
		cache = append(cache, store.TreeNode{
			ID: n.ID, ProjectVersionID: ver.ID, ParentID: n.ParentID, Kind: string(n.Kind),
			Path: n.Path, Status: string(n.Status), Title: n.Title, Fingerprint: n.Fingerprint,
			FingerprintAlg: n.FingerprintAlg, DocPath: intenttree.NodePath(dir, n.ID),
			CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		})
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	mig := store.TreeMigration{ID: "tmig_test", ProjectVersionID: ver.ID, Source: "test",
		SnapshotPath: "", NodeCount: len(cache), CreatedAt: now.Format(time.RFC3339Nano)}
	if err := st.SaveTree(ver.ID, cache, mig); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"root": "root", "m": "m", "m2": "m2", "f": "f", "t1": "t1", "t2": "t2", "t3": "t3"}
}

func freeze(t *testing.T, proj, id string) (string, error) {
	t.Helper()
	return runCLI(t, "truth", "node", "freeze", "--project", proj, "--node", id, "--json")
}

func TestFreezeGuards(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, true)

	if _, err := freeze(t, proj, "t3"); err == nil || !strings.Contains(err.Error(), "tree_verification_missing") {
		t.Fatalf("unverified leaf must be refused, got %v", err)
	}
	if _, err := freeze(t, proj, "m2"); err == nil || !strings.Contains(err.Error(), "tree_module_empty") {
		t.Fatalf("empty module must be refused, got %v", err)
	}
	if _, err := freeze(t, proj, "root"); err == nil || !strings.Contains(err.Error(), "tree_parent_not_frozen") {
		t.Fatalf("root before children must be refused, got %v", err)
	}
}

func TestFreezeBottomUpBindsChildren(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		out, err := freeze(t, proj, id)
		if err != nil {
			t.Fatalf("freeze %s: %v\n%s", id, err, out)
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	f, err := intenttree.ReadNode(dir, "f")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.ChildFingerprints) != 2 {
		t.Fatalf("feature freeze must bind 2 child fingerprints, got %d", len(f.ChildFingerprints))
	}
	// Idempotent.
	out, err := freeze(t, proj, "t1")
	if err != nil || !strings.Contains(out, "true") {
		t.Fatalf("second freeze must be idempotent: %v\n%s", err, out)
	}
	_ = os.Getenv
}

func TestPlanApplyThenFreezeWholeTree(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	rep := parseDoc(t, mustRun(t, "migrate", "tree", "--project", proj, "--json"))
	rootID := rep["root_id"].(string)
	base := t.TempDir()

	apply := func(parent, body string) map[string]any {
		p := filepath.Join(base, "p.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", parent, "--proposal", p, "--json"))
	}
	mod := apply(rootID, `{"children":[{"kind":"module","title":"auth"}]}`)
	modID := mod["nodes"].([]any)[0].(map[string]any)["id"].(string)
	feat := apply(modID, `{"children":[{"kind":"feature","title":"login"}]}`)
	featID := feat["nodes"].([]any)[0].(map[string]any)["id"].(string)
	leaf := apply(featID, `{"children":[
		{"kind":"touchpoint","title":"submit","acs":[{"id":"a1","text":"可提交"}],"verification":{"type":"test","spec":"当提交时返回结果"}},
		{"kind":"touchpoint","title":"cancel","acs":[{"id":"a2","text":"可取消"}],"verification":{"type":"test","spec":"当取消时释放名额"}}
	]}`)
	leaves := leaf["nodes"].([]any)

	// Freeze the new branch bottom-up.
	for _, l := range leaves {
		id := l.(map[string]any)["id"].(string)
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze leaf %s: %v", id, err)
		}
	}
	for _, id := range []string{featID, modID} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	// Freeze the imported branch, then the root.
	doc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	var legacyFeature, legacyModule string
	for _, n := range doc["nodes"].([]any) {
		m := n.(map[string]any)
		if m["kind"] == "feature" && m["title"] == "demo" {
			legacyFeature = m["id"].(string)
		}
		if m["kind"] == "module" && m["title"] == "Legacy packages" {
			legacyModule = m["id"].(string)
		}
	}
	for _, id := range []string{legacyFeature, legacyModule, rootID} {
		if id == "" {
			t.Fatal("missing legacy node id")
		}
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	root := parseDoc(t, mustRun(t, "truth", "tree", "--project", proj, "--node", rootID, "--json"))
	if root["status"] != "frozen" {
		t.Fatalf("root status = %v", root["status"])
	}
	if out := mustRun(t, "verify", "audit", "--project", proj, "--json"); parseDoc(t, out)["ok"] != true {
		t.Fatalf("audit failed after freeze: %s", out)
	}
}

func TestPlanApplyRejections(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "demo", "--json")
	rep := parseDoc(t, mustRun(t, "migrate", "tree", "--project", proj, "--json"))
	rootID := rep["root_id"].(string)
	doc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	var featureID string
	for _, n := range doc["nodes"].([]any) {
		m := n.(map[string]any)
		if m["kind"] == "feature" {
			featureID = m["id"].(string)
		}
	}
	if featureID == "" {
		t.Fatal("migrated feature missing")
	}
	base := t.TempDir()
	apply := func(parent, body string) error {
		p := filepath.Join(base, "p.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := runCLI(t, "plan", "apply", "--project", proj, "--node", parent, "--proposal", p, "--json")
		return err
	}
	eight := `{"children":[` + strings.Repeat(`{"kind":"module","title":"m"},`, 7) + `{"kind":"module","title":"m8"}]}`
	if err := apply(rootID, eight); err == nil || !strings.Contains(err.Error(), "tree_child_limit_exceeded") {
		t.Fatalf("8 children: %v", err)
	}
	if err := apply(featureID, `{"children":[{"kind":"touchpoint","title":"no-ac","verification":{"type":"test","spec":"when x then y"}}]}`); err == nil || !strings.Contains(err.Error(), "tree_ac_missing") {
		t.Fatalf("missing AC: %v", err)
	}
	if err := apply(featureID, `{"children":[{"kind":"touchpoint","title":"no-verify","acs":[{"id":"a","text":"x"}]}]}`); err == nil || !strings.Contains(err.Error(), "tree_verification_missing") {
		t.Fatalf("missing verification: %v", err)
	}
	if err := apply(rootID, `{"children":[{"kind":"module","title":""}]}`); err == nil || !strings.Contains(err.Error(), "tree_index_invalid") {
		t.Fatalf("empty title: %v", err)
	}
	if _, err := freeze(t, proj, "nope"); err == nil || !strings.Contains(err.Error(), "tree_node_not_found") {
		t.Fatalf("missing node freeze: %v", err)
	}
}

func TestSubtreeChangeCascadeAndRefreeze(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v", id, err)
		}
	}
	doc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	base := ""
	for _, n := range doc["nodes"].([]any) {
		m := n.(map[string]any)
		if m["id"] == "t1" {
			base = m["fingerprint"].(string)
		}
	}
	out := mustRun(t, "change", "request", "--project", proj, "--node", "t1", "--summary", "widen AC", "--by", "tester", "--base-fingerprint", base, "--json")
	rec := parseDoc(t, out)
	if rec["node_id"] != "t1" {
		t.Fatalf("change record: %v", rec)
	}
	doc = parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	status := map[string]string{}
	for _, n := range doc["nodes"].([]any) {
		m := n.(map[string]any)
		status[m["id"].(string)] = m["status"].(string)
	}
	if status["t1"] != "draft" || status["root"] != "change_pending" || status["m"] != "change_pending" || status["f"] != "change_pending" {
		t.Fatalf("cascade statuses: %v", status)
	}
	if got := parseDoc(t, mustRun(t, "change", "list", "--project", proj, "--json")); len(got["changes"].([]any)) != 1 {
		t.Fatalf("change list: %v", got)
	}
	// The changed leaf is draft now; changing it again is refused.
	if _, err := runCLI(t, "change", "request", "--project", proj, "--node", "t1", "--summary", "x", "--json"); err == nil || !strings.Contains(err.Error(), "tree_change_not_allowed") {
		t.Fatalf("draft change: %v", err)
	}
	// Parent cannot refreeze before the changed leaf.
	if _, err := freeze(t, proj, "root"); err == nil || !strings.Contains(err.Error(), "tree_parent_not_frozen") {
		t.Fatalf("root before leaf refreeze: %v", err)
	}
	for _, id := range []string{"t1", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatalf("refreeze %s: %v", id, err)
		}
	}
	// Base mismatch is rejected after refreeze.
	if _, err := freeze(t, proj, "t2"); err != nil {
		t.Fatalf("freeze t2: %v", err)
	}
	if _, err := runCLI(t, "change", "request", "--project", proj, "--node", "t2", "--summary", "x", "--base-fingerprint", "deadbeef", "--json"); err == nil || !strings.Contains(err.Error(), "tree_change_base_mismatch") {
		t.Fatalf("base mismatch: %v", err)
	}
}
