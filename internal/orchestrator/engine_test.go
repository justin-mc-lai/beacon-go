package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// testEnv seeds one project/version/feature in a temp global DB.
type testEnv struct {
	st      *store.Store
	eng     *Engine
	rec     *RecordingExecutor
	feature store.FeaturePackage
	root    string
}

func newTestEnv(t *testing.T, featureStatus string) testEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(t.TempDir(), "proj")
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	status := featureStatus
	f, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md", Status: status,
	})
	if err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(st)
	rec := NewRecordingExecutor(st, "rec-1")
	eng.RegisterExecutorForAllStages(rec)
	return testEnv{st: st, eng: eng, rec: rec, feature: f, root: root}
}

func (env testEnv) createRun(t *testing.T) RunView {
	t.Helper()
	v, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "build it", ExecutorID: "host-1",
	})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return v
}

func (env testEnv) seedQAPass(t *testing.T) {
	t.Helper()
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "qa", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
}

func (env testEnv) seedTruthFrozen(t *testing.T) {
	t.Helper()
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
}

// CreateRun: stage order without UX, loop parked at governance_precheck,
// first stage active with its progressive skills.
func TestCreateRunStageOrder(t *testing.T) {
	env := newTestEnv(t, "active")
	v := env.createRun(t)
	if v.LoopStage != string(core.LoopGovernancePrecheck) {
		t.Errorf("loop stage = %s", v.LoopStage)
	}
	if v.CurrentStage != StagePlan || len(v.StagePlan) != 5 {
		t.Fatalf("plan = %+v", v.StagePlan)
	}
	want := []string{StagePlan, StageTruth, StageImplement, StageQA, StageRelease}
	for i, s := range want {
		if v.StagePlan[i].Stage != s {
			t.Errorf("stage[%d] = %s want %s", i, v.StagePlan[i].Stage, s)
		}
	}
	if v.StagePlan[0].Status != "active" || v.StagePlan[0].Skills[0] != "beacon-plan" {
		t.Errorf("first stage = %+v", v.StagePlan[0])
	}
	// UX inserts design after truth.
	v2, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-a", UXRequired: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(v2.StagePlan) != 6 || v2.StagePlan[2].Stage != StageDesign {
		t.Errorf("ux plan = %+v", v2.StagePlan)
	}
}

// Tick idempotency (AC-143): a blocked tick consumes the state; repeating
// it changes nothing (no extra executor call, no streak growth).
func TestTickIdempotentWhenBlocked(t *testing.T) {
	env := newTestEnv(t, "draft") // truth stays unfrozen
	run := env.createRun(t)
	ctx := context.Background()
	// plan ticks through.
	v, err := env.eng.Tick(ctx, run.RunID)
	if err != nil || v.CurrentStage != StageTruth {
		t.Fatalf("tick plan: %+v %v", v, err)
	}
	// truth exit gate blocks (draft, not frozen).
	v, err = env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != ReasonStageExitTruthNotFroz {
		t.Fatalf("truth tick = %+v want blocked/%s", v, ReasonStageExitTruthNotFroz)
	}
	calls := env.rec.CallCount()
	streak := v.BlockedStreak
	// Repeat: up_to_date, no re-execution, no new block record.
	v2, err := env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v2.UpToDate {
		t.Errorf("repeat tick not up_to_date: %+v", v2)
	}
	if env.rec.CallCount() != calls {
		t.Errorf("executor re-ran on idempotent tick")
	}
	if v2.BlockedStreak != streak {
		t.Errorf("streak grew on idempotent tick: %d -> %d", streak, v2.BlockedStreak)
	}
}

