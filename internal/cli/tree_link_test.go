package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func writeJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "proposal.json")
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTruthCreateAutoLinksFeatureNode(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "auth", "intent": "auth"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	_ = mod
	feature := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "auth", "intent": "auth"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	_ = feature
	if _, err := runCLI(t, "truth", "create", "--project", proj, "--slug", "auth", "--json"); err != nil {
		t.Fatalf("create: %v", err)
	}
	tree := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	for _, raw := range tree["unlinked"].([]any) {
		if raw == "auth" {
			t.Fatal("plan tree still reports auth as unlinked after truth create")
		}
	}
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Nodes {
		if e.Kind != intenttree.KindFeature || e.Title != "auth" {
			continue
		}
		n, err := intenttree.ReadNode(dir, e.ID)
		if err != nil {
			t.Fatal(err)
		}
		if n.EffectiveTruthPath() == "" {
			t.Fatal("truth create must auto-link the same-slug feature node")
		}
		return
	}
	t.Fatal("feature node not found")
}

func TestPlanTreeReportsUnlinked(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "m", "intent": "m"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	mustRun(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "Unlinked Feature", "intent": "x"}}}), "--json")
	out := mustRun(t, "plan", "tree", "--project", proj)
	if !strings.Contains(out, "tree-native: 1") {
		t.Fatalf("tree-native count missing: %s", out)
	}
	doc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	found := false
	for _, raw := range doc["unlinked"].([]any) {
		if raw == "unlinked-feature" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unlinked JSON must use the package slug: %s", doc)
	}
}

func TestSyncStatusApplyForFeatureWithoutChildren(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "m", "intent": "m"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	fid := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "pkg", "intent": "pkg"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	pkgdir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "features", "pkg")
	if err := os.MkdirAll(pkgdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgdir, "truth.md"), []byte("---\nslug: pkg\nstatus: frozen\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "truth", "node", "link", "--project", proj, "--node", fid, "--truth", "docs/beacon/v0.1.0/features/pkg/truth.md", "--json")
	out := mustRun(t, "truth", "node", "sync-status", "--project", proj, "--node", fid, "--apply", "--json")
	doc := parseDoc(t, out)
	if doc["ok"] != true {
		t.Fatalf("apply did not succeed: %s", out)
	}
	nodes := doc["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["applied"] != true {
		t.Fatalf("apply not reported: %s", out)
	}
	treeDoc := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))
	for _, raw := range treeDoc["nodes"].([]any) {
		e := raw.(map[string]any)
		if e["id"] == fid && e["status"] != "frozen" {
			t.Fatalf("tree cache status = %v, want frozen", e["status"])
		}
	}
	n, err := intenttree.ReadNode(filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree"), fid)
	if err != nil {
		t.Fatal(err)
	}
	if n.Status != intenttree.StatusFrozen {
		t.Fatalf("status = %s, want frozen", n.Status)
	}
	// A later package change is visible as a directed mismatch in plan tree.
	if err := os.WriteFile(filepath.Join(pkgdir, "truth.md"), []byte("---\nslug: pkg\nstatus: draft\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	summary := mustRun(t, "plan", "tree", "--project", proj)
	if !strings.Contains(summary, "状态不一致 1") {
		t.Fatalf("plan tree must report the package/node mismatch: %s", summary)
	}
	if !statusMatches(intenttree.StatusVerified, "verified") {
		t.Fatal("verified package status must match a verified node")
	}
	// A read-only sync report must never write the package body.
	before, err := os.ReadFile(filepath.Join(pkgdir, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	mustRun(t, "truth", "node", "sync-status", "--project", proj, "--node", fid, "--json")
	after, err := os.ReadFile(filepath.Join(pkgdir, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("sync-status report mutated the package")
	}
}

func TestPlanTreeOldVersionExplainsMaterialization(t *testing.T) {
	proj := treeTestEnv(t)
	st, err := store.Open(os.Getenv(store.EnvStorePath))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().GetByRootPath(store.ResolveRootPath(proj))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: "v0.2.0",
		DocsRoot: filepath.Join("docs", "beacon", "v0.2.0"), IsCurrent: false,
	}); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "plan", "tree", "--project", proj, "--version", "v0.2.0")
	if err == nil {
		t.Fatalf("old version without tree should fail explicitly; output: %s", out)
	}
	if !strings.Contains(err.Error(), "Old versions are not materialized by default") {
		t.Fatalf("missing explicit old-version message: %v", err)
	}
}

func seedFeatureNode(t *testing.T, proj, title, slug string) (string, string) {
	t.Helper()
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "m", "intent": "m"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	child := map[string]any{"kind": "feature", "title": title, "intent": "x"}
	if slug != "" {
		child["slug"] = slug
	}
	doc := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{child}}), "--json"))
	return doc["nodes"].([]any)[0].(map[string]any)["id"].(string), doc["nodes"].([]any)[0].(map[string]any)["path"].(string)
}

func TestProposalSlugControlsFeaturePath(t *testing.T) {
	proj := treeTestEnv(t)
	_, path := seedFeatureNode(t, proj, "派工会话复用", "dispatch-thread-reuse")
	if !strings.HasSuffix(path, "/dispatch-thread-reuse") {
		t.Fatalf("path = %q, want slug terminal segment", path)
	}
}

func TestProposalChineseTitleWithoutSlugIsRejected(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "m", "intent": "m"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	_, err := runCLI(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "派工会话复用", "intent": "x"}}}), "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_node_slug_missing") {
		t.Fatalf("Chinese title without slug must fail with tree_node_slug_missing: %v", err)
	}
}

