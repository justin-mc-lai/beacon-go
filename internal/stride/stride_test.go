package stride

import (
	"errors"
	"testing"
)

// TC-001 / AC-001: one turn records the effect, delivery and authority
// layers, each with a boundary and a stop reason from the vocabulary.
func TestRecordStrideBoundariesAndStopReason(t *testing.T) {
	turnID := "turn-001"
	records := []Record{
		{TurnID: turnID, Layer: LayerEffect, Boundary: "tool_calls=8", StopReason: StopSemanticCloseout, Closeout: "effect step settled"},
		{TurnID: turnID, Layer: LayerDelivery, Boundary: "delivery=stage_exit", StopReason: StopSemanticCloseout, Closeout: "delivery gate passed"},
		{TurnID: turnID, Layer: LayerAuthority, Boundary: "authority=unchanged", StopReason: StopSemanticCloseout, Closeout: "no authority borrowed"},
	}
	turn, err := BuildTurn(turnID, "settlement-001", records)
	if err != nil {
		t.Fatalf("build turn: %v", err)
	}
	if len(turn.Records) != 3 {
		t.Fatalf("records = %d, want 3", len(turn.Records))
	}
	layers := map[Layer]bool{}
	for _, rec := range turn.Records {
		if rec.TurnID != turnID {
			t.Fatalf("record turn_id = %q, want %q", rec.TurnID, turnID)
		}
		if rec.Boundary == "" {
			t.Fatalf("record %s has an empty boundary", rec.Layer)
		}
		if !rec.StopReason.Valid() {
			t.Fatalf("record %s has stop_reason %q outside the vocabulary", rec.Layer, rec.StopReason)
		}
		layers[rec.Layer] = true
	}
	for _, want := range []Layer{LayerEffect, LayerDelivery, LayerAuthority} {
		if !layers[want] {
			t.Fatalf("layer %q missing from the turn record: %+v", want, turn.Records)
		}
	}
	if !turn.Settled() {
		t.Fatal("turn must carry its settlement")
	}
}

