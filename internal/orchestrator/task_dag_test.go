package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const taskRunnerHeader = "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n"

func taskRunnerRow(id, depends, command, verify, evidence string) string {
	return "|" + id + "|implement|" + depends + "|AC-001|" + command + "|" + verify + "|" + evidence + "|true|\n"
}

func newTaskRunnerEnv(t *testing.T, body string) (gateTestEnv, string) {
	t.Helper()
	env := newGateEnv(t, frozenTruthDoc)
	writeFile(t, filepath.Join(env.root, "tasks.md"), taskRunnerHeader+body)
	run := env.createRun(t)
	admission, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil || admission.Status != "pass" {
		t.Fatalf("fixture admission: %+v %v", admission, err)
	}
	return env, run.RunID
}

func taskExecutable(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nset -eu\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTaskRunnerStopsDownstreamAndRetainsFailure(t *testing.T) {
	env, run := newTaskRunnerEnv(t,
		taskRunnerRow("TASK-A", "-", "./fail.sh", "./verify.sh", ".beacon/a.txt")+
			taskRunnerRow("TASK-B", "TASK-A", "./downstream.sh", "./verify.sh", ".beacon/b.txt"))
	taskExecutable(t, env.root, "fail.sh", "echo command-output\necho command-error >&2\nexit 7")
	taskExecutable(t, env.root, "verify.sh", "echo should-not-verify > .beacon/verified")
	taskExecutable(t, env.root, "downstream.sh", "echo should-not-run > .beacon/downstream")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err == nil || report.Status != "failed" || len(report.Results[0].Attempts) != 1 || report.Results[1].Status != "pending" {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	for _, name := range []string{"verified", "downstream"} {
		if _, err := os.Stat(filepath.Join(env.root, ".beacon", name)); !os.IsNotExist(err) {
			t.Fatalf("%s executed: %v", name, err)
		}
	}
	a := report.Results[0].Attempts[0]
	stdout, err := os.ReadFile(a.StdoutPath)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := os.ReadFile(a.StderrPath)
	if err != nil {
		t.Fatal(err)
	}
	if a.ExitCode != 7 || !strings.Contains(string(stdout), "command-output") || !strings.Contains(string(stderr), "command-error") {
		t.Fatalf("failure capture=%+v stdout=%s stderr=%s", a, stdout, stderr)
	}
	status, err := env.eng.TaskStatus(run)
	if err != nil || status.Status != "failed" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	refs, err := env.eng.ListRunArtifacts(run)
	if err != nil || len(refs) < 4 {
		t.Fatalf("registered artifacts=%v err=%v", refs, err)
	}
}

func TestTaskRunnerRealRepairAndImmutableAttempts(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "command.sh", "echo implementation\ntest -f repaired.go")
	taskExecutable(t, env.root, "verify.sh", "test -f repaired.go\necho verified > .beacon/proof.txt")
	repair := taskExecutable(t, env.root, "repair.sh", "test -f \"$BEACON_FAILURE_CONTEXT\"\ntest -f \"$BEACON_FAILURE_STDOUT\"\ntest -f \"$BEACON_FAILURE_STDERR\"\ntest \"$1\" = 'literal argument'\nprintf 'package repaired\\n' > repaired.go")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{RepairCommand: repair, RepairArgs: []string{"literal argument"}, MaxRepairs: 1})
	if err != nil || report.Status != "passed" || report.Repairs != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	attempts := report.Results[0].Attempts
	if len(attempts) != 5 || attempts[0].Status != "failed" || attempts[1].Phase != "repair" || attempts[2].Phase != "command" || attempts[3].Phase != "verify" || attempts[4].Phase != "final-verify" {
		t.Fatalf("attempts=%+v", attempts)
	}
	if attempts[1].CandidateBefore == attempts[1].CandidateAfter || report.CandidateBefore == report.CandidateAfter {
		t.Fatal("actual source repair not bound to a new candidate")
	}
	before, err := os.ReadFile(attempts[0].ResultPath)
	if err != nil {
		t.Fatal(err)
	}
	status, err := env.eng.TaskStatus(run)
	if err != nil || status.Status != "passed" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	second, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err != nil || second.Status != "passed" || second.InvocationID == report.InvocationID || len(second.Results[0].Attempts) != 3 {
		t.Fatalf("fresh execution=%+v err=%v", second, err)
	}
	after, err := os.ReadFile(attempts[0].ResultPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("failed attempt overwritten")
	}
	writeFile(t, filepath.Join(env.root, "repaired.go"), "package changed\n")
	if _, err := env.eng.TaskStatus(run); err == nil || !strings.Contains(err.Error(), "task_candidate_stale") {
		t.Fatalf("stale status accepted: %v", err)
	}
	if _, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{}); err == nil || !strings.Contains(err.Error(), "task_candidate_stale") {
		t.Fatalf("stale resume accepted: %v", err)
	}
}

