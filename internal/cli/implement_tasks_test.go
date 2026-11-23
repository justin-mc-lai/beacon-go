package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func runTasksCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newImplementTasksCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestImplementTasksCLIStatusRunAndFailureReport(t *testing.T) {
	root, _ := seedGoalProject(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"truth.md":   "---\nslug: feat-a\nstatus: frozen\n---\n# Truth\n\n- **AC-001**: actual execution\n",
		"tests.md":   "# Tests\n",
		"tasks.md":   "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n|TASK-A|work|-|AC-001|./command.sh|./verify.sh|.beacon/proof.txt|true|\n",
		"command.sh": "#!/bin/sh\nexit 4\n",
		"verify.sh":  "#!/bin/sh\necho forbidden > .beacon/proof.txt\n",
	}
	for name, text := range files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), mode); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(os.Getenv(store.EnvStorePath))
	if err != nil {
		t.Fatal(err)
	}
	eng := orchestrator.NewEngine(st)
	run, err := eng.CreateRun(orchestrator.CreateRunOptions{ProjectRoot: root, FeatureSlug: "feat-a", Utterance: "execute tasks", ExecutorID: "cli-test"})
	if err != nil {
		st.Close()
		t.Fatal(err)
	}
	st.Close()
	out, err := runTasksCLI(t, "status", "--run-id", run.RunID)
	if err != nil || strField(t, parseDoc(t, out), "status") != "not_started" {
		t.Fatalf("status output=%s err=%v", out, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "task-runs")); !os.IsNotExist(err) {
		t.Fatal("status wrote task evidence")
	}
	out, err = runTasksCLI(t, "run", "--run-id", run.RunID)
	if err == nil {
		t.Fatal("task execution failure returned success")
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "status") != "failed" {
		t.Fatalf("failure report=%s", out)
	}
	out, err = runTasksCLI(t, "status", "--run-id", run.RunID)
	if err != nil || strField(t, parseDoc(t, out), "status") != "failed" {
		t.Fatalf("persisted failure=%s err=%v", out, err)
	}
}

func TestImplementTasksCLIFlags(t *testing.T) {
	for _, args := range [][]string{{"status"}, {"run"}, {"run", "--run-id", "x", "--max-repairs", "6"}, {"run", "--run-id", "x", "--timeout-sec", "-1"}} {
		if _, err := runTasksCLI(t, args...); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
	cmd := newImplementTasksCmd()
	run, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.ParseFlags([]string{"--repair-cmd", "worker", "--repair-arg", "literal,comma", "--repair-arg", "two words"}); err != nil {
		t.Fatal(err)
	}
	args, err := run.Flags().GetStringArray("repair-arg")
	if err != nil || len(args) != 2 || args[0] != "literal,comma" || args[1] != "two words" {
		t.Fatalf("repair argv=%v err=%v", args, err)
	}
}
