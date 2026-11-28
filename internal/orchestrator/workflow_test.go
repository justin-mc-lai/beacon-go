package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestStableWorkflowOwnershipAndRefusals(t *testing.T) {
	nodes := []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Title: "root", Path: "root", Status: intenttree.StatusDraft},
		{ID: "m", Kind: intenttree.KindModule, ParentID: "root", Title: "module", Path: "root/m", Status: intenttree.StatusDraft},
		{ID: "f", Kind: intenttree.KindFeature, ParentID: "m", Title: "feature", Path: "root/m/f", Status: intenttree.StatusDraft},
		{ID: "t", Kind: intenttree.KindTouchpoint, ParentID: "f", Title: "leaf", Path: "root/m/f/t", Status: intenttree.StatusDraft, ACs: []intenttree.AC{{ID: "AC-001", Text: "works"}}, Verification: &intenttree.Verification{Type: "test", Spec: "go test ."}},
	}
	idx := intenttree.Index{Schema: intenttree.Schema, RootID: "root", Version: "v0.0.1"}
	bodies := map[string]intenttree.Node{}
	for _, node := range nodes {
		n, err := intenttree.Stamp(node)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, Kind: n.Kind, ParentID: n.ParentID, Title: n.Title, Path: n.Path, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	original := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "observable goal", Scope: "one feature", ScopeNodes: []string{"f"}, SourceNodes: []string{"t"}, Modules: []WorkflowModule{{ID: "module", NodeID: "m", Features: []string{"f"}}}, Coverage: []WorkflowCoverage{{Source: "t", AC: "AC-001", Target: "t"}}}
	if err := validateWorkflow(original, idx, bodies); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*WorkflowContract)
	}{
		{"zero", func(c *WorkflowContract) { c.Modules = nil }},
		{"blank", func(c *WorkflowContract) { c.Goal = "  " }},
		{"missing_coverage", func(c *WorkflowContract) { c.Coverage = nil }},
		{"duplicate_target", func(c *WorkflowContract) { c.Coverage = append(c.Coverage, c.Coverage[0]) }},
		{"duplicate_owner", func(c *WorkflowContract) { c.Modules[0].Features = append(c.Modules[0].Features, "f") }},
		{"cycle", func(c *WorkflowContract) { c.Modules[0].Depends = []string{"module"} }},
		{"unknown_dependency", func(c *WorkflowContract) { c.Modules[0].Depends = []string{"missing"} }},
		{"unknown_source", func(c *WorkflowContract) { c.Coverage[0].Source = "missing" }},
		{"unknown_ac", func(c *WorkflowContract) { c.Coverage[0].AC = "AC-999" }},
		{"missing_source_universe", func(c *WorkflowContract) { c.SourceNodes = nil }},
		{"duplicate_scope", func(c *WorkflowContract) { c.ScopeNodes = []string{"f", "f"} }},
		{"scope_omission", func(c *WorkflowContract) { c.ScopeNodes = []string{"missing"} }},
		{"source_replaced_with_self_mapping", func(c *WorkflowContract) { c.SourceNodes = []string{"missing"}; c.Coverage[0].Source = "missing" }},
		{"overlapping_scope", func(c *WorkflowContract) { c.ScopeNodes = []string{"root", "f"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(original)
			var c WorkflowContract
			_ = json.Unmarshal(raw, &c)
			tc.mutate(&c)
			if err := validateWorkflow(c, idx, bodies); err == nil {
				t.Fatal("invalid workflow accepted")
			}
		})
	}
	n := bodies["t"]
	n.Intent = "unrestamped drift"
	bodies["t"] = n
	if err := validateWorkflow(original, idx, bodies); err == nil {
		t.Fatal("body fingerprint drift accepted")
	}
}

