// Regression tests for the shared frozen determination (P4b,
// fixing the P4a observation item): admission and the truth exit
// gate must agree with the truth document itself, not only with
// the lifecycle snapshot a goal run keeps moving.
package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// writeTruthDoc lays a real truth.md at the test env's package
// path (the shared determination reads the document itself).
func writeTruthDoc(t *testing.T, env testEnv, status string) {
	t.Helper()
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nslug: feat-a\nstatus: " + status + "\n---\n# Truth\n\n- **AC-001**：the one requirement\n"
	if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func admissionItemOf(t *testing.T, res AdmissionResult, check string) AdmissionItem {
	t.Helper()
	for _, it := range res.Items {
		if it.Check == check {
			return it
		}
	}
	t.Fatalf("admission has no %s item: %+v", check, res.Items)
	return AdmissionItem{}
}

// The P4a observation scenario, end to end: the package is frozen
// on disk and its lifecycle sits at Freeze/Completed; CreateRun
// then advances the lifecycle (Prd/InProgress) — admission must
// NOT fall back to freeze_ack_required, and the truth exit gate
// must pass on the document's authority.
func TestFrozenFactObservationScenario(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeTruthDoc(t, env, "frozen")
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
	run := env.createRun(t)
	// Precondition of the observation: the run moved the lifecycle
	// off Freeze (this is what used to void the frozen evidence).
	lc, err := env.st.Features().GetLifecycle(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Stage == core.StageFreeze && lc.Status == core.StatusCompleted {
		t.Fatalf("precondition broken: CreateRun left lifecycle at Freeze/Completed (%s/%s)", lc.Stage, lc.Status)
	}
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ff.Frozen || ff.Source != FrozenSourceFrontmatter {
		t.Fatalf("fact = %+v, want frozen via frontmatter", ff)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	item := admissionItemOf(t, adm, "truth_frozen")
	if !item.OK {
		t.Fatalf("truth_frozen item must pass for a document-frozen package, got %+v (all codes %v)", item, adm.ReasonCodes)
	}
	for _, c := range adm.ReasonCodes {
		if c == core.ReasonFreezeAckRequired {
			t.Fatalf("freeze_ack_required must not appear, codes %v", adm.ReasonCodes)
		}
	}
	// Truth exit gate agrees (build the input for the truth stage
	// directly through the engine's input builder).
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	in, err := env.eng.buildExitInput(mustRun(t, env, run.RunID), m)
	if err != nil {
		t.Fatal(err)
	}
	in.Stage = StageTruth
	v := EvaluateStageExit(in)
	if !v.Allowed || v.FrozenSource != FrozenSourceFrontmatter {
		t.Fatalf("truth exit = %+v, want allowed via frontmatter", v)
	}
}

func mustRun(t *testing.T, env testEnv, runID string) store.Run {
	t.Helper()
	run, _, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// The stricter direction: the document says draft. A lifecycle
// parked at Freeze/Completed must NOT resurrect frozenness — the
// lifecycle is reference information only now.
func TestFrozenFactDraftDocumentLifecycleCannotResurrect(t *testing.T) {
	env := newTestEnv(t, "draft")
	writeTruthDoc(t, env, "draft")
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: env.feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ff.Frozen {
		t.Fatalf("draft document must not be frozen, fact = %+v", ff)
	}
	if ff.Source != FrozenSourceFrontmatter || !ff.LifecycleFrozen {
		t.Fatalf("fact = %+v, want source frontmatter with lifecycle reference true", ff)
	}
	run := env.createRun(t)
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	item := admissionItemOf(t, adm, "truth_frozen")
	if item.OK || item.ReasonCode != core.ReasonFreezeAckRequired {
		t.Fatalf("admission item = %+v, want freeze_ack_required block", item)
	}
}

// A stale formal pass cannot resurrect a document that speaks
// draft either: the document outranks the verdict minted earlier.
func TestFrozenFactDraftDocumentBeatsStaleGatePass(t *testing.T) {
	env := newTestEnv(t, "draft")
	writeTruthDoc(t, env, "draft")
	env.seedTruthFrozen(t) // formal truth_freeze pass on record
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ff.Frozen || ff.Source != FrozenSourceFrontmatter || !ff.FormalGatePass {
		t.Fatalf("fact = %+v, want not frozen via frontmatter despite formal pass", ff)
	}
}

// A pending (unrefrozen) change voids frozenness even though the
// document still declares frozen and the gate once passed.
func TestFrozenFactChangePendingVoidsFrozen(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeTruthDoc(t, env, "frozen")
	env.seedTruthFrozen(t)
	if err := env.eng.appendFeatureEvent(env.feature.ID, EventTruthChangeRequested, map[string]any{
		"change_id": "chg_test1", "summary": "widen AC-001", "by": "lai",
	}); err != nil {
		t.Fatal(err)
	}
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ff.Frozen || !ff.ChangePending || ff.Source != FrozenSourceChangePending {
		t.Fatalf("fact = %+v, want not frozen via change_pending", ff)
	}
	run := env.createRun(t)
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	item := admissionItemOf(t, adm, "truth_frozen")
	if item.OK || item.ReasonCode != core.ReasonTruthChangePending {
		t.Fatalf("admission item = %+v, want truth_change_pending block", item)
	}
	// The truth exit gate blocks with the same code.
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	in, err := env.eng.buildExitInput(mustRun(t, env, run.RunID), m)
	if err != nil {
		t.Fatal(err)
	}
	in.Stage = StageTruth
	if v := EvaluateStageExit(in); v.Allowed || v.ReasonCode != core.ReasonTruthChangePending {
		t.Fatalf("truth exit = %+v, want blocked truth_change_pending", v)
	}
	// A refreeze event closes the change; frozenness returns.
	if err := env.eng.appendFeatureEvent(env.feature.ID, EventTruthRefrozen, map[string]any{"by": "lai"}); err != nil {
		t.Fatal(err)
	}
	ff, err = env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ff.Frozen || ff.Source != FrozenSourceFrontmatter {
		t.Fatalf("after refreeze fact = %+v, want frozen via frontmatter", ff)
	}
}

// When the document cannot speak (no file on disk at all), the
// formal truth_freeze verdict is the corroborating source — the
// pre-P4b derivation the seeded tests rely on.
func TestFrozenFactFormalGateCorroboratesSilentDocument(t *testing.T) {
	env := newTestEnv(t, "active") // no truth.md is written to disk
	env.seedTruthFrozen(t)
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ff.Frozen || ff.Source != FrozenSourceFormalGate {
		t.Fatalf("fact = %+v, want frozen via formal_gate", ff)
	}
	// And with neither document nor verdict: fail-closed.
	env2 := newTestEnv(t, "active")
	ctx2, err := env2.st.ResolveFeature(env2.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	ff2, err := env2.eng.FrozenFact(ctx2)
	if err != nil {
		t.Fatal(err)
	}
	if ff2.Frozen || ff2.Source != FrozenSourceNone {
		t.Fatalf("fact = %+v, want fail-closed not frozen", ff2)
	}
}
