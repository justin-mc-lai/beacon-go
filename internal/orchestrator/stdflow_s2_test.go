// Standard flow S2 tests (AC-138/141/142): the eight-step flow
// state machine, the AC-141 baseline-merged + dirty-policy
// admission items and the AC-142 admission evidence bundle —
// against REAL git repositories (behavior-level, no git mocks).
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// s2Setup builds the committed-baseline state on main: package
// created, frozen, committed, baseline recorded (S1 machinery).
func s2Setup(t *testing.T, slug string) (s1GitEnv, store.FeatureContext) {
	t.Helper()
	env := newS1GitEnv(t, "main")
	env.create(t, slug)
	ctx := env.ctx(t, slug)
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	env.commitAll(t, "freeze "+slug)
	if _, err := env.eng.BaselineTruth(ctx, "tester"); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	// S3 seed (declared change): git projects now require a
	// recorded, confirmed BranchContext at the implement entry
	// (AC-173 topology_gate_missing otherwise). This fixture models
	// a project that HAS projected its single-repo topology, so the
	// S2 scenarios keep judging their own legs.
	seedSingleRepoTopology(t, env.eng, ctx.Feature.ID, env.root)
	return env, ctx
}

func (env s1GitEnv) s2Run(t *testing.T, slug string) RunView {
	t.Helper()
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: slug, Utterance: "build", ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func advanceOK(t *testing.T, env s1GitEnv, runID, step string) FlowView {
	t.Helper()
	v, err := env.eng.FlowAdvance(runID, step)
	if err != nil {
		t.Fatalf("advance %s: %v (view %+v)", step, err, v)
	}
	if !v.Advanced {
		t.Fatalf("advance %s: not advanced: %+v", step, v)
	}
	return v
}

// ---- AC-138: order, full walk, criteria, idempotency ----

func TestS2FlowSkipStepRejected(t *testing.T) {
	env, _ := s2Setup(t, "feat-skip")
	run := env.s2Run(t, "feat-skip")
	// Jumping straight to canonical_commit skips three steps.
	_, err := env.eng.FlowAdvance(run.RunID, FlowStepCommit)
	if rc := s1Reason(t, err); rc != core.ReasonStandardFlowOrderViolation {
		t.Fatalf("skip reason = %s (%v)", rc, err)
	}
	// Unknown steps are order violations too.
	_, err = env.eng.FlowAdvance(run.RunID, "teleport_to_implement")
	if rc := s1Reason(t, err); rc != core.ReasonStandardFlowOrderViolation {
		t.Fatalf("unknown step reason = %s (%v)", rc, err)
	}
	// Nothing was recorded.
	v, err := env.eng.FlowStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CompletedCount != 0 || v.NextStep != FlowStepAdvisory {
		t.Fatalf("status after rejected skips = %+v", v)
	}
}

func TestS2FlowFullWalkAndIdempotent(t *testing.T) {
	env, ctx := s2Setup(t, "feat-walk")
	run := env.s2Run(t, "feat-walk")
	advanceOK(t, env, run.RunID, FlowStepAdvisory)
	// Idempotent repeat: already_done, no error, no second event.
	v, err := env.eng.FlowAdvance(run.RunID, FlowStepAdvisory)
	if err != nil {
		t.Fatalf("repeat advance: %v", err)
	}
	if !v.AlreadyDone || v.Advanced {
		t.Fatalf("repeat advance view = %+v, want already_done", v)
	}
	advanceOK(t, env, run.RunID, FlowStepAuthoring)
	advanceOK(t, env, run.RunID, FlowStepRefreeze)
	advanceOK(t, env, run.RunID, FlowStepCommit)
	advanceOK(t, env, run.RunID, FlowStepReconcile)
	// governance_verify needs an integrity evaluation on record.
	if _, err := env.eng.EvaluateIntegrity(ctx, "tester", ""); err != nil {
		t.Fatalf("EvaluateIntegrity: %v", err)
	}
	advanceOK(t, env, run.RunID, FlowStepGovernance)
	bv := advanceOK(t, env, run.RunID, FlowStepAdmission)
	if bv.Bundle == nil || !bv.Bundle.AllowImplement {
		t.Fatalf("admission step bundle = %+v, want persisted passing bundle", bv.Bundle)
	}
	advanceOK(t, env, run.RunID, FlowStepResume)
	v, err = env.eng.FlowStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Complete || v.CompletedCount != 8 || v.NextStep != "" {
		t.Fatalf("final status = %+v, want complete 8/8", v)
	}
	// Exactly one flow_step_completed event per step.
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, ev := range events {
		if ev.EventType == EventFlowStepCompleted {
			counts[string(ev.Payload)]++
		}
	}
	if len(counts) != 8 {
		t.Fatalf("flow_step_completed events = %d distinct payloads, want 8 (%v)", len(counts), counts)
	}
}

func TestS2FlowCommitCriterionBlocks(t *testing.T) {
	env := newS1GitEnv(t, "main")
	env.create(t, "feat-nc")
	ctx := env.ctx(t, "feat-nc")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	// Frozen but NOT committed: advisory/authoring/refreeze pass,
	// canonical_commit must be refused with the S1 code.
	run := env.s2Run(t, "feat-nc")
	advanceOK(t, env, run.RunID, FlowStepAdvisory)
	advanceOK(t, env, run.RunID, FlowStepAuthoring)
	advanceOK(t, env, run.RunID, FlowStepRefreeze)
	_, err := env.eng.FlowAdvance(run.RunID, FlowStepCommit)
	if rc := s1Reason(t, err); rc != core.ReasonCanonicalRefreezeNotCommitted {
		t.Fatalf("commit step reason = %s (%v)", rc, err)
	}
	v, err := env.eng.FlowStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.CompletedCount != 3 || v.NextStep != FlowStepCommit {
		t.Fatalf("status = %+v, want parked at canonical_commit with 3 done", v)
	}
}

func TestS2FlowGovernanceCriterionBlocks(t *testing.T) {
	// A package frozen WITHOUT the truth service (frontmatter
	// flipped by hand + a freeze event appended directly): the
	// freeze path's own integrity evaluation (truth.go) never ran,
	// so no truth_integrity row exists when governance_verify is
	// attempted — the missing-fact scenario for this step.
	env := newS1GitEnv(t, "main")
	env.create(t, "feat-gov")
	ctx := env.ctx(t, "feat-gov")
	truthPath := env.pkgTruthPath("feat-gov")
	raw := readFile(t, truthPath)
	flipped, changed := setFrontmatterStatus(raw, "frozen")
	if !changed {
		t.Fatal("status flip did not apply")
	}
	if err := os.WriteFile(truthPath, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	env.commitAll(t, "freeze feat-gov by hand")
	pkg := env.eng.packageFiles(ctx)
	if err := env.eng.appendFeatureEvent(ctx.Feature.ID, EventTruthFrozen, map[string]any{
		"package_fingerprint": core.PackageFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text)),
		"surface_fingerprint": core.PackageSurfaceFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text),
			[]byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text)),
		"by": "tester",
	}); err != nil {
		t.Fatal(err)
	}
	run := env.s2Run(t, "feat-gov")
	for _, s := range []string{FlowStepAdvisory, FlowStepAuthoring, FlowStepRefreeze, FlowStepCommit, FlowStepReconcile} {
		advanceOK(t, env, run.RunID, s)
	}
	// No integrity evaluation on record yet.
	_, err := env.eng.FlowAdvance(run.RunID, FlowStepGovernance)
	if rc := s1Reason(t, err); rc != core.ReasonStandardFlowVerificationIncm {
		t.Fatalf("governance step reason = %s (%v)", rc, err)
	}
	// Record one, and the same step now passes.
	if _, err := env.eng.EvaluateIntegrity(ctx, "tester", ""); err != nil {
		t.Fatal(err)
	}
	advanceOK(t, env, run.RunID, FlowStepGovernance)
}