func TestStableWorkflowCannotOmitCanonicalSourceAndFeature(t *testing.T) {
	nodes := []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Path: "root", Status: intenttree.StatusDraft},
		{ID: "m", ParentID: "root", Kind: intenttree.KindModule, Path: "root/m", Status: intenttree.StatusDraft},
		{ID: "f", ParentID: "m", Kind: intenttree.KindFeature, Path: "root/m/f", Status: intenttree.StatusDraft},
		{ID: "old", ParentID: "f", Kind: intenttree.KindTouchpoint, Path: "root/m/f/old", Status: intenttree.StatusDraft, ACs: []intenttree.AC{{ID: "a", Text: "old"}}, Verification: &intenttree.Verification{Spec: "test"}},
		{ID: "new", ParentID: "f", Kind: intenttree.KindTouchpoint, Path: "root/m/f/new", Status: intenttree.StatusDraft, ACs: []intenttree.AC{{ID: "b", Text: "new"}}, Verification: &intenttree.Verification{Spec: "test"}},
	}
	idx := intenttree.Index{Schema: intenttree.Schema, RootID: "root", Version: "v0.0.1"}
	bodies := map[string]intenttree.Node{}
	for _, n := range nodes {
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, Kind: n.Kind, ParentID: n.ParentID, Path: n.Path, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "goal", Scope: "all", ScopeNodes: []string{"root"}, SourceNodes: []string{"new"}, Modules: []WorkflowModule{{ID: "m", NodeID: "m", Features: []string{"f"}}}, Coverage: []WorkflowCoverage{{Source: "new", Target: "new", AC: "b"}}}
	if err := validateWorkflow(c, idx, bodies); err == nil {
		t.Fatal("omitting source and using target self-map accepted")
	}
	c.SourceNodes = []string{"old", "new"}
	c.Coverage = append(c.Coverage, WorkflowCoverage{Source: "old", Target: "old", AC: "a"})
	if err := validateWorkflow(c, idx, bodies); err != nil {
		t.Fatal(err)
	}
	c.Modules[0].Features = nil
	if err := validateWorkflow(c, idx, bodies); err == nil {
		t.Fatal("omitted scheduling ownership accepted")
	}
}

func TestStableWorkflowFeatureSourcesCanMergeIntoLeaf(t *testing.T) {
	nodes := []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Path: "root", Status: intenttree.StatusDraft},
		{ID: "m", ParentID: "root", Kind: intenttree.KindModule, Path: "root/m", Status: intenttree.StatusDraft},
		{ID: "f", ParentID: "m", Kind: intenttree.KindFeature, Path: "root/m/f", Status: intenttree.StatusDraft, ACs: []intenttree.AC{{ID: "a", Text: "submit succeeds"}}},
		{ID: "t", ParentID: "f", Kind: intenttree.KindTouchpoint, Path: "root/m/f/t", Status: intenttree.StatusDraft, ACs: []intenttree.AC{{ID: "a", Text: "submit succeeds"}}, Verification: &intenttree.Verification{Spec: "test submit"}},
	}
	idx := intenttree.Index{Schema: intenttree.Schema, RootID: "root", Version: "v0.0.1"}
	bodies := map[string]intenttree.Node{}
	for _, n := range nodes {
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Path: n.Path, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "goal", Scope: "scope", ScopeNodes: []string{"f"}, SourceNodes: []string{"f"}, Modules: []WorkflowModule{{ID: "m", NodeID: "m", Features: []string{"f"}}}, Coverage: []WorkflowCoverage{{Source: "f", Target: "t", AC: "a"}}}
	if err := validateWorkflow(c, idx, bodies); err != nil {
		t.Fatalf("feature AC descent rejected: %v", err)
	}
	c.SourceNodes = append(c.SourceNodes, "t")
	c.Coverage = append(c.Coverage, WorkflowCoverage{Source: "t", Target: "t", AC: "a"})
	if err := validateWorkflow(c, idx, bodies); err != nil {
		t.Fatalf("same-text sources merge rejected: %v", err)
	}
	c.Coverage = append(c.Coverage, c.Coverage[0])
	if err := validateWorkflow(c, idx, bodies); err == nil {
		t.Fatal("duplicate source mapping accepted")
	}
	c.Coverage = c.Coverage[:2]
	_, _, before, err := workflowRenderScope(c, idx, bodies)
	if err != nil {
		t.Fatal(err)
	}
	n := bodies["t"]
	n.Status = intenttree.StatusImplemented
	n.Implementation = &intenttree.ImplementationRecord{By: "maker"}
	bodies["t"] = n
	_, _, after, err := workflowRenderScope(c, idx, bodies)
	if err != nil || before != after {
		t.Fatalf("lifecycle invalidated contract review: %s %s %v", before, after, err)
	}
	n.Doc = "changed contract"
	bodies["t"] = n
	_, _, changed, err := workflowRenderScope(c, idx, bodies)
	if err != nil || changed == before {
		t.Fatal("contract change retained review source hash")
	}
}

