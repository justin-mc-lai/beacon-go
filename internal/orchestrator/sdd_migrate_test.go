package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"beacon-go/internal/gate"
)

const migrationDAG = "# Authored task DAG\n\n| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n| TASK-build | Implement behavior | - | AC-001 | go build ./internal/core | go test ./internal/core -run TestBehavior -count=1 | evidence/build.txt | false |\n| TASK-verify | Verify dependent behavior | TASK-build | AC-001 | go test ./internal/core -run TestBehavior -count=1 | go test ./internal/core -run TestBehavior -count=1 | evidence/verify.txt | true |\n"

const migrationTests = "# Tests\n\n| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |\n|---|---|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | `go test ./internal/core -run 'TestBehavior|TestRefusal' -count=1` | behavior and refusal asserted | evidence/behavior.txt | TestBehavior, TestRefusal |\n\n## Another suite\n\n| AC ID | TC ID | Layer | Command | Assertion | Evidence |\n|---|---|---|---|---|---|\n| AC-001 | TC-002 | negative | `go test ./internal/core -run TestRefusal -count=1` | invalid input rejected | evidence/refusal.txt |\n"

func migrationEnv(t *testing.T) gateTestEnv {
	t.Helper()
	truth := strings.Replace(frozenTruthDoc, "status: frozen", "status: draft", 1) + "\n## User Intent Snapshot\n\nOriginal user intent.\n\n## Open Questions\n\nQuestion remains open.\n\n## User Journeys\n\nHuman-authored journey.\n"
	env := newGateEnv(t, truth)
	writeFile(t, filepath.Join(env.root, "tests.md"), migrationTests)
	writeFile(t, filepath.Join(env.root, "tasks.md"), migrationDAG)
	return env
}

