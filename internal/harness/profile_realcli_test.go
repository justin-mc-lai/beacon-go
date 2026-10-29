package harness

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
)

// writeExec drops a runnable script so failure paths can be exercised
// against a real child process.
func writeExec(path, body string) error {
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		return err
	}
	return os.Chmod(path, 0o755)
}

// helpArgvFor builds the "<cmd> [<subcommand>] --help" invocation whose
// output is the authority for whether a template switch really exists.
// It stops at the first placeholder: everything from there on is data
// handed to the agent, not a switch the CLI must recognise.
func helpArgvFor(argv []string) []string {
	out := []string{}
	for _, a := range argv {
		if strings.HasPrefix(a, "{") {
			break
		}
		out = append(out, a)
	}
	return append(out, "--help")
}

// runHelp runs the real CLI's help. It reports (output, true) only when
// the binary exists and produced help text; (_, false) means "cannot
// verify here", which callers must turn into a skip and never a pass.
func runHelp(argv []string) (string, bool) {
	if len(argv) == 0 {
		return "", false
	}
	if _, err := exec.LookPath(argv[0]); err != nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	b, err := cmd.CombinedOutput()
	if err != nil && len(b) == 0 {
		return "", false
	}
	return string(b), true
}

// TestBuiltinProfileFlagsExistInRealCLI (AC-001): every switch a
// built-in template passes must actually appear in the real CLI's help.
// The previous templates shipped `--context-file`, which neither
// `codex exec` nor `claude -p` accepts; the shape had only ever been
// exercised against a stand-in command, so it never ran for real.
func TestBuiltinProfileFlagsExistInRealCLI(t *testing.T) {
	for _, p := range BuiltinProfiles() {
		if len(p.Argv) == 0 {
			t.Fatalf("profile %s has an empty argv", p.Name)
		}
		if strings.Contains(strings.Join(p.Argv, " "), "--context-file") {
			t.Errorf("profile %s passes --context-file, which no real codex/claude CLI accepts (AC-001)", p.Name)
		}
		if _, err := exec.LookPath(p.Argv[0]); err != nil {
			t.Skipf("real binary %q not on PATH: cannot verify switches for profile %s (skip is not a pass)", p.Argv[0], p.Name)
		}
		for _, a := range p.Argv {
			if !strings.HasPrefix(a, "-") || strings.HasPrefix(a, "{") {
				continue
			}
			helpArgv := helpArgvFor(p.Argv)
			out, ok := runHelp(helpArgv)
			if !ok {
				t.Skipf("cannot read help for %s: cannot verify switch %q (skip is not a pass)", strings.Join(helpArgv, " "), a)
			}
			if !strings.Contains(out, a) {
				t.Errorf("profile %s passes switch %q, but it is absent from `%s` (AC-001)",
					p.Name, a, strings.Join(helpArgv, " "))
			}
		}
	}
}

// TestProfilePromptCarriesContextPath (AC-004): the stage-context path
// is spliced into the prompt text, so the agent can read it. The
// template must reference {context}, the resolved argv must contain the
// absolute path, and a {context} template run without a context file
// must refuse instead of silently going context-less.
func TestProfilePromptCarriesContextPath(t *testing.T) {
	const ctxFile = "/abs/run/dir/stage-context.json"
	for _, p := range BuiltinProfiles() {
		if !strings.Contains(strings.Join(p.Argv, " "), "{context}") {
			t.Errorf("profile %s template has no {context}: the stage context would never reach the agent (AC-004)", p.Name)
			continue
		}
		vars := map[string]string{
			"context":   ctxFile,
			"prompt":    "execute this stage",
			"workdir":   "/abs/run/dir",
			"iteration": "1",
			"stage":     "implement",
		}
		var resolved []string
		for _, a := range p.Argv {
			resolved = append(resolved, ResolveTemplate(a, vars))
		}
		if !strings.Contains(strings.Join(resolved, " "), ctxFile) {
			t.Errorf("profile %s resolved argv does not carry the context path (AC-004): %v", p.Name, resolved)
		}
	}

	prof, ok := LookupProfile("codex-style")
	if !ok {
		t.Fatal("codex-style profile missing")
	}
	b := NewProfileBackend(prof, "", "do it", time.Second)
	if _, err := b.Iterate(context.Background(), IterationRequest{}); err == nil {
		t.Error("a {context} template ran with no stage-context file; want executor_profile_incomplete (AC-004)")
	}
}

// TestProfileBackendFailureSurfaces (AC-005): a missing binary, a
// non-zero exit and a timeout each surface as a failure with a reason
// code — never as a fabricated success.
func TestProfileBackendFailureSurfaces(t *testing.T) {
	ctx := context.Background()

	// 1. binary that does not exist.
	missing := NewSubprocessBackend([]string{"definitely-not-a-real-binary-xyz"}, 2*time.Second)
	if _, err := missing.Iterate(ctx, IterationRequest{}); err == nil {
		t.Error("missing binary: want an error, got success (AC-005)")
	} else {
		var ce *core.Error
		if errors.As(err, &ce) && ce.ReasonCode == "" {
			t.Error("missing binary: error carries no reason code (AC-005)")
		}
		if err.Error() == "" {
			t.Error("missing binary: error message is empty (AC-005)")
		}
	}
	if hs := missing.HealthCheck(ctx); hs.OK {
		t.Error("missing binary: HealthCheck should not be OK (AC-005)")
	}

	// 2. a command that exits non-zero.
	dir := t.TempDir()
	script := filepath.Join(dir, "fail.sh")
	if err := writeExec(script, "#!/bin/sh\necho boom >&2\nexit 3\n"); err != nil {
		t.Fatal(err)
	}
	failing := NewSubprocessBackend([]string{script}, 5*time.Second)
	res, err := failing.Iterate(ctx, IterationRequest{Workdir: dir})
	if err != nil {
		// A non-zero exit may surface as an error; either way it must not be success.
		t.Logf("non-zero exit surfaced as error (acceptable): %v", err)
	} else {
		if res.Success {
			t.Error("non-zero exit reported success=true (AC-005)")
		}
		if res.ExitCode != 3 {
			t.Errorf("exit code = %d, want 3 (AC-005)", res.ExitCode)
		}
	}

	// 3. a command that overruns its timeout.
	slow := filepath.Join(dir, "slow.sh")
	if err := writeExec(slow, "#!/bin/sh\nsleep 5\n"); err != nil {
		t.Fatal(err)
	}
	slowBackend := NewSubprocessBackend([]string{slow}, 300*time.Millisecond)
	sres, serr := slowBackend.Iterate(ctx, IterationRequest{Workdir: dir})
	if serr == nil && sres.Success {
		t.Error("timed-out command reported success=true (AC-005)")
	}
	if serr == nil && !sres.TimedOut && sres.Success {
		t.Error("timed-out command neither flagged TimedOut nor failed (AC-005)")
	}
}
