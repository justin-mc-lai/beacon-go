package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func seedModuleTree(t *testing.T, proj string) map[string]string {
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
		t.Fatalf("versions: %v", err)
	}
	ver := vers[0]
	dir := intenttree.TreeDir(pr.RootPath, ver.DocsRoot)
	doc := "## 问题\n- x\n## 目标用户\n- x\n## 场景\n- x\n## 范围\n- x\n## 非目标\n- x\n## 验收标准\n- x\n"
	stamp := func(n intenttree.Node) intenttree.Node {
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	one := stamp(intenttree.Node{ID: "alpha-one", Kind: intenttree.KindFeature, ParentID: "alpha", Title: "one", Path: "root/alpha/one", Status: intenttree.StatusFrozen, Doc: doc})
	two := stamp(intenttree.Node{ID: "alpha-two", Kind: intenttree.KindFeature, ParentID: "alpha", Title: "two", Path: "root/alpha/two", Status: intenttree.StatusFrozen, Doc: doc})
	betaOne := stamp(intenttree.Node{ID: "beta-one", Kind: intenttree.KindFeature, ParentID: "beta", Title: "beta one", Path: "root/beta/beta-one", Status: intenttree.StatusFrozen, Doc: doc})
	nodes := []intenttree.Node{
		stamp(intenttree.Node{ID: "root", Kind: intenttree.KindIntent, Title: "root", Path: "root", Status: intenttree.StatusFrozen}),
		stamp(intenttree.Node{ID: "alpha", Kind: intenttree.KindModule, ParentID: "root", Title: "alpha", Path: "root/alpha", Status: intenttree.StatusFrozen,
			// Declaration order deliberately differs from path order.
			ChildFingerprints: []string{two.Fingerprint, one.Fingerprint}}),
		one, two,
		stamp(intenttree.Node{ID: "beta", Kind: intenttree.KindModule, ParentID: "root", Title: "beta", Path: "root/beta", Status: intenttree.StatusFrozen,
			ChildFingerprints: []string{betaOne.Fingerprint}}),
		betaOne,
		stamp(intenttree.Node{ID: "empty", Kind: intenttree.KindModule, ParentID: "root", Title: "empty", Path: "root/empty", Status: intenttree.StatusFrozen}),
	}
	idx := intenttree.Index{Schema: intenttree.Schema, Version: ver.Version, RootID: "root"}
	now := time.Now().UTC()
	var cache []store.TreeNode
	for _, n := range nodes {
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{
			ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Status: n.Status,
			Title: n.Title, Path: n.Path, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg,
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
	mig := store.TreeMigration{ID: "tmig_module_test", ProjectVersionID: ver.ID, Source: "test",
		NodeCount: len(cache), CreatedAt: now.Format(time.RFC3339Nano)}
	if err := st.SaveTree(ver.ID, cache, mig); err != nil {
		t.Fatal(err)
	}
	return map[string]string{"alpha": "alpha", "one": "alpha-one", "two": "alpha-two", "beta": "beta", "empty": "empty"}
}

func TestGoalModuleRunExpandsFeatureRuns(t *testing.T) {
	proj := treeTestEnv(t)
	ids := seedModuleTree(t, proj)
	out, err := runCLI(t, "goal", "module", "run", "--project", proj, "--node", ids["alpha"], "--json")
	if err != nil {
		t.Fatalf("goal module run: %v\n%s", err, out)
	}
	var runs []map[string]any
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		t.Fatalf("runs must be one JSON array: %v\n%s", err, out)
	}
	if len(runs) != 2 {
		t.Fatalf("run count = %d, want 2", len(runs))
	}
	if runs[0]["feature_node_id"] != ids["two"] || runs[1]["feature_node_id"] != ids["one"] {
		t.Fatalf("runs must follow ChildFingerprints declaration order [two, one]: %v", runs)
	}

	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	parents := map[string]string{}
	for _, e := range idx.Nodes {
		parents[e.ID] = e.ParentID
	}
	for _, r := range runs {
		nodeID, _ := r["feature_node_id"].(string)
		if parents[nodeID] != ids["alpha"] {
			t.Fatalf("feature %s is not under module %s: %v", nodeID, ids["alpha"], r)
		}
		featureID, _ := r["feature_id"].(string)
		if featureID == "" {
			t.Fatalf("run has no feature_id: %v", r)
		}
		found := false
		for _, fc := range mustFeatureContexts(t, st, proj) {
			if fc.Feature.ID == featureID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("feature_id %s not registered", featureID)
		}
	}

	secondID, _ := runs[1]["run_id"].(string)
	statusOut, err := runCLI(t, "goal", "status", "--run-id", secondID, "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := parseDoc(t, statusOut)
	if status["blocked"] != false || status["current_stage"] != "plan" {
		t.Fatalf("second module run must be pending at plan: %v", status)
	}
	moduleRun, ok := status["module_run"].(map[string]any)
	if !ok || moduleRun["state"] != "pending" || moduleRun["module_node_id"] != ids["alpha"] {
		t.Fatalf("module_run pending state missing: %v", status)
	}

	firstID, _ := runs[0]["run_id"].(string)
	firstStatusOut, err := runCLI(t, "goal", "status", "--run-id", firstID, "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	firstStatus := parseDoc(t, firstStatusOut)
	implement, ok := firstStatus["implement"].(map[string]any)
	if !ok || implement["worktree_ref"] != ".beacon/worktrees/satellite/alpha" {
		t.Fatalf("first module run must be bound to its module workstation: %v", firstStatus)
	}
	wtOut, err := runCLI(t, "goal", "worktree", "status", "--run-id", firstID, "--json")
	if err != nil {
		t.Fatalf("worktree status: %v", err)
	}
	view := parseDoc(t, wtOut)
	plan, ok := view["parallel_plan"].(map[string]any)
	if !ok || plan["mode"] != "pool" {
		t.Fatalf("parallel_plan mode=pool missing: %v", view)
	}
	if got := len(plan["satellites"].([]any)); got != 3 {
		t.Fatalf("satellites count = %d, want 3 modules", got)
	}

	pendingTickOut, err := runCLI(t, "goal", "tick", "--run-id", secondID, "--by", "module-test", "--json")
	if err != nil {
		t.Fatalf("tick pending module run: %v\n%s", err, pendingTickOut)
	}
	pendingTick := parseDoc(t, pendingTickOut)
	if pendingTick["reason_code"] != core.ReasonModuleRunPending {
		t.Fatalf("pending module run must refuse tick: %v", pendingTick)
	}
	statusOut, err = runCLI(t, "goal", "status", "--run-id", secondID, "--json")
	if err != nil {
		t.Fatalf("status after refused tick: %v", err)
	}
	status = parseDoc(t, statusOut)

	if status["current_stage"] != "plan" {
		t.Fatalf("pending module run advanced after tick: %v", status)
	}

	tickOut, err := runCLI(t, "goal", "tick", "--run-id", firstID, "--by", "module-test")
	if err != nil {
		t.Fatalf("tick first module run: %v\n%s", err, tickOut)
	}
	// The module-prefixed package slug must still resolve to its frozen
	// tree node: the first run must be able to leave the truth stage.
	truthTickOut, err := runCLI(t, "goal", "tick", "--run-id", firstID, "--by", "module-test", "--json")
	if err != nil {
		t.Fatalf("tick first module run past truth: %v\n%s", err, truthTickOut)
	}
	truthTick := parseDoc(t, truthTickOut)
	if truthTick["blocked"] == true {
		t.Fatalf("module-prefixed slug broke tree stage gate: %v", truthTick)
	}
	statusOut, err = runCLI(t, "goal", "status", "--run-id", secondID, "--json")
	if err != nil {
		t.Fatalf("status after predecessor stage: %v", err)
	}
	status = parseDoc(t, statusOut)
	released, _ := status["module_run"].(map[string]any)
	if released == nil || released["state"] != "running" {
		t.Fatalf("second module run must be released after predecessor stage: %v", status)
	}
}

// CLI contract refusal probes: empty module, wrong node kind, no --node.
func TestGoalModuleRunRejectsInvalidRequests(t *testing.T) {
	proj := treeTestEnv(t)
	ids := seedModuleTree(t, proj)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"empty module", []string{"goal", "module", "run", "--project", proj, "--node", ids["empty"]}, "tree_module_empty"},
		{"wrong kind", []string{"goal", "module", "run", "--project", proj, "--node", ids["one"]}, "tree_invalid_child_kind"},
		{"missing node", []string{"goal", "module", "run", "--project", proj}, "required flag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runCLI(t, tc.args...)
			if err == nil {
				t.Fatalf("expected refusal, got success: %s", out)
			}
			if !strings.Contains(out+err.Error(), tc.want) {
				t.Fatalf("refusal missing %q: %s %v", tc.want, out, err)
			}
		})
	}
}

func mustFeatureContexts(t *testing.T, st *store.Store, proj string) []store.FeatureContext {
	t.Helper()
	out, err := st.ListFeatureContexts(proj)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
