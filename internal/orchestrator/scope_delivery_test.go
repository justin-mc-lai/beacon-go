package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestScopeDeliveryConcurrentBindingPlansOnce(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c, idx, bodies := scopeDeliveryFixture(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, fc.Version.DocsRoot)
	idx.Version = fc.Version.Version
	for _, n := range bodies {
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	path := filepath.Join(env.root, "scope.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	ready := make(chan struct{}, 2)
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			ready <- struct{}{}
			<-start
			_, err := env.eng.BindWorkflow(run.RunID, path, "owner")
			results <- err
		}()
	}
	<-ready
	<-ready
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	bound, planned := 0, 0
	for _, event := range events {
		if event.EventType == "workflow_contract_bound" {
			bound++
		}
		if event.EventType == "workflow_tasks_planned" {
			planned++
			var d WorkflowDelivery
			if err := json.Unmarshal(event.Payload, &d); err != nil || len(d.Tasks) != 2 {
				t.Fatalf("invalid persisted tasks: %+v %v", d, err)
			}
		}
	}
	if bound != 1 || planned != 1 {
		t.Fatalf("duplicate writes: bound=%d planned=%d", bound, planned)
	}
	view, err := env.eng.WorkflowTasks(run.RunID)
	if err != nil || len(view.Tasks) != 2 {
		t.Fatalf("recovered task view: %+v %v", view, err)
	}
	n := bodies["t1"]
	n.Doc = "approved requirement change"
	n, err = intenttree.Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteNode(dir, n); err != nil {
		t.Fatal(err)
	}
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == n.ID {
			idx.Nodes[i].Fingerprint = n.Fingerprint
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.WorkflowTasks(run.RunID); err == nil {
		t.Fatal("changed canonical scope silently replaced persisted plan")
	}
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	events, err = env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	planned = 0
	for _, event := range events {
		if event.EventType == "workflow_tasks_planned" {
			planned++
		}
	}
	if planned != 2 {
		t.Fatalf("explicit scope rebind did not persist new plan: %d", planned)
	}
}

func TestScopeDeliveryDeprecatedDependencyTransmitsPrerequisites(t *testing.T) {
	c, idx, bodies := scopeDeliveryFixture(t)
	for _, n := range []intenttree.Node{{ID: "bridge", Kind: intenttree.KindModule, ParentID: "root", Path: "root/bridge", Status: intenttree.StatusFrozen}, {ID: "dead", Kind: intenttree.KindFeature, ParentID: "bridge", Path: "root/bridge/dead", Status: intenttree.StatusDeprecated}} {
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, Kind: n.Kind, ParentID: n.ParentID, Path: n.Path, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	c.ScopeNodes = append(c.ScopeNodes, "bridge")
	c.Modules = append(c.Modules, WorkflowModule{ID: "bridge", NodeID: "bridge", Depends: []string{"first"}, Features: []string{"dead"}})
	c.Modules[0].Depends = []string{"bridge"}
	d, err := resolveWorkflowDelivery(c, idx, bodies)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Tasks) != 2 || len(d.Tasks[1].DependsOn) != 1 || d.Tasks[1].DependsOn[0] != d.Tasks[0].ID {
		t.Fatalf("dependency barrier lost: %+v", d)
	}
}

func TestScopeDeliveryResumeRejectsChangedContract(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "goal", Scope: "scope", ScopeNodes: []string{"root"}}
	path := filepath.Join(env.root, "scope.json")
	raw, _ := json.Marshal(c)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.eng.Checkpoint(run.RunID, "continue investigation", "plan"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RecordInterrupt(run.RunID, "plan"); err != nil {
		t.Fatal(err)
	}
	c.Goal = "changed commitment"
	raw, _ = json.Marshal(c)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	rv, err := env.eng.Resume(run.RunID, "continue")
	if err != nil {
		t.Fatal(err)
	}
	if rv.Status != "blocked" || rv.ReasonCode != "workflow_contract_changed" {
		t.Fatalf("changed binding resumed: %+v", rv)
	}
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil || !m.InterruptPending {
		t.Fatal("rejected resume consumed interrupt")
	}
}

func TestScopeDeliveryCheckpointBindsContract(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "first", Scope: "scope", ScopeNodes: []string{"root"}}
	path := filepath.Join(env.root, "scope.json")
	write := func() {
		t.Helper()
		raw, _ := json.Marshal(c)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	cp, _, err := env.eng.Checkpoint(run.RunID, "continue", "plan")
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err := json.Unmarshal(cp.State, &state); err != nil {
		t.Fatal(err)
	}
	if state["workflow_binding_hash"] == nil || state["workflow_binding_hash"] == "" {
		t.Fatal("checkpoint has no contract binding")
	}
	c.Goal = "second"
	write()
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	rv, err := env.eng.Resume(run.RunID, "continue")
	if err != nil {
		t.Fatal(err)
	}
	if rv.Status != "blocked" || rv.ReasonCode != "workflow_checkpoint_stale" {
		t.Fatalf("old checkpoint reused after explicit rebind: %+v", rv)
	}
}

func scopeDeliveryFixture(t *testing.T) (WorkflowContract, intenttree.Index, map[string]intenttree.Node) {
	t.Helper()
	idx := intenttree.Index{Schema: intenttree.Schema, Version: "v0.0.1", RootID: "root"}
	bodies := map[string]intenttree.Node{}
	for _, n := range []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Path: "root"},
		{ID: "m1", Kind: intenttree.KindModule, ParentID: "root", Path: "root/m1"},
		{ID: "m2", Kind: intenttree.KindModule, ParentID: "root", Path: "root/m2"},
		{ID: "f1", Kind: intenttree.KindFeature, ParentID: "m1", Path: "root/m1/f", Title: "same-name"},
		{ID: "f2", Kind: intenttree.KindFeature, ParentID: "m2", Path: "root/m2/f", Title: "same-name"},
		{ID: "t1", Kind: intenttree.KindTouchpoint, ParentID: "f1", Path: "root/m1/f/t", ACs: []intenttree.AC{{ID: "a", Text: "first"}}, Verification: &intenttree.Verification{Type: "test", Spec: "go test ./..."}},
		{ID: "t2", Kind: intenttree.KindTouchpoint, ParentID: "f2", Path: "root/m2/f/t", ACs: []intenttree.AC{{ID: "a", Text: "second"}}, Verification: &intenttree.Verification{Type: "test", Spec: "go test ./..."}},
	} {
		n.Status = intenttree.StatusFrozen
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Path: n.Path, Title: n.Title, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "deliver", Scope: "two modules", ScopeNodes: []string{"m1", "m2"}, SourceNodes: []string{"t1", "t2"}, Modules: []WorkflowModule{{ID: "second", NodeID: "m2", Depends: []string{"first"}, Features: []string{"f2"}}, {ID: "first", NodeID: "m1", Features: []string{"f1"}}}, Coverage: []WorkflowCoverage{{Source: "t1", Target: "t1", AC: "a"}, {Source: "t2", Target: "t2", AC: "a"}}}
	return c, idx, bodies
}

