// Pure evaluator + integrity tests (P3a): no store, no filesystem —
// facts and contents are injected.
package gate

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func codes(ev Evaluation) []string { return ev.ReasonCodes }

func TestTruthFreezePassAndBlock(t *testing.T) {
	pass := EvaluateTruthFreeze(TruthFreezeFacts{
		TruthPresent: true, TruthNonEmpty: true, TestsPresent: true, TestsNonEmpty: true,
		FrozenMarker: true,
	}, "qa-bot")
	if pass.Verdict != VerdictPass || len(pass.ReasonCodes) != 0 {
		t.Fatalf("want pass, got %+v", pass)
	}
	blocked := EvaluateTruthFreeze(TruthFreezeFacts{
		TruthPresent: true, TruthNonEmpty: true, TestsPresent: true, TestsNonEmpty: true,
		FrozenMarker: false, PendingChange: true,
	}, "qa-bot")
	if blocked.Verdict != VerdictBlocked {
		t.Fatalf("want blocked, got %+v", blocked)
	}
	joined := strings.Join(codes(blocked), ",")
	if !strings.Contains(joined, core.ReasonFreezeAckRequired) || !strings.Contains(joined, core.ReasonTruthChangePending) {
		t.Fatalf("blocked codes must name both failures item by item, got %v", codes(blocked))
	}
	missing := EvaluateTruthFreeze(TruthFreezeFacts{}, "qa-bot")
	if missing.Verdict != VerdictBlocked || codes(missing)[0] != core.ReasonStageExitTruthMissing {
		t.Fatalf("missing package must block with the missing code first, got %+v", missing)
	}
}

func TestDesignGate(t *testing.T) {
	na := EvaluateDesign(DesignFacts{HasRun: true, Required: false}, "qa-bot")
	if na.Verdict != VerdictPass {
		t.Fatalf("non-UX run: design gate must pass as n/a, got %+v", na)
	}
	noRun := EvaluateDesign(DesignFacts{HasRun: false, Required: true}, "qa-bot")
	if noRun.Verdict != VerdictBlocked || codes(noRun)[0] != core.ReasonGateRunRequired {
		t.Fatalf("no run must block with gate_run_required, got %+v", noRun)
	}
	unbound := EvaluateDesign(DesignFacts{HasRun: true, Required: true}, "qa-bot")
	if unbound.Verdict != VerdictBlocked || codes(unbound)[0] != core.ReasonDesignPrimitiveNotBound {
		t.Fatalf("unbound must block with design_primitive_not_bound, got %+v", unbound)
	}
	mismatch := EvaluateDesign(DesignFacts{
		HasRun: true, Required: true, Bound: true, BindingSet: "ds", BindingVersion: "v2",
		HasBoundEvent: true, EventVersion: "v1",
	}, "qa-bot")
	if mismatch.Verdict != VerdictBlocked || codes(mismatch)[0] != core.ReasonDesignBindingVersionMism {
		t.Fatalf("version drift must block with mismatch, got %+v", mismatch)
	}
	ok := EvaluateDesign(DesignFacts{
		HasRun: true, Required: true, Bound: true, BindingSet: "ds", BindingVersion: "v2",
		HasBoundEvent: true, EventVersion: "v2",
	}, "qa-bot")
	if ok.Verdict != VerdictPass {
		t.Fatalf("bound + matching version must pass, got %+v", ok)
	}
}

func TestImplementGateMirrorsAdmission(t *testing.T) {
	noRun := EvaluateImplement(ImplementFacts{HasRun: false}, "qa-bot")
	if noRun.Verdict != VerdictBlocked || codes(noRun)[0] != core.ReasonGateRunRequired {
		t.Fatalf("no run must block with gate_run_required, got %+v", noRun)
	}
	items := []CheckItem{
		okItem("truth_frozen", "frozen"),
		failItem("steering_ack", core.ReasonSteeringUnacknowledged, "pending steering"),
	}
	blocked := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: false, Items: items}, "qa-bot")
	if blocked.Verdict != VerdictBlocked || codes(blocked)[0] != core.ReasonSteeringUnacknowledged {
		t.Fatalf("admission block must surface its code, got %+v", blocked)
	}
	drift := EvaluateImplement(ImplementFacts{HasRun: true, AllowImplement: false, HeadlineCode: "x_block", Items: []CheckItem{okItem("a", "ok")}}, "qa-bot")
	if drift.Verdict != VerdictBlocked {
		t.Fatalf("headline/items drift must fail closed, got %+v", drift)
	}
}

