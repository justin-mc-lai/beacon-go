package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSDDStaleFrozenPackageCannotReFreezeOrAdmit(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeFile(t, filepath.Join(env.root, "truth.md"), "---\nslug: feat-a\nstatus: frozen\npackage_contract: truth-tasks\nintent_confirmed: true\n---\n## Acceptance Criteria\n| AC ID | Criterion |\n|---|---|\n| AC-001 | Changed after confirmation |\n")
	writeFile(t, filepath.Join(env.root, "tasks.md"), "task definitions")
	fc, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = env.eng.FreezeTruthPackage(fc, "owner"); err == nil {
		t.Fatal("stale frozen content accepted as idempotent freeze")
	}
	run := env.createRun(t)
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if adm.Status == "pass" {
		t.Fatal("stale frozen content admitted")
	}
	if _, err = os.Stat(filepath.Join(env.root, ".beacon", "task-runs")); !os.IsNotExist(err) {
		t.Fatal("admission should not execute work")
	}
}