// Exit gate not passed -> tick blocked, reason persisted in lifecycle
// snapshot reason_codes.
func TestTickBlockedLeavesLifecycleReason(t *testing.T) {
	env := newTestEnv(t, "draft")
	run := env.createRun(t)
	ctx := context.Background()
	if _, err := env.eng.Tick(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Tick(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
	lc, err := env.st.Features().GetLifecycle(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Status != core.StatusBlocked || len(lc.ReasonCodes) != 1 || lc.ReasonCodes[0] != ReasonStageExitTruthNotFroz {
		t.Errorf("lifecycle = %+v", lc)
	}
}

// No executor registered -> blocked with stage_executor_not_registered,
// never a fake pass.
func TestTickWithoutExecutorBlocks(t *testing.T) {
	env := newTestEnv(t, "active")
	bare := NewEngine(env.st) // no executors
	v, err := bare.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-a"})
	if err != nil {
		t.Fatal(err)
	}
	v, err = bare.Tick(context.Background(), v.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != ReasonExecutorNotReg {
		t.Errorf("tick without executor = %+v", v)
	}
}

// Repair loop: implement failures 1..3 block for repair; the 4th routes
// the run to error_recovery (MAX_REPAIR_LOOPS=3).
func TestRepairLoopCeilingRoutesToErrorRecovery(t *testing.T) {
	env := newTestEnv(t, "active")
	env.rec.FailStages[StageImplement] = "compile_failed"
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	ctx := context.Background()
	// plan, truth advance.
	for i := 0; i < 2; i++ {
		if _, err := env.eng.Tick(ctx, run.RunID); err != nil {
			t.Fatal(err)
		}
	}
	// A failed execution is retryable by ticking again (the failure does
	// not consume the tick fingerprint); each retry counts one repair.
	for attempt := 1; attempt <= 3; attempt++ {
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Blocked || v.RepairLoopCount != attempt {
			t.Fatalf("attempt %d: %+v", attempt, v)
		}
		if v.LoopStage == string(core.LoopErrorRecovery) {
			t.Fatalf("attempt %d routed to error_recovery too early", attempt)
		}
	}
	// The 4th failure exceeds MAX_REPAIR_LOOPS=3.
	v, err := env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.LoopStage != string(core.LoopErrorRecovery) || v.Status != "error_recovery" {
		t.Fatalf("after ceiling: %+v", v)
	}
	if v.ReasonCode != core.ReasonMaxRepairLoopsExceeded {
		t.Errorf("reason = %q", v.ReasonCode)
	}
}

// Complete rejections: (1) not at release_gate, (2) at release_gate
// without human confirmation, (3) confirmed but no verifier,
// (4) verifier attached but qa gate not pass. Then all four satisfied
// -> completed.
func TestCompleteFourGates(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	ctx := context.Background()

	// (1) not at release_gate.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true}); err == nil {
		t.Fatal("complete before release_gate must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonNotAtReleaseGate {
		t.Errorf("reason = %q", rc)
	}
	// Walk to release_gate: plan, truth, implement, qa(seeded pass).
	env.seedQAPass(t)
	for i := 0; i < 4; i++ {
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if v.Blocked {
			t.Fatalf("tick %d blocked: %+v", i, v)
		}
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.LoopStage != string(core.LoopReleaseGate) {
		t.Fatalf("status at release: %+v %v", st, err)
	}
	// Tick at release is blocked by the human gate (never auto-exits).
	v, err := env.eng.Tick(ctx, run.RunID)
	if err != nil || !v.Blocked || v.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("release tick = %+v %v", v, err)
	}
	// (2) no human confirmation.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{}); err == nil {
		t.Fatal("complete without human confirmation must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleaseRequiresHuman {
		t.Errorf("reason = %q", rc)
	}
	// (3) confirmed, no verifier.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "lai"}); err == nil {
		t.Fatal("complete without verifier must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonVerifierRequired {
		t.Errorf("reason = %q", rc)
	}
	// Attach verifier, then remove the qa pass by recording a newer
	// blocked qa verdict -> (4) qa not pass.
	if _, err := env.eng.AttachVerifier(run.RunID, []string{"evidence/qa.md"}, "qa-bot", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"regression"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{}); err == nil {
		t.Fatal("complete with qa blocked must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonQAGateNotPassed {
		t.Errorf("reason = %q", rc)
	}
	// All four satisfied.
	env.seedQAPass(t)
	v, err = env.eng.Complete(run.RunID, CompleteOptions{})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if v.Status != "completed" || v.LoopStage != string(core.LoopDone) {
		t.Errorf("completed view = %+v", v)
	}
	row, err := env.st.Runs().GetByID(run.RunID)
	if err != nil || row.Status != core.RunSucceeded || !row.EndedAt.Valid {
		t.Errorf("run row = %+v %v", row, err)
	}
}

// Maker != checker: verifier source equal to the executor identity is
// rejected with stop_when_unverified.
func TestAttachVerifierMakerChecker(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t) // ExecutorID = host-1
	if _, err := env.eng.AttachVerifier(run.RunID, []string{"e.md"}, "host-1", ""); err == nil {
		t.Fatal("verifier == executor must be rejected")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonStopWhenUnverified {
		t.Errorf("reason = %q", rc)
	}
	v, err := env.eng.AttachVerifier(run.RunID, []string{"e.md"}, "qa-bot", "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Verifier == nil || !v.Verifier.Present || v.Verifier.Source != "qa-bot" {
		t.Errorf("verifier = %+v", v.Verifier)
	}
	// Empty refs rejected.
	if _, err := env.eng.AttachVerifier(run.RunID, nil, "qa-bot", ""); err == nil {
		t.Error("empty refs must be rejected")
	}
}

// Checkpoint -> resume returns the current stage and load_next_skill;
// after a host interrupt, resume returns replay_substep (AC-034/035).
func TestCheckpointResumeAndInterrupt(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.Heartbeat(run.RunID, "working", "plan:draft"); err != nil {
		t.Fatal(err)
	}
	ckpt, _, err := env.eng.Checkpoint(run.RunID, "load plan skill", "plan:draft")
	if err != nil {
		t.Fatal(err)
	}
	if ckpt.Iteration != 1 {
		t.Errorf("checkpoint iteration = %d", ckpt.Iteration)
	}
	ckpt2, _, err := env.eng.Checkpoint(run.RunID, "next", "")
	if err != nil || ckpt2.Iteration != 2 {
		t.Errorf("checkpoint 2 = %+v %v", ckpt2, err)
	}
	rv, err := env.eng.Resume(run.RunID, "继续")
	if err != nil {
		t.Fatal(err)
	}
	if rv.CurrentStage != StagePlan || rv.ResumeAction != core.ResumeLoadNextSkill || rv.NextSkill != "beacon-plan" {
		t.Errorf("resume = %+v", rv)
	}
	// Interrupt: next resume replays the substep, once.
	if _, err := env.eng.RecordInterrupt(run.RunID, "plan:draft"); err != nil {
		t.Fatal(err)
	}
	rv, err = env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatal(err)
	}
	if rv.ResumeAction != core.ResumeReplaySubstep {
		t.Errorf("resume after interrupt = %+v", rv)
	}
	rv, err = env.eng.Resume(run.RunID, "go")
	if err != nil {
		t.Fatal(err)
	}
	if rv.ResumeAction != core.ResumeLoadNextSkill {
		t.Errorf("second resume after interrupt consumed = %+v", rv)
	}
}

// Trace aggregates the run's events in append order.
func TestTraceOrderAndBundle(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	ctx := context.Background()
	if _, err := env.eng.Tick(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	bundle, err := env.eng.Trace(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.RunID != run.RunID || bundle.EventCount != len(bundle.Events) || bundle.EventCount < 4 {
		t.Fatalf("bundle = %+v", bundle)
	}
	types := []string{}
	for _, ev := range bundle.Events {
		types = append(types, ev.EventType)
	}
	wantPrefix := []string{"goal_run_created", "stage_executed", "stage_advanced", "goal_blocked"}
	for i, w := range wantPrefix {
		if types[i] != w {
			t.Errorf("event[%d] = %s want %s (all: %v)", i, types[i], w, types)
		}
	}
	for i := 1; i < len(bundle.Events); i++ {
		if bundle.Events[i].CreatedAt < bundle.Events[i-1].CreatedAt {
			t.Errorf("events out of order at %d", i)
		}
	}
	// Unknown run -> typed not-found.
	if _, err := env.eng.Trace("run_nope"); err == nil {
		t.Error("trace of unknown run must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonRunNotFound {
		t.Errorf("reason = %q", rc)
	}
}

// Pure exit-gate branches (evaluate_stage_exit port).
func TestEvaluateStageExitBranches(t *testing.T) {
	cases := []struct {
		name string
		in   ExitInput
		want string // reason code; "" means allowed
	}{
		{"plan ok", ExitInput{Stage: StagePlan, LatestGates: map[string]GateFact{}}, ""},
		{"plan vetoed", ExitInput{Stage: StagePlan, LatestGates: map[string]GateFact{"plan_to_truth": {Verdict: "blocked", ReasonCodes: []string{"plan_gap"}}}}, "plan_gap"},
		{"truth no feature skips", ExitInput{Stage: StageTruth, HasFeature: false}, ""},
		{"truth frozen", ExitInput{Stage: StageTruth, HasFeature: true, TruthFrozen: true, TruthPackagePresent: true}, ""},
		{"truth missing", ExitInput{Stage: StageTruth, HasFeature: true}, ReasonStageExitTruthMissing},
		{"truth draft", ExitInput{Stage: StageTruth, HasFeature: true, TruthPackagePresent: true, TruthDraftPresent: true}, ReasonStageExitTruthNotFroz},
		{"truth ack", ExitInput{Stage: StageTruth, HasFeature: true, TruthPackagePresent: true}, ReasonFreezeAckRequired},
		{"design missing", ExitInput{Stage: StageDesign}, ReasonStageExitDesignIncompl},
		{"design ok", ExitInput{Stage: StageDesign, DesignEvidence: true}, ""},
		{"implement admission blocked", ExitInput{Stage: StageImplement, LatestGates: map[string]GateFact{"implement_admission": {Verdict: "blocked"}}}, ReasonStageExitImplementBlk},
		{"qa no evidence", ExitInput{Stage: StageQA, LatestGates: map[string]GateFact{}}, ReasonStageExitQAEvidence},
		{"qa artifacts", ExitInput{Stage: StageQA, ArtifactCount: 2, LatestGates: map[string]GateFact{}}, ""},
		{"qa gate pass", ExitInput{Stage: StageQA, LatestGates: map[string]GateFact{"qa": {Verdict: "pass"}}}, ""},
		{"release always blocked", ExitInput{Stage: StageRelease}, core.ReasonReleaseRequiresHuman},
	}
	for _, c := range cases {
		v := EvaluateStageExit(c.in)
		if c.want == "" && !v.Allowed {
			t.Errorf("%s: blocked with %q, want allowed", c.name, v.ReasonCode)
		}
		if c.want != "" && (v.Allowed || v.ReasonCode != c.want) {
			t.Errorf("%s: verdict = %+v want reason %q", c.name, v, c.want)
		}
	}
}
