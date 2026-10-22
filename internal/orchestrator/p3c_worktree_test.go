package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// ---- Real worktrees (AC-099) ----

func poolRun(t *testing.T, env gitEnv) RunView {
	t.Helper()
	run := env.createRun(t)
	if _, _, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{
		{Slug: "feat-a", Touches: []string{"a.go"}},
		{Slug: "lake-b", Touches: []string{"b.go"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestWorktreeCreateReal(t *testing.T) {
	env := newGitEnv(t)
	run := poolRun(t, env)
	wv, err := env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(wv.Records) != 2 {
		t.Fatalf("records = %+v", wv.Records)
	}
	for _, rec := range wv.Records {
		if _, err := os.Stat(filepath.Join(rec.Path, ".git")); err != nil {
			t.Errorf("worktree %s missing .git: %v", rec.Path, err)
		}
		if !branchExists(t, env.root, rec.Branch) {
			t.Errorf("branch %s not created", rec.Branch)
		}
		if rec.State != WorktreeActive {
			t.Errorf("state = %s", rec.State)
		}
	}
	// Second create refuses (already active).
	_, err = env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{Lake: "feat-a"})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeExists {
		t.Errorf("second create reason = %s (%v)", rc, err)
	}
}

func branchExists(t *testing.T, root, branch string) bool {
	t.Helper()
	return harness.BranchExists(context.Background(), root, branch)
}

func TestWorktreeMergeDirectionAndSuccess(t *testing.T) {
	env := newGitEnv(t)
	ctx := context.Background()
	run := poolRun(t, env)
	if _, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{}); err != nil {
		t.Fatal(err)
	}
	// Reverse direction refused before git runs.
	_, err := env.eng.WorktreeMerge(ctx, run.RunID, "feat-a", "main_star", "feat-a")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeMergeDirection {
		t.Errorf("reverse merge reason = %s (%v)", rc, err)
	}
	// Satellite -> satellite refused too.
	_, err = env.eng.WorktreeMerge(ctx, run.RunID, "feat-a", "", "lake-b")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeMergeDirection {
		t.Errorf("satellite->satellite reason = %s (%v)", rc, err)
	}
	// Real work in the feat-a satellite, then merge it home.
	wv, err := env.eng.WorktreeStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var sat string
	for _, r := range wv.Records {
		if r.Lake == "feat-a" {
			sat = r.Path
		}
	}
	if err := os.WriteFile(filepath.Join(sat, "feature.txt"), []byte("from satellite\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, sat, "add", ".")
	gitCmd(t, sat, "commit", "-m", "satellite work")
	wv, err = env.eng.WorktreeMerge(ctx, run.RunID, "feat-a", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(env.root, "feature.txt")); err != nil {
		t.Errorf("merged file missing on main star: %v", err)
	}
	for _, r := range wv.Records {
		if r.Lake == "feat-a" && r.State != WorktreeMerged {
			t.Errorf("feat-a state = %s, want merged", r.State)
		}
	}
}

func TestWorktreeMergeConflictStops(t *testing.T) {
	env := newGitEnv(t)
	ctx := context.Background()
	run := poolRun(t, env)
	if _, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{}); err != nil {
		t.Fatal(err)
	}
	wv, _ := env.eng.WorktreeStatus(run.RunID)
	var sat string
	for _, r := range wv.Records {
		if r.Lake == "feat-a" {
			sat = r.Path
		}
	}
	// Both sides rewrite README.md differently.
	if err := os.WriteFile(filepath.Join(sat, "README.md"), []byte("satellite version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, sat, "add", ".")
	gitCmd(t, sat, "commit", "-m", "satellite readme")
	if err := os.WriteFile(filepath.Join(env.root, "README.md"), []byte("main version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, env.root, "add", ".")
	gitCmd(t, env.root, "commit", "-m", "main readme")
	_, err := env.eng.WorktreeMerge(ctx, run.RunID, "feat-a", "", "")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeMergeConflict {
		t.Fatalf("merge reason = %s (%v), want conflict", rc, err)
	}
	// Main star is NOT left mid-merge.
	if _, err := os.Stat(filepath.Join(env.root, ".git", "MERGE_HEAD")); !os.IsNotExist(err) {
		t.Error("main star left mid-merge (MERGE_HEAD present)")
	}
	raw, _ := os.ReadFile(filepath.Join(env.root, "README.md"))
	if string(raw) != "main version\n" {
		t.Errorf("main README = %q, merge must not force-resolve", raw)
	}
}

func TestWorktreeDirtyMainRefused(t *testing.T) {
	env := newGitEnv(t)
	run := poolRun(t, env)
	if err := os.WriteFile(filepath.Join(env.root, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeMainDirty {
		t.Errorf("reason = %s (%v)", rc, err)
	}
	// --allow-dirty is the recorded exception.
	if _, err := env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{AllowDirty: true}); err != nil {
		t.Errorf("allow-dirty create: %v", err)
	}
}

func TestWorktreeCleanupUnmergedRefusedThenDiscard(t *testing.T) {
	env := newGitEnv(t)
	ctx := context.Background()
	run := poolRun(t, env)
	if _, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{}); err != nil {
		t.Fatal(err)
	}
	wv, _ := env.eng.WorktreeStatus(run.RunID)
	var sat string
	for _, r := range wv.Records {
		if r.Lake == "lake-b" {
			sat = r.Path
		}
	}
	if err := os.WriteFile(filepath.Join(sat, "b.txt"), []byte("unmerged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, sat, "add", ".")
	gitCmd(t, sat, "commit", "-m", "unmerged work")
	_, err := env.eng.WorktreeCleanup(ctx, run.RunID, "lake-b", false)
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeUnmerged {
		t.Fatalf("cleanup reason = %s (%v), want unmerged refusal", rc, err)
	}
	if _, err := os.Stat(sat); err != nil {
		t.Error("worktree must survive a refused cleanup")
	}
	wv, err = env.eng.WorktreeCleanup(ctx, run.RunID, "lake-b", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sat); !os.IsNotExist(err) {
		t.Error("discarded worktree directory should be gone")
	}
	for _, r := range wv.Records {
		if r.Lake == "lake-b" && r.State != WorktreeRemoved {
			t.Errorf("lake-b state = %s, want removed", r.State)
		}
	}
}

func TestWorktreeNonGitAndInvalidCombo(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	_, _, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{{Slug: "feat-a"}, {Slug: "lake-b"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeNotGitRepo {
		t.Errorf("non-git reason = %s (%v)", rc, err)
	}
	_, err = env.eng.WorktreeCreate(context.Background(), run.RunID, WorktreeCreateOptions{Mode: WorktreeModeCurrentBranch, Isolated: true})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeInvalidCombo {
		t.Errorf("combo reason = %s (%v)", rc, err)
	}
}

// AC-099/100 alignment: with a real satellite bound, a real
// executor's cwd IS the satellite worktree, not the project root.
func TestExecutorRunsInsideBoundWorktree(t *testing.T) {
	env := newGitEnv(t)
	ctx := context.Background()
	run := poolRun(t, env)
	if _, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{}); err != nil {
		t.Fatal(err)
	}
	script := writeExecScript(t, env.root, "pwd.sh", "#!/bin/sh\necho \"cwd=$PWD\"\nexit 0\n")
	eng := NewEngine(env.st)
	eng.RegisterExecutorForAllStages(NewSubprocessExecutor(eng, []string{script}, 0))
	v, err := eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CurrentStage != StageTruth {
		t.Fatalf("stage = %s", v.CurrentStage)
	}
	arts, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range arts {
		if a.Kind != "stage_stdout" {
			continue
		}
		raw, err := os.ReadFile(a.AbsPath)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), ".beacon-worktrees/feat-a") {
			found = true
		}
	}
	if !found {
		t.Errorf("no capture shows the satellite cwd; artifacts = %+v", arts)
	}
}

// ---- Planner review (AC-102) ----

func reviewerScript(t *testing.T, dir, name, line string) string {
	return writeExecScript(t, dir, name, "#!/bin/sh\necho '"+line+"'\nexit 0\n")
}

func TestPlannerReviewBlockWins(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	pass := reviewerScript(t, env.root, "pass.sh", `{"verdict":"pass","reasons":[]}`)
	block := reviewerScript(t, env.root, "block.sh", `{"verdict":"block","reasons":["missing rollback step"]}`)
	view, err := env.eng.RunPlannerReview(context.Background(), run.RunID, PlannerReviewerSpecs{
		{Name: "rev-pass", Argv: []string{pass}},
		{Name: "rev-block", Argv: []string{block}},
	}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Verdict != "blocked" || view.ReasonCode != core.ReasonPlannerReviewGateBlocked {
		t.Errorf("view = %+v", view)
	}
	if len(view.ReviewerLanes) != 2 || !view.RuntimeEvidenceOnly || view.ExecutionMode != "multi_agent_sequential" {
		t.Errorf("lanes/mode = %+v", view)
	}
	latest, err := env.st.Gates().Latest(env.feature.ID, "planner_review")
	if err != nil || latest.Verdict != "blocked" {
		t.Errorf("gate row = %+v err=%v", latest, err)
	}
}

func TestPlannerReviewInvalidNeverPass(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	garbage := writeExecScript(t, env.root, "noise.sh", "#!/bin/sh\necho not-json\nexit 0\n")
	pass := reviewerScript(t, env.root, "pass.sh", `{"verdict":"pass","reasons":[]}`)
	// Garbage alone: no valid lane -> blocked, never a pass.
	view, err := env.eng.RunPlannerReview(context.Background(), run.RunID, PlannerReviewerSpecs{
		{Name: "rev-noise", Argv: []string{garbage}},
	}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Verdict != "blocked" || view.ReasonCode != core.ReasonPlannerReviewerInvalid {
		t.Errorf("garbage-only view = %+v", view)
	}
	// Garbage + pass: pass stands, invalid is recorded, not counted.
	view, err = env.eng.RunPlannerReview(context.Background(), run.RunID, PlannerReviewerSpecs{
		{Name: "rev-noise", Argv: []string{garbage}},
		{Name: "rev-pass", Argv: []string{pass}},
	}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Verdict != "pass" {
		t.Errorf("mixed view = %+v, want pass with the invalid lane recorded", view)
	}
	// A block with NO reasons is invalid, not a veto.
	emptyBlock := reviewerScript(t, env.root, "empty-block.sh", `{"verdict":"block","reasons":[]}`)
	view, err = env.eng.RunPlannerReview(context.Background(), run.RunID, PlannerReviewerSpecs{
		{Name: "rev-empty", Argv: []string{emptyBlock}},
		{Name: "rev-pass", Argv: []string{pass}},
	}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if view.Verdict != "pass" || view.ReviewerLanes[0].Verdict != LaneInvalid {
		t.Errorf("empty-block view = %+v", view)
	}
}
