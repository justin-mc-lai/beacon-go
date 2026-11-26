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
	if !strings.Contains(out, "unlinked:") || !strings.Contains(out, "unlinked-feature") {
		t.Fatalf("unlinked report must use the package slug: %s", out)
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
