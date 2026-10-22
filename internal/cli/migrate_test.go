// CLI tests for the P3b surfaces: `migrate v1` and `doctor rebuild`
// (AC-077..079, AC-011). The sample project follows the probed real v1
// format (see internal/migration/scan.go).
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
)

func buildCLIV1Sample(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("beacon/version-truth.json", `{"docs_default_line":"v1.0.0"}`)
	write("docs/beacon/v1.0.0/features/feat-a/truth.md", "---\nslug: feat-a\nstatus: frozen\n---\n# Truth\n")
	write("docs/beacon/v1.0.0/features/feat-a/tests.md", "# Tests\n")
	write("docs/beacon/v1.0.0/.machine/execution/feat-a.revision.json",
		`{"feature_slug":"feat-a","doc_status":"frozen","lifecycle_status":"accepted"}`)
	return root
}

func TestMigrateAndDoctorCLI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := buildCLIV1Sample(t)

	// Dry-run through the CLI reports counts and writes nothing.
	out, err := runCLI(t, "migrate", "v1", "--path", root, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("migrate dry-run: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	counts := doc["counts"].(map[string]any)
	if counts["feature_packages"].(map[string]any)["imported"].(float64) != 1 {
		t.Fatalf("dry-run counts wrong: %s", out)
	}
	out, err = runCLI(t, "projects", "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if len(parseDoc(t, out)["projects"].([]any)) != 0 {
		t.Fatal("dry-run registered a project")
	}

	// Real import, then a JSON doctor report with no drift.
	if _, err := runCLI(t, "migrate", "v1", "--path", root, "--json"); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "doctor", "rebuild", "--project", root, "--json")
	if err != nil {
		t.Fatalf("doctor rebuild: %v\n%s", err, out)
	}
	doc = parseDoc(t, out)
	if len(doc["drift"].([]any)) != 0 {
		t.Fatalf("unexpected drift after clean import: %s", out)
	}

	// A package appearing on disk afterwards is drift; --apply fixes it
	// and spine reads it back.
	late := filepath.Join(root, "docs", "beacon", "v1.0.0", "features", "feat-late")
	if err := os.MkdirAll(late, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(late, "truth.md"), []byte("---\nslug: feat-late\nstatus: draft\n---\n# Truth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "doctor", "rebuild", "--project", root, "--apply", "--json")
	if err != nil {
		t.Fatalf("doctor apply: %v\n%s", err, out)
	}
	doc = parseDoc(t, out)
	if doc["events_written"].(float64) == 0 {
		t.Fatalf("apply wrote no events: %s", out)
	}
	out, err = runCLI(t, "spine", "list", "--project", root, "--json")
	if err != nil {
		t.Fatalf("spine list: %v\n%s", err, out)
	}
	found := false
	for _, item := range parseDoc(t, out)["snapshots"].([]any) {
		if item.(map[string]any)["feature_slug"] == "feat-late" {
			found = true
		}
	}
	if !found {
		t.Fatalf("spine list does not show feat-late after apply: %s", out)
	}
}
