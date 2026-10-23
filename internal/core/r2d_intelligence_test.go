package core

import (
	"strings"
	"testing"
)

// M1 grader: boundary phrases fire first, even with a ROUTE line.
func TestGradeTranscriptBoundaryFirst(t *testing.T) {
	g := GradeTranscript("ROUTE: qa\nrelease verdict: GO", "qa")
	if g.Passed || g.ReasonCode != ReasonBoundaryForbiddenPhrase {
		t.Fatalf("boundary phrases must fail first: %+v", g)
	}
	if len(g.ForbiddenHits) != 1 || g.ForbiddenHits[0] != "release verdict" {
		t.Fatalf("hits = %v", g.ForbiddenHits)
	}
}

func TestGradeTranscriptProbeBranches(t *testing.T) {
	missing := GradeTranscript("did the work", "qa")
	if missing.Passed || missing.ReasonCode != ReasonRouteProbeMissing {
		t.Fatalf("want route_probe_missing, got %+v", missing)
	}
	mismatch := GradeTranscript("ROUTE: truth", "qa")
	if mismatch.Passed || mismatch.ReasonCode != ReasonRouteProbeMismatch {
		t.Fatalf("want route_probe_mismatch, got %+v", mismatch)
	}
	ok := GradeTranscript("ROUTE: qa\nall good", "qa")
	if !ok.Passed || ok.ParsedRoute != "qa" {
		t.Fatalf("want pass, got %+v", ok)
	}
}

// Dimension math for a replay runner, hand-computed from
// _compute_dimension_scores: ratio 1-1/2 = 0.5 -> route 50,
// boundary = route (replay), source = 55, behavior = 45,
// fake = 95 (one failure), catalog 1/2 = 50.
func TestAgentDimensionsReplay(t *testing.T) {
	dims := ComputeAgentDimensions(AgentDimensionInputs{
		CatalogP0Total: 2, CatalogP0Covered: 1,
		FailureCount: 1, CaseCount: 2, Runner: "replay",
	})
	want := map[string]float64{
		"route_stability":             50,
		"boundary_preservation":       50,
		"source_anchor_coverage":      55,
		"capability_catalog_coverage": 50,
		"behavior_evidence_quality":   45,
		"fake_delivery_resistance":    95,
	}
	for k, v := range want {
		if dims[k] != v {
			t.Fatalf("%s = %v, want %v (all dims %v)", k, dims[k], v, dims)
		}
	}
	// Composite: .15*50 + .20*50 + .10*55 + .20*50 + .25*45 + .10*95
	// = 7.5 + 10 + 5.5 + 10 + 11.25 + 9.5 = 53.75.
	if got := AgentComposite(dims, "replay"); got != 53.75 {
		t.Fatalf("composite = %v, want 53.75", got)
	}
	// Simulated cap: a perfect card caps at 85.
	perfect := map[string]float64{}
	for _, k := range AgentEvalDimensions {
		perfect[k] = 100
	}
	if got := AgentComposite(perfect, "simulated"); got != AgentEvalSimulatedCap {
		t.Fatalf("simulated composite = %v, want cap %v", got, AgentEvalSimulatedCap)
	}
	// The ci_default quirk, ported verbatim: host_pass * 100.
	ci := ComputeAgentDimensions(AgentDimensionInputs{
		Runner: "live", LiveTier: "ci_default", LiveTotal: 10, LiveHostPass: 2,
	})
	if ci["behavior_evidence_quality"] != 200 {
		t.Fatalf("ci_default behavior must be verbatim host_pass*100=200, got %v", ci["behavior_evidence_quality"])
	}
}

// M1 admission: the three states.
func TestAgentAdmissionStates(t *testing.T) {
	if v := AgentAdmissionFromRecord(false, 0, false); v.State != "warn" || v.ReasonCode != ReasonAgentEvalNoRecord {
		t.Fatalf("no record must warn: %+v", v)
	}
	if v := AgentAdmissionFromRecord(true, 42, false); v.State != "blocked" || v.ReasonCode != ReasonAgentEvalBelowThreshold {
		t.Fatalf("low score must block: %+v", v)
	}
	if v := AgentAdmissionFromRecord(true, 91, true); v.State != "blocked" || v.ReasonCode != ReasonBoundaryForbiddenPhrase {
		t.Fatalf("boundary failure must block: %+v", v)
	}
	if v := AgentAdmissionFromRecord(true, 91, false); v.State != "admitted" {
		t.Fatalf("good record must admit: %+v", v)
	}
}

