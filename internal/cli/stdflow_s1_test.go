package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func gitCLI(t *testing.T, dir string, args ...string) string {
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

// seedGitProject registers a project whose root is a real git
// repository on branch main (S1 CLI surface tests).
func seedGitProject(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "s1-proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCLI(t, root, "init", "-b", "main")
	gitCLI(t, root, "config", "user.email", "test@example.com")
	gitCLI(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCLI(t, root, "add", ".")
	gitCLI(t, root, "commit", "-m", "init")
	out, err := runCLI(t, "projects", "register", "--path", root, "--version", "v2.0.0", "--json")
	if err != nil {
		t.Fatalf("projects register: %v (%s)", err, out)
	}
	return root
}

// TestS1TruthBaselineCLI walks AC-136/139 on the real command
// surface: freeze refused off-canonical, baseline refused before
// the commit, recorded after it, idempotent on re-run.
func TestS1TruthBaselineCLI(t *testing.T) {
	root := seedGitProject(t)
	truthCreate(t, root, "feat-s1")

	// AC-136 at the CLI: freeze on a side branch is refused.
	gitCLI(t, root, "checkout", "-b", "dev")
	out, err := runCLI(t, "truth", "freeze", "--project", root, "--feature", "feat-s1", "--json")
	if err == nil || !strings.Contains(out+err.Error(), "truth_canonical_branch_mismatch") {
		t.Fatalf("freeze on dev: err=%v out=%s", err, out)
	}
	gitCLI(t, root, "checkout", "main")

	doc := truthFreeze(t, root, "feat-s1")
	if doc["branch_check"] != "ok" || doc["canonical_branch"] != "main" {
		t.Fatalf("freeze doc branch fact = %v", doc)
	}
	if doc["baseline_status"] != "uncommitted" {
		t.Fatalf("freeze doc baseline = %v, want uncommitted", doc["baseline_status"])
	}

	// Baseline before the commit: refused.
	out, err = runCLI(t, "truth", "baseline", "--project", root, "--feature", "feat-s1", "--json")
	if err == nil || !strings.Contains(out+err.Error(), "canonical_refreeze_not_committed") {
		t.Fatalf("baseline before commit: err=%v out=%s", err, out)
	}

	gitCLI(t, root, "add", "-A")
	gitCLI(t, root, "commit", "-m", "freeze feat-s1")
	out, err = runCLI(t, "truth", "baseline", "--project", root, "--feature", "feat-s1", "--json")
	if err != nil {
		t.Fatalf("baseline after commit: %v (%s)", err, out)
	}
	doc = parseDoc(t, out)
	if doc["baseline_status"] != "committed" || doc["baseline_recorded"] != true || doc["baseline_commit"] == "" {
		t.Fatalf("baseline doc = %v", doc)
	}
	// Idempotent re-run.
	out, err = runCLI(t, "truth", "baseline", "--project", root, "--feature", "feat-s1", "--json")
	if err != nil {
		t.Fatalf("baseline re-run: %v (%s)", err, out)
	}
	if doc := parseDoc(t, out); doc["baseline_already_recorded"] != true {
		t.Fatalf("baseline re-run doc = %v, want baseline_already_recorded", doc)
	}
	// Show surfaces the recorded baseline without a git probe.
	out, err = runCLI(t, "truth", "show", "--project", root, "--feature", "feat-s1", "--json")
	if err != nil {
		t.Fatalf("truth show: %v (%s)", err, out)
	}
	if doc := parseDoc(t, out); doc["baseline_status"] != "committed" {
		t.Fatalf("show doc baseline = %v", doc["baseline_status"])
	}
}