func TestTaskRunnerRepairExhaustion(t *testing.T) {
	env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./fail.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "fail.sh", "exit 2")
	taskExecutable(t, env.root, "verify.sh", "echo bad > .beacon/proof.txt")
	repair := taskExecutable(t, env.root, "repair.sh", "echo attempted")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{RepairCommand: repair, MaxRepairs: 2})
	if err == nil || report.Status != "failed" || report.Repairs != 2 || len(report.Results[0].Attempts) != 5 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".beacon", "proof.txt")); !os.IsNotExist(err) {
		t.Fatal("verification ran after failing command")
	}
}

func TestTaskRunnerCancellationAndTimeout(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			env, run := newTaskRunnerEnv(t,
				taskRunnerRow("TASK-A", "-", "./wait.sh", "./verify.sh", ".beacon/proof.txt")+
					taskRunnerRow("TASK-B", "TASK-A", "./verify.sh", "./verify.sh", ".beacon/proof.txt"))
			taskExecutable(t, env.root, "wait.sh", "echo started > .beacon/started\nsleep 30")
			taskExecutable(t, env.root, "verify.sh", "echo incorrect > .beacon/proof.txt")
			repair := taskExecutable(t, env.root, "repair.sh", "echo incorrect > .beacon/repaired")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			opts := TaskRunOptions{Timeout: 50 * time.Millisecond, RepairCommand: repair, MaxRepairs: 1}
			if mode == "cancel" {
				opts.Timeout = 5 * time.Second
				go func() {
					deadline := time.NewTimer(5 * time.Second)
					defer deadline.Stop()
					ticker := time.NewTicker(time.Millisecond)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-deadline.C:
							cancel()
							return
						case <-ticker.C:
							if _, err := os.Stat(filepath.Join(env.root, ".beacon", "started")); err == nil {
								cancel()
								return
							}
						}
					}
				}()
			}
			report, err := env.eng.RunTasks(ctx, run, opts)
			if err == nil || report.Status != "failed" || report.Repairs != 0 || report.Results[1].Status != "pending" {
				t.Fatalf("report=%+v err=%v", report, err)
			}
			a := report.Results[0].Attempts[0]
			if mode == "cancel" && !a.Cancelled {
				t.Fatalf("cancellation not recorded: %+v", a)
			}
			if mode == "timeout" && !a.TimedOut {
				t.Fatalf("timeout not recorded: %+v", a)
			}
			if _, err := os.Stat(a.ResultPath); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskRunnerRefusesBeforeWrites(t *testing.T) {
	for _, mode := range []string{"admission", "manual", "release", "shell", "repair-bound"} {
		t.Run(mode, func(t *testing.T) {
			body := taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt")
			if mode == "manual" {
				body = strings.Replace(body, "|true|", "|false|", 1)
			}
			if mode == "release" {
				body = strings.Replace(body, "./command.sh", "npm install", 1)
			}
			if mode == "shell" {
				body = strings.Replace(body, "./command.sh", "echo ok && touch marker", 1)
			}
			env, run := newTaskRunnerEnv(t, body)
			if mode == "admission" {
				writeFile(t, filepath.Join(env.root, "truth.md"), strings.Replace(frozenTruthDoc, "status: frozen", "status: draft", 1))
			}
			opts := TaskRunOptions{}
			if mode == "repair-bound" {
				opts.MaxRepairs = MaxTaskRepairs + 1
			}
			if _, err := env.eng.RunTasks(context.Background(), run, opts); err == nil {
				t.Fatal("unsafe invocation accepted")
			}
			if _, err := os.Stat(filepath.Join(env.root, ".beacon", "task-runs")); !os.IsNotExist(err) {
				t.Fatalf("refused invocation wrote evidence: %v", err)
			}
		})
	}
}

