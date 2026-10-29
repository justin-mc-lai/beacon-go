package skills

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// H9: the embedded set is the full Scheme A 1+6 — plan included —
// and every skill carries at least one references file.
func TestEmbeddedSetIsOnePlusSix(t *testing.T) {
	list := List()
	if len(list) != 7 {
		t.Fatalf("embedded skills = %d, want 7 (goal + 1+6 cores)", len(list))
	}
	var plan *Skill
	for i := range list {
		if list[i].ID == "plan" {
			plan = &list[i]
		}
		if refs := References(list[i].ID); len(refs) == 0 {
			t.Errorf("skill %s has no references", list[i].ID)
		}
	}
	if plan == nil {
		t.Fatal("plan skill missing from the embedded set")
	}
	if plan.Name != "beacon-plan" {
		t.Errorf("plan name = %q, want beacon-plan", plan.Name)
	}
	if _, ok := Get("plan"); !ok {
		t.Error("Get(plan) failed")
	}
}

func TestInstallMaterializesHostLayout(t *testing.T) {
	dir := t.TempDir()
	res, err := Install(dir, false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(res.Skills) != 7 {
		t.Fatalf("installed %d skills, want 7", len(res.Skills))
	}
	for _, id := range IDs() {
		skillPath := filepath.Join(dir, "beacon-"+id, "SKILL.md")
		raw, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("installed SKILL.md for %s: %v", id, err)
		}
		if !strings.Contains(string(raw), "name: beacon-"+id) {
			t.Errorf("installed %s does not carry its name frontmatter", skillPath)
		}
		if _, err := os.Stat(filepath.Join(dir, "beacon-"+id, "references", "core.md")); err != nil {
			t.Errorf("installed references for %s: %v", id, err)
		}
		// The host must get the examples and the eval cases too —
		// they are what anchors behaviour and what `qa score` reads.
		if _, err := os.Stat(filepath.Join(dir, "beacon-"+id, "evals", "evals.json")); err != nil {
			t.Errorf("installed evals for %s: %v", id, err)
		}
		if n := len(Examples(id)); n < 3 {
			t.Errorf("skill %s ships %d examples, want >= 3", id, n)
		}
	}
	// Host discovery shape: a flat beacon-plan dir at the top level.
	if _, err := os.Stat(filepath.Join(dir, "beacon-plan", "SKILL.md")); err != nil {
		t.Errorf("host discovery layout broken: %v", err)
	}
}

func TestInstallConflictRefusesWholeInstall(t *testing.T) {
	dir := t.TempDir()
	if _, err := Install(dir, false); err != nil {
		t.Fatal(err)
	}
	// Tamper a file: the refused pass must leave it alone, the
	// forced pass must restore the shipped bytes.
	skillPath := filepath.Join(dir, "beacon-plan", "SKILL.md")
	raw, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skillPath, append(raw, []byte("\nTAMPERED\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Install(dir, false)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("second install err = %v, want ConflictError", err)
	}
	if len(conflict.Paths) != 7 {
		t.Errorf("conflicts = %d, want all 7 skill dirs", len(conflict.Paths))
	}
	after, _ := os.ReadFile(skillPath)
	if !strings.Contains(string(after), "TAMPERED") {
		t.Error("refused install overwrote a file")
	}
	if _, err := Install(dir, true); err != nil {
		t.Fatalf("forced install: %v", err)
	}
	after, _ = os.ReadFile(skillPath)
	if strings.Contains(string(after), "TAMPERED") {
		t.Error("forced install did not restore the shipped SKILL.md")
	}
}
