package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/gate"
)

// AC-008 / TC-012: one delivery end to end. A run is created and walks
// plan -> truth -> design -> implement -> qa -> release; the plan exit
// record plus the truth_freeze / design / implement / qa / release gate
// verdicts all exist with evaluated=true, and conformance recognises
// the delivery — then the same judgement demonstrably fails when a
// required fact is missing.
func TestDeliveryParadigmEndToEnd(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	ctx := context.Background()
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", UXRequired: true,
		ExecutorID: "maker", ExecutorMode: ExecutorModeRecording,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.BindDesignPrimitive(run.RunID, DesignBinding{
		PrimitiveSet: "beacon-design", Version: "1.0.0", SourceRef: "docs/design.md", BoundBy: "designer",
	}); err != nil {
		t.Fatal(err)
	}

	// plan -> truth
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
	step(StageDesign)

	// The design gate is a real evaluation over the bound primitives.
	ev, err := env.eng.EvaluateGate(env.ctx, string(gate.GateDesign), run.RunID, "designer")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("design gate = %+v, want pass", ev)
	}
	step(StageImplement)
	step(StageQA)

	// QA needs its evidence pointer on record before the stage exits.
	qaArtifact := filepath.Join(t.TempDir(), "qa-report.md")
	if err := os.WriteFile(qaArtifact, []byte("# qa report\nall suites pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_evidence", qaArtifact, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	step(StageRelease)

	// Release is the human gate: verifier attached, human confirmation,
	// and then the release verdict itself is recorded.
	verifierRef := filepath.Join(t.TempDir(), "verifier-report.md")
	if err := os.WriteFile(verifierRef, []byte("# independent verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{verifierRef}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner"}); err != nil {
		t.Fatalf("complete: %v", err)
	}
	relEv, err := env.eng.EvaluateGate(env.ctx, string(gate.GateRelease), run.RunID, "release-audit")
	if err != nil {
		t.Fatal(err)
	}
	if relEv.Verdict != gate.VerdictPass {
		t.Fatalf("release gate = %+v, want pass", relEv)
	}

	for _, gt := range []string{"plan", "truth_freeze", "design", "implement", "qa", "release"} {
		g, err := env.st.Gates().Latest(env.feature.ID, gt)
		if err != nil {
			t.Fatalf("gate %s has no recorded verdict: %v", gt, err)
		}
		if g.Verdict != "pass" {
			t.Fatalf("gate %s verdict = %s, want pass", gt, g.Verdict)
		}
	}

	// The plan exit is its own recorded judgement, not a bare advance.
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	planExit := false
	for _, e := range events {
		if e.EventType != "stage_exit_evaluated" {
			continue
		}
		var p struct {
			Stage    string `json:"stage"`
			GateType string `json:"gate_type"`
		}
		if len(e.Payload) > 0 {
			_ = json.Unmarshal(e.Payload, &p)
		}
		if p.Stage == StagePlan && p.GateType == "plan" {
			planExit = true
		}
	}
	if !planExit {
		t.Fatal("plan exit record missing from the trail")
	}

	// Conformance: exported state makes the four facts hold.
	if _, _, err := env.eng.ExportProjectState(env.root); err != nil {
		t.Fatal(err)
	}
	conf, err := env.eng.Conformance(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !conf.Pass {
		t.Fatalf("conformance must pass for the delivered run: %+v", conf)
	}

	// Negative proof: drop the verifier and conformance must fail,
	// naming the missing item.
	rraw, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Verifier = &VerifierInfo{RequiredForComplete: true, Present: false}
	if err := env.eng.saveRun(rraw, m); err != nil {
		t.Fatal(err)
	}
	conf, err = env.eng.Conformance(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if conf.Pass {
		t.Fatal("conformance must fail without a verifier")
	}
	failedVerifier := false
	for _, f := range conf.Failed {
		if f == "verifier_attached" {
			failedVerifier = true
		}
	}
	if !failedVerifier {
		t.Fatalf("failed items must name verifier_attached: %+v", conf.Failed)
	}
}