func TestStableWorkflowDeliveryChecksOtherFeatureEvidence(t *testing.T) {
	env := newTestEnv(t, "draft")
	// The run feature has no leaf in this probe. A different scoped feature's
	// persisted verified claim still needs its own live evidence receipt.
	bodies := map[string]intenttree.Node{
		"other":      {ID: "other", Kind: intenttree.KindFeature, Status: intenttree.StatusVerified},
		"other-leaf": {ID: "other-leaf", ParentID: "other", Kind: intenttree.KindTouchpoint, Status: intenttree.StatusVerified},
	}
	if err := env.eng.validateWorkflowDelivery(env.root, map[string]bool{"other": true, "other-leaf": true}, bodies); err == nil {
		t.Fatal("other feature's evidence was skipped")
	}
	n := bodies["other"]
	n.Status = intenttree.StatusDeprecated
	bodies["other"] = n
	if err := env.eng.validateWorkflowDelivery(env.root, map[string]bool{"other": true, "other-leaf": true}, bodies); err != nil {
		t.Fatalf("deprecated subtree required delivery evidence: %v", err)
	}
}

func TestStableWorkflowPlanWithoutTreeAndExplicitRebind(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "investigate options", Scope: "current delivery", ScopeNodes: []string{"root"}}
	p := filepath.Join(env.root, "workflow.json")
	raw, _ := json.Marshal(c)
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.BindWorkflow(run.RunID, p, "owner")
	if err != nil || !v.Ready || v.Mode != "stable" {
		t.Fatalf("plan draft: %+v %v", v, err)
	}
	r, _, _, _ := env.eng.loadRun(run.RunID)
	ready, reason, err := env.eng.treeStageGate(r, StageTruth)
	if err != nil || ready || reason != "workflow_tree_missing" {
		t.Fatalf("truth without tree: %v %s %v", ready, reason, err)
	}
	admission, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, code := range admission.ReasonCodes {
		if code == "workflow_tree_missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("direct admission bypassed workflow: %+v", admission)
	}
	_, meta, _, _ := env.eng.loadRun(run.RunID)
	meta.CurrentStageIdx = 1
	if err := env.eng.saveRun(r, meta); err != nil {
		t.Fatal(err)
	}
	exit, err := env.eng.ExitGate(run.RunID)
	if err != nil || exit.Allowed || exit.ReasonCode != "workflow_tree_missing" {
		t.Fatalf("direct exit bypassed workflow: %+v %v", exit, err)
	}
	fc, err := env.eng.featureContext(r.FeatureID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.EvaluateGate(fc, "truth_freeze", "", "reviewer"); err == nil {
		t.Fatal("stable direct gate selected implicit latest run")
	}
	for _, gateType := range []string{"truth_freeze", "implement", "qa", "release"} {
		ev, err := env.eng.EvaluateGate(fc, gateType, run.RunID, "reviewer")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, code := range ev.ReasonCodes {
			if code == "workflow_tree_missing" {
				found = true
			}
		}
		if string(ev.Verdict) != "blocked" || !found {
			t.Fatalf("direct %s bypassed workflow: %+v", gateType, ev)
		}
	}
	eventsBefore, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Status(run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.WorkflowCheck(run.RunID); err != nil {
		t.Fatal(err)
	}
	eventsAfter, err := env.st.ListEventsByRun(run.RunID)
	if err != nil || len(eventsAfter) != len(eventsBefore) {
		t.Fatalf("read-only checks wrote events: %d -> %d %v", len(eventsBefore), len(eventsAfter), err)
	}
	if err := os.WriteFile(p, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.WorkflowCheck(run.RunID)
	if err != nil || v.Ready || len(v.Missing) == 0 || v.Missing[0] != "workflow_contract_changed" {
		t.Fatalf("changed file: %+v %v", v, err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.WorkflowCheck(run.RunID)
	if err != nil || v.Mode != "stable" || v.Ready {
		t.Fatalf("deleted contract downgraded: %+v %v", v, err)
	}
}
