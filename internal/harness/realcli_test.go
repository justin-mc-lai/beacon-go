//go:build realcli

// Real-CLI integration evidence for AC-002 (codex) and AC-045/AC-003
// (claude). These drive the ACTUAL vendor binary, so they spend real
// tokens: they are behind the `realcli` build tag and never run in the
// default `go test ./...` (zero cost by default).
//
//   go test ./internal/harness/ -run 'RealIteration' -tags=realcli -v
//
// A failure here is reported with the child's stderr so a reader can
// tell an external outage (provider/model unavailable) from a Beacon
// defect (bad argv, missing context).

package harness

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// realWorkdir prepares a git worktree, because `codex exec` refuses to
// run outside a trusted directory.
func realWorkdir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "realcli@local"},
		{"config", "user.name", "realcli"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("cannot prepare a git workdir (%v: %s): skip is not a pass", err, out)
		}
	}
	return dir
}

// runRealProfile drives ONE real iteration through a built-in profile
// and asserts the AC-042 result shape came back truthfully.
func runRealProfile(t *testing.T, profileName string) {
	t.Helper()
	prof, ok := LookupProfile(profileName)
	if !ok {
		t.Fatalf("profile %s not found", profileName)
	}
	if _, err := exec.LookPath(prof.Argv[0]); err != nil {
		t.Skipf("real binary %q not on PATH: cannot produce %s evidence (skip is not a pass)", prof.Argv[0], profileName)
	}

	dir := realWorkdir(t)
	ctxPath := filepath.Join(dir, "stage-context.json")
	if err := os.WriteFile(ctxPath, []byte(`{"stage":"plan","prompt":"reply OK"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write stage context: %v", err)
	}

	const prompt = "Reply with exactly the word OK and nothing else. Do not run any commands."
	b := NewProfileBackend(prof, ctxPath, prompt, 150*time.Second)
	res, err := b.Iterate(context.Background(), IterationRequest{
		Workdir: dir, Stage: "plan", Iteration: 1, ContextPath: ctxPath,
	})
	if err != nil {
		t.Fatalf("%s real iteration errored: %v\nstdout: %s\nstderr: %s",
			profileName, err, res.StdoutText, res.StderrText)
	}
	if !res.Success {
		t.Fatalf("%s real iteration reported success=false (exit=%d)\nstdout: %s\nstderr: %s",
			profileName, res.ExitCode, res.StdoutText, res.StderrText)
	}
	if res.ExitCode != 0 {
		t.Fatalf("%s exit code = %d, want 0", profileName, res.ExitCode)
	}
	if res.AgentName == "" {
		t.Errorf("%s iteration carries no agent name", profileName)
	}
	t.Logf("%s: agent=%s exit=%d tokens=%d", profileName, res.AgentName, res.ExitCode, res.TokenUsage)
}

func TestCodexRealIteration(t *testing.T)  { runRealProfile(t, "codex-style") }
func TestClaudeRealIteration(t *testing.T) { runRealProfile(t, "claude-style") }
