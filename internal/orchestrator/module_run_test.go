package orchestrator

import (
	"context"
	"testing"

	"beacon-go/internal/intenttree"
)

func TestModuleWorktreeSlugsStayUniqueForDuplicatePaths(t *testing.T) {
	idx := intenttree.Index{Nodes: []intenttree.IndexEntry{
		{ID: "m-alpha", Kind: intenttree.KindModule, Path: "one/alpha"},
		{ID: "m-beta", Kind: intenttree.KindModule, Path: "two/alpha"},
		{ID: "m-gamma", Kind: intenttree.KindModule, Path: "three/gamma"},
	}}
	slugs := moduleSlugs(idx)
	plan := BuildParallelPlan([]LakeInput{
		{Slug: slugs["m-alpha"]}, {Slug: slugs["m-beta"]}, {Slug: slugs["m-gamma"]},
	}, true)
	if got, want := len(plan.Satellites), 3; got != want {
		t.Fatalf("satellites = %d, want one per module (%d): %+v", got, want, plan.Satellites)
	}
	seen := map[string]bool{}
	for _, slug := range slugs {
		if seen[slug] {
			t.Fatalf("duplicate worktree slug %q for modules: %+v", slug, slugs)
		}
		seen[slug] = true
	}
}

func TestCompleteReleasesModuleSuccessor(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	first := env.createRun(t)
	second := env.createRun(t)
	prepareProductQA(t, env, first.RunID, "qa-bot")

	firstRaw, firstMeta, _, err := env.eng.loadRun(first.RunID)
	if err != nil {
		t.Fatal(err)
	}
	firstMeta.ModuleRun = &ModuleRunInfo{ModuleNodeID: "alpha", FeatureNodeID: "one", Position: 0, State: ModuleRunStateRunning}
	if err := env.eng.saveRun(firstRaw, firstMeta); err != nil {
		t.Fatal(err)
	}
	secondRaw, secondMeta, _, err := env.eng.loadRun(second.RunID)
	if err != nil {
		t.Fatal(err)
	}
	secondMeta.ModuleRun = &ModuleRunInfo{
		ModuleNodeID: "alpha", FeatureNodeID: "two", Position: 1,
		State: ModuleRunStatePending, PredecessorRunID: first.RunID,
	}
	secondMeta.StagePlan[0].Status = "pending"
	if err := env.eng.saveRun(secondRaw, secondMeta); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4; i++ {
		if _, err := env.eng.Tick(context.Background(), first.RunID); err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
	}
	if _, err := env.eng.Complete(first.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "lai"}); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	view, err := env.eng.Status(second.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if view.ModuleRun == nil || view.ModuleRun.State != ModuleRunStateRunning {
		t.Fatalf("second module run not released by Complete: %+v", view)
	}
}

func TestExpectedWorktreeUsesModuleSlug(t *testing.T) {
	m := &runMeta{
		ModuleRun: &ModuleRunInfo{ModuleSlug: "alpha-m-alpha"},
		ParallelPlan: &ParallelPlan{
			Mode: ModulePoolMode,
			Satellites: []SatellitePlan{
				{Lake: "alpha-m-alpha", Worktree: ".beacon/worktrees/satellite/alpha-m-alpha"},
			},
		},
	}
	if got, want := expectedWorktree(m, "module-feature"), ".beacon/worktrees/satellite/alpha-m-alpha"; got != want {
		t.Fatalf("expectedWorktree = %q, want %q", got, want)
	}
}
