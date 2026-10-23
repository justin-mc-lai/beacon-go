// Formal gate evaluation behavior tests (P3a): evaluations persist,
// re-runs are deterministic, the implement gate is the admission
// judgement, and the tick exit gates consume formal verdicts first.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// gateTestEnv seeds a project whose feature package really exists on
// disk under the project root (the gates read package files).
type gateTestEnv struct {
	testEnv
	ctx store.FeatureContext
}

const frozenTruthDoc = "---\nslug: feat-a\nversion: v1.6.19\nstatus: frozen\nlanguage: en\nparser_contract: beacon-feature-package-v2\ncanonical_refs:\n  truth: truth.md\n  tests: tests.md\n  tasks: tasks.md\n  evidence: evidence.md\n---\n# Truth\n\n- **AC-001**：the one requirement\n"

const frozenTestsDoc = "---\nslug: feat-a\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-001 | TC-001 | unit | `go test ./...` | passes |\n"

func newGateEnv(t *testing.T, truthDoc string) gateTestEnv {
	t.Helper()
	env := newTestEnv(t, "active")
	if err := os.MkdirAll(env.root, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(env.root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", truthDoc)
	write("tests.md", frozenTestsDoc)
	write("tasks.md", "---\nslug: feat-a\n---\n- [ ] the work\n")
	write("evidence.md", "# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| qa | qa_verdict | qa-report.md |\n\n(no artifacts referenced yet)\n")
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	return gateTestEnv{testEnv: env, ctx: ctx}
}

func TestEvaluateGateRequiresEvaluator(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	if _, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "  "); err == nil {
		t.Fatal("empty evaluator must be refused")
	} else if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonGateEvaluatorRequired {
		t.Fatalf("want gate_evaluator_required, got %v", err)
	}
	if _, err := env.eng.EvaluateGate(env.ctx, "nonsense", "", "qa-bot"); err == nil {
		t.Fatal("unknown gate type must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonGateTypeUnknown {
		t.Fatalf("want gate_type_unknown, got %v", err)
	}
	// truth_integrity does not run through EvaluateGate.
	if _, err := env.eng.EvaluateGate(env.ctx, "truth_integrity", "", "qa-bot"); err == nil {
		t.Fatal("truth_integrity must be refused by EvaluateGate")
	}
}

func TestTruthFreezeGatePassWriteBackAndDeterminism(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	ev, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass || ev.GateResultID == "" {
		t.Fatalf("frozen package must pass and persist, got %+v", ev)
	}
	// Spine/LatestByType read the written verdict back.
	latest, err := env.st.Gates().Latest(env.feature.ID, "truth_freeze")
	if err != nil || latest.Verdict != "pass" || latest.ID != ev.GateResultID {
		t.Fatalf("Latest must read the evaluation back, got %+v err=%v", latest, err)
	}
	// The trail carries gate_evaluated with the evaluator identity.
	events, err := env.st.ListEvents(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.EventType != EventGateEvaluated {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(e.Payload, &p)
		if p["evaluator"] == "qa-bot" && p["gate_type"] == "truth_freeze" {
			found = true
		}
	}
	if !found {
		t.Fatal("gate_evaluated event with evaluator missing")
	}
	// Re-run: same verdict and codes, a second ledger row.
	ev2, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev2.Verdict != ev.Verdict || strings.Join(ev2.ReasonCodes, ",") != strings.Join(ev.ReasonCodes, ",") {
		t.Fatalf("re-evaluation must be deterministic: %+v vs %+v", ev, ev2)
	}
	rows, err := env.st.Gates().ListByFeature(env.feature.ID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("two evaluations must be two rows, got %d err=%v", len(rows), err)
	}
}

func TestTruthFreezeGateBlockedDraft(t *testing.T) {
	draft := strings.Replace(frozenTruthDoc, "status: frozen", "status: draft", 1)
	env := newGateEnv(t, draft)
	ev, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("draft truth must block, got %+v", ev)
	}
	if !strings.Contains(strings.Join(ev.ReasonCodes, ","), core.ReasonFreezeAckRequired) {
		t.Fatalf("draft block must carry freeze_ack_required, got %v", ev.ReasonCodes)
	}
}

func TestDesignGateFlow(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-a", UXRequired: true, ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "design", run.RunID, "design-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || ev.ReasonCodes[0] != core.ReasonDesignPrimitiveNotBound {
		t.Fatalf("unbound UX run must block design, got %+v", ev)
	}
	if _, err := env.eng.BindDesignPrimitive(run.RunID, DesignBinding{PrimitiveSet: "ds-core", Version: "ds-v1", BoundBy: "lai"}); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "design", run.RunID, "design-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("bound run must pass design, got %+v", ev)
	}
	// A feature-scoped drift event must NOT leak into the design
	// facts (they read the RUN trail only).
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID,
		Payload:   json.RawMessage(`{"primitive_set":"ds-core","version":"ds-v2"}`),
		EventType: "design_primitive_bound",
	}); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "design", run.RunID, "design-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("feature-scoped event must not affect the run's design gate, got %+v", ev)
	}
	// The same drift on the RUN trail blocks with the mismatch code
	// (snapshot binding version no longer matches the latest
	// binding event).
	run2, _, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.appendEvent(run2, "design_primitive_bound", map[string]any{"primitive_set": "ds-core", "version": "ds-v2"}); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "design", run.RunID, "design-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || !strings.Contains(strings.Join(ev.ReasonCodes, ","), core.ReasonDesignBindingVersionMism) {
		t.Fatalf("run-trail version drift must block with mismatch, got %+v", ev)
	}
}

