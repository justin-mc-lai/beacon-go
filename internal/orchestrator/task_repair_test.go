package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskRepairCannotChangeContract(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "command.sh", "exit 1")
	taskExecutable(t, env.root, "verify.sh", "echo should-not-run > .beacon/verified")
	repair := taskExecutable(t, env.root, "repair.sh", "printf '\\nchanged requirement\\n' >> truth.md")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{RepairCommand: repair, MaxRepairs: 1})
	if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), "task_contract_stale") {
		t.Fatalf("contract repair accepted: %+v %v", report, err)
	}
	if len(report.Results[0].Attempts) != 2 || report.Results[0].Attempts[1].Status != "failed" {
		t.Fatalf("repair attempt not rejected: %+v", report.Results)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".beacon", "verified")); !os.IsNotExist(err) {
		t.Fatal("verify executed after contract-changing repair")
	}
}

func TestTaskSourceCandidateIgnoresRuntimeButIncludesSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "work.go"), "package before\n")
	before, err := taskSourceCandidate(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".beacon"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".beacon", "generated.go"), "package runtime\n")
	after, err := taskSourceCandidate(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("runtime evidence changed source candidate")
	}
	writeFile(t, filepath.Join(root, "work.go"), "package after\n")
	after, err = taskSourceCandidate(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("source edit did not change candidate")
	}
}

func TestTaskCommandAndRepairBoundaries(t *testing.T) {
	for _, command := range []string{"go install ./cmd/tool", "npm publish", "npm i", "npm ci", "npm add x", "pnpm i", "pnpm add x", "pnpm install", "yarn add x", "yarn install", "bun add x", "bun install", "beacon release", "git push", "sh -c 'echo fake'", "env npm install", "echo pass && release", ""} {
		if _, err := taskArgv(t.TempDir(), command); err == nil {
			t.Fatalf("unsafe command accepted: %s", command)
		}
	}
	argv, err := taskArgv(t.TempDir(), `tool 'literal spaces' "arg;not-shell"`)
	if err != nil || len(argv) != 3 || argv[1] != "literal spaces" || argv[2] != "arg;not-shell" {
		t.Fatalf("argv=%v err=%v", argv, err)
	}
	for _, opts := range []TaskRunOptions{{MaxRepairs: -1}, {MaxRepairs: 6}, {MaxRepairs: 1}, {RepairArgs: []string{"x"}}, {RepairCommand: "sh", RepairArgs: []string{"-c", "bad"}}, {Timeout: -1}} {
		if err := opts.validate(); err == nil {
			t.Fatalf("invalid repair configuration accepted: %+v", opts)
		}
	}
}

func TestTaskVerificationCannotMoveCandidate(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "command.sh", "echo command")
	taskExecutable(t, env.root, "verify.sh", "printf 'package mutated\\n' > mutated.go\necho forged > .beacon/proof.txt")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), "task_candidate_stale") {
		t.Fatalf("source-mutating verify accepted: %+v %v", report, err)
	}
}

func TestTaskRunnerRepairsVerificationFailure(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "command.sh", "echo command-ran >> .beacon/commands")
	taskExecutable(t, env.root, "verify.sh", "test -f repaired.go\necho verified > .beacon/proof.txt")
	repair := taskExecutable(t, env.root, "repair.sh", "grep '\"phase\": \"verify\"' \"$BEACON_FAILURE_CONTEXT\"\nprintf 'package repaired\\n' > repaired.go")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{RepairCommand: repair, MaxRepairs: 1})
	if err != nil || report.Status != "passed" {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	attempts := report.Results[0].Attempts
	if len(attempts) != 6 || attempts[1].Phase != "verify" || attempts[1].Status != "failed" || attempts[2].Phase != "repair" || attempts[3].Phase != "command" || attempts[4].Phase != "verify" || attempts[5].Phase != "final-verify" {
		t.Fatalf("attempts=%+v", attempts)
	}
	commands, err := os.ReadFile(filepath.Join(env.root, ".beacon", "commands"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(commands), "command-ran") != 2 {
		t.Fatal("repair did not rerun Command before Verify")
	}
}

func TestTaskFinalVerificationCannotMoveCandidate(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", "proof.txt"))
	taskExecutable(t, env.root, "command.sh", "echo command")
	taskExecutable(t, env.root, "verify.sh", "if test \"$BEACON_TASK_PHASE\" = final-verify; then echo mutated > index.html; fi\necho verified > proof.txt")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), "task_candidate_stale") {
		t.Fatalf("mutating final verify accepted: %+v %v", report, err)
	}
	a := report.Results[0].Attempts[2]
	if a.Phase != "final-verify" || a.Status != "failed" || a.CandidateBefore == a.CandidateAfter {
		t.Fatalf("final verify=%+v", a)
	}
}
