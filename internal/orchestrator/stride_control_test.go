package orchestrator

import (
	"context"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/stride"
)

// TC-002 / AC-001: after a turn settles, the turn's stride record is
// queryable through the persisted event log, carries the same turn_id,
// and covers the effect, delivery and authority layers.
func TestTurnRecordCarriesStrideRecord(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "room-host", ExecutorMode: ExecutorModeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.TickBy(context.Background(), run.RunID, "room-host")
	if err != nil {
		t.Fatal(err)
	}
	if view.Blocked || view.CurrentStage != StageTruth {
		t.Fatalf("manual tick did not advance plan -> truth: %+v", view)
	}
	turns, err := env.eng.StrideTurns(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("turns = %d, want 1 settled turn for the advanced stage", len(turns))
	}
	turn := turns[0]
	if turn.TurnID == "" {
		t.Fatal("turn record has an empty turn_id")
	}
	if !turn.Settled() {
		t.Fatalf("turn %s carries no settlement", turn.TurnID)
	}
	for _, rec := range turn.Records {
		if rec.TurnID != turn.TurnID {
			t.Fatalf("record turn_id = %q, want %q", rec.TurnID, turn.TurnID)
		}
		if rec.Boundary == "" || rec.StopReason == "" {
			t.Fatalf("record %s is missing boundary or stop reason: %+v", rec.Layer, rec)
		}
	}
	layers := map[stride.Layer]bool{}
	for _, layer := range turn.Layers() {
		layers[layer] = true
	}
	for _, want := range []stride.Layer{stride.LayerEffect, stride.LayerDelivery, stride.LayerAuthority} {
		if !layers[want] {
			t.Fatalf("stride layer %q missing from turn %+v", want, turn.Records)
		}
	}
	// A second engine over the same store proves the fact is persisted in
	// the run snapshot, not held in process memory.
	fresh, err := NewEngine(env.st).StrideTurns(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh) != 1 || fresh[0].TurnID != turn.TurnID {
		t.Fatalf("reloaded turns = %+v, want the settled turn %s", fresh, turn.TurnID)
	}
}

// TC-006 / AC-003: an outer continuation whose previous turn left no
// settlement is blocked, writes no progress or completion, and passes
// once a settlement is recorded.
func TestOuterContinuityCannotSkipSettlement(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "room-host", ExecutorMode: ExecutorModeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	records := []stride.Record{{
		Layer: stride.LayerDelivery, Boundary: "delivery=stage_exit",
		StopReason: stride.StopSemanticCloseout, Closeout: "stage advanced",
	}}
	unsettled, err := env.eng.SettleTurn(run.RunID, "turn-unsettled", "", records)
	if err != nil {
		t.Fatal(err)
	}
	if unsettled.Settled() {
		t.Fatal("turn must be recorded as unsettled when no settlement is given")
	}
	if unsettled.Progress != 0 {
		t.Fatalf("progress = %d on an unsettled turn, want 0", unsettled.Progress)
	}
	if err := env.eng.ContinueOuterTurn(run.RunID); err == nil {
		t.Fatal("outer continuation must be refused without settlement")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonSettlementRequired {
		t.Fatalf("reason = %q, want %q", rc, ReasonSettlementRequired)
	}
	view, err := env.eng.TickBy(context.Background(), run.RunID, "room-host")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Blocked || view.ReasonCode != ReasonSettlementRequired {
		t.Fatalf("tick after an unsettled turn = %+v, want blocked with %s", view, ReasonSettlementRequired)
	}
	if view.CurrentStage != StagePlan {
		t.Fatalf("blocked continuation advanced to %q; it must not write progress", view.CurrentStage)
	}
	turns, err := env.eng.StrideTurns(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("blocked continuation wrote a new stride turn (now %d)", len(turns))
	}
	if turns[0].TurnID != "turn-unsettled" || turns[0].Progress != 0 {
		t.Fatalf("blocked continuation changed the turn fact: %+v", turns[0])
	}

	settled, err := env.eng.SettleTurn(run.RunID, "turn-settled", "settlement-1", records)
	if err != nil {
		t.Fatal(err)
	}
	if settled.Progress != 1 || !settled.Complete {
		t.Fatalf("settled turn = %+v, want progress 1 and complete", settled)
	}
	if err := env.eng.ContinueOuterTurn(run.RunID); err != nil {
		t.Fatalf("settled turn must allow outer continuation: %v", err)
	}
}
