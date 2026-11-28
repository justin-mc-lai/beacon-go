package orchestrator

import (
	"beacon-go/internal/intenttree"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCorruptLineageRejectsRuntimeConsumers(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = env.st.DB().Exec(`UPDATE project_versions SET docs_root = ? WHERE id = ?`, "docs/beacon/v2.0.0", fc.Version.ID); err != nil {
		t.Fatal(err)
	}
	fc, err = env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(env.root, "docs", "beacon", "v1.0.0", "tree", "lineage", "v2.0.0.json")
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, []byte(`{"broken":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	row, err := env.st.Runs().GetByID(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _, err := env.eng.treeStageFacts(row, StageQA, false); err == nil || ok {
		t.Fatalf("stage facts failed open: %v %v", ok, err)
	}
	if err := env.eng.validateTreeDelivery(fc); err == nil {
		t.Fatal("delivery failed open")
	}
	if err := env.eng.validateDeliveryTasks(row); err == nil {
		t.Fatal("tasks failed open")
	}
	if _, _, _, _, err := env.eng.workflowInput(run.RunID); err == nil {
		t.Fatal("workflow failed open")
	}
	direct := filepath.Join(env.root, fc.Version.DocsRoot, "tree")
	if ok, _, err := intenttree.StageGate(direct, StageQA, fc.Feature.Slug); err == nil || ok {
		t.Fatalf("direct stage failed open: %v %v", ok, err)
	}
	if _, err := env.eng.EvaluateGate(fc, "qa", run.RunID, "checker"); err == nil {
		t.Fatal("direct qa gate accepted corrupt lineage")
	}
	rewriteRunMeta(t, env, run.RunID, func(m map[string]any) { m["loop_stage"] = "qa_routing"; m["current_stage_idx"] = 3 })
	if _, err := env.eng.TickBy(context.Background(), run.RunID, "maker"); err == nil || !strings.Contains(err.Error(), "lineage") {
		t.Fatalf("tick did not propagate lineage error: %v", err)
	}
	rewriteRunMeta(t, env, run.RunID, func(m map[string]any) { m["loop_stage"] = "release_gate"; m["current_stage_idx"] = 4 })
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "human"}); err == nil || !strings.Contains(err.Error(), "lineage") {
		t.Fatalf("complete did not propagate lineage error: %v", err)
	}
}
