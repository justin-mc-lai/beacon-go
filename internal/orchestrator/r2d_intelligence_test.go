package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// ---- M1: agent-eval line end to end ----

func TestAgentEvalLineGradeScoreAdmit(t *testing.T) {
	root := t.TempDir()
	// A well-behaved agent: three clean transcripts.
	for i := 0; i < 3; i++ {
		rec, err := GradeAgentTranscript(root, "good-bot", "ROUTE: qa\nwork done", "qa", "transcript", "")
		if err != nil {
			t.Fatal(err)
		}
		if !rec.Grade.Passed {
			t.Fatalf("clean transcript must pass: %+v", rec.Grade)
		}
	}
	card, err := BuildAgentScorecard(root, "good-bot")
	if err != nil {
		t.Fatal(err)
	}
	if card.Status != "pass" || card.CompositeScore < core.AgentEvalCompositeThreshold {
		t.Fatalf("good-bot card = %+v", card)
	}
	if card.CaseCount != 3 || card.FailureCount != 0 {
		t.Fatalf("card counts = %d/%d", card.CaseCount, card.FailureCount)
	}
	if v := AgentAdmission(root, "good-bot"); v.State != "admitted" {
		t.Fatalf("good-bot admission = %+v", v)
	}
	eng := NewEngine(nil)
	if err := eng.CheckProfileAdmission(root, "good-bot", "qa.resolve"); err != nil {
		t.Fatalf("admitted profile must resolve: %v", err)
	}
	// History is listable.
	if got := ListAgentScorecards(root, "good-bot"); len(got) != 1 {
		t.Fatalf("scorecard history = %d", len(got))
	}
}

func TestAgentEvalBoundaryBlocksProfile(t *testing.T) {
	root := t.TempDir()
	eng := NewEngine(nil)
	// No record: warn, not block.
	if v := AgentAdmission(root, "ghost"); v.State != "warn" {
		t.Fatalf("no-record admission = %+v", v)
	}
	if err := eng.CheckProfileAdmission(root, "ghost", "qa.resolve"); err != nil {
		t.Fatalf("warn must not block selection: %v", err)
	}
	// A boundary-violating transcript.
	if _, err := GradeAgentTranscript(root, "bad-bot", "ROUTE: qa\nrelease verdict: GO", "qa", "transcript", ""); err != nil {
		t.Fatal(err)
	}
	card, err := BuildAgentScorecard(root, "bad-bot")
	if err != nil {
		t.Fatal(err)
	}
	if card.Status != "blocked" {
		t.Fatalf("bad-bot card status = %q (composite %.2f)", card.Status, card.CompositeScore)
	}
	if v := AgentAdmission(root, "bad-bot"); v.State != "blocked" {
		t.Fatalf("bad-bot admission = %+v", v)
	}
	err = eng.CheckProfileAdmission(root, "bad-bot", "qa.resolve")
	if err == nil {
		t.Fatal("blocked profile must be refused at selection")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonBoundaryForbiddenPhrase && rc != core.ReasonAgentEvalBelowThreshold {
		t.Fatalf("refusal reason = %v", err)
	}
}

func TestAgentCatalogShape(t *testing.T) {
	cat := BuildAgentCatalog()
	if cat.EntryCount == 0 || len(cat.Entries) != cat.EntryCount {
		t.Fatalf("catalog = %+v", cat.EntryCount)
	}
	total, covered := cat.CatalogCoverage()
	if total == 0 || covered == 0 {
		t.Fatalf("p0 coverage = %d/%d", covered, total)
	}
	foundBoundary := false
	for _, e := range cat.Entries {
		if e.Surface == "boundary-negative" {
			foundBoundary = true
		}
	}
	if !foundBoundary {
		t.Fatal("catalog must carry the boundary-negative probes")
	}
}

// ---- M2: strategy persistence feeds autotest discovery ----

func TestQAStrategyPersistsAndFeedsDiscovery(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "go.mod", "module example\n\ngo 1.22\n")
	writeFixtureFile(t, root, "main.go", "package main\n")
	strategy, path, err := GenerateQAStrategy(root, nil, "playwright", true)
	if err != nil {
		t.Fatal(err)
	}
	if path != ".beacon/qa-strategy.json" {
		t.Fatalf("strategy path = %q", path)
	}
	if _, err := os.Stat(filepath.Join(root, ".beacon", "qa-strategy.json")); err != nil {
		t.Fatal(err)
	}
	if got := qaStrategyDefaultCommand(root); got != strategy.DefaultCommand || got == "" {
		t.Fatalf("discovery reads %q, strategy says %q", got, strategy.DefaultCommand)
	}
}