func TestQAGate(t *testing.T) {
	pass := EvaluateQA(QAFacts{QAArtifactCount: 2, QAStageExecuted: true}, "qa-bot")
	if pass.Verdict != VerdictPass {
		t.Fatalf("complete evidence must pass, got %+v", pass)
	}
	empty := EvaluateQA(QAFacts{}, "qa-bot")
	if empty.Verdict != VerdictBlocked ||
		!strings.Contains(strings.Join(codes(empty), ","), core.ReasonStageExitQAEvidenceMiss) ||
		!strings.Contains(strings.Join(codes(empty), ","), core.ReasonQAStageNotExecuted) {
		t.Fatalf("empty evidence must block with both codes, got %+v", empty)
	}
	stale := EvaluateQA(QAFacts{
		QAArtifactCount: 1, QAStageExecuted: true,
		StaleArtifacts: []StaleRef{{Kind: "qa_report", Path: "/x/qa.md", Reason: "hash_mismatch"}},
	}, "qa-bot")
	if stale.Verdict != VerdictBlocked || codes(stale)[0] != core.ReasonQAArtifactStale {
		t.Fatalf("stale artifact must block, got %+v", stale)
	}
}

func TestReleaseGateComposition(t *testing.T) {
	passFact := GateFact{Present: true, Verdict: "pass"}
	full := ReleaseFacts{
		TruthFreeze: passFact, Design: passFact, Implement: passFact, QA: passFact,
		DesignApplicable: true, HumanConfirmed: true, HumanConfirmedBy: "lai",
	}
	if ev := EvaluateRelease(full, "qa-bot"); ev.Verdict != VerdictPass {
		t.Fatalf("all prereqs pass + human must pass, got %+v", ev)
	}
	// Missing evaluations are named gate by gate.
	none := EvaluateRelease(ReleaseFacts{DesignApplicable: false}, "qa-bot")
	if none.Verdict != VerdictBlocked {
		t.Fatalf("want blocked, got %+v", none)
	}
	joined := strings.Join(codes(none), ",")
	if !strings.Contains(joined, core.ReasonGateNotEvaluated) || !strings.Contains(joined, core.ReasonReleaseRequiresHuman) {
		t.Fatalf("codes must name unevaluated gates + missing human, got %v", codes(none))
	}
	// A blocked prereq is gate_prerequisite_not_passed, not "unevaluated".
	withBlocked := ReleaseFacts{
		TruthFreeze:      GateFact{Present: true, Verdict: "blocked", ReasonCodes: []string{"x"}},
		DesignApplicable: false, Implement: passFact, QA: passFact, HumanConfirmed: true,
	}
	ev := EvaluateRelease(withBlocked, "qa-bot")
	if ev.Verdict != VerdictBlocked || codes(ev)[0] != core.ReasonGatePrereqNotPassed {
		t.Fatalf("blocked prereq must be named as such, got %+v", ev)
	}
}

func TestEvaluationDeterminism(t *testing.T) {
	f := TruthFreezeFacts{TruthPresent: true, PendingChange: true}
	a := EvaluateTruthFreeze(f, "qa-bot")
	b := EvaluateTruthFreeze(f, "qa-bot")
	if a.Verdict != b.Verdict || strings.Join(a.ReasonCodes, ",") != strings.Join(b.ReasonCodes, ",") {
		t.Fatalf("same facts must yield identical verdicts: %+v vs %+v", a, b)
	}
}

// ---- integrity ----

const goodTruth = `---
slug: demo
status: frozen
---
# Truth

- **AC-001**：first requirement
- **AC-002**：second requirement
`

const goodTests = `---
slug: demo
---
| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | ` + "`go test ./...`" + ` | passes |
| AC-002 | TC-002 | unit | ` + "`go test ./...`" + ` | passes |
`

func baseIntegrityInput() IntegrityInput {
	return IntegrityInput{
		Pkg: PackageFiles{
			Slug:  "demo",
			Truth: PackageFile{Path: "features/demo/truth.md", Present: true, Text: goodTruth},
			Tests: PackageFile{Path: "features/demo/tests.md", Present: true, Text: goodTests},
			Tasks: PackageFile{Path: "features/demo/tasks.md", Present: true, Text: "---\nslug: demo\n---\n- [ ] work\n"},
			Evidence: PackageFile{Path: "features/demo/evidence.md", Present: true,
				Text: "# Evidence\n\n| Surface | Artifact |\n|---|---|\n| qa | artifacts/qa-report.md |\n"},
		},
		Artifacts:  []ArtifactFact{{AbsPath: "/proj/artifacts/qa-report.md"}},
		ResolveRef: func(ref string) string { return "/proj/" + ref },
		FileExists: func(abs string) bool { return abs == "/proj/artifacts/qa-report.md" },
	}
}

