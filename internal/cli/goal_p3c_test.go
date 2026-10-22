package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
)

// seedGitGoalProject is seedGoalProject over a REAL git repository
// root (committed README) with the truth package files on disk, so
// the P3c real integrations (worktrees, planner review package read)
// run for real. Scripts live OUTSIDE the repo so they never dirty
// the main tree.
func seedGitGoalProject(t *testing.T) (string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "git-proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	pkgDir := filepath.Join(root, "docs", "features", "feat-a")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "truth.md"), []byte("---\nstatus: frozen\n---\n# truth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "tests.md"), []byte("# tests\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "init")

	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "git-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	scripts := filepath.Join(t.TempDir(), "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, scripts
}

func cliScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// P3c CLI chain: run (record exec + ralph + reviewer) -> subprocess
// ticks -> delegate-ralph REAL -> parallel-plan + worktree create
// (real) -> satellite commit -> worktree merge (real) ->
// planner-review (real) -> acp-probe (real handshake). Plus the
// honesty check: subprocess without configuration refuses.
func TestGoalP3CRealChain(t *testing.T) {
	root, scripts := seedGitGoalProject(t)
	// S3 seed (declared change): the real delegation entry consumes
	// the AC-142 bundle, which since AC-173 requires a recorded,
	// confirmed topology projection on git projects — projected
	// here through the real CLI.
	if out, err := runCLI(t, "goal", "topology", "project", "--project", root, "--feature", "feat-a",
		"--graph", writeGraphFile(t, root), "--by", "tester", "--json"); err != nil {
		t.Fatalf("topology project seed: %v (%s)", err, out)
	}
	agent := cliScript(t, scripts, "agent.sh", "#!/bin/sh\necho agent-stage-done\nexit 0\n")
	// The ralph script keeps its counter next to the loop's own
	// context file (inside .beacon/, beacon's reserved dir) so the
	// agent's bookkeeping never dirties the user's main tree.
	ralph := cliScript(t, scripts, "ralph.sh",
		"#!/bin/sh\nc=$(dirname \"$BEACON_CONTEXT_FILE\")/count\nn=$(cat $c 2>/dev/null || echo 0)\nn=$((n+1))\necho $n > $c\necho \"ralph $n\"\nif [ \"$n\" -ge 2 ]; then echo RALPH_DONE; fi\nexit 0\n")
	reviewer := cliScript(t, scripts, "reviewer.sh", "#!/bin/sh\necho '{\"verdict\":\"pass\",\"reasons\":[]}'\nexit 0\n")

	out, err := runCLI(t, "goal", "run", "build it for real",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1",
		"--exec-cmd", agent, "--ralph-cmd", ralph, "--reviewer-cmd", reviewer, "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	// Subprocess ticks walk plan + truth with the REAL agent command
	// (recorded on the run — no --exec-cmd repeated here).
	for i := 0; i < 2; i++ {
		out, err = runCLI(t, "goal", "tick", "--run-id", runID, "--executor", "subprocess", "--json")
		if err != nil {
			t.Fatalf("subprocess tick %d: %v (out %s)", i, err, out)
		}
	}
	if got := strField(t, parseDoc(t, out), "current_stage"); got != "implement" {
		t.Fatalf("current_stage = %q, want implement", got)
	}

	// delegate-ralph with the recorded ralph command runs the REAL
	// loop and collects through the harvest — simulated must be false.
	out, err = runCLI(t, "goal", "delegate-ralph", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("delegate-ralph real: %v (out %s)", err, out)
	}
	dlg := parseDoc(t, out)["delegation"].(map[string]any)
	if dlg["simulated"] != false || dlg["status"] != "returned" {
		t.Errorf("delegation = %v, want real (simulated=false) + returned", dlg)
	}

	// Parallel plan, then REAL worktrees.
	out, err = runCLI(t, "goal", "parallel-plan", "--run-id", runID,
		"--lake", "feat-a", "--lake", "lake-b", "--json")
	if err != nil {
		t.Fatalf("parallel-plan: %v", err)
	}
	out, err = runCLI(t, "goal", "worktree", "create", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("worktree create: %v (out %s)", err, out)
	}
	sat := filepath.Join(root, ".beacon-worktrees", "feat-a")
	if _, err := os.Stat(filepath.Join(sat, "README.md")); err != nil {
		t.Fatalf("satellite worktree not real: %v", err)
	}

	// Commit in the satellite, merge it home through the CLI.
	if err := os.WriteFile(filepath.Join(sat, "merged.txt"), []byte("merged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
		}
	}
	gitIn(sat, "add", ".")
	gitIn(sat, "commit", "-m", "satellite work")
	out, err = runCLI(t, "goal", "worktree", "merge", "--run-id", runID, "--lake", "feat-a", "--json")
	if err != nil {
		t.Fatalf("worktree merge: %v (out %s)", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "merged.txt")); err != nil {
		t.Errorf("merged file missing on main star: %v", err)
	}

	// Planner review with the recorded reviewer runs for real.
	out, err = runCLI(t, "goal", "planner-review", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("planner-review: %v (out %s)", err, out)
	}
	rev := parseDoc(t, out)
	if rev["verdict"] != "pass" || rev["execution_mode"] != "multi_agent_sequential" || rev["runtime_evidence_only"] != true {
		t.Errorf("review = %v", rev)
	}

	// ACP probe against a fake agent: responded, probe-labelled.
	fakeACP := cliScript(t, scripts, "fake-acp.py", "#!/usr/bin/env python3\nimport sys, json\nreq = json.loads(sys.stdin.readline())\nresp = {\"jsonrpc\": \"2.0\", \"id\": req[\"id\"], \"result\": {\"protocolVersion\": 1, \"agentCapabilities\": {\"loadSession\": True}, \"agentInfo\": {\"name\": \"fake-agent\"}}}\nsys.stdout.write(json.dumps(resp) + \"\\n\")\nsys.stdout.flush()\n")
	out, err = runCLI(t, "goal", "acp-probe", "--target", fakeACP, "--json")
	if err != nil {
		t.Fatalf("acp-probe: %v (out %s)", err, out)
	}
	probe := parseDoc(t, out)
	if probe["responded"] != true || probe["probe"] != true || probe["full_backend"] != false {
		t.Errorf("probe = %v", probe)
	}

	// Honesty: a fresh run with NO recorded command refuses a
	// subprocess tick — no fallback to the placeholder.
	out2, err := runCLI(t, "goal", "run", "no commands here",
		"--project", root, "--feature", "feat-a", "--json")
	if err != nil {
		t.Fatalf("goal run 2: %v", err)
	}
	run2 := strField(t, parseDoc(t, out2), "run_id")
	_, err = runCLI(t, "goal", "tick", "--run-id", run2, "--executor", "subprocess", "--json")
	if err == nil {
		t.Fatal("subprocess tick without configuration must refuse")
	}
}
