package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

// gitEnv mirrors testEnv but the project root is a real git repository.
type gitEnv struct {
	testEnv
}

func newGitEnv(t *testing.T) gitEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-b", "main")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "qa-source.py"), []byte("print('implementation')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "init")
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md", Status: "frozen",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(st)
	rec := NewRecordingExecutor(st, "rec-1")
	eng.RegisterExecutorForAllStages(rec)
	return gitEnv{testEnv{st: st, eng: eng, rec: rec, feature: f, root: root}}
}

func poolRun(t *testing.T, env gitEnv) RunView {
	t.Helper()
	run := env.createRun(t)
	if _, _, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{
		{Slug: "feat-a", Touches: []string{"a.go"}},
		{Slug: "lake-b", Touches: []string{"b.go"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