func TestTruthNodeLinkChineseLegacyNodeByPackageSlug(t *testing.T) {
	proj := treeTestEnv(t)
	_, path := seedFeatureNode(t, proj, "派工会话复用", "node")
	if !strings.HasSuffix(path, "/node") {
		t.Fatalf("fixture path = %q", path)
	}
	if _, err := runCLI(t, "truth", "create", "--project", proj, "--slug", "dispatch-thread-reuse", "--json"); err != nil {
		t.Fatal(err)
	}
	nodeID := ""
	treeDir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	idx, err := intenttree.ReadIndex(treeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Nodes {
		if e.Kind == intenttree.KindFeature {
			nodeID = e.ID
		}
	}
	if nodeID == "" {
		t.Fatal("feature node not found")
	}
	mustRun(t, "truth", "node", "link", "--project", proj, "--node", nodeID,
		"--truth", "docs/beacon/v0.1.0/features/dispatch-thread-reuse/truth.md", "--json")
	ok, reason, err := intenttree.StageGate(treeDir, "implement", "dispatch-thread-reuse")
	if err != nil {
		t.Fatal(err)
	}
	if reason == "tree_feature_not_found" {
		t.Fatal("linked legacy node must match by truth_path directory slug")
	}
	if ok || reason != "tree_feature_not_frozen" {
		t.Fatalf("StageGate = %v %q, want frozen gate after link", ok, reason)
	}
	// Once linked, the package status can be applied even though the node
	// title/path are Chinese legacy values.
	if err := os.WriteFile(filepath.Join(proj, "docs", "beacon", "v0.1.0", "features", "dispatch-thread-reuse", "truth.md"), []byte("---\nslug: dispatch-thread-reuse\nstatus: frozen\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "truth", "node", "sync-status", "--project", proj, "--node", nodeID, "--apply", "--json")
	if ok, reason, err := intenttree.StageGate(treeDir, "implement", "dispatch-thread-reuse"); err != nil || !ok {
		t.Fatalf("after sync, linked legacy node must pass implement gate: ok=%v reason=%q err=%v", ok, reason, err)
	}
}

func TestProposalInvalidAndCollidingExplicitSlug(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	root := parseDoc(t, mustRun(t, "plan", "tree", "--project", proj, "--json"))["root_id"].(string)
	mod := parseDoc(t, mustRun(t, "plan", "apply", "--project", proj, "--node", root, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "module", "title": "m", "intent": "m"}}}), "--json"))["nodes"].([]any)[0].(map[string]any)["id"].(string)
	_, err := runCLI(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "Bad", "slug": "Bad--Slug", "intent": "x"}}}), "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_node_slug_invalid") {
		t.Fatalf("invalid explicit slug must fail: %v", err)
	}
	mustRun(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "First", "slug": "same", "intent": "x"}}}), "--json")
	_, err = runCLI(t, "plan", "apply", "--project", proj, "--node", mod, "--proposal", writeJSON(t, map[string]any{"children": []any{map[string]any{"kind": "feature", "title": "Second", "slug": "same", "intent": "x"}}}), "--json")
	if err == nil || !strings.Contains(err.Error(), "tree_node_slug_collision") {
		t.Fatalf("colliding explicit slug must fail: %v", err)
	}
}

func TestTreeSummaryShowsStatusCounts(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "plan", "init", "--project", proj, "--title", "root", "--intent", "root", "--json")
	out := mustRun(t, "plan", "tree", "--project", proj)
	if !strings.Contains(out, "状态：草稿 1 · 冻结 0 · 已交付/实现 0 · 已验证 0") {
		t.Fatalf("status counts missing or wrong: %s", out)
	}
}

func TestGoalRunAdoptsTreeFeatureWithoutPackage(t *testing.T) {
	proj := treeTestEnv(t)
	fid, _ := seedFeatureNode(t, proj, "tree-only-feature", "tree-only-feature")
	mustRun(t, "truth", "node", "freeze", "--project", proj, "--node", fid, "--json")
	out := mustRun(t, "goal", "run", "--project", proj, "--feature", "tree-only-feature", "--json")
	run := parseDoc(t, out)
	if run["run_id"] == "" || run["feature_slug"] != "tree-only-feature" {
		t.Fatalf("goal run did not adopt tree feature: %s", out)
	}
	truthPath := filepath.Join(proj, "docs", "beacon", "v0.1.0", "features", "tree-only-feature", "truth.md")
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatalf("derived truth.md missing: %v", err)
	}
	if !strings.Contains(string(raw), "source: tree") || !strings.Contains(string(raw), "tree_node_id: "+fid) {
		t.Fatalf("derived package lacks tree provenance: %s", raw)
	}
}

func TestTruthNodeDeliverRequiresVerifiedChildren(t *testing.T) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	for _, id := range []string{"t1", "t2", "f"} {
		mustRun(t, "truth", "node", "freeze", "--project", proj, "--node", id, "--json")
	}
	if _, err := runCLI(t, "truth", "node", "deliver", "--project", proj, "--node", "f", "--by", "qa"); err == nil {
		t.Fatal("delivery before verification must fail")
	}
	artifact := filepath.Join(t.TempDir(), "proof.txt")
	if err := os.WriteFile(artifact, []byte("verified\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"t1", "t2"} {
		mustRun(t, "implement", "report", "--project", proj, "--node", id, "--by", "dev", "--artifact", artifact, "--json")
		mustRun(t, "verify", "leaf", "--project", proj, "--node", id, "--by", "qa", "--artifact", artifact, "--json")
	}
	mustRun(t, "verify", "rollup", "--project", proj, "--node", "f", "--json")
	out := mustRun(t, "truth", "node", "deliver", "--project", proj, "--node", "f", "--by", "qa", "--json")
	if parseDoc(t, out)["status"] != "delivered" {
		t.Fatalf("deliver output: %s", out)
	}
}