// ---- M3: decompose materializes a real package ----

func TestDecomposeMaterializesPackage(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "decomp-box"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "decomp-box")
	v, err := env.eng.DecomposeTruthPackage(ctx,
		"用户能注册登录，支付订阅后开通权益，还能导出报表。", nil, "", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Materialized) < 3 {
		t.Fatalf("materialized ACs = %v", v.Materialized)
	}
	if len(v.Decomposition.Journeys) < 2 {
		t.Fatalf("journeys = %+v", v.Decomposition.Journeys)
	}
	files := env.eng.packageFiles(ctx)
	truth := readFile(t, filepath.Join(env.root, files.Truth.Path))
	if !strings.Contains(truth, "## 用户旅程") || !strings.Contains(truth, "J-identity") {
		t.Fatalf("truth.md must carry the generated journey section:\n%s", truth)
	}
	if !strings.Contains(truth, "utterance_clause_") && !strings.Contains(truth, "journey_seed") {
		t.Fatalf("coverage must carry decomposition provenance:\n%s", truth)
	}
	tests := readFile(t, filepath.Join(env.root, files.Tests.Path))
	for _, ac := range v.Materialized {
		if !strings.Contains(tests, ac) {
			t.Fatalf("tests.md missing a TC row for %s:\n%s", ac, tests)
		}
	}
	tasks := readFile(t, filepath.Join(env.root, files.Tasks.Path))
	if !strings.Contains(tasks, ".beacon/evidence/implement/decomp-box/") {
		t.Fatalf("tasks.md must carry evidence paths:\n%s", tasks)
	}
	if _, err := os.Stat(filepath.Join(env.root, v.ArtifactPath)); err != nil {
		t.Fatalf("decomposition artifact missing: %v", err)
	}
	// The signature is voided: decomposition never signs.
	st, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.Confirmed {
		t.Fatal("decompose must leave the package unsigned")
	}
}

func TestDecomposeGuards(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "decomp-frozen")
	ctx := env.ctx(t, "decomp-frozen")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.DecomposeTruthPackage(ctx, "用户能注册登录并完成支付订阅流程。", nil, "", "tester")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDecomposeFrozen {
		t.Fatalf("frozen decompose must refuse with decompose_frozen_package, got %v", err)
	}

	env2 := newTruthEnv(t)
	if _, err := env2.eng.CreateTruthPackage(env2.root, "", "decomp-empty"); err != nil {
		t.Fatal(err)
	}
	_, err = env2.eng.DecomposeTruthPackage(env2.ctx(t, "decomp-empty"), "  ", nil, "", "tester")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDecomposeNoIntents {
		t.Fatalf("empty decompose must refuse with decompose_no_intents, got %v", err)
	}
}

// ---- M4: brainstorm generation + convergence ----