func TestIntegrityCleanPackagePasses(t *testing.T) {
	rep := CheckIntegrity(baseIntegrityInput(), IntegrityModeBlock)
	if rep.Verdict != VerdictPass || len(rep.Findings) != 0 {
		t.Fatalf("clean package must pass with no findings, got %+v", rep)
	}
}

func TestIntegrityFractures(t *testing.T) {
	in := baseIntegrityInput()
	// tests references an AC truth does not define + one soft TC.
	in.Pkg.Tests.Text = `---
slug: demo
---
| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | ` + "`go test`" + ` | passes |
| AC-999 | TC-999 | unit | ` + "`go test`" + ` | passes |
| AC-002 | TC-002 | unit | integration | 目录存在 |
`
	rep := CheckIntegrity(in, IntegrityModeBlock)
	if rep.Verdict != VerdictBlocked {
		t.Fatalf("want blocked, got %+v", rep)
	}
	kinds := map[string]bool{}
	for _, f := range rep.Findings {
		kinds[f.Kind] = true
	}
	for _, want := range []string{"tests_unknown_ac", "soft_tc"} {
		if !kinds[want] {
			t.Fatalf("missing fracture kind %s in %+v", want, rep.Findings)
		}
	}
	joined := strings.Join(rep.ReasonCodes, ",")
	if !strings.Contains(joined, core.ReasonIntegrityTestsUnknownAC) || !strings.Contains(joined, core.ReasonIntegritySoftTC) {
		t.Fatalf("codes must carry both fractures, got %v", rep.ReasonCodes)
	}
}

func TestIntegrityEvidenceAndSlugFractures(t *testing.T) {
	in := baseIntegrityInput()
	in.Artifacts = nil // nothing registered
	in.FileExists = func(string) bool { return false }
	in.Pkg.Tasks.Text = "---\nslug: other-slug\n---\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	if rep.Verdict != VerdictBlocked {
		t.Fatalf("want blocked, got %+v", rep)
	}
	kinds := map[string]bool{}
	for _, f := range rep.Findings {
		kinds[f.Kind] = true
	}
	for _, want := range []string{"evidence_artifact_unregistered", "evidence_artifact_missing", "slug_mismatch"} {
		if !kinds[want] {
			t.Fatalf("missing fracture kind %s in %+v", want, rep.Findings)
		}
	}
}

func TestIntegrityAuditOnlyDoesNotBlock(t *testing.T) {
	in := baseIntegrityInput()
	in.Pkg.Tasks.Present = false // fracture: missing tasks
	rep := CheckIntegrity(in, IntegrityModeAuditOnly)
	if rep.Verdict != VerdictAudit {
		t.Fatalf("audit_only must report audit verdict, got %+v", rep)
	}
	if len(rep.ReasonCodes) == 0 || rep.ReasonCodes[0] != core.ReasonIntegrityAuditOnly {
		t.Fatalf("audit verdict must carry the audit_only marker first, got %v", rep.ReasonCodes)
	}
	block := CheckIntegrity(in, IntegrityModeBlock)
	if block.Verdict != VerdictBlocked {
		t.Fatalf("same fracture in block mode must block, got %+v", block)
	}
}

func TestIntegrityFakeDeliveryAndDeferral(t *testing.T) {
	in := baseIntegrityInput()
	in.Pkg.Truth.Text = goodTruth + "\n本功能以 placeholder implementation 交付。\n| D-1 | pending | 待拍板 |\n"
	rep := CheckIntegrity(in, IntegrityModeBlock)
	kinds := map[string]bool{}
	for _, f := range rep.Findings {
		kinds[f.Kind] = true
	}
	if !kinds["fake_delivery_placeholder_implementation"] {
		t.Fatalf("fake delivery line must be caught, got %+v", rep.Findings)
	}
	if !kinds["deferral_pending"] && !kinds["fake_delivery_pending_deferral"] {
		t.Fatalf("pending deferral must be caught, got %+v", rep.Findings)
	}
}