func TestTaskRunnerFinalVerificationCatchesLaterRegression(t *testing.T) {
	env, run := newTaskRunnerEnv(t,
		taskRunnerRow("TASK-A", "-", "./first.sh", "./verify-first.sh", "first-proof.txt")+
			taskRunnerRow("TASK-B", "TASK-A", "./second.sh", "./verify-second.sh", "second-proof.txt"))
	taskExecutable(t, env.root, "first.sh", "echo good > state.json")
	taskExecutable(t, env.root, "verify-first.sh", "grep good state.json\necho first > first-proof.txt")
	taskExecutable(t, env.root, "second.sh", "echo broken > state.json")
	taskExecutable(t, env.root, "verify-second.sh", "echo second > second-proof.txt")
	report, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err == nil || report.Status != "failed" || !strings.Contains(err.Error(), "task_final_verify_failed") {
		t.Fatalf("later regression passed: %+v %v", report, err)
	}
	attempts := report.Results[0].Attempts
	if len(attempts) != 3 || attempts[2].Phase != "final-verify" || attempts[2].Status != "failed" {
		t.Fatalf("final attempt=%+v", attempts)
	}
	if report.Results[0].Status != "failed" {
		t.Fatal("regressed task remained passed")
	}
}

func TestTaskRunnerSharedEvidenceSnapshotsSurviveReruns(t *testing.T) {
	env, run := newTaskRunnerEnv(t,
		taskRunnerRow("TASK-A", "-", "./command.sh", "./verify-a.sh", "shared.txt")+
			taskRunnerRow("TASK-B", "TASK-A", "./command.sh", "./verify-b.sh", "shared.txt"))
	taskExecutable(t, env.root, "command.sh", "echo command")
	taskExecutable(t, env.root, "verify-a.sh", "echo first > shared.txt")
	taskExecutable(t, env.root, "verify-b.sh", "echo second > shared.txt")
	first, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err != nil || first.Status != "passed" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if first.Results[0].Evidence == first.Results[1].Evidence {
		t.Fatal("shared authored path reused as registered snapshot")
	}
	for idx, expected := range []string{"first", "second"} {
		raw, err := os.ReadFile(first.Results[idx].Evidence)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(raw)) != expected {
			t.Fatalf("task %d snapshot=%s", idx, raw)
		}
	}
	second, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{})
	if err != nil || second.Status != "passed" {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if first.Results[0].Evidence == second.Results[0].Evidence {
		t.Fatal("rerun overwrote snapshot")
	}
	for _, file := range first.Files {
		if err := qaFileMatches(env.root, file); err != nil {
			t.Fatalf("historical evidence became stale: %v", err)
		}
	}
	if _, err := env.eng.TaskStatus(run); err != nil {
		t.Fatal(err)
	}
}

func TestTaskRunnerAllProjectInputsInvalidateResume(t *testing.T) {
	for _, name := range []string{"index.html", "settings.json", "worker", "style.css", "component.vue", "docs/runtime.json"} {
		t.Run(name, func(t *testing.T) {
			env, run := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", "proof.txt"))
			taskExecutable(t, env.root, "command.sh", "echo command")
			taskExecutable(t, env.root, "verify.sh", "echo verified > proof.txt")
			path := filepath.Join(env.root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, "before")
			if _, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{}); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, "after")
			if _, err := env.eng.TaskStatus(run); err == nil || !strings.Contains(err.Error(), "task_candidate_stale") {
				t.Fatalf("input edit accepted: %v", err)
			}
			if _, err := env.eng.RunTasks(context.Background(), run, TaskRunOptions{}); err == nil || !strings.Contains(err.Error(), "task_candidate_stale") {
				t.Fatalf("stale resume accepted: %v", err)
			}
		})
	}
}