func TestScopeDeliveryResolution(t *testing.T) {
	c, idx, bodies := scopeDeliveryFixture(t)
	resolved, err := resolveWorkflowDelivery(c, idx, bodies)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Tasks) != 2 || resolved.Tasks[0].FeatureNodeID != "f1" || resolved.Tasks[1].FeatureNodeID != "f2" || len(resolved.Tasks[1].DependsOn) != 1 || resolved.Tasks[1].DependsOn[0] != resolved.Tasks[0].ID {
		t.Fatalf("dependency plan: %+v", resolved)
	}
	again, err := resolveWorkflowDelivery(c, idx, bodies)
	if err != nil || again.Fingerprint != resolved.Fingerprint || again.Tasks[0].ID != resolved.Tasks[0].ID {
		t.Fatal("unstable replay")
	}
	// Runtime progress must not change a contract or its task identities.
	n := bodies["t1"]
	n.Status = intenttree.StatusImplemented
	n, err = intenttree.Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	bodies[n.ID] = n
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == n.ID {
			idx.Nodes[i].Status = n.Status
			idx.Nodes[i].Fingerprint = n.Fingerprint
		}
	}
	progress, err := resolveWorkflowDelivery(c, idx, bodies)
	if err != nil || progress.Fingerprint != resolved.Fingerprint {
		t.Fatalf("lifecycle changed contract: %v", err)
	}
	n.Doc = "new requirement"
	n, err = intenttree.Stamp(n)
	if err != nil {
		t.Fatal(err)
	}
	bodies[n.ID] = n
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == n.ID {
			idx.Nodes[i].Fingerprint = n.Fingerprint
		}
	}
	changed, err := resolveWorkflowDelivery(c, idx, bodies)
	if err != nil || changed.Fingerprint == resolved.Fingerprint {
		t.Fatal("requirement change retained identity")
	}
}

