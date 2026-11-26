package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

// ---- helpers ----

// deliverParadigmRun walks one run through plan -> release exactly as a
// real delivery does (recording executor, QA artifact, verifier, human
// completion, release verdict) and returns its id. Exported state is
// the caller's choice (B5).
func deliverParadigmRun(t *testing.T, env gateTestEnv) string {
	t.Helper()
	ctx := context.Background()
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "maker", ExecutorMode: ExecutorModeRecording,
	})
	if err != nil {
		t.Fatal(err)
	}
	step := func(want string) {
		t.Helper()
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatalf("tick to %s: %v", want, err)
		}
		if v.Blocked || v.CurrentStage != want {
			t.Fatalf("tick = %+v, want stage %s", v, want)
		}
	}
	step(StageTruth)
	step(StageImplement)
	step(StageQA)
	productReport := prepareProductQA(t, env.testEnv, run.RunID, "independent-verifier")
	qaArtifact := filepath.Join(t.TempDir(), "qa-report.md")
	if err := os.WriteFile(qaArtifact, []byte("# qa report\npass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_evidence", qaArtifact, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	step(StageRelease)
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{productReport}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner"}); err != nil {
		t.Fatal(err)
	}
	if ev, err := env.eng.EvaluateGate(env.ctx, "release", run.RunID, "release-audit"); err != nil || ev.Verdict != "pass" {
		t.Fatalf("release gate = %+v err=%v", ev, err)
	}
	return run.RunID
}

func conformanceItem(v ConformanceView, check string) ConformanceItem {
	for _, it := range v.Items {
		if it.Check == check {
			return it
		}
	}
	return ConformanceItem{}
}

// B1: an external completion is not a stage-gate evaluation. A run that
// only attached a verifier and received an external result must fail
// conformance on gates_evaluated (the r1 false positive).
func TestConformanceRejectsUnevaluatedExternalResult(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "room", ExecutorMode: ExecutorModeExternal,
	})
	if err != nil {
		t.Fatal(err)
	}
	productReport := prepareProductQA(t, env.testEnv, run.RunID, "independent-verifier")
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{productReport}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(t.TempDir(), "lake-report.md")
	if err := os.WriteFile(artifact, []byte("# lake report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ReceiveExternalResult(run.RunID, ExternalResultInput{
		By: "renlink-dev-room", Summary: "lake finished",
		Artifacts: []ArtifactInput{{Kind: "evidence", AbsPath: artifact}},
	}); err != nil {
		t.Fatal(err)
	}
	conf, err := env.eng.Conformance(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if conf.Pass {
		t.Fatalf("zero-evaluation run must not conform: %+v", conf)
	}
	if it := conformanceItem(conf, "gates_evaluated"); it.Status != "fail" {
		t.Fatalf("gates_evaluated must fail, got %+v", conf.Items)
	}
	// The external write-back also must not fake the stage plan done.
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.StagePlan[0].Status == "done" && m.CurrentStageIdx == len(m.StagePlan) {
		t.Fatalf("external result marked unevaluated stages done: %+v", m.StagePlan)
	}
}