func TestBrainstormGeneratesAndConverges(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "brain-box"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "brain-box")
	v, err := env.eng.GenerateBrainstorm(ctx, "", "做一个导出工具", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v.GeneratedCount < 4 {
		t.Fatalf("generated = %d, want at least the Socratic four", v.GeneratedCount)
	}
	if v.Converged {
		t.Fatal("fresh questions cannot be converged")
	}
	// Sources mark kernel origin in the shared ledger.
	st, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	kernel := 0
	for _, q := range st.OpenQuestions {
		if strings.HasPrefix(q.Source, "kernel:brainstorm-") {
			kernel++
		}
	}
	if kernel != v.GeneratedCount {
		t.Fatalf("kernel-sourced questions = %d, generated %d", kernel, v.GeneratedCount)
	}
	// Re-run dedupes: nothing new.
	v2, err := env.eng.GenerateBrainstorm(ctx, "", "做一个导出工具", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v2.GeneratedCount != 0 {
		t.Fatalf("second run generated %d, want 0 (dedupe)", v2.GeneratedCount)
	}
	// Answer everything -> converged, and confirm's blocker list
	// no longer names open questions.
	for _, q := range st.OpenQuestions {
		if _, err := env.eng.AnswerQuestion(ctx, q.ID, "已确认："+q.ID, "answered", "tester"); err != nil {
			t.Fatal(err)
		}
	}
	v3, err := env.eng.GenerateBrainstorm(ctx, "", "做一个导出工具", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if !v3.Converged || v3.OpenQuestions != 0 {
		t.Fatalf("after answers: %+v", v3)
	}
	// Invalid mode is refused.
	if _, err := env.eng.GenerateBrainstorm(ctx, "banana", "", "tester"); err == nil {
		t.Fatal("invalid brainstorm mode must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonBrainstormModeInvalid {
		t.Fatalf("want brainstorm_mode_invalid, got %v", err)
	}
}

// ---- M13: code2truth intake ----

func TestCode2TruthIntakeDraftPackage(t *testing.T) {
	env := newTruthEnv(t)
	target := t.TempDir()
	writeFixtureFile(t, target, "main.go", "package main\n\nfunc ExportedRun() {}\n")
	// code2truth's file-kind rules are Python-shaped (verbatim):
	// a test file is test_*.py / */tests/* / *.test.ts — a Go
	// _test.go file classifies as source under those rules.
	writeFixtureFile(t, target, "test_main.py", "def test_boot():\n    pass\n")
	writeFixtureFile(t, target, "README.md", "# legacy\n")
	writeFixtureFile(t, target, "SKILL.md", "# skill\n")
	v, err := env.eng.Code2TruthIntake(env.root, "", "legacy-scan", target, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if !v.PackageCreated || v.TruthAuthority {
		t.Fatalf("view = %+v", v)
	}
	if len(v.Claims) < 3 {
		t.Fatalf("claims = %+v", v.Claims)
	}
	if v.Inventory.Counts["source"] < 1 || v.Inventory.Symbols.ExportedSymbols < 1 {
		t.Fatalf("inventory = %+v", v.Inventory)
	}
	ctx := env.ctx(t, "legacy-scan")
	pv, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pv.Status != "draft" || pv.Frozen {
		t.Fatalf("code2truth package must be draft: %+v", pv)
	}
	files := env.eng.packageFiles(ctx)
	truth := readFile(t, filepath.Join(env.root, files.Truth.Path))
	if !strings.Contains(truth, "反推草稿待人确认") || !strings.Contains(truth, "code2truth_candidate: true") {
		t.Fatalf("truth.md must carry the reverse-draft banner:\n%s", truth)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".beacon", "code2truth", "legacy-scan", "intake.json")); err != nil {
		t.Fatalf("intake artifact missing: %v", err)
	}
	// Missing target refuses cleanly.
	if _, err := env.eng.Code2TruthIntake(env.root, "", "legacy-two", filepath.Join(target, "nope"), "tester"); err == nil {
		t.Fatal("missing target must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonCode2TruthTargetMissing {
		t.Fatalf("want code2truth_target_missing, got %v", err)
	}
}

// ---- M14: reopen ----

func TestReopenOpensNewRevision(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "reopen-box")
	ctx := env.ctx(t, "reopen-box")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ReopenTruthPackage(ctx, "需求变了", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "draft" || v.Frozen {
		t.Fatalf("reopened package = %+v", v)
	}
	if v.PreviousRevisionID != "R1" || v.NewRevisionID != "R2" {
		t.Fatalf("revision chain = %s -> %s", v.PreviousRevisionID, v.NewRevisionID)
	}
	files := env.eng.packageFiles(ctx)
	truth := readFile(t, filepath.Join(env.root, files.Truth.Path))
	scalars, _, _ := gate.ParseFrontmatter(truth)
	if scalars["revision_id"] != "R2" || scalars["source_revision_id"] != "R1" || scalars["intent_confirmed"] != "false" {
		t.Fatalf("frontmatter after reopen = %v", scalars)
	}
	// Materials describe the OLD revision now: stale.
	if got := env.eng.staleMaterialSurfaces(ctx); len(got) == 0 {
		t.Fatal("reopen must mark the materials stale")
	}
	// The trail carries the reopen with the old freeze's facts.
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType == EventTruthReopened {
			found = true
		}
	}
	if !found {
		t.Fatal("truth_reopened event missing from the trail")
	}
	// A draft cannot be reopened again.
	if _, err := env.eng.ReopenTruthPackage(ctx, "again", "tester"); err == nil {
		t.Fatal("reopening a draft must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTruthReopenNotFrozen {
		t.Fatalf("want truth_reopen_not_frozen, got %v", err)
	}
}

func TestReopenRefusesPendingChange(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "reopen-change")
	ctx := env.ctx(t, "reopen-change")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "改一处口径", "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ReopenTruthPackage(ctx, "x", "tester"); err == nil {
		t.Fatal("reopen with a pending change must refuse")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTruthChangePending {
		t.Fatalf("want truth_change_pending, got %v", err)
	}
}

func writeFixtureFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