func TestScopeDeliveryProjectVersionIsolation(t *testing.T) {
	c, idx, bodies := scopeDeliveryFixture(t)
	resolve := func(project, version string) WorkflowDelivery {
		t.Helper()
		d, err := resolveWorkflowDelivery(c, idx, bodies)
		if err != nil {
			t.Fatal(err)
		}
		return namespaceWorkflowDelivery(d, project, version)
	}
	a, b, otherVersion := resolve("project-a", "version-a"), resolve("project-b", "version-a"), resolve("project-a", "version-b")
	if a.Fingerprint == b.Fingerprint || a.Fingerprint == otherVersion.Fingerprint || a.Tasks[0].ID == b.Tasks[0].ID || a.Tasks[0].ID == otherVersion.Tasks[0].ID {
		t.Fatal("cross-project/version identity collision")
	}
	if len(a.Tasks[1].DependsOn) != 1 || a.Tasks[1].DependsOn[0] != a.Tasks[0].ID {
		t.Fatal("namespace broke dependencies")
	}
}

func TestScopeDeliveryResolutionRefusals(t *testing.T) {
	for _, kind := range []string{"empty", "overlap", "duplicate", "unknown-dependency", "cycle", "reverse-owner", "zero-active", "blank-module"} {
		t.Run(kind, func(t *testing.T) {
			c, idx, bodies := scopeDeliveryFixture(t)
			switch kind {
			case "empty":
				c.ScopeNodes = nil
			case "overlap":
				c.ScopeNodes = append(c.ScopeNodes, "f1")
			case "duplicate":
				c.Modules[1].Features = append(c.Modules[1].Features, "f1")
			case "unknown-dependency":
				c.Modules[0].Depends = []string{"missing"}
			case "cycle":
				c.Modules[1].Depends = []string{"second"}
			case "reverse-owner":
				c.Modules[0].Features = []string{"f1"}
				c.Modules[1].Features = []string{"f2"}
			case "zero-active":
				for id, n := range bodies {
					n.Status = intenttree.StatusDeprecated
					n, _ = intenttree.Stamp(n)
					bodies[id] = n
					for i := range idx.Nodes {
						if idx.Nodes[i].ID == id {
							idx.Nodes[i].Status = n.Status
							idx.Nodes[i].Fingerprint = n.Fingerprint
						}
					}
				}
			case "blank-module":
				c.Modules[0].ID = " second "
			}
			if _, err := resolveWorkflowDelivery(c, idx, bodies); err == nil {
				t.Fatal("invalid scope accepted")
			}
		})
	}
}

