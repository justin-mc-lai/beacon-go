package cli

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

func TestProgramValidateCLIBlockedExit(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "ok.json")
	if err := os.WriteFile(valid, []byte(`{"program_slug":"p","version":"v2.0.0","scope_mode":"lake",
		"lakes":[{"slug":"a","depends_on":[],"parity_subset":[],"status":"released"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runCLI(t, "goal", "program", "validate", "--file", valid, "--json"); err != nil {
		t.Fatalf("valid graph must pass: %v", err)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"program_slug":"p","version":"v2.0.0","scope_mode":"lake",
		"lakes":[{"slug":"a","depends_on":["a"],"parity_subset":[],"status":"released"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCLI(t, "goal", "program", "validate", "--file", bad, "--json")
	if err == nil {
		t.Fatal("blocked graph must return a non-nil error")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProgramDependencySelf {
		t.Fatalf("want self-dependency reason, got %v", err)
	}
}

func TestProgramExistingSubcommandsIntact(t *testing.T) {
	got := map[string]bool{}
	for _, c := range newGoalProgramCmd().Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"plan", "change", "freeze", "admit", "route", "validate", "import"} {
		if !got[want] {
			t.Errorf("program subcommand %q missing", want)
		}
	}
}
