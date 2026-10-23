package orchestrator

import (
	"sort"
	"testing"

	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// completeIntentForTest drives the ask-before-sign flow to a signed
// state so existing lifecycle tests keep testing their own subject.
func completeIntentForTest(t *testing.T, eng *Engine, ctx store.FeatureContext) {
	t.Helper()
	snap := gate.IntentSnapshot{
		Utterance: "test utterance", Goal: "test goal",
		NonGoals: "test non-goals", Constraints: "test constraints",
		ScopeMode: "focused",
	}
	if _, err := eng.UpdateIntent(ctx, snap, "tester"); err != nil {
		t.Fatalf("UpdateIntent: %v", err)
	}
	text, err := readTruthText(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defined := gate.DefinedACs(text)
	ids := make([]string, 0, len(defined))
	for id := range defined {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows := make([]gate.IntentCoverageRow, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, gate.IntentCoverageRow{AC: id, Source: "test utterance", Evidence: "conversation:test"})
	}
	text = replaceSection(text, "## Intent Coverage", rebuildCoverage(rows))
	text = replaceSection(text, "## 人话", "测试需求：目标、用户与完成标准。")
	if err := writeTruthText(ctx, text); err != nil {
		t.Fatal(err)
	}
	view, err := eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range view.OpenQuestions {
		if q.Status == "answered" || q.Status == "dropped" {
			continue
		}
		if _, err := eng.AnswerQuestion(ctx, q.ID, "test answer", "answered", "tester"); err != nil {
			t.Fatalf("AnswerQuestion(%s): %v", q.ID, err)
		}
	}
	if _, err := eng.ConfirmIntent(ctx, "tester"); err != nil {
		t.Fatalf("ConfirmIntent: %v", err)
	}
}
