package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

func writeGraph(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "feature-graph.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadProgramGraphFileMissingKeys(t *testing.T) {
	p := writeGraph(t, `{"program_slug":"p","lakes":[{"slug":"a"}]}`)
	_, problems, err := LoadProgramGraphFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"missing:version", "missing:scope_mode",
		"lakes[0]:missing:depends_on", "lakes[0]:missing:parity_subset", "lakes[0]:missing:status"}
	for _, w := range want {
		found := false
		for _, pr := range problems {
			if pr.Detail == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing diagnostic %q in %+v", w, problems)
		}
	}
}

func TestLoadProgramGraphFileGraphProblems(t *testing.T) {
	p := writeGraph(t, `{"program_slug":"p","version":"v2.0.0","scope_mode":"lake",
		"lakes":[{"slug":"a","depends_on":["a"],"parity_subset":[],"status":"released"}]}`)
	_, problems, err := LoadProgramGraphFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) == 0 || problems[0].ReasonCode != core.ReasonProgramDependencySelf {
		t.Fatalf("self-dependency not reported: %+v", problems)
	}
}

func TestLoadProgramGraphFileMissing(t *testing.T) {
	_, _, err := LoadProgramGraphFile(filepath.Join(t.TempDir(), "nope.json"))
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProgramGraphFileMissing {
		t.Fatalf("want file_missing, got %v", err)
	}
}

func TestProgramImportCarriesLakeFields(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	p := writeGraph(t, `{"program_slug":"prog","version":"v2.0.0","scope_mode":"lake","goal":"big",
		"lakes":[{"slug":"feat-a","goal":"lake goal","depends_on":[],"parity_subset":["a","b"],"status":"released"}]}`)
	if _, err := env.eng.ProgramImportFile(run.RunID, p, false); err != nil {
		t.Fatal(err)
	}
	st, err := env.eng.ProgramFreeze(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.ScopeMode != "lake" || len(st.Lakes) != 1 {
		t.Fatalf("state = %+v", st)
	}
	l := st.Lakes[0]
	if l.Status != "released" || len(l.ParitySubset) != 2 || l.Goal != "lake goal" {
		t.Fatalf("lake fields lost: %+v", l)
	}
}

func TestProgramImportRefusesInvalidGraph(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	p := writeGraph(t, `{"program_slug":"prog","version":"v2.0.0","scope_mode":"lake",
		"lakes":[{"slug":"a","depends_on":["a"],"parity_subset":[],"status":"released"}]}`)
	if _, err := env.eng.ProgramImportFile(run.RunID, p, false); err == nil {
		t.Fatal("invalid graph must be refused")
	}
	if _, err := env.eng.ProgramFreeze(run.RunID); reasonOf(err) != core.ReasonProgramPlanMissing {
		t.Fatalf("invalid import must not write the store, got %v", err)
	}
}
