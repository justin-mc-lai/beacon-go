// lifecycle-monotonic: Release/Completed is terminal. Bookkeeping
// writes (new run, freeze/refreeze, tick/block) may upgrade to it but
// never move a released feature back.
package orchestrator

import (
	"os"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func setCompletedLifecycle(t *testing.T, env testEnv) {
	t.Helper()
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageRelease, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
}

func lifecycleOf(t *testing.T, env testEnv) store.Lifecycle {
	t.Helper()
	lc, err := env.st.Features().GetLifecycle(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	return lc
}

func TestUpdateLifecycleMonotonicBlocksDowngrade(t *testing.T) {
	env := newTestEnv(t, "frozen")
	setCompletedLifecycle(t, env)
	for _, l := range []store.Lifecycle{
		{FeatureID: env.feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted},
		{FeatureID: env.feature.ID, Stage: core.StageImplement, Status: core.StatusInProgress},
		{FeatureID: env.feature.ID, Stage: core.StageQa, Status: core.StatusBlocked, ReasonCodes: []string{"x"}},
	} {
		if err := updateLifecycleMonotonic(env.st.Features(), l); err != nil {
			t.Fatal(err)
		}
		lc := lifecycleOf(t, env)
		if lc.Stage != core.StageRelease || lc.Status != core.StatusCompleted {
			t.Fatalf("downgrade leaked: %+v", lc)
		}
	}
}

func TestUpdateLifecycleMonotonicNonCompletedStillUpdates(t *testing.T) {
	env := newTestEnv(t, "frozen")
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageImplement, Status: core.StatusInProgress,
	}); err != nil {
		t.Fatal(err)
	}
	if err := updateLifecycleMonotonic(env.st.Features(), store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageQa, Status: core.StatusInProgress,
	}); err != nil {
		t.Fatal(err)
	}
	lc := lifecycleOf(t, env)
	if lc.Stage != core.StageQa || lc.Status != core.StatusInProgress {
		t.Fatalf("non-completed update was skipped: %+v", lc)
	}
}

func TestUpdateLifecycleMonotonicAllowsUpgrade(t *testing.T) {
	env := newTestEnv(t, "frozen")
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
	if err := updateLifecycleMonotonic(env.st.Features(), store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageRelease, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
	lc := lifecycleOf(t, env)
	if lc.Stage != core.StageRelease || lc.Status != core.StatusCompleted {
		t.Fatalf("upgrade to release was blocked: %+v", lc)
	}
}

func TestCreateRunDoesNotDowngradeCompletedFeature(t *testing.T) {
	env := newTestEnv(t, "frozen")
	setCompletedLifecycle(t, env)
	if _, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "another run",
	}); err != nil {
		t.Fatal(err)
	}
	lc := lifecycleOf(t, env)
	if lc.Stage != core.StageRelease || lc.Status != core.StatusCompleted {
		t.Fatalf("CreateRun downgraded a completed feature: %+v", lc)
	}
}

func TestFreezeDoesNotDowngradeCompletedFeature(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	setCompletedLifecycle(t, env)
	if _, err := env.eng.FreezeTruthPackage(ctx, "host"); err != nil {
		t.Fatal(err)
	}
	lc := lifecycleOf(t, env)
	if lc.Stage != core.StageRelease || lc.Status != core.StatusCompleted {
		t.Fatalf("freeze downgraded a completed feature: %+v", lc)
	}
}

func TestLifecycleGuardIsShared(t *testing.T) {
	for _, f := range []string{"engine.go", "truth.go", "tick.go"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "updateLifecycleMonotonic(") {
			t.Fatalf("%s does not use the shared lifecycle guard", f)
		}
	}
}