func TestImplementGateIsAdmission(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	// Freeze formally first (the admission derivation reads the gate).
	if _, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "qa-bot"); err != nil {
		t.Fatal(err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("frozen + bound run must pass implement gate, got %+v", ev)
	}
	// The gate checks ARE the admission items (same names, same count).
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Checks) != len(adm.Items) {
		t.Fatalf("gate checks (%d) must mirror admission items (%d)", len(ev.Checks), len(adm.Items))
	}
	// Pending steering blocks BOTH surfaces with the same headline.
	if _, err := env.eng.Steer(run.RunID, "objective", "change direction"); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || adm.ReasonCode == "" || ev.ReasonCodes[0] != adm.ReasonCode {
		t.Fatalf("gate and admission must agree: gate=%+v admission=%+v", ev, adm)
	}
}

func TestQAGateEvidenceFlow(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("no QA evidence must block, got %+v", ev)
	}
	joined := strings.Join(ev.ReasonCodes, ",")
	if !strings.Contains(joined, core.ReasonStageExitQAEvidenceMiss) || !strings.Contains(joined, core.ReasonQAStageNotExecuted) {
		t.Fatalf("block must name both missing facts, got %v", ev.ReasonCodes)
	}
	// Provide evidence: a real blob + pointer + a qa stage execution.
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	content := []byte("# qa report\nall green\n")
	if err := os.WriteFile(blob, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if _, err := env.st.Artifacts().Add(store.ArtifactRef{
		RunID: run.RunID, Kind: "qa_report", AbsPath: blob,
		ContentHash: hex.EncodeToString(sum[:]), SizeBytes: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.rec.ExecuteStage(context.Background(), RunView{RunID: run.RunID, FeatureID: env.feature.ID}, StageQA); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("complete evidence must pass, got %+v", ev)
	}
	// Tamper with the blob: the same gate now blocks as stale.
	if err := os.WriteFile(blob, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || ev.ReasonCodes[0] != core.ReasonQAArtifactStale {
		t.Fatalf("tampered evidence must block as stale, got %+v", ev)
	}
}

func TestReleaseGateComposition(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	ev, err := env.eng.EvaluateGate(env.ctx, "release", "", "release-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("nothing evaluated must block release, got %+v", ev)
	}
	joined := strings.Join(ev.ReasonCodes, ",")
	if !strings.Contains(joined, core.ReasonGateNotEvaluated) || !strings.Contains(joined, core.ReasonReleaseRequiresHuman) {
		t.Fatalf("release block must itemise missing gates + human, got %v", ev.ReasonCodes)
	}
	// Satisfy the chain: freeze + implement + qa + human confirmation.
	run := env.createRun(t)
	if _, err := env.eng.EvaluateGate(env.ctx, "truth_freeze", "", "release-bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "release-bot"); err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(env.root, "qa-report.md")
	content := []byte("qa evidence")
	if err := os.WriteFile(blob, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	if _, err := env.st.Artifacts().Add(store.ArtifactRef{
		RunID: run.RunID, Kind: "qa_report", AbsPath: blob,
		ContentHash: hex.EncodeToString(sum[:]), SizeBytes: int64(len(content)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.rec.ExecuteStage(context.Background(), RunView{RunID: run.RunID, FeatureID: env.feature.ID}, StageQA); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.EvaluateGate(env.ctx, "qa", "", "release-bot"); err != nil {
		t.Fatal(err)
	}
	runRow, _, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.appendEvent(runRow, "release_confirmed", map[string]any{"by": "lai"}); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "release", "", "release-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("full chain + human confirmation must pass release, got %+v", ev)
	}
}

func TestIntegrityGateBlockVsAudit(t *testing.T) {
	// Fractured evidence: references an artifact nobody registered.
	fractured := func(env gateTestEnv) {
		if err := os.WriteFile(filepath.Join(env.root, "evidence.md"),
			[]byte("# Evidence\n\n| qa | artifacts/qa-report.md |\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Draft package -> default mode is block.
	env := newGateEnv(t, strings.Replace(frozenTruthDoc, "status: frozen", "status: draft", 1))
	fractured(env)
	v, err := env.eng.EvaluateIntegrity(env.ctx, "qa-bot", "")
	if err != nil {
		t.Fatal(err)
	}
	if v.Verdict != string(gate.VerdictBlocked) || v.Mode != gate.IntegrityModeBlock {
		t.Fatalf("draft package fractures must block, got %+v", v)
	}
	kinds := map[string]bool{}
	for _, f := range v.Findings {
		kinds[f.Kind] = true
	}
	if !kinds["evidence_artifact_unregistered"] || !kinds["evidence_artifact_missing"] {
		t.Fatalf("findings must itemise the evidence fracture, got %+v", v.Findings)
	}
	// Frozen package -> default mode is audit_only (AC-101).
	env2 := newGateEnv(t, frozenTruthDoc)
	fractured(env2)
	v2, err := env2.eng.EvaluateIntegrity(env2.ctx, "qa-bot", "")
	if err != nil {
		t.Fatal(err)
	}
	if v2.Verdict != string(gate.VerdictAudit) || v2.Mode != gate.IntegrityModeAuditOnly {
		t.Fatalf("frozen package must default to audit_only, got %+v", v2)
	}
	latest, err := env2.st.Gates().Latest(env2.feature.ID, "truth_integrity")
	if err != nil || latest.Verdict != "audit" {
		t.Fatalf("integrity verdict must persist, got %+v err=%v", latest, err)
	}
	// Explicit block mode overrides the default.
	v3, err := env2.eng.EvaluateIntegrity(env2.ctx, "qa-bot", "block")
	if err != nil {
		t.Fatal(err)
	}
	if v3.Verdict != string(gate.VerdictBlocked) {
		t.Fatalf("explicit block mode must block, got %+v", v3)
	}
}

// Pure exit-gate linkage (item 4): formal verdicts decide qa/release.
func TestExitGateFormalFirst(t *testing.T) {
	// Formal qa block beats the derived artifact-presence pass.
	blocked := EvaluateStageExit(ExitInput{
		Stage: StageQA, ArtifactCount: 5,
		LatestGates: map[string]GateFact{"qa": {Verdict: "blocked", ReasonCodes: []string{core.ReasonQAArtifactStale}}},
	})
	if blocked.Allowed || blocked.Derived || blocked.ReasonCode != core.ReasonQAArtifactStale || blocked.FormalVerdict != "blocked" {
		t.Fatalf("formal qa block must decide, got %+v", blocked)
	}
	// Formal qa pass opens the exit even with zero raw artifacts.
	pass := EvaluateStageExit(ExitInput{
		Stage: StageQA, LatestGates: map[string]GateFact{"qa": {Verdict: "pass"}},
	})
	if !pass.Allowed || pass.Derived || pass.FormalVerdict != "pass" {
		t.Fatalf("formal qa pass must decide, got %+v", pass)
	}
	// No formal verdict: derived judgement, marked Derived.
	derived := EvaluateStageExit(ExitInput{Stage: StageQA, ArtifactCount: 1})
	if !derived.Allowed || !derived.Derived {
		t.Fatalf("artifact presence stays a derived pass, got %+v", derived)
	}
	// Release: formal pass is surfaced but tick still cannot exit —
	// completion is Complete's human act.
	relPass := EvaluateStageExit(ExitInput{
		Stage: StageRelease, LatestGates: map[string]GateFact{"release": {Verdict: "pass"}},
	})
	if relPass.Allowed || relPass.Derived || relPass.ReasonCode != core.ReasonReleaseAwaitingComplete {
		t.Fatalf("formal release pass must surface as awaiting_complete, got %+v", relPass)
	}
	relDerived := EvaluateStageExit(ExitInput{Stage: StageRelease})
	if relDerived.Allowed || !relDerived.Derived || relDerived.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("no formal verdict keeps the derived human block, got %+v", relDerived)
	}
}

// End-to-end tick linkage: a formal qa block stops the tick at qa;
// a formal pass re-arms and releases it.
func TestTickStopsAtFormalQABlock(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	ctx := context.Background()
	// Drive to qa: plan, truth, implement ticks.
	for i := 0; i < 3; i++ {
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if v.Blocked {
			t.Fatalf("tick %d blocked early: %+v", i, v)
		}
	}
	// Seed a FORMAL qa block (as `gate evaluate qa` would write).
	if _, err := env.st.Gates().Record(store.GateResult{
		FeatureID: env.feature.ID, GateType: "qa", Verdict: "blocked",
		ReasonCodes: []string{core.ReasonQAArtifactStale},
	}); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.CurrentStage != StageQA || v.ReasonCode != core.ReasonQAArtifactStale {
		t.Fatalf("tick must stop at qa on the formal block, got %+v", v)
	}
	// Formal pass re-arms the tick (new gate fact changes the key).
	if _, err := env.st.Gates().Record(store.GateResult{
		FeatureID: env.feature.ID, GateType: "qa", Verdict: "pass",
	}); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Blocked || v.CurrentStage != StageRelease {
		t.Fatalf("formal pass must let the tick advance to release, got %+v", v)
	}
}
