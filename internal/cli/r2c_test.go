package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

// H9: skills install materializes the embedded set into a host dir,
// refuses a second install without --force, and references resolve.
func TestSkillsInstallCLI(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hostskills")
	out, err := runCLI(t, "skills", "install", "--dir", dir, "--json")
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if got, _ := doc["skills"].([]any); len(got) != 7 {
		t.Fatalf("installed skills = %v, want 7", doc)
	}
	if _, err := os.Stat(filepath.Join(dir, "beacon-plan", "SKILL.md")); err != nil {
		t.Fatalf("materialized plan skill: %v", err)
	}
	if _, err := runCLI(t, "skills", "install", "--dir", dir); err == nil {
		t.Error("second install without --force must refuse")
	} else if !strings.Contains(err.Error(), "skill_install_conflict") {
		t.Errorf("conflict error = %v, want skill_install_conflict", err)
	}
	if _, err := runCLI(t, "skills", "install", "--dir", dir, "--force"); err != nil {
		t.Errorf("forced install: %v", err)
	}
	out, err = runCLI(t, "skills", "references", "plan")
	if err != nil {
		t.Fatalf("references: %v", err)
	}
	if !strings.Contains(out, "pln-review") {
		t.Errorf("plan references missing the mode names:\n%s", out)
	}
}

// H10 + M11: init assembles a project on disk, export-state writes
// the portable snapshot, and a fresh DB auto-recognizes the repo on
// a state-dependent command.
func TestProjectsInitStateRoundTripCLI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "fresh-proj")

	out, err := runCLI(t, "projects", "init", "--project", root, "--version", "v1.0.0", "--json")
	if err != nil {
		t.Fatalf("projects init: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "beacon", "v1.0.0", "SUMMARY.md")); err != nil {
		t.Fatalf("init file set missing: %v", err)
	}
	out, err = runCLI(t, "projects", "export-state", "--project", root, "--json")
	if err != nil {
		t.Fatalf("export-state: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if got := strField(t, doc, "project_key"); got == "" {
		t.Errorf("export doc missing project_key: %v", doc)
	}
	snapPath := filepath.Join(root, ".beacon", "state", "project-state.json")
	if _, err := os.Stat(snapPath); err != nil {
		t.Fatalf("snapshot file: %v", err)
	}
	// Fresh DB (machine B): commands on the unregistered but
	// snapshot-carrying repo auto-import instead of refusing.
	dbB := filepath.Join(t.TempDir(), "b.db")
	t.Setenv(store.EnvStorePath, dbB)
	out, err = runCLI(t, "projects", "show", "--project", root, "--json")
	if err != nil {
		t.Fatalf("projects show on machine B (auto-import): %v\n%s", err, out)
	}
	docB := parseDoc(t, out)
	if got := strField(t, docB, "current_version"); got != "v1.0.0" {
		t.Errorf("machine B current_version = %q, want v1.0.0", got)
	}
}

// M10: archive face lists the inventory and applies a status.
func TestArchiveCLI(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "docs", "beacon", "v1.6.0", "features", "feat-z")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "truth.md"), []byte("---\nstatus: frozen\n---\n\nAC-001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "archive", "list", "--project", root, "--all", "--json")
	if err != nil {
		t.Fatalf("archive list: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	entries, ok := doc["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("entries = %v, want 1", doc)
	}
	out, err = runCLI(t, "archive", "apply", "--project", root, "--version", "v1.6.0",
		"--feature", "feat-z", "--status", "archived-reference-only", "--json")
	if err != nil {
		t.Fatalf("archive apply: %v\n%s", err, out)
	}
	out, err = runCLI(t, "archive", "lineage", "--project", root, "feat-z", "--json")
	if err != nil {
		t.Fatalf("archive lineage: %v\n%s", err, out)
	}
	if !strings.Contains(out, "archived-reference-only") {
		t.Errorf("lineage does not show the applied status:\n%s", out)
	}
}

// M12: hooks face lists, plans, dry-runs and disables a contract.
func TestHooksCLI(t *testing.T) {
	root := t.TempDir()
	hooksDir := filepath.Join(root, "docs", "beacon", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	contract := "---\nhook_id: nightly\nhook_code: post_release\nenabled: true\ntrigger_policy: auto_allowed\nfailure_route: release\ncommand_template: \"true\"\nproduces_evidence:\n  - hook_run_record\n---\n\n## Purpose\nAlign.\n\n## Admission\nAdmitted.\n\n## Evidence\nRecord.\n\n## Failure\nRoute to release.\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "nightly.md"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "hooks", "list", "--project", root, "--json")
	if err != nil {
		t.Fatalf("hooks list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "nightly") {
		t.Errorf("hooks list missing the contract:\n%s", out)
	}
	out, err = runCLI(t, "hooks", "run", "nightly", "--project", root, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("hooks run --dry-run: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"status":"planned"`) && !strings.Contains(out, `"status": "planned"`) {
		t.Errorf("dry run not planned:\n%s", out)
	}
	out, err = runCLI(t, "hooks", "disable", "nightly", "--project", root, "--json")
	if err != nil {
		t.Fatalf("hooks disable: %v\n%s", err, out)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("disable output:\n%s", out)
	}
}

// H8: the qa autotest face runs the goal run's matrix for real.
func TestQAAutotestCLI(t *testing.T) {
	root, _ := seedGoalProject(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tests.md"), []byte(
		"| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | cli | true | exit code 0 |\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v\n%s", err, out)
	}
	runID := strField(t, parseDoc(t, out), "run_id")
	out, err = runCLI(t, "qa", "autotest", "run", "--run-id", runID, "--by", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("qa autotest run: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if got, _ := doc["passed"].(float64); got != 1 {
		t.Fatalf("autotest passed = %v, want 1 (%s)", doc, out)
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "autotest", runID, "report.json")); err != nil {
		t.Errorf("autotest report on disk: %v", err)
	}
}
