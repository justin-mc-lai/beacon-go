package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeTestScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	// Deliberately 0644: shell TCs must not need an execute bit.
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveShellTCPicksInterpreter(t *testing.T) {
	root := t.TempDir()
	writeTestScript(t, filepath.Join(root, "scripts", "ok.sh"), "#!/bin/sh\necho ok\n")
	writeTestScript(t, filepath.Join(root, "scripts", "z.zsh"), "#!/bin/zsh\necho z\n")
	writeTestScript(t, filepath.Join(root, "scripts", "check-noext"), "#!/usr/bin/env zsh\necho shebang\n")
	writeTestScript(t, filepath.Join(root, "scripts", "plain.sh"), "#!/bin/zsh\necho ext-wins\n")
	writeTestScript(t, filepath.Join(root, "scripts", "notes.txt"), "hello\n")
	writeTestScript(t, filepath.Join(root, "scripts", "bash-script"), "#!/bin/bash\necho no\n")

	cases := []struct {
		name string
		argv []string
		want []string
	}{
		{"sh extension", []string{"scripts/ok.sh", "--flag", "a b"},
			[]string{"/bin/sh", filepath.Join(root, "scripts", "ok.sh"), "--flag", "a b"}},
		{"zsh extension", []string{"scripts/z.zsh"},
			[]string{"/bin/zsh", filepath.Join(root, "scripts", "z.zsh")}},
		{"noext env shebang", []string{"scripts/check-noext"},
			[]string{"/bin/zsh", filepath.Join(root, "scripts", "check-noext")}},
		{"extension wins over shebang", []string{"scripts/plain.sh"},
			[]string{"/bin/sh", filepath.Join(root, "scripts", "plain.sh")}},
		{"explicit interpreter unchanged", []string{"sh", "scripts/ok.sh"},
			[]string{"sh", "scripts/ok.sh"}},
		{"unsupported extension unchanged", []string{"scripts/notes.txt"},
			[]string{"scripts/notes.txt"}},
		{"unsupported shebang unchanged", []string{"scripts/bash-script"},
			[]string{"scripts/bash-script"}},
		{"missing script unchanged", []string{"scripts/missing.sh"},
			[]string{"scripts/missing.sh"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveShellTC(root, tc.argv)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("argv = %q, want %q", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("argv = %q, want %q", got, tc.want)
				}
			}
		})
	}

	// An absolute path outside the project keeps PATH semantics.
	outside := filepath.Join(t.TempDir(), "outside.sh")
	writeTestScript(t, outside, "#!/bin/sh\ntrue\n")
	got, err := resolveShellTC(root, []string{outside})
	if err != nil {
		t.Fatalf("resolve outside: %v", err)
	}
	if len(got) != 1 || got[0] != outside {
		t.Fatalf("outside script argv = %q, want [%s]", got, outside)
	}
}

func TestRunAutoTestExecutesProjectShellTC(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	script := filepath.Join(env.root, "scripts", "ok_shell.sh")
	writeTestScript(t, script, "#!/bin/sh\necho hello-from-shell\n")
	doc := "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n" +
		"| AC-001 | TC-001 | behavior | `scripts/ok_shell.sh` | prints hello |\n"
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "tests.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := env.eng.RunAutoTest(context.Background(), run.RunID, AutoTestOptions{By: "qa-bot"})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Total != 1 || report.Passed != 1 || report.Failed != 0 {
		t.Fatalf("report tallies = %+v, want 1/1/0", report)
	}
	res := report.Results[0]
	if res.Status != AutoTestPassed || res.ExitCode != 0 {
		t.Fatalf("result = %+v, want passed/0", res)
	}
	out, err := os.ReadFile(filepath.Join(env.root, ".beacon", "autotest", run.RunID, "TC-001", "stdout.log"))
	if err != nil {
		t.Fatalf("stdout evidence: %v", err)
	}
	if string(out) != "hello-from-shell\n" {
		t.Fatalf("stdout evidence = %q", out)
	}
}