func migrationRead(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSDDMigrationDryRunAndExactPreservation(t *testing.T) {
	env := migrationEnv(t)
	writeFile(t, filepath.Join(env.root, "restatement.md"), "legacy signed restatement bytes\n")
	original := map[string]string{}
	for _, name := range []string{"truth.md", "tasks.md", "tests.md", "evidence.md", "restatement.md"} {
		original[name] = migrationRead(t, filepath.Join(env.root, name))
	}
	before, err := os.ReadDir(env.root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author", DryRun: true})
	if err != nil || report.CoverageRows != 2 || !report.DryRun || report.BackupDir != "" {
		t.Fatalf("dry-run: %+v %v", report, err)
	}
	after, err := os.ReadDir(env.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("dry-run created a directory or file")
	}
	for name, text := range original {
		if migrationRead(t, filepath.Join(env.root, name)) != text {
			t.Fatalf("dry-run rewrote %s", name)
		}
	}
	report, err = env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"})
	if err != nil {
		t.Fatal(err)
	}
	if report.BackupDir == "" || report.AlreadySDD {
		t.Fatalf("missing migration report: %+v", report)
	}
	for _, name := range []string{"truth.md", "tasks.md"} {
		if migrationRead(t, filepath.Join(report.BackupDir, name)) != original[name] {
			t.Fatalf("backup changed original %s bytes", name)
		}
	}
	for _, name := range []string{"tasks.md", "tests.md", "evidence.md", "restatement.md"} {
		if migrationRead(t, filepath.Join(env.root, name)) != original[name] {
			t.Fatalf("migration changed preserved %s", name)
		}
	}
	truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
	_, expected, _ := migrationCoverage(migrationTests)
	_, actual, err := migrationCoverage(truth)
	if err != nil || !reflect.DeepEqual(expected, actual) {
		t.Fatalf("coverage data changed: %#v %#v %v", expected, actual, err)
	}
	for _, retained := range []string{"Original user intent.", "Question remains open.", "Human-authored journey."} {
		if !strings.Contains(truth, retained) {
			t.Fatalf("lost %q", retained)
		}
	}
	info, err := os.Stat(filepath.Join(env.root, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"})
	if err != nil || !repeat.AlreadySDD || repeat.BackupDir != "" {
		t.Fatalf("idempotent repeat: %+v %v", repeat, err)
	}
	newInfo, err := os.Stat(filepath.Join(env.root, "truth.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(newInfo.ModTime()) || truth != migrationRead(t, filepath.Join(env.root, "truth.md")) {
		t.Fatal("repeat rewrote truth")
	}
}

func TestSDDMigrationInvalidatesPriorConfirmation(t *testing.T) {
	env := migrationEnv(t)
	truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
	for _, pair := range [][2]string{{"intent_confirmed", "true"}, {"confirmed_by", "old-owner"}, {"confirmed_at", "old-time"}, {"hash", "old-hash"}, {"sdd_review_by", "old-reviewer"}, {"sdd_review_verdict", "pass"}, {"sdd_review_fingerprint", "old-fingerprint"}, {"sdd_review_at", "old-time"}} {
		truth, _ = setFrontmatterScalar(truth, pair[0], pair[1])
	}
	writeFile(t, filepath.Join(env.root, "truth.md"), truth)
	if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "migration-operator"}); err != nil {
		t.Fatal(err)
	}
	scalars, _, _ := gate.ParseFrontmatter(migrationRead(t, filepath.Join(env.root, "truth.md")))
	if scalars["intent_confirmed"] != "false" || scalars["package_maturity"] != "scaffold" || scalars["package_contract"] != SDDContract {
		t.Fatalf("unsafe contract: %+v", scalars)
	}
	for _, key := range []string{"confirmed_by", "confirmed_at", "hash", "sdd_review_by", "sdd_review_at", "sdd_review_verdict", "sdd_review_fingerprint"} {
		if scalars[key] != "" {
			t.Fatalf("old signature retained: %s=%s", key, scalars[key])
		}
	}
}

func TestSDDMigrationRefusesFrozenAndForgedChangePending(t *testing.T) {
	for _, status := range []string{"frozen", "change_pending"} {
		t.Run(status, func(t *testing.T) {
			env := migrationEnv(t)
			truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
			truth, _ = setFrontmatterScalar(truth, "status", status)
			writeFile(t, filepath.Join(env.root, "truth.md"), truth)
			if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author", DryRun: true}); err == nil {
				t.Fatal("unsafe lifecycle accepted")
			}
			if migrationRead(t, filepath.Join(env.root, "truth.md")) != truth {
				t.Fatal("refusal changed truth")
			}
		})
	}
}

func TestSDDMigrationAcceptsGenuineOpenedChange(t *testing.T) {
	env := migrationEnv(t)
	truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
	truth, _ = setFrontmatterScalar(truth, "status", "frozen")
	writeFile(t, filepath.Join(env.root, "truth.md"), truth)
	if _, err := env.eng.RequestTruthChange(env.ctx, "adopt SDD contract", "owner"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"}); err != nil {
		t.Fatal(err)
	}
	scalars, _, _ := gate.ParseFrontmatter(migrationRead(t, filepath.Join(env.root, "truth.md")))
	if scalars["status"] != "change_pending" {
		t.Fatalf("change was closed implicitly: %+v", scalars)
	}
}

func TestSDDMigrationMatrixConflictAndLossyRows(t *testing.T) {
	for _, matrix := range []string{
		strings.Replace(migrationTests, "behavior and refusal asserted", "different assertion", 1),
		strings.Replace(migrationTests, "| Evidence | Required Tests |", "| Evidence | Unsupported |", 1),
		strings.Replace(migrationTests, "| AC-001 | TC-002 |", "| unknown | TC-002 |", 1),
		strings.Replace(migrationTests, "| invalid input rejected | evidence/refusal.txt |", "| invalid input rejected | evidence/refusal.txt | extra |", 1),
	} {
		env := migrationEnv(t)
		truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
		writeFile(t, filepath.Join(env.root, "truth.md"), truth+"\n## Coverage Matrix\n\n"+matrix)
		if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"}); err == nil {
			t.Fatalf("conflict/lossy coverage accepted: %s", matrix)
		}
		if _, err := os.Stat(filepath.Join(env.root, ".beacon", "migrations")); !os.IsNotExist(err) {
			t.Fatalf("refusal wrote migration backup: %v", err)
		}
	}
}

func TestSDDMigrationExplicitDAGAndConflicts(t *testing.T) {
	env := migrationEnv(t)
	legacy := "# Tasks\n\n- [ ] implement behavior, then verify; ac=AC-001\n"
	writeFile(t, filepath.Join(env.root, "tasks.md"), legacy)
	if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"}); err == nil {
		t.Fatal("checkbox commands were fabricated")
	}
	supplied := filepath.Join(env.root, "authored-dag.md")
	writeFile(t, supplied, migrationDAG)
	report, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author", TasksFile: supplied})
	if err != nil {
		t.Fatal(err)
	}
	if !report.TasksReplaced || migrationRead(t, filepath.Join(env.root, "tasks.md")) != migrationDAG || migrationRead(t, filepath.Join(report.BackupDir, "tasks.md")) != legacy {
		t.Fatalf("DAG/source not preserved: %+v", report)
	}

	conflict := migrationEnv(t)
	writeFile(t, supplied, strings.Replace(migrationDAG, "TASK-build | AC-001", "- | AC-001", 1))
	if _, err := conflict.eng.MigrateTruthSDD(conflict.ctx, SDDMigrationOptions{By: "author", TasksFile: supplied}); err == nil {
		t.Fatal("conflicting dependency accepted")
	}
}

