//go:build !windows

package orchestrator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func TestWorkflowConcurrentPrepareChild(t *testing.T) {
	if os.Getenv("BEACON_WORKFLOW_CHILD") != "1" {
		return
	}
	f, err := os.OpenFile(filepath.Join(os.Getenv("WORKFLOW_ROOT"), ".beacon", "program.lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != syscall.EWOULDBLOCK {
		t.Fatalf("confirm did not hold cross-process lock: %v", err)
	}
	fmt.Println("writer-blocked")
	st, err := store.Open(os.Getenv("WORKFLOW_DB"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := NewEngine(st).PrepareWorkflow(os.Getenv("WORKFLOW_RUN")); err != nil {
		t.Fatal(err)
	}
}

func TestStableWorkflowConfirmDoesNotApproveConcurrentPreparation(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, fc.Version.DocsRoot)
	nodes := []intenttree.Node{
		{ID: "root", Kind: intenttree.KindIntent, Path: "root", Status: intenttree.StatusDraft},
		{ID: "m", ParentID: "root", Kind: intenttree.KindModule, Path: "root/m", Status: intenttree.StatusDraft},
		{ID: "feat-a", ParentID: "m", Kind: intenttree.KindFeature, Path: "root/m/feat-a", Status: intenttree.StatusFrozen},
		{ID: "t", ParentID: "feat-a", Kind: intenttree.KindTouchpoint, Path: "root/m/feat-a/t", Status: intenttree.StatusFrozen, ACs: []intenttree.AC{{ID: "a", Text: "works"}}, Verification: &intenttree.Verification{Spec: "test"}},
		{ID: "dead", ParentID: "feat-a", Kind: intenttree.KindTouchpoint, Path: "root/m/feat-a/dead", Status: intenttree.StatusDeprecated},
		{ID: "dead-feature", ParentID: "m", Kind: intenttree.KindFeature, Path: "root/m/dead-feature", Status: intenttree.StatusDeprecated},
		{ID: "unfinished", ParentID: "dead-feature", Kind: intenttree.KindTouchpoint, Path: "root/m/dead-feature/unfinished", Status: intenttree.StatusDraft},
	}
	idx := intenttree.Index{Schema: intenttree.Schema, RootID: "root", Version: fc.Version.Version}
	for _, n := range nodes {
		n, err = intenttree.Stamp(n)
		if err != nil {
			t.Fatal(err)
		}
		if err := intenttree.WriteNode(dir, n); err != nil {
			t.Fatal(err)
		}
		idx.Nodes = append(idx.Nodes, intenttree.IndexEntry{ID: n.ID, ParentID: n.ParentID, Kind: n.Kind, Path: n.Path, Status: n.Status, Fingerprint: n.Fingerprint, FingerprintAlg: n.FingerprintAlg})
	}
	if err := intenttree.WriteIndex(dir, idx); err != nil {
		t.Fatal(err)
	}
	c := WorkflowContract{Schema: "beacon-workflow.v1", Goal: "goal", Scope: "scope", ScopeNodes: []string{"feat-a", "dead-feature"}, SourceNodes: []string{"t"}, Modules: []WorkflowModule{{ID: "m", NodeID: "m", Features: []string{"feat-a", "dead-feature"}}}, Coverage: []WorkflowCoverage{{Source: "t", Target: "t", AC: "a"}}}
	raw, _ := json.Marshal(c)
	path := filepath.Join(env.root, "workflow.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindWorkflow(run.RunID, path, "owner"); err != nil {
		t.Fatal(err)
	}
	r, m, _, _ := env.eng.loadRun(run.RunID)
	m.CurrentStageIdx = 1
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.PrepareWorkflow(run.RunID); err != nil {
		t.Fatal(err)
	}
	_, a, err := env.eng.workflowBinding(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, bodies, err := env.eng.workflowInput(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	_, rendered, source, err := workflowRenderScope(c, idx, bodies)
	if err != nil {
		t.Fatal(err)
	}
	if rendered["dead-feature"].Status != intenttree.StatusDeprecated || rendered["unfinished"].Status != intenttree.StatusDeprecated {
		t.Fatal("review lost tombstones")
	}
	dead := bodies["dead"]
	dead.Doc = "tombstone history changed"
	bodies["dead"] = dead
	_, _, changed, err := workflowRenderScope(c, idx, bodies)
	if err != nil || changed == source {
		t.Fatal("tombstone missing from contract fingerprint")
	}
	dead.Doc = ""
	bodies["dead"] = dead
	zero := c
	zero.ScopeNodes = []string{"dead-feature"}
	zero.Modules = []WorkflowModule{{ID: "m", NodeID: "m", Features: []string{"dead-feature"}}}
	if err := validateWorkflow(zero, idx, bodies); err == nil || !strings.Contains(err.Error(), "zero active target") {
		t.Fatalf("zero active target workflow rejection: %v", err)
	}
	var seq int
	var name, db string
	if err := env.st.DB().QueryRow("PRAGMA database_list").Scan(&seq, &name, &db); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkflowConcurrentPrepareChild$")
	cmd.Env = append(os.Environ(), "BEACON_WORKFLOW_CHILD=1", "WORKFLOW_ROOT="+env.root, "WORKFLOW_DB="+db, "WORKFLOW_RUN="+run.RunID)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	env.eng.workflowConfirmCaptured = func() {
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(stdout)
		if !scanner.Scan() || scanner.Text() != "writer-blocked" {
			t.Fatal("second executor did not reach lock barrier")
		}
	}
	if _, err := env.eng.ConfirmWorkflow(run.RunID, workflowPreparedHash(*a), "human"); err != nil {
		t.Fatal(err)
	}
	if ready, reason, err := env.eng.treeStageFacts(r, StageTruth, true); err != nil || !ready {
		t.Fatalf("deprecated draft blocked truth: %v %s %v", ready, reason, err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	_, b, err := env.eng.workflowBinding(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if workflowPreparedHash(*a) == workflowPreparedHash(*b) {
		t.Fatal("second executor did not prepare B")
	}
	confirmed, err := env.eng.workflowConfirmed(run.RunID, *b)
	if err != nil || confirmed {
		t.Fatalf("A approved concurrent B: %v %v", confirmed, err)
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.EventType == "workflow_confirmed" {
			var got workflowConfirmation
			if err := json.Unmarshal(event.Payload, &got); err != nil {
				t.Fatal(err)
			}
			if got.PreparedHash != workflowPreparedHash(*a) {
				t.Fatal("confirmation did not write captured A")
			}
		}
	}
}