// ---- AC-141: baseline merged (real satellite worktree) ----

func TestS2BaselineNotMergedBlocksAndMergeFixes(t *testing.T) {
	env, _ := s2Setup(t, "feat-mrg")
	initHead := strings.TrimSpace(gitCmd(t, env.root, "rev-list", "--max-parents=0", "HEAD"))
	// A satellite worktree cut from the INIT commit (before the
	// baseline commit existed), bound to the run as a real record.
	wtPath := filepath.Join(env.root, ".beacon-worktrees", "feat-mrg")
	gitCmd(t, env.root, "worktree", "add", "-b", "feat-dev", wtPath, initHead)
	run := env.s2Run(t, "feat-mrg")
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	m.Implement.Worktrees = &WorktreeState{Records: []WorktreeRecord{{
		Lake: "feat-mrg", Branch: "feat-dev", Path: wtPath,
		Ref: ".beacon/worktrees/satellite/feat-mrg", State: WorktreeActive, CreatedAt: nowISO(),
	}}}
	if err := env.eng.saveRun(s2RunRow(t, env, run.RunID), m); err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it := admissionItem(adm, AdmissionCheckBaselineMerged)
	if adm.AllowImplement || !it.Applicable || it.OK || it.ReasonCode != core.ReasonTruthBaselineNotMerged {
		t.Fatalf("admission with unmerged baseline = %+v item=%+v, want blocked truth_baseline_not_merged", adm, it)
	}
	if adm.ReasonCode != core.ReasonTruthBaselineNotMerged {
		t.Fatalf("headline = %s, want truth_baseline_not_merged", adm.ReasonCode)
	}
	// Merge the canonical branch into the satellite: turns green.
	gitCmd(t, wtPath, "merge", "--no-edit", "main")
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it = admissionItem(adm, AdmissionCheckBaselineMerged)
	if !it.OK || !adm.AllowImplement {
		t.Fatalf("admission after merge = %+v item=%+v, want green", adm, it)
	}
}

