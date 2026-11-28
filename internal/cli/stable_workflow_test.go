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

func TestStableWorkflowPrepareCheckAndTick(t *testing.T) {
	proj := treeTestEnv(t)
	mustRun(t, "truth", "create", "--project", proj, "--slug", "f", "--json")
	run := parseDoc(t, mustRun(t, "goal", "run", "--project", proj, "--version", "v0.1.0", "--feature", "f", "--executor", "manual", "--json"))["run_id"].(string)
	p := filepath.Join(proj, "workflow.json")
	if err := os.WriteFile(p, []byte(`{"schema":"beacon-workflow.v1","goal":"investigate submission","scope":"this round","scope_nodes":["f"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "goal", "workflow", "bind", "--run-id", run, "--file", p, "--by", "owner")
	tick := parseDoc(t, mustRun(t, "goal", "tick", "--run-id", run, "--by", "host", "--json"))
	if tick["current_stage"] != "truth" {
		t.Fatalf("plan without tree: %v", tick)
	}
	if tick["workflow"].(map[string]any)["mode"] != "stable" {
		t.Fatalf("tick lost workflow mode: %v", tick)
	}
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil || !strings.Contains(err.Error(), "workflow_tree_missing") {
		t.Fatalf("truth without tree: %v", err)
	}
	seedFreezeTree(t, proj, false)
	contract := map[string]any{"schema": "beacon-workflow.v1", "goal": "users can submit and cancel", "scope": "feature f only", "scope_nodes": []string{"f"}, "source_nodes": []string{"t1", "t2"}, "modules": []any{map[string]any{"id": "module", "node_id": "m", "depends_on": []string{}, "features": []string{"f"}}}, "coverage": []any{map[string]string{"source_node": "t1", "ac": "t1-1", "target_node": "t1"}, map[string]string{"source_node": "t2", "ac": "t2-1", "target_node": "t2"}}}
	raw, _ := json.Marshal(contract)
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "goal", "workflow", "bind", "--run-id", run, "--file", p, "--by", "owner")
	_, err := runCLI(t, "goal", "workflow", "check", "--run-id", run)
	if err == nil || !strings.Contains(err.Error(), "workflow_artifacts_missing") {
		t.Fatalf("truth without artifacts: %v", err)
	}
	for _, id := range []string{"t1", "t2", "f"} {
		if out, err := freeze(t, proj, id); err != nil {
			t.Fatalf("freeze %s: %v %s", id, err, out)
		}
	}
	// Preserve the existing independent package freeze acknowledgement gate.
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	r, err := st.Runs().GetByID(run)
	if err != nil {
		t.Fatal(err)
	}
	feature, err := st.Features().GetByID(r.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	truthPath := filepath.Join(proj, feature.TruthPath)
	if err := os.WriteFile(truthPath, []byte("---\nstatus: frozen\n---\n# Confirmed delivery\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: r.FeatureID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	st.Close()
	prepared := parseDoc(t, mustRun(t, "goal", "workflow", "prepare", "--run-id", run))
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil || !strings.Contains(err.Error(), "workflow_confirmation_required") {
		t.Fatalf("prepare silently confirmed: %v", err)
	}
	if _, err := runCLI(t, "goal", "workflow", "confirm", "--run-id", run, "--by", "human", "--prepared-hash", "wrong"); err == nil {
		t.Fatal("wrong expected prepared hash accepted")
	}
	mustRun(t, "goal", "workflow", "confirm", "--run-id", run, "--by", "human", "--prepared-hash", prepared["prepared_hash"].(string))
	html := prepared["html_path"].(string)
	if _, err := os.Stat(html); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(html)
	if err != nil {
		t.Fatal(err)
	}
	check := parseDoc(t, mustRun(t, "goal", "workflow", "check", "--run-id", run))
	if check["ready"] != true || check["mode"] != "stable" {
		t.Fatalf("check: %v", check)
	}
	status := parseDoc(t, mustRun(t, "goal", "status", "--run-id", run, "--json"))
	if status["workflow"].(map[string]any)["html_path"] != html {
		t.Fatalf("status/check drift: %v", status)
	}
	after, _ := os.ReadFile(html)
	if string(before) != string(after) {
		t.Fatal("check wrote HTML")
	}
	manifest := prepared["coverage_manifest"].(string)
	if err := os.WriteFile(manifest, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil {
		t.Fatal("tampered manifest accepted")
	}
	prepared = parseDoc(t, mustRun(t, "goal", "workflow", "prepare", "--run-id", run))
	html = prepared["html_path"].(string)
	if err := os.WriteFile(html, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil {
		t.Fatal("tampered HTML accepted")
	}
	prepared = parseDoc(t, mustRun(t, "goal", "workflow", "prepare", "--run-id", run))
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil || !strings.Contains(err.Error(), "workflow_confirmation_required") {
		t.Fatalf("HTML replacement retained confirmation: %v", err)
	}
	mustRun(t, "goal", "workflow", "confirm", "--run-id", run, "--by", "human", "--prepared-hash", prepared["prepared_hash"].(string))
	// Canonical alias ambiguity must fail even when the scheduled node matches.
	dir := filepath.Join(proj, "docs", "beacon", "v0.1.0", "tree")
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := intenttree.ReadNode(dir, "t1")
	if err != nil {
		t.Fatal(err)
	}
	leaf.Status = intenttree.StatusImplemented
	leaf, err = intenttree.Stamp(leaf)
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, leaf); err != nil {
		t.Fatal(err)
	}
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == leaf.ID {
			idx.Nodes[i].Status = leaf.Status
			idx.Nodes[i].Fingerprint = leaf.Fingerprint
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	mustRun(t, "goal", "workflow", "check", "--run-id", run)
	original := idx
	alias, err := intenttree.Stamp(intenttree.Node{ID: "alias", ParentID: "m", Kind: intenttree.KindFeature, Title: "alias", Path: "m/alias", TruthPath: "docs/beacon/v0.1.0/features/f/truth.md", Status: intenttree.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, alias); err != nil {
		t.Fatal(err)
	}
	idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: alias.ID, ParentID: alias.ParentID, Kind: alias.Kind, Title: alias.Title, Path: alias.Path, Status: alias.Status, Fingerprint: alias.Fingerprint, FingerprintAlg: alias.FingerprintAlg})
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "goal", "workflow", "check", "--run-id", run); err == nil || !strings.Contains(err.Error(), "workflow_feature_ambiguous") {
		t.Fatalf("ambiguous alias: %v", err)
	}
	if err := intenttree.WriteIndex(dir, original); err != nil {
		t.Fatal(err)
	}
	// Valid future work outside this delivery must not stale the scoped HTML.
	future, err := intenttree.Stamp(intenttree.Node{ID: "future", ParentID: "m", Kind: intenttree.KindFeature, Title: "future", Path: "m/future", Status: intenttree.StatusDraft})
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, future); err != nil {
		t.Fatal(err)
	}
	futureIdx := original
	futureIdx.Nodes = append(futureIdx.Nodes, intenttree.IndexEntry{ID: future.ID, ParentID: future.ParentID, Kind: future.Kind, Title: future.Title, Path: future.Path, Status: future.Status, Fingerprint: future.Fingerprint, FingerprintAlg: future.FingerprintAlg})
	if err := intenttree.WriteIndex(dir, futureIdx); err != nil {
		t.Fatal(err)
	}
	// A fresh read-only engine reconstructs the prior prepared evidence.
	mustRun(t, "goal", "workflow", "check", "--run-id", run)
	tick = parseDoc(t, mustRun(t, "goal", "tick", "--run-id", run, "--by", "truth-host", "--json"))
	if tick["current_stage"] != "implement" {
		t.Fatalf("truth failed to advance: %v", tick)
	}
}
