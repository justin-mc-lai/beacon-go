package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
)

const cliMigrationDAG = "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n| TASK-check | Check behavior | - | AC-001 | go test ./internal/core -run TestBehavior -count=1 | go test ./internal/core -run TestBehavior -count=1 | evidence/check.txt | false |\n"

func cliMigrationFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := seedP4AProject(t)
	out, err := runCLI(t, "truth", "create", "--project", root, "--slug", "migration", "--json")
	if err != nil {
		t.Fatalf("create: %v %s", err, out)
	}
	dir := filepath.Join(root, "docs", "beacon", "v2.0.0", "features", "migration")
	truth := "---\nslug: migration\nstatus: draft\nintent_confirmed: true\nconfirmed_by: prior-owner\nsdd_review_verdict: pass\ncanonical_refs:\n  truth: docs/beacon/v2.0.0/features/migration/truth.md\n  tests: docs/beacon/v2.0.0/features/migration/tests.md\n  tasks: docs/beacon/v2.0.0/features/migration/tasks.md\n---\n# Truth\n\n- **AC-001**: observable behavior\n\n## Open Questions\n\nKeep this original question.\n"
	for name, text := range map[string]string{
		"truth.md":       truth,
		"tests.md":       "| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |\n|---|---|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | `go test ./internal/core -run TestBehavior -count=1` | observable behavior holds | evidence/check.txt | TestBehavior |\n",
		"tasks.md":       "- [ ] implement observable behavior; ac=AC-001\n",
		"restatement.md": "prior restatement bytes\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dag := filepath.Join(root, "real-tasks.md")
	if err := os.WriteFile(dag, []byte(cliMigrationDAG), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, dir, dag
}

func TestTruthMigrateCLIDryRunAndApply(t *testing.T) {
	root, dir, dag := cliMigrationFixture(t)
	before := map[string][]byte{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		before[entry.Name()] = raw
	}
	args := []string{"truth", "migrate", "--project", root, "--feature", "migration", "--by", "operator", "--tasks-file", dag, "--json"}
	out, err := runCLI(t, append(args, "--dry-run")...)
	if err != nil {
		t.Fatalf("dry-run: %v %s", err, out)
	}
	report := parseDoc(t, out)
	if report["dry_run"] != true || report["package_contract"] != "truth-tasks" || report["tasks_replaced"] != true {
		t.Fatalf("dry-run report: %s", out)
	}
	for name, raw := range before {
		now, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil || !bytes.Equal(raw, now) {
			t.Fatalf("dry-run changed %s: %v", name, readErr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "migrations")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote backup: %v", err)
	}
	out, err = runCLI(t, args...)
	if err != nil {
		t.Fatalf("apply: %v %s", err, out)
	}
	report = parseDoc(t, out)
	backup := report["backup_dir"].(string)
	for _, name := range []string{"truth.md", "tasks.md"} {
		raw, readErr := os.ReadFile(filepath.Join(backup, name))
		if readErr != nil || !bytes.Equal(raw, before[name]) {
			t.Fatalf("backup changed %s: %v", name, readErr)
		}
	}
	for _, name := range []string{"tests.md", "evidence.md", "restatement.md"} {
		raw, readErr := os.ReadFile(filepath.Join(dir, name))
		if readErr != nil || !bytes.Equal(raw, before[name]) {
			t.Fatalf("preserved member changed %s: %v", name, readErr)
		}
	}
	out, err = runCLI(t, "truth", "migrate", "--project", root, "--feature", "migration", "--by", "operator", "--json")
	if err != nil || parseDoc(t, out)["already_sdd"] != true {
		t.Fatalf("repeat: %v %s", err, out)
	}
}

func TestTruthMigrateCLIDryRunMissingStoreCreatesNothing(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "not-created.db")
	t.Setenv(store.EnvStorePath, dbPath)
	_, err := runCLI(t, "truth", "migrate", "--project", root, "--feature", "missing", "--by", "operator", "--dry-run", "--json")
	if err == nil {
		t.Fatal("missing store accepted")
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run created store: %v", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("dry-run wrote project: %+v %v", entries, err)
	}
}

func TestTruthMigrateCLIRequiresExplicitExecutableDAG(t *testing.T) {
	root, dir, _ := cliMigrationFixture(t)
	original, err := os.ReadFile(filepath.Join(dir, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCLI(t, "truth", "migrate", "--project", root, "--feature", "migration", "--by", "operator", "--json")
	if err == nil {
		t.Fatal("checkbox-only tasks accepted")
	}
	now, err := os.ReadFile(filepath.Join(dir, "truth.md"))
	if err != nil || !bytes.Equal(original, now) {
		t.Fatalf("refusal changed truth: %v", err)
	}
}

func TestTruthMigrateCLIRefusesFrozenAndMatrixConflict(t *testing.T) {
	for _, scenario := range []string{"frozen", "matrix-conflict"} {
		t.Run(scenario, func(t *testing.T) {
			root, dir, dag := cliMigrationFixture(t)
			path := filepath.Join(dir, "truth.md")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "frozen" {
				raw = bytes.Replace(raw, []byte("status: draft"), []byte("status: frozen"), 1)
			} else {
				raw = append(raw, []byte("\n## Coverage Matrix\n\n| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | `go test ./...` | distinct assertion |\n")...)
			}
			if err = os.WriteFile(path, raw, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = runCLI(t, "truth", "migrate", "--project", root, "--feature", "migration", "--by", "operator", "--tasks-file", dag, "--json")
			if err == nil {
				t.Fatal("unsafe migration accepted")
			}
			now, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(raw, now) {
				t.Fatalf("refusal changed truth: %v", readErr)
			}
			if _, statErr := os.Stat(filepath.Join(root, ".beacon", "migrations")); !os.IsNotExist(statErr) {
				t.Fatalf("refusal wrote backups: %v", statErr)
			}
		})
	}
}
