// CLI behavior for `goal artifacts register/list` (P6 E2E fix):
// register lands the pointer with a real hash, list shows it with
// the shared spine presentation, and refusals carry stable codes.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoalArtifactsRegisterAndList(t *testing.T) {
	root, _ := seedGoalProject(t)
	out, err := runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(root, "qa-report.md")
	if err := os.WriteFile(blob, []byte("# qa report\nall green\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "goal", "artifacts", "register",
		"--run-id", runID, "--kind", "qa_report", "--path", blob, "--by", "e2e-maker", "--json")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	doc := parseDoc(t, out)
	art, ok := doc["artifact"].(map[string]any)
	if !ok || art["kind"] != "qa_report" || art["run_id"] != runID {
		t.Fatalf("register output wrong: %s", out)
	}
	if h, _ := art["content_hash"].(string); len(h) != 64 {
		t.Fatalf("content_hash must be a sha256 hex, got %v", art["content_hash"])
	}

	out, err = runCLI(t, "goal", "artifacts", "list", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "qa_report") || !strings.Contains(out, blob) {
		t.Fatalf("list must show the registered pointer: %s", out)
	}

	// Idempotent from the CLI too.
	out, err = runCLI(t, "goal", "artifacts", "register",
		"--run-id", runID, "--kind", "qa_report", "--path", blob, "--by", "e2e-maker", "--json")
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if !strings.Contains(out, "\"already_registered\": true") {
		t.Fatalf("re-register must report already_registered: %s", out)
	}

	// Refusals: missing file, bad kind, unknown run.
	if _, err := runCLI(t, "goal", "artifacts", "register",
		"--run-id", runID, "--kind", "qa_report", "--path", filepath.Join(root, "nope.md"), "--by", "me"); err == nil ||
		!strings.Contains(err.Error(), "artifact_file_missing") {
		t.Fatalf("missing file must fail with artifact_file_missing, got %v", err)
	}
	if _, err := runCLI(t, "goal", "artifacts", "register",
		"--run-id", runID, "--kind", "banana", "--path", blob, "--by", "me"); err == nil ||
		!strings.Contains(err.Error(), "artifact_kind_not_allowed") {
		t.Fatalf("bad kind must fail with artifact_kind_not_allowed, got %v", err)
	}
	if _, err := runCLI(t, "goal", "artifacts", "list", "--run-id", "run_nope"); err == nil ||
		!strings.Contains(err.Error(), "product_goal_run_not_found") {
		t.Fatalf("unknown run must fail with product_goal_run_not_found, got %v", err)
	}
}
