// Stride control (kernel-stride-control): every settled turn leaves a
// persisted stride fact, and outer continuation is refused when the
// previous turn carries no settlement.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
	"beacon-go/internal/stride"
)

// ReasonSettlementRequired refuses an outer continuation whose previous
// turn left no settlement.
const ReasonSettlementRequired = "settlement_required"

// StageTurnID is the stable turn identifier for one stage settlement.
// The same run state and tick key always produce the same id.
func StageTurnID(runID, stage, key string) string {
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%s:stage:%s:%s", runID, stage, hex.EncodeToString(sum[:6]))
}

// StageSettlementRef names the settlement a stage advance records.
func StageSettlementRef(runID, stage string) string {
	return "stage_exit:" + stage + ":" + runID
}

func strideRecordsForStageAdvance(stage, by string) []stride.Record {
	proof := "stage " + stage + " exit gate judged and advanced"
	if by = strings.TrimSpace(by); by != "" {
		proof += " by " + by
	}
	gate := stageGateType(stage)
	if gate == "" {
		gate = "stage_gate"
	}
	return []stride.Record{
		{Layer: stride.LayerEffect, Boundary: "stage_exit:" + gate, StopReason: stride.StopSemanticCloseout, Closeout: proof},
		{Layer: stride.LayerDelivery, Boundary: "settlement:stage_exit:" + stage, StopReason: stride.StopSemanticCloseout, Closeout: proof},
		{Layer: stride.LayerAuthority, Boundary: "authority:unchanged", StopReason: stride.StopSemanticCloseout, Closeout: "no authority borrowed across the stage boundary"},
	}
}

// settleStageAdvance records the stride fact for an advanced stage on
// the run snapshot the caller is about to save, so the turn record and
// the stage advance commit together.
func (e *Engine) settleStageAdvance(m *runMeta, run store.Run, stage, key, by string) error {
	turnID := StageTurnID(run.ID, stage, key)
	turn, err := stride.BuildTurn(turnID, StageSettlementRef(run.ID, stage), strideRecordsForStageAdvance(stage, by))
	if err != nil {
		return core.NewError(stride.ReasonInvalidRecord, "goal.settleStageAdvance", err.Error())
	}
	m.StrideTurns = append(m.StrideTurns, turn)
	return nil
}

// SettleTurn records one turn's stride fact for a run. An empty
// settlement is legal (the turn is recorded as unsettled) and makes an
// outer continuation block until a settlement arrives.
func (e *Engine) SettleTurn(runID, turnID, settlement string, records []stride.Record) (stride.Turn, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return stride.Turn{}, err
	}
	turn, err := stride.BuildTurn(turnID, settlement, records)
	if err != nil {
		return stride.Turn{}, core.NewError(stride.ReasonInvalidRecord, "goal.SettleTurn", err.Error())
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		m.StrideTurns = append(m.StrideTurns, turn)
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return stride.Turn{}, err
	}
	return turn, nil
}

// StrideTurns reads the persisted stride facts of one run in append
// order. The run snapshot is the authority; the read always reloads it.
func (e *Engine) StrideTurns(runID string) ([]stride.Turn, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return nil, err
	}
	out := make([]stride.Turn, len(m.StrideTurns))
	copy(out, m.StrideTurns)
	return out, nil
}

// ContinueOuterTurn judges whether the run's outer loop may continue.
// The latest turn must be settled; a missing settlement blocks with
// settlement_required and no progress or completion is written.
func (e *Engine) ContinueOuterTurn(runID string) error {
	turns, err := e.StrideTurns(runID)
	if err != nil {
		return err
	}
	if len(turns) == 0 {
		return nil
	}
	last := turns[len(turns)-1]
	if !last.Settled() {
		return core.NewError(ReasonSettlementRequired, "goal.ContinueOuterTurn",
			"turn "+last.TurnID+" carries no settlement; outer continuation refused")
	}
	return nil
}