// M3: simple sentence decomposes as a lake with intents derived.
func TestDecomposeSimpleSentence(t *testing.T) {
	d := DecomposeIntent("先做一个给商家看的订单汇总工作台，能导出 Excel 报表。", nil, "", "2026-10-23T00:00:00Z")
	if len(d.Intents) < 1 {
		t.Fatalf("no intents from the utterance: %+v", d)
	}
	if d.ScopeMode != "lake" && d.ScopeMode != "ocean" {
		t.Fatalf("scope = %q", d.ScopeMode)
	}
	if !strings.Contains(d.DeepWant, "表层：") {
		t.Fatalf("deep want must carry the surface ask: %q", d.DeepWant)
	}
	if d.Schema != DecompositionSchema {
		t.Fatalf("schema = %q", d.Schema)
	}
}

func TestDecomposeJourneySeedsReal(t *testing.T) {
	d := DecomposeIntent("用户能注册登录，支付订阅后开通权益。", nil, "", "")
	found := map[string]bool{}
	for _, j := range d.Journeys {
		found[j.ID] = true
		if len(j.StepsSeed) == 0 || len(j.AcceptanceSeeds) == 0 {
			t.Fatalf("journey seed incomplete: %+v", j)
		}
	}
	if !found["J-identity"] || !found["J-billing"] {
		t.Fatalf("expected identity + billing journey seeds, got %v", d.Journeys)
	}
}

func TestDecomposeOcean(t *testing.T) {
	// Python's ocean triggers: an ocean keyword hit (完整复刻 /
	// 全功能 / l-labels) or >= 3 explicit lakes.
	d := DecomposeIntent("把旧系统完整复刻一遍，全功能 parity 交付。", nil, "", "")
	if d.ScopeMode != "ocean" {
		t.Fatalf("ocean-keyword utterance must be ocean, got %q (lakes %v)", d.ScopeMode, d.Lakes)
	}
	if d.Readiness.Ready {
		t.Fatal("ocean must never be decomposition-ready")
	}
	if !d.Readiness.BlocksAutoTick {
		t.Fatal("ocean must block auto-tick")
	}
	// Three explicit lakes also force ocean.
	d2 := DecomposeIntent("按计划推进。", []string{"orders", "members", "billing"}, "", "")
	if d2.ScopeMode != "ocean" {
		t.Fatalf("3 explicit lakes must be ocean, got %q", d2.ScopeMode)
	}
}

// _extract_lakes reads EXPLICIT labels only: L<n>-name and 湖<n>.
func TestDecompExtractLakes(t *testing.T) {
	lakes := DecompExtractLakes("先做 L1-orders 和 L2-members，再补湖3 的内容", nil)
	joined := strings.Join(lakes, ",")
	for _, want := range []string{"l1-orders", "l2-members", "lake-3"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("lakes = %v, want %q among them", lakes, want)
		}
	}
	if got := DecompExtractLakes("做一个订单系统和会员中心", nil); len(got) != 0 {
		t.Fatalf("domain nouns are NOT lakes in Python either, got %v", got)
	}
}

func TestDecompSplitClauses(t *testing.T) {
	clauses := DecompSplitClauses("第一个子句足够长啊；第二个子句也足够长。短")
	if len(clauses) != 2 {
		t.Fatalf("clauses = %v, want the two substantive ones", clauses)
	}
}

// intent_first capture: sentences >= 10 chars, per-sentence refs.
func TestCaptureIntentsSplitsSentences(t *testing.T) {
	items := CaptureIntents("这是一个足够长的第一句话，描述导出功能。然后这是第二句，描述导入功能。")
	if len(items) != 2 {
		t.Fatalf("want 2 captured intents, got %d: %+v", len(items), items)
	}
	if !strings.HasPrefix(items[0].SourceRef, "utterance_sentence_1:") {
		t.Fatalf("source ref = %q", items[0].SourceRef)
	}
	if items[0].SourceType != "user_intent" {
		t.Fatalf("source type = %q", items[0].SourceType)
	}
}

// M4: mode canonicalization (the real alias table) + the
// four-question Socratic set.
func TestBrainstormModeCanonical(t *testing.T) {
	cases := map[string]string{
		"requirement-convergence":  "requirement-convergence",
		"project-evolution":        "project-evolution",
		"evolution-exploration":    "project-evolution",
		"implementation-readiness": "implementation-readiness",
		"frozen-truth-delta":       "implementation-readiness",
		"scope-simplification":     "scope-simplification",
		"redundancy-pruning":       "scope-simplification",
	}
	for in, want := range cases {
		got, ok := CanonicalBrainstormMode(in)
		if !ok || got != want {
			t.Fatalf("CanonicalBrainstormMode(%q) = %q,%v, want %q", in, got, ok, want)
		}
	}
	if _, ok := CanonicalBrainstormMode("banana"); ok {
		t.Fatal("unknown mode must not canonicalize")
	}
	if len(BrainstormSocraticQuestions) != 4 {
		t.Fatal("the Socratic set is four questions")
	}
	rec, route := BrainstormModeRecommendation("requirement-convergence")
	if !strings.Contains(rec, "Converge") || route == "" {
		t.Fatalf("recommendation = %q / %q", rec, route)
	}
}
