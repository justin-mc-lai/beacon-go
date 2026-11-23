package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSDDDecomposePreservesAuthoredContract(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackageSDD(env.root, "", "draft"); err != nil {
		t.Fatal(err)
	}
	fc := env.ctx(t, "draft")
	text, err := readTruthText(fc)
	if err != nil {
		t.Fatal(err)
	}
	text = replaceSection(text, "## Acceptance Criteria", "| AC ID | Criterion |\n|---|---|\n| AC-001 | Preserve exact authored behavior |")
	text = replaceSection(text, "## Grounding", "Human-authored source reference")
	if err = writeTruthText(fc, text); err != nil {
		t.Fatal(err)
	}
	tasksPath := filepath.Join(env.root, filepath.FromSlash("docs/beacon/v2.0.0/features/draft/tasks.md"))
	before, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.DecomposeTruthPackage(fc, "报价保留两位小数", nil, "", "writer")
	if err != nil {
		t.Fatal(err)
	}
	after, err := readTruthText(fc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after, "Preserve exact authored behavior") || !strings.Contains(after, "Human-authored source reference") || !strings.Contains(after, "## Decomposition Suggestions") {
		t.Fatal("decomposition destroyed authored truth")
	}
	now, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(now) {
		t.Fatal("decomposition overwrote task DAG")
	}
	if _, err = os.Stat(filepath.Join(filepath.Dir(tasksPath), "tests.md")); !os.IsNotExist(err) {
		t.Fatal("SDD decomposition created legacy tests.md")
	}
	if view.Method != "deterministic_non_llm" || view.SemanticReady || view.Decomposition.Readiness.Ready {
		t.Fatalf("candidate falsely ready: %+v", view)
	}
	if _, err = env.eng.FreezeTruthPackage(fc, "owner"); err == nil {
		t.Fatal("candidate suggestions froze without authored tests/review")
	}
}

func TestSDDDecomposeRefusesArtifactSymlink(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackageSDD(env.root, "", "draft"); err != nil {
		t.Fatal(err)
	}
	fc := env.ctx(t, "draft")
	tasks := filepath.Join(env.root, "docs/beacon/v2.0.0/features/draft/tasks.md")
	before, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(env.root, ".beacon/state/decompose")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(tasks, filepath.Join(dir, "draft.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DecomposeTruthPackage(fc, "报价两位小数", nil, "", "writer"); err == nil {
		t.Fatal("artifact symlink accepted")
	}
	after, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("artifact symlink overwrote task definitions")
	}
}