func s2RunRow(t *testing.T, env s1GitEnv, runID string) store.Run {
	t.Helper()
	run, err := env.st.Runs().GetByID(runID)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// ---- AC-141: dirty policy ----

func TestS2DirtyPolicyPackageAndOrdinary(t *testing.T) {
	env, _ := s2Setup(t, "feat-dirty")
	run := env.s2Run(t, "feat-dirty")
	// Ordinary code file dirty: NOT a violation at this point.
	codePath := filepath.Join(env.root, "main.go")
	if err := os.WriteFile(codePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !adm.AllowImplement {
		t.Fatalf("ordinary dirty code must not block admission: %+v", adm)
	}
	// Requirement-package dirty: violation, unscoped_dirty_state.
	testsPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-dirty", "tests.md")
	raw := readFile(t, testsPath)
	if err := os.WriteFile(testsPath, []byte(raw+"\n<!-- local tweak -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it := admissionItem(adm, AdmissionCheckDirtyPolicy)
	if adm.AllowImplement || it.OK || it.ReasonCode != core.ReasonUnscopedDirtyState {
		t.Fatalf("admission with dirty package = %+v item=%+v, want blocked unscoped_dirty_state", adm, it)
	}
	// Protected path dirty: also a violation.
	if err := os.WriteFile(testsPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	envPath := filepath.Join(env.root, ".env")
	if err := os.WriteFile(envPath, []byte("X=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it = admissionItem(adm, AdmissionCheckDirtyPolicy)
	if adm.AllowImplement || it.OK || it.ReasonCode != core.ReasonUnscopedDirtyState {
		t.Fatalf("admission with dirty .env = %+v item=%+v, want blocked unscoped_dirty_state", adm, it)
	}
}