// TC-003 / AC-002: hitting the numeric ceiling is a safety stop, never
// completion, and carries no closeout proof.
func TestNumericCeilingIsNotCompletion(t *testing.T) {
	rec, err := Evaluate(Record{
		TurnID: "turn-ceiling", Layer: LayerEffect,
		Boundary: "tool_calls_max=8", StopReason: StopNumericCeiling,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if rec.Outcome != OutcomeIncomplete {
		t.Fatalf("outcome = %q, want %q: a numeric ceiling must not complete a turn", rec.Outcome, OutcomeIncomplete)
	}
	if rec.Closeout != "" {
		t.Fatalf("closeout = %q, want empty on a numeric ceiling", rec.Closeout)
	}
	turn, err := BuildTurn("turn-ceiling", "settlement-ceiling", []Record{rec})
	if err != nil {
		t.Fatalf("build turn: %v", err)
	}
	if turn.Complete {
		t.Fatal("a ceiling-stopped turn must not be complete")
	}
	if turn.Progress != 0 {
		t.Fatalf("progress = %d, want 0 on a numeric ceiling", turn.Progress)
	}
}

// TC-004 / AC-002: completion requires a semantic closeout with a
// non-empty proof; the same stop reason without a proof is incomplete.
func TestSemanticCloseoutRequiredForCompletion(t *testing.T) {
	withProof, err := Evaluate(Record{
		TurnID: "turn-close", Layer: LayerDelivery,
		Boundary: "delivery=stage_exit", StopReason: StopSemanticCloseout, Closeout: "stage truth frozen and recorded",
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if withProof.Outcome != OutcomeComplete {
		t.Fatalf("outcome = %q, want %q with a semantic closeout", withProof.Outcome, OutcomeComplete)
	}
	withoutProof, err := Evaluate(Record{
		TurnID: "turn-close", Layer: LayerDelivery,
		Boundary: "delivery=stage_exit", StopReason: StopSemanticCloseout,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if withoutProof.Outcome != OutcomeIncomplete {
		t.Fatalf("outcome = %q, want %q without a closeout proof", withoutProof.Outcome, OutcomeIncomplete)
	}
	turn, err := BuildTurn("turn-close", "settlement-close", []Record{withoutProof})
	if err != nil {
		t.Fatalf("build turn: %v", err)
	}
	if turn.Complete {
		t.Fatal("a closeout without proof must not complete the turn")
	}
	if _, err := Evaluate(Record{
		TurnID: "turn-close", Layer: LayerEffect, Boundary: "tool_calls=4",
		StopReason: StopNumericCeiling, Closeout: "forged completion",
	}); !errors.Is(err, ErrCloseoutNotAllowed) {
		t.Fatalf("closeout on a numeric ceiling must be refused, got %v", err)
	}
}

// TC-005 / AC-003: the inner effect loop stops at the permission and
// acceptance boundaries and never borrows the outer authority.
func TestInnerLoopCannotCrossAuthorityBoundary(t *testing.T) {
	rec, err := Guard("turn-authority", GuardRequest{
		Layer: LayerEffect, Boundary: "tool.write", CrossesAuthority: true,
	})
	if err == nil {
		t.Fatal("crossing the authority boundary must be refused")
	}
	var boundaryErr *BoundaryError
	if !errors.As(err, &boundaryErr) || boundaryErr.Reason != ReasonAuthorityBoundary {
		t.Fatalf("error = %v, want reason %q", err, ReasonAuthorityBoundary)
	}
	if rec.StopReason != StopAuthorityBoundary {
		t.Fatalf("stop_reason = %q, want %q", rec.StopReason, StopAuthorityBoundary)
	}
	if rec.Outcome != OutcomeBlocked {
		t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeBlocked)
	}
	if rec.Closeout != "" {
		t.Fatalf("a refused inner step must not carry a closeout, got %q", rec.Closeout)
	}

	borrowed, err := Guard("turn-authority", GuardRequest{
		Layer: LayerEffect, Boundary: "tool.write", BorrowOuterAuthority: true,
	})
	if err == nil {
		t.Fatal("borrowing the outer authority must be refused")
	}
	if borrowed.StopReason != StopAuthorityBoundary || borrowed.Outcome != OutcomeBlocked {
		t.Fatalf("borrowed outer authority record = %+v, want blocked at %s", borrowed, StopAuthorityBoundary)
	}

	accepted, err := Guard("turn-acceptance", GuardRequest{
		Layer: LayerEffect, Boundary: "acceptance.gate", CrossesAcceptance: true,
	})
	if err == nil {
		t.Fatal("crossing the acceptance boundary must be refused")
	}
	if accepted.StopReason != StopAcceptanceBoundary || accepted.Outcome != OutcomeBlocked {
		t.Fatalf("acceptance record = %+v, want blocked at %s", accepted, StopAcceptanceBoundary)
	}

	if rec, err := Guard("turn-clear", GuardRequest{Layer: LayerEffect, Boundary: "tool.read"}); err != nil || rec.Layer != "" {
		t.Fatalf("a request that crosses nothing must pass, got %+v / %v", rec, err)
	}
}

// TC-007 / AC-004: a scheduler wake only permits a state check; it adds
// no semantic progress and never completes the turn.
func TestSchedulerWakeIsNotProgress(t *testing.T) {
	turn, err := BuildTurn("turn-wake", "settlement-wake", []Record{
		{TurnID: "turn-wake", Layer: LayerDelivery, Boundary: "scheduler.cadence", StopReason: StopSchedulerWake},
	})
	if err != nil {
		t.Fatalf("build turn: %v", err)
	}
	if turn.Progress != 0 {
		t.Fatalf("progress = %d, want 0 on a scheduler wake", turn.Progress)
	}
	if turn.Complete {
		t.Fatal("a scheduler wake must not complete the turn")
	}
	if turn.Records[0].Outcome != OutcomeIncomplete {
		t.Fatalf("wake outcome = %q, want %q", turn.Records[0].Outcome, OutcomeIncomplete)
	}
}

// TC-008 / AC-004: a real state advance counts as progress only when it
// carries a settlement, and is distinguishable from a scheduler wake.
func TestStateAdvanceCountsAsProgress(t *testing.T) {
	advance := []Record{{
		TurnID: "turn-advance", Layer: LayerDelivery,
		Boundary: "delivery=stage_exit", StopReason: StopSemanticCloseout, Closeout: "plan stage closed out",
	}}
	unsettled, err := BuildTurn("turn-advance", "", advance)
	if err != nil {
		t.Fatalf("build unsettled turn: %v", err)
	}
	if unsettled.Progress != 0 {
		t.Fatalf("progress = %d without settlement, want 0", unsettled.Progress)
	}
	settled, err := BuildTurn("turn-advance", "settlement-advance", advance)
	if err != nil {
		t.Fatalf("build settled turn: %v", err)
	}
	if settled.Progress != 1 {
		t.Fatalf("progress = %d with settlement, want 1", settled.Progress)
	}
	if !settled.Complete {
		t.Fatal("a settled semantic closeout must complete the turn")
	}
	wake, err := BuildTurn("turn-wake-2", "settlement-wake-2", []Record{{
		TurnID: "turn-wake-2", Layer: LayerDelivery, Boundary: "scheduler.cadence", StopReason: StopSchedulerWake,
	}})
	if err != nil {
		t.Fatalf("build wake turn: %v", err)
	}
	if wake.Progress != 0 {
		t.Fatalf("wake progress = %d, want 0 and distinguishable from state advance %d", wake.Progress, settled.Progress)
	}
}