func TestSDDMigrationCanonicalTasksReference(t *testing.T) {
	env := migrationEnv(t)
	truth := migrationRead(t, filepath.Join(env.root, "truth.md"))
	writeFile(t, filepath.Join(env.root, "truth.md"), strings.Replace(truth, "tasks: tasks.md", "tasks: authored.md", 1))
	writeFile(t, filepath.Join(env.root, "authored.md"), migrationDAG)
	writeFile(t, filepath.Join(env.root, "tasks.md"), "must not be consumed or overwritten\n")
	report, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"})
	if err != nil || report.TasksPath != "authored.md" {
		t.Fatalf("canonical tasks ignored: %+v %v", report, err)
	}
	if migrationRead(t, filepath.Join(env.root, "tasks.md")) != "must not be consumed or overwritten\n" {
		t.Fatal("noncanonical sibling changed")
	}
}

func TestSDDMigrationRollsBackTaskCommit(t *testing.T) {
	root := t.TempDir()
	truth, tasks := filepath.Join(root, "truth.md"), filepath.Join(root, "tasks.md")
	writeFile(t, truth, "old truth")
	writeFile(t, tasks, "old tasks")
	injected := errors.New("injected truth rename failure")
	err := commitSDDMigration(truth, tasks, []byte("old truth"), []byte("old tasks"), []byte("new truth"), []byte("new tasks"), func(from, to string) error {
		if to == truth {
			return injected
		}
		return os.Rename(from, to)
	})
	if !errors.Is(err, injected) {
		t.Fatalf("unexpected error: %v", err)
	}
	if migrationRead(t, truth) != "old truth" || migrationRead(t, tasks) != "old tasks" {
		t.Fatal("failed migration left a half-migrated package")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary transaction files leaked: %+v %v", entries, err)
	}
}

func TestSDDMigrationRejectsUnboundPendingChange(t *testing.T) {
	env := migrationEnv(t)
	if err := env.eng.appendFeatureEvent(env.ctx.Feature.ID, EventTruthChangeRequested, map[string]any{"change_id": "chg_missing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author", DryRun: true}); err == nil {
		t.Fatal("pending event without matching change document accepted")
	}
}

