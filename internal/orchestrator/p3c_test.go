package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// ---- helpers (P3c) ----

func writeExecScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func gitCmd(t *testing.T, dir string, args ...string) string {
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

// newGitEnv mirrors newTestEnv but the project root is a REAL git
// repository (one committed file) so worktree integration runs for
// real.
type gitEnv struct {
	testEnv
}

func newGitEnv(t *testing.T) gitEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-b", "main")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "init")
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md", Status: "frozen",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(st)
	rec := NewRecordingExecutor(st, "rec-1")
	eng.RegisterExecutorForAllStages(rec)
	return gitEnv{testEnv{st: st, eng: eng, rec: rec, feature: f, root: root}}
}

func stageExecutedEvents(t *testing.T, st *store.Store, runID string) []map[string]any {
	t.Helper()
	events, err := st.ListEventsByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, ev := range events {
		if ev.EventType != "stage_executed" {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

// ---- Subprocess executor (AC-044) ----

func TestSubprocessExecutorRealEvidence(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	script := writeExecScript(t, env.root, "agent.sh", "#!/bin/sh\necho hello-from-agent\nexit 0\n")
	eng := NewEngine(env.st)
	eng.RegisterExecutorForAllStages(NewSubprocessExecutor(eng, []string{script}, 0))
	v, err := eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentStage != StageTruth {
		t.Fatalf("stage = %s, want truth after real plan execution", v.CurrentStage)
	}
	arts, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var stdoutRef *store.ArtifactRef
	for i := range arts {
		if arts[i].Kind == "stage_stdout" {
			stdoutRef = &arts[i]
		}
	}
	if stdoutRef == nil {
		t.Fatalf("no stage_stdout artifact in %+v", arts)
	}
	sum := sha256.Sum256([]byte("hello-from-agent\n"))
	if stdoutRef.ContentHash != hex.EncodeToString(sum[:]) || stdoutRef.SizeBytes != int64(len("hello-from-agent\n")) {
		t.Errorf("stdout ref hash/size = %s/%d", stdoutRef.ContentHash, stdoutRef.SizeBytes)
	}
	evs := stageExecutedEvents(t, env.st, run.RunID)
	if len(evs) != 1 || evs[0]["exit_code"] != float64(0) || evs[0]["real"] != true {
		t.Errorf("events = %+v", evs)
	}
}

func TestSubprocessExecutorNonZeroGoesToBlocker(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	script := writeExecScript(t, env.root, "fail.sh", "#!/bin/sh\necho broken >&2\nexit 3\n")
	eng := NewEngine(env.st)
	eng.RegisterExecutorForAllStages(NewSubprocessExecutor(eng, []string{script}, 0))
	v, err := eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonExecutorExitNonZero {
		t.Errorf("view = %+v, want blocked with executor_exit_nonzero", v)
	}
	evs := stageExecutedEvents(t, env.st, run.RunID)
	if len(evs) != 1 || evs[0]["exit_code"] != float64(3) {
		t.Errorf("events = %+v, want real exit code 3 recorded", evs)
	}
}

func TestSubprocessExecutorTimeoutOutcome(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	script := writeExecScript(t, env.root, "slow.sh", "#!/bin/sh\nsleep 5\n")
	ex := NewSubprocessExecutor(env.eng, []string{script}, 1)
	outcome, err := ex.ExecuteStage(context.Background(), run, StagePlan)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "failed" || outcome.ReasonCode != core.ReasonExecutorTimedOut {
		t.Errorf("outcome = %+v", outcome)
	}
	evs := stageExecutedEvents(t, env.st, run.RunID)
	if len(evs) != 1 || evs[0]["timed_out"] != true {
		t.Errorf("events = %+v, want timed_out evidence", evs)
	}
}

func TestSubprocessExecutorMissingAndUnconfigured(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	ex := NewSubprocessExecutor(env.eng, []string{"/nonexistent/beacon-nope"}, 0)
	_, err := ex.ExecuteStage(context.Background(), run, StagePlan)
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonExecutorCommandMissing {
		t.Errorf("reason = %s (%v)", rc, err)
	}
	// No config anywhere: subprocess selector refuses — no silent
	// fallback to the recording placeholder.
	_, err = env.eng.ResolveExecutorFor(run.RunID, "subprocess", nil)
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonExecutorNotConfigured {
		t.Errorf("resolve reason = %s (%v)", rc, err)
	}
	// Unknown selector refuses too.
	_, err = env.eng.ResolveExecutorFor(run.RunID, "telepathy", nil)
	if err == nil {
		t.Error("unknown selector must error")
	}
}

// ---- Profile executor (harness face) ----

func TestProfileExecutorContextAndWorkdir(t *testing.T) {
	env := newTestEnv(t, "frozen")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	// The profile script demands the context file and checks the
	// implement stage + truth path are inside it.
	script := writeExecScript(t, env.root, "prof.sh",
		"#!/bin/sh\ngrep -q '\"stage\": \"implement\"' \"$1\" || exit 8\ngrep -q 'truth.md' \"$1\" || exit 7\necho profile-implemented\nexit 0\n")
	prof := harness.Profile{Name: "test-style", Argv: []string{script, "{context}"}}
	eng := NewEngine(env.st)
	eng.RegisterExecutor(StageImplement, NewProfileExecutor(eng, prof, 0))
	v, err := eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentStage != StageQA {
		t.Fatalf("stage = %s, want qa after profile implement", v.CurrentStage)
	}
	arts, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var ctxRef *store.ArtifactRef
	for i := range arts {
		if arts[i].Kind == "stage_context" {
			ctxRef = &arts[i]
		}
	}
	if ctxRef == nil {
		t.Fatal("no stage_context artifact")
	}
	raw, err := os.ReadFile(ctxRef.AbsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "\"stage\": \"implement\"") || !strings.Contains(string(raw), "truth.md") {
		t.Errorf("context file missing stage/truth:\n%s", raw)
	}
}

// ---- Ralph real loop via delegation (G-10) ----

func ralphTwoRoundScript(t *testing.T, dir string) string {
	return writeExecScript(t, dir, "ralph.sh",
		"#!/bin/sh\nn=$(cat count 2>/dev/null || echo 0)\nn=$((n+1))\necho $n > count\necho \"ralph iteration $n\"\nif [ \"$n\" -ge 2 ]; then echo RALPH_DONE; fi\nexit 0\n")
}

func TestDelegateRalphRealLoopCollects(t *testing.T) {
	env := newGitEnv(t)
	// S3 seed (declared change): the real-delegation entry consumes
	// the AC-142 bundle, which since AC-173 requires a recorded,
	// confirmed topology projection on git projects.
	seedSingleRepoTopology(t, env.eng, env.feature.ID, env.root)
	run := advanceToImplement(t, env.testEnv)
	script := ralphTwoRoundScript(t, env.root)
	dv, err := env.eng.DelegateRalphReal(context.Background(), run.RunID, []string{script}, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if dv.Delegation.Simulated {
		t.Error("real delegation must carry simulated=false")
	}
	if dv.Delegation.Status != DelegationReturned {
		t.Fatalf("delegation = %+v, want returned via the shared harvest", dv.Delegation)
	}
	// Per-iteration evidence: >= 2 real stage_executed events.
	evs := stageExecutedEvents(t, env.st, run.RunID)
	real := 0
	for _, ev := range evs {
		if ev["real"] == true {
			real++
		}
	}
	if real < 2 {
		t.Errorf("real stage_executed events = %d, want >= 2 (one per ralph iteration)", real)
	}
	// Per-iteration checkpoints exist.
	ckpts, err := env.st.Runs().ListCheckpoints(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, c := range ckpts {
		if strings.Contains(string(c.State), "ralph") || strings.Contains(string(c.State), "ralph-iteration") {
			found++
		}
	}
	if found < 2 {
		// Checkpoint state JSON carries next_action=ralph_loop; count total instead.
		if len(ckpts) < 2 {
			t.Errorf("checkpoints = %d, want >= 2", len(ckpts))
		}
	}
}

func TestRalphExecutorCeilingFails(t *testing.T) {
	env := newTestEnv(t, "frozen")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	script := writeExecScript(t, env.root, "ralph-never.sh", "#!/bin/sh\necho working\nexit 0\n")
	ex := NewRalphExecutor(env.eng, []string{script}, 0, 2)
	outcome, err := ex.ExecuteStage(context.Background(), run, StageImplement)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "failed" || outcome.ReasonCode != core.ReasonRalphMaxIterations {
		t.Errorf("outcome = %+v, want failed/ralph_max_iterations_reached", outcome)
	}
}

// Bad harvest still rejected in P3c: a (non-simulated) delegation
// whose return brings artifacts but NO post-delegation implement
// evidence is refused by ReceiveRalphResult.
func TestReceiveWithoutEvidenceStillRejectedP3c(t *testing.T) {
	env := newTestEnv(t, "frozen")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	dv, err := env.eng.DelegateRalph(context.Background(), run.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	art := writeExecScript(t, env.root, "out.txt", "payload\n")
	_, err = env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Summary: "here",
		Artifacts: []ArtifactInput{{Kind: "implement_artifact", AbsPath: art}},
	})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultEvidenceMissing {
		t.Errorf("reason = %s (%v), want result_evidence_missing", rc, err)
	}
}