func scopeAdmissionEnv(t *testing.T, secondStatus intenttree.Status) (testEnv, string) {
	env := newTestEnv(t, "frozen")
	run := env.createRun(t)
	c, idx, bodies := scopeDeliveryFixture(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, fc.Version.DocsRoot)
	idx.Version = fc.Version.Version
	second := bodies["f2"]
	second.Status = secondStatus
	second, err = intenttree.Stamp(second)
	if err != nil {
		t.Fatal(err)
	}
	bodies["f2"] = second
	for i := range idx.Nodes {
		if idx.Nodes[i].ID == "f2" {
			idx.Nodes[i].Status = second.Status
			idx.Nodes[i].Fingerprint = second.Fingerprint
		}
	}
	for _, n := range bodies {
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	path := filepath.Join(env.root, "scope.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	return env, run.RunID
}

func TestScopeAdmissionRejectsUnfrozenChild(t *testing.T) {
	env, runID := scopeAdmissionEnv(t, intenttree.StatusDraft)
	item, err := env.eng.scopeFrozenAdmissionItem(runID)
	if err != nil {
		t.Fatal(err)
	}
	if !item.Applicable || item.OK || item.ReasonCode == "" {
		t.Fatalf("unfrozen scope child admitted: %+v", item)
	}
}

func TestScopeAdmissionPassesAllFrozen(t *testing.T) {
	env, runID := scopeAdmissionEnv(t, intenttree.StatusFrozen)
	item, err := env.eng.scopeFrozenAdmissionItem(runID)
	if err != nil {
		t.Fatal(err)
	}
	if !item.Applicable || !item.OK {
		t.Fatalf("fully frozen scope blocked: %+v", item)
	}
}

func TestScopeAdmissionGateBlocksImplementAdmission(t *testing.T) {
	env, runID := scopeAdmissionEnv(t, intenttree.StatusDraft)
	result, err := env.eng.CheckImplementAdmission(runID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, code := range result.ReasonCodes {
		if code == reasonScopeNotFrozen {
			found = true
		}
	}
	if !found {
		t.Fatalf("scope freeze gate not wired into admission: %+v", result)
	}
}

func TestScopeAdmissionGateAllowsFullyFrozen(t *testing.T) {
	env, runID := scopeAdmissionEnv(t, intenttree.StatusFrozen)
	result, err := env.eng.CheckImplementAdmission(runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range result.ReasonCodes {
		if code == reasonScopeNotFrozen || code == reasonScopeChangePending {
			t.Fatalf("fully frozen scope blocked by freeze gate: %+v", result)
		}
	}
}

func TestScopeTaskStatusDerivedFromCanonicalLeaves(t *testing.T) {
	c, idx, bodies := scopeDeliveryFixture(t)
	stamp := func(id string, status intenttree.Status) {
		t.Helper()
		n := bodies[id]
		n.Status = status
		n, err := intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[id] = n
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == id {
				idx.Nodes[i].Status = n.Status
				idx.Nodes[i].Fingerprint = n.Fingerprint
			}
		}
	}
	resolve := func() WorkflowDelivery {
		t.Helper()
		d, err := resolveWorkflowDelivery(c, idx, bodies)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	if d := resolve(); d.Tasks[0].Status != "pending" || d.Tasks[1].Status != "pending" {
		t.Fatalf("frozen leaves not pending: %+v", d.Tasks)
	}
	stamp("t1", intenttree.StatusImplemented)
	if d := resolve(); d.Tasks[0].Status != "implemented" || d.Tasks[1].Status != "pending" {
		t.Fatalf("implemented leaf not reflected: %+v", d.Tasks)
	}
	stamp("t1", intenttree.StatusVerified)
	if d := resolve(); d.Tasks[0].Status != "verified" {
		t.Fatalf("verified leaf not reflected: %+v", d.Tasks)
	}
	// Re-evaluating status must not rotate task identity or scope fingerprint.
	a, b := resolve(), resolve()
	if a.Tasks[0].ID != b.Tasks[0].ID || a.Fingerprint != b.Fingerprint {
		t.Fatal("task status changed contract identity")
	}
}

func TestScopeDeliveryOldBindingRequiresRebind(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c, idx, bodies := scopeDeliveryFixture(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, fc.Version.DocsRoot)
	idx.Version = fc.Version.Version
	for _, n := range bodies {
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(c)
	path := filepath.Join(env.root, "scope.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	// Append a binding recorded before scope fingerprints existed; the latest
	// binding wins and must be refused, not silently recomputed.
	r, _, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	legacy := workflowBinding{Contract: c, Hash: digestOf(string(raw)), By: "owner", Path: path, FileHash: mustHash(t, path)}
	payload, _ := json.Marshal(legacy)
	var obj map[string]any
	_ = json.Unmarshal(payload, &obj)
	if err := env.eng.appendEvent(r, "workflow_contract_bound", obj); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.WorkflowTasks(run.RunID); err == nil {
		t.Fatal("legacy binding silently accepted a scoped delivery")
	}
	if reason, ok := env.eng.workflowScopeReady(run.RunID, &legacy); ok || reason != "workflow_scope_unbound" {
		t.Fatalf("legacy scope not refused: %s %v", reason, ok)
	}
}

func mustHash(t *testing.T, path string) string {
	t.Helper()
	h, err := qaHash(path)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func scopeAnchorFixture(t *testing.T, env testEnv) (WorkflowContract, intenttree.Index, map[string]intenttree.Node, string) {
	t.Helper()
	c, idx, bodies := scopeDeliveryFixture(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, fc.Version.DocsRoot)
	idx.Version = fc.Version.Version
	// Two scheduled features share the run's slug: the legacy slug-only rule
	// cannot disambiguate them; only an explicit anchor can.
	for _, id := range []string{"f1", "f2"} {
		n := bodies[id]
		n.Title = "feat-a"
		n, err = intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[id] = n
		for i := range idx.Nodes {
			if idx.Nodes[i].ID == id {
				idx.Nodes[i].Title = n.Title
				idx.Nodes[i].Fingerprint = n.Fingerprint
			}
		}
	}
	mwrap := intenttree.Node{ID: "mwrap", Kind: intenttree.KindModule, ParentID: "root", Path: "root/mwrap", Status: intenttree.StatusFrozen}
	wrapper := intenttree.Node{ID: "wrapper", Kind: intenttree.KindFeature, ParentID: "mwrap", Path: "root/mwrap/wrapper", Title: "feat-a", Status: intenttree.StatusFrozen}
	wleaf := intenttree.Node{ID: "wleaf", Kind: intenttree.KindTouchpoint, ParentID: "wrapper", Path: "root/mwrap/wrapper/wleaf", Status: intenttree.StatusFrozen,
		ACs: []intenttree.AC{{ID: "w", Text: "integration accepted"}}, Verification: &intenttree.Verification{Type: "test", Spec: "go test ./..."}}
	for _, n := range []intenttree.Node{mwrap, wrapper, wleaf} {
		n, err = intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		bodies[n.ID] = n
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, Kind: n.Kind, ParentID: n.ParentID, Path: n.Path, Title: n.Title, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	for _, n := range bodies {
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	return c, idx, bodies, dir
}

func bindScopeAtTruth(t *testing.T, env testEnv, runID string, c WorkflowContract) {
	t.Helper()
	raw, _ := json.Marshal(c)
	path := filepath.Join(env.root, "anchor.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindWorkflow(runID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	r, m, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	m.CurrentStageIdx = 1
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
}

func TestRunAnchorResolvesAmbiguousFeature(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	c, _, bodies, _ := scopeAnchorFixture(t, env)
	// Legacy contract: no anchor, ambiguous slug -> prepare must stay blocked.
	legacy := c
	legacy.ScopeNodes = []string{"m1", "m2"}
	legacy.SourceNodes = []string{"t1", "t2"}
	legacy.Coverage = []WorkflowCoverage{{Source: "t1", Target: "t1", AC: "a"}, {Source: "t2", Target: "t2", AC: "a"}}
	bindScopeAtTruth(t, env, run.RunID, legacy)
	view, err := env.eng.WorkflowCheck(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range view.Missing {
		if m == "workflow_feature_ambiguous" {
			found = true
		}
	}
	if !found {
		t.Fatalf("legacy slug rule accepted ambiguous feature: %+v", view)
	}

	// Explicit anchor contract: same ambiguity, but the named node wins.
	env2 := newTestEnv(t, "draft")
	run2 := env2.createRun(t)
	c2, _, bodies2, _ := scopeAnchorFixture(t, env2)
	_ = bodies2
	c2.RunAnchor = "wrapper"
	c2.ScopeNodes = append(c2.ScopeNodes, "mwrap")
	c2.SourceNodes = append(c2.SourceNodes, "wleaf")
	c2.Coverage = append(c2.Coverage, WorkflowCoverage{Source: "wleaf", Target: "wleaf", AC: "w"})
	bindScopeAtTruth(t, env2, run2.RunID, c2)
	view2, err := env2.eng.WorkflowCheck(run2.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range view2.Missing {
		if m == "workflow_feature_ambiguous" {
			t.Fatalf("anchor did not resolve ambiguity: %+v", view2)
		}
	}
	_ = bodies

	// A mismatched anchor must be refused, not silently accepted.
	env3 := newTestEnv(t, "draft")
	run3 := env3.createRun(t)
	c3, idx3, bodies3, dir3 := scopeAnchorFixture(t, env3)
	off := bodies3["f1"]
	off.Title = "different"
	off, err = intenttree.Stamp(off)
	if err != nil {
		t.Fatal(err)
	}
	bodies3["f1"] = off
	if err := intenttree.WriteNode(dir3, off); err != nil {
		t.Fatal(err)
	}
	for i := range idx3.Nodes {
		if idx3.Nodes[i].ID == "f1" {
			idx3.Nodes[i].Title = off.Title
			idx3.Nodes[i].Fingerprint = off.Fingerprint
		}
	}
	if err := intenttree.WriteIndex(dir3, idx3); err != nil {
		t.Fatal(err)
	}
	c3.RunAnchor = "f1"
	bindScopeAtTruth(t, env3, run3.RunID, c3)
	view3, err := env3.eng.WorkflowCheck(run3.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, m := range view3.Missing {
		if m == "workflow_feature_ambiguous" {
			found = true
		}
	}
	if !found {
		t.Fatalf("mismatched anchor accepted: %+v", view3)
	}
}