// B2: a manual run entering implement passes the SAME AC-142 admission
// bundle as every other executor: the gate result and trail are present.
func TestManualImplementEntryEvaluatesAdmissionBundle(t *testing.T) {
	env, fctx := s2Setup(t, "feat-manual-adm")
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-manual-adm",
		ExecutorID: "room-host", ExecutorMode: ExecutorModeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, want := range []string{StageTruth, StageImplement, StageQA} {
		v, err := env.eng.TickBy(ctx, run.RunID, "room-host")
		if err != nil {
			t.Fatalf("tick to %s: %v", want, err)
		}
		if v.Blocked || v.CurrentStage != want {
			t.Fatalf("manual tick = %+v, want %s", v, want)
		}
	}
	found := false
	gates, err := env.st.Gates().LatestByType(fctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range gates {
		if g.GateType == GateTypeAdmissionBundle && g.Verdict == "pass" {
			found = true
		}
	}
	if !found {
		t.Fatal("manual implement entry left no admission_bundle gate result")
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
		t.Fatal("manual implement entry left no admission_bundle_evaluated event")
	}
}

// B2 negative: the same manual path parks when the bundle forbids entry.
func TestManualImplementEntryBlocksOnAdmission(t *testing.T) {
	env, _ := s2Setup(t, "feat-manual-blk")
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-manual-blk",
		ExecutorID: "room-host", ExecutorMode: ExecutorModeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Break one bundle leg: the evidence map names a file that vanished.
	evidence := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-manual-blk", "evidence.md")
	if err := os.Remove(evidence); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if v, err := env.eng.TickBy(ctx, run.RunID, "room-host"); err != nil || v.CurrentStage != StageTruth {
		t.Fatalf("plan exit = %+v err=%v", v, err)
	}
	if v, err := env.eng.TickBy(ctx, run.RunID, "room-host"); err != nil || v.CurrentStage != StageImplement {
		t.Fatalf("truth exit = %+v err=%v", v, err)
	}
	v, err := env.eng.TickBy(ctx, run.RunID, "room-host")
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.CurrentStage != StageImplement {
		t.Fatalf("manual implement entry must park on a blocked bundle, got %+v", v)
	}
	if v.ReasonCode == "" || v.ReasonCode == core.ReasonManualTickRequired {
		t.Fatalf("park must name the admission reason, got %q", v.ReasonCode)
	}
}

// B3: gate reads are run-scoped. A fresh run reports evaluated=false;
// a sibling run's QA gate pass is not this run's formal verdict, and
// a sibling run's artifact does not open this run's QA exit.
func TestGateFactsAreRunScoped(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	ctx := context.Background()

	// Run A: walk to QA, register its own artifact and QA verdict,
	// then leave QA for release — leaving a feature-level qa pass
	// and artifact a sibling must not inherit.
	a, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "maker-a", ExecutorMode: ExecutorModeRecording,
	})
	if err != nil {
		t.Fatal(err)
	}
	stepA := func(want string) {
		t.Helper()
		v, err := env.eng.Tick(ctx, a.RunID)
		if err != nil || v.CurrentStage != want {
			t.Fatalf("run A tick = %+v err=%v", v, err)
		}
	}
	stepA(StageTruth)
	stepA(StageImplement)
	stepA(StageQA)
	qaArtifact := filepath.Join(t.TempDir(), "qa-a.md")
	if err := os.WriteFile(qaArtifact, []byte("# qa\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(a.RunID, "qa_evidence", qaArtifact, "qa-a"); err != nil {
		t.Fatal(err)
	}
	prepareProductQA(t, env.testEnv, a.RunID, "qa-a")
	stepA(StageRelease)

	// Run B: same feature, new run, staged at QA with none of A's
	// verdicts or artifacts on its own trail.
	b, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "maker-b", ExecutorMode: ExecutorModeRecording,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := env.eng.Status(b.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.StageGate == nil || st.StageGate.Evaluated {
		t.Fatalf("a fresh run must not inherit another run's gate green: %+v", st.StageGate)
	}
	for _, want := range []string{StageTruth, StageImplement, StageQA} {
		v, err := env.eng.Tick(ctx, b.RunID)
		if err != nil || v.CurrentStage != want {
			t.Fatalf("run B tick = %+v err=%v", v, err)
		}
	}
	if st, err := env.eng.Status(b.RunID); err != nil {
		t.Fatal(err)
	} else if st.StageGate == nil || st.StageGate.Evaluated {
		t.Fatalf("run B status inherited a sibling qa verdict: %+v", st.StageGate)
	}
	v, err := env.eng.ExitGate(b.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Allowed || v.ReasonCode != core.ReasonStageExitQAEvidenceMiss {
		t.Fatalf("run B qa exit must not borrow run A's pass/artifact: %+v", v)
	}
	if v.FormalVerdict == "pass" {
		t.Fatalf("run B borrowed run A's qa stage-exit row as a formal pass: %+v", v)
	}
}

// B5: conformance accepts a state_export record on its own (truth Q5),
// and still fails when neither the snapshot nor the record exists.
func TestConformanceStateExportedAcceptsRecordOnly(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	runID := deliverParadigmRun(t, env)
	if _, abs, err := env.eng.ExportProjectState(env.root); err != nil {
		t.Fatal(err)
	} else if err := os.Remove(abs); err != nil {
		t.Fatal(err)
	}
	conf, err := env.eng.Conformance(runID)
	if err != nil {
		t.Fatal(err)
	}
	if it := conformanceItem(conf, "state_exported"); it.Status != "pass" {
		t.Fatalf("state_exported must accept the trail record alone: %+v (%+v)", it, conf.Items)
	}
	if !conf.Pass {
		t.Fatalf("delivered run must conform with a record-only export: %+v", conf)
	}

	// Neither evidence: a fresh project that never exported.
	bare := newGateEnv(t, frozenTruthDoc)
	bareID := deliverParadigmRun(t, bare)
	conf, err = bare.eng.Conformance(bareID)
	if err != nil {
		t.Fatal(err)
	}
	if it := conformanceItem(conf, "state_exported"); it.Status != "fail" {
		t.Fatalf("state_exported must fail with neither evidence: %+v", it)
	}
	if conf.Pass {
		t.Fatalf("a run without exported state must not conform: %+v", conf)
	}
}

// Record shape guard: the exit evidence carries evaluator, time and the
// gate_result id (B1/B3 trust the stored row, not a bare boolean).
func TestStageExitRecordCarriesAttribution(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "maker", ExecutorMode: ExecutorModeRecording,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Tick(context.Background(), run.RunID); err != nil {
		t.Fatal(err)
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType != "stage_exit_evaluated" {
			continue
		}
		var p struct {
			Evaluator    string `json:"evaluator"`
			EvaluatedAt  string `json:"evaluated_at"`
			GateResultID string `json:"gate_result_id"`
		}
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p.Evaluator == "" || p.EvaluatedAt == "" || p.GateResultID == "" {
			t.Fatalf("exit record lacks attribution: %+v", p)
		}
		if g, err := env.st.Gates().GetByID(p.GateResultID); err != nil || g.Verdict != "pass" {
			t.Fatalf("gate result pointer must resolve to a pass: %+v err=%v", g, err)
		}
		return
	}
	t.Fatal("no stage_exit_evaluated event")
}
