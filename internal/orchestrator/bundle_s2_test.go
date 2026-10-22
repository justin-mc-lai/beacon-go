// Standard flow S2 tests (AC-142 + shared entry judgement):
// the admission evidence bundle's five legs, the tick/delegate
// shared verdict and the non-git caliber — real git repositories.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func bundleItem(res BundleResult, check string) BundleItem {
	for _, it := range res.Items {
		if it.Check == check {
			return it
		}
	}
	return BundleItem{}
}

// ---- AC-142: the bundle ----

func TestS2BundlePassesAndPersists(t *testing.T) {
	env, _ := s2Setup(t, "feat-bnd")
	run := env.s2Run(t, "feat-bnd")
	res, err := env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AllowImplement || res.Status != "pass" || len(res.Items) != 5 {
		t.Fatalf("bundle = %+v, want pass with 5 items", res)
	}
	for _, it := range res.Items {
		if !it.OK {
			t.Fatalf("bundle item %s not ok: %+v", it.Check, it)
		}
	}
	if res.BaselineCommit == "" || res.GateResultID == "" {
		t.Fatalf("bundle identity fields missing: %+v", res)
	}
	// Persisted as gate_results gate_type=admission_bundle + the
	// admission_bundle_evaluated trail event.
	gates, err := env.st.Gates().LatestByType(mustFeatID(t, env, run.RunID))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range gates {
		if g.GateType == GateTypeAdmissionBundle && g.Verdict == "pass" {
			found = true
		}
	}
	if !found {
		t.Fatalf("gate_results lacks the admission_bundle pass row: %+v", gates)
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	evFound := false
	for _, ev := range events {
		if ev.EventType == EventAdmissionBundleEvaluated {
			evFound = true
		}
	}
	if !evFound {
		t.Fatal("no admission_bundle_evaluated event on the run trail")
	}
}

func mustFeatID(t *testing.T, env s1GitEnv, runID string) string {
	t.Helper()
	run := s2RunRow(t, env, runID)
	return run.FeatureID
}

func TestS2BundleEachLegBlocks(t *testing.T) {
	// (a) truth-map leg: a registered member vanishes from disk.
	env, _ := s2Setup(t, "feat-leg-a")
	run := env.s2Run(t, "feat-leg-a")
	evPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-leg-a", "evidence.md")
	if err := os.Remove(evPath); err != nil {
		t.Fatal(err)
	}
	res, err := env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := bundleItem(res, BundleTruthMap); it.OK || res.AllowImplement ||
		res.ReasonCode != core.ReasonStandardFlowVerificationIncm {
		t.Fatalf("(a) bundle = %+v truth-map=%+v", res, it)
	}

	// (b) verify-materials leg: tests references an unknown AC.
	env, _ = s2Setup(t, "feat-leg-b")
	run = env.s2Run(t, "feat-leg-b")
	testsPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-leg-b", "tests.md")
	raw := readFile(t, testsPath)
	if err := os.WriteFile(testsPath, []byte(raw+"\n| AC-999 | TC-999 | behavior | `go test ./...` | holds |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := bundleItem(res, BundleMaterials); it.OK || res.AllowImplement {
		t.Fatalf("(b) bundle = %+v materials=%+v", res, it)
	}

	// (c) git diff leg: trailing whitespace in a tracked ordinary
	// file fails `git diff --check` — and ONLY that leg fails.
	env, _ = s2Setup(t, "feat-leg-c")
	run = env.s2Run(t, "feat-leg-c")
	readme := filepath.Join(env.root, "README.md")
	raw = readFile(t, readme)
	if err := os.WriteFile(readme, []byte(raw+"trailing space here \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := bundleItem(res, BundleGitDiff); it.OK || it.ReasonCode != core.ReasonGitDiffCheckFailed || res.AllowImplement {
		t.Fatalf("(c) bundle = %+v diff=%+v", res, it)
	}
	for _, check := range []string{BundleTruthMap, BundleMaterials, BundleBranchGov, BundleWorkspaceAdmin} {
		if it := bundleItem(res, check); !it.OK {
			t.Fatalf("(c) leg %s must stay green in the diff-only scenario: %+v", check, it)
		}
	}

	// (d) branch-governance leg: satellite HEAD lacks the baseline.
	env, _ = s2Setup(t, "feat-leg-d")
	run = env.s2Run(t, "feat-leg-d")
	initHead := strings.TrimSpace(gitCmd(t, env.root, "rev-list", "--max-parents=0", "HEAD"))
	wtPath := filepath.Join(env.root, ".beacon-worktrees", "feat-leg-d")
	gitCmd(t, env.root, "worktree", "add", "-b", "feat-dev-d", wtPath, initHead)
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	m.Implement.Worktrees = &WorktreeState{Records: []WorktreeRecord{{
		Lake: "feat-leg-d", Branch: "feat-dev-d", Path: wtPath,
		Ref: ".beacon/worktrees/satellite/feat-leg-d", State: WorktreeActive, CreatedAt: nowISO(),
	}}}
	if err := env.eng.saveRun(s2RunRow(t, env, run.RunID), m); err != nil {
		t.Fatal(err)
	}
	res, err = env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it := bundleItem(res, BundleBranchGov)
	if it.OK || res.AllowImplement {
		t.Fatalf("(d) bundle = %+v branch-gov=%+v", res, it)
	}
	foundCode := false
	for _, c := range it.ReasonCodes {
		if c == core.ReasonTruthBaselineNotMerged {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatalf("(d) branch-gov codes = %v, want truth_baseline_not_merged", it.ReasonCodes)
	}

	// (e) workspace-admission leg alone: an unacknowledged steering
	// blocks ONLY item 5; the headline stays the admission code,
	// not the bundle structural code.
	env, _ = s2Setup(t, "feat-leg-e")
	run = env.s2Run(t, "feat-leg-e")
	if _, err := env.eng.Steer(run.RunID, "objective", "change objective"); err != nil {
		t.Fatal(err)
	}
	res, err = env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := bundleItem(res, BundleWorkspaceAdmin); it.OK || res.AllowImplement {
		t.Fatalf("(e) bundle = %+v ws=%+v", res, it)
	}
	if res.ReasonCode != core.ReasonSteeringUnacknowledged {
		t.Fatalf("(e) headline = %s, want the admission headline %s", res.ReasonCode, core.ReasonSteeringUnacknowledged)
	}
	for _, check := range []string{BundleTruthMap, BundleMaterials, BundleGitDiff, BundleBranchGov} {
		if it := bundleItem(res, check); !it.OK {
			t.Fatalf("(e) leg %s must stay green in the steering scenario: %+v", check, it)
		}
	}
}

// Tick and delegate judge by the SAME bundle: a structurally
// incomplete bundle blocks both with the same headline code.
func TestS2TickAndDelegateShareBundleJudgement(t *testing.T) {
	env, _ := s2Setup(t, "feat-share")
	env.eng.RegisterExecutorForAllStages(NewRecordingExecutor(env.st, "rec-s2"))
	env.eng.RegisterLaneExecutorForAllLanes(NewRecordingExecutor(env.st, "rec-s2"))
	ctx := context.Background()
	mkRunAtImplement := func() RunView {
		run := env.s2Run(t, "feat-share")
		for i := 0; i < 2; i++ {
			v, err := env.eng.Tick(ctx, run.RunID)
			if err != nil {
				t.Fatal(err)
			}
			run = v
		}
		if run.CurrentStage != StageImplement {
			t.Fatalf("stage = %s, want implement", run.CurrentStage)
		}
		return run
	}
	// Break the bundle structurally (evidence member missing).
	evPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-share", "evidence.md")
	saved := readFile(t, evPath)
	if err := os.Remove(evPath); err != nil {
		t.Fatal(err)
	}
	runTick := mkRunAtImplement()
	v, err := env.eng.Tick(ctx, runTick.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonStandardFlowVerificationIncm {
		t.Fatalf("tick under broken bundle = %+v, want blocked standard_flow_verification_incomplete", v)
	}
	runDlg := mkRunAtImplement()
	_, err = env.eng.DelegateRalph(ctx, runDlg.RunID, false)
	if rc := s1Reason(t, err); rc != core.ReasonStandardFlowVerificationIncm {
		t.Fatalf("delegate under broken bundle reason = %s (%v)", rc, err)
	}
	// Repair the package: the tick path re-judges (a fresh bundle
	// evaluation re-arms the tick key) and implement executes.
	if err := os.WriteFile(evPath, []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.Tick(ctx, runTick.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Blocked || v.CurrentStage != StageQA {
		t.Fatalf("tick after repair = %+v, want advanced to qa", v)
	}
}

// Non-git projects are never painted red by the git-only legs.
func TestS2NonGitBundleNotRed(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-ng2")
	ctx := env.ctx(t, "feat-ng2")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-ng2", Utterance: "build", ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.AllowImplement {
		t.Fatalf("non-git bundle must pass: %+v", res)
	}
	if it := bundleItem(res, BundleGitDiff); it.Applicable {
		t.Fatalf("git diff leg must be not_applicable in a non-git project: %+v", it)
	}
	if it := bundleItem(res, BundleBranchGov); it.Applicable {
		t.Fatalf("branch-governance leg must be not_applicable in a non-git project: %+v", it)
	}
	adm := res.Admission
	if it := admissionItem(adm, AdmissionCheckBaselineMerged); it.Applicable || !it.OK {
		t.Fatalf("baseline_merged must be n/a-ok in non-git: %+v", it)
	}
	if it := admissionItem(adm, AdmissionCheckDirtyPolicy); it.Applicable || !it.OK {
		t.Fatalf("dirty_policy must be n/a-ok in non-git: %+v", it)
	}
}