func TestSDDMigrationRefusesInvalidExplicitDAG(t *testing.T) {
	for _, dag := range []string{
		strings.Replace(migrationDAG, "TASK-build | AC-001", "TASK-verify | AC-001", 1),
		strings.ReplaceAll(migrationDAG, "AC-001", "AC-999"),
		strings.Replace(migrationDAG, "go build ./internal/core", "-", 1),
	} {
		env := migrationEnv(t)
		writeFile(t, filepath.Join(env.root, "tasks.md"), "- [ ] original task\n")
		file := filepath.Join(env.root, "invalid-dag.md")
		writeFile(t, file, dag)
		if _, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author", TasksFile: file}); err == nil {
			t.Fatalf("invalid explicit DAG accepted: %s", dag)
		}
		if migrationRead(t, filepath.Join(env.root, "tasks.md")) != "- [ ] original task\n" {
			t.Fatal("refusal changed legacy tasks")
		}
	}
}

func TestSDDMigrationStagingFailureLeavesOriginals(t *testing.T) {
	root := t.TempDir()
	truth, tasks := filepath.Join(root, "truth.md"), filepath.Join(root, "missing-tasks.md")
	writeFile(t, truth, "original truth")
	err := commitSDDMigration(truth, tasks, []byte("original truth"), nil, []byte("new truth"), []byte("new tasks"), os.Rename)
	if err == nil {
		t.Fatal("missing canonical task unexpectedly committed")
	}
	if migrationRead(t, truth) != "original truth" {
		t.Fatal("staging failure changed truth")
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil || len(entries) != 1 {
		t.Fatalf("staging leaked files: %+v %v", entries, readErr)
	}
}

func TestSDDMigrationPreservesBlankSeparatedCoverageRows(t *testing.T) {
	env := migrationEnv(t)
	header := "| AC ID | TC ID | Layer | Command | Assertion | Evidence | Required Tests |\n|---|---|---|---|---|---|---|\n"
	first := "| AC-001 | TC-001 | behavior | `go test ./internal/core -run TestBehavior -count=1` | original positive assertion | evidence/positive.txt | TestBehavior |\n"
	second := "| AC-001 | TC-002 | negative | `go test ./internal/core -run TestRefusal -count=1` | original refusal assertion | evidence/negative.txt | TestRefusal |\n"
	for _, gap := range []string{"\n", "\n \t\n"} {
		t.Run(strings.ReplaceAll(gap, "\n", "newline"), func(t *testing.T) {
			// Each case starts from the same legacy truth, not the prior SDD output.
			truth := strings.Replace(frozenTruthDoc, "status: frozen", "status: draft", 1)
			writeFile(t, filepath.Join(env.root, "truth.md"), truth)
			original := "# Tests\n\n" + header + first + gap + second
			writeFile(t, filepath.Join(env.root, "tests.md"), original)
			expected, _, found := gate.ParseCoverageMatrix(original)
			if !found || len(expected) != 2 {
				t.Fatal("fixture must expose both rows through the authoritative parser")
			}
			report, err := env.eng.MigrateTruthSDD(env.ctx, SDDMigrationOptions{By: "author"})
			if err != nil || report.CoverageRows != 2 {
				t.Fatalf("blank-separated migration: %+v %v", report, err)
			}
			migrated := migrationRead(t, filepath.Join(env.root, "truth.md"))
			actual, _, _ := gate.ParseCoverageMatrix(migrated)
			for i := range expected {
				expected[i].Line = 0
			}
			for i := range actual {
				actual[i].Line = 0
			}
			if !reflect.DeepEqual(expected, actual) || !strings.Contains(migrated, first+gap+second) {
				t.Fatalf("coverage commands/assertions or exact blank-separated rows changed: expected=%+v actual=%+v", expected, actual)
			}
			if migrationRead(t, filepath.Join(env.root, "tests.md")) != original {
				t.Fatal("migration rewrote original tests bytes")
			}
		})
	}
}
