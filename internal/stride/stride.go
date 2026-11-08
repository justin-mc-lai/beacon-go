// Package stride carries the per-turn stride record contract
// (kernel-stride-control): which of the effect / delivery / authority
// layers a turn touched, at which boundary it stopped, and why.
//
// A stride record is a read-only fact about one turn. It is not a
// completion claim: numeric ceilings are safety limits only, and a
// turn is complete only when a semantic closeout proves it.
package stride

import (
	"errors"
	"fmt"
	"strings"
)

// Layer is the stride layer a record belongs to.
type Layer string

const (
	LayerEffect    Layer = "effect"
	LayerDelivery  Layer = "delivery"
	LayerAuthority Layer = "authority"
)

// StopReason is the controlled vocabulary of why a turn stopped.
type StopReason string

const (
	StopSemanticCloseout   StopReason = "semantic_closeout"
	StopNumericCeiling     StopReason = "numeric_ceiling"
	StopAuthorityBoundary  StopReason = "authority_boundary"
	StopAcceptanceBoundary StopReason = "acceptance_boundary"
	StopSettlementRequired StopReason = "settlement_required"
	StopSchedulerWake      StopReason = "scheduler_wake"
	StopBlocked            StopReason = "blocked"
)

// Outcome is what the record proves about the turn.
type Outcome string

const (
	OutcomeComplete   Outcome = "complete"
	OutcomeIncomplete Outcome = "incomplete"
	OutcomeBlocked    Outcome = "blocked"
)

// Reason codes surfaced by this package.
const (
	ReasonInvalidRecord      = "stride_record_invalid"
	ReasonCloseoutNotAllowed = "stride_closeout_not_allowed"
	ReasonAuthorityBoundary  = "authority_boundary"
	ReasonAcceptanceBoundary = "acceptance_boundary"
)

var (
	// ErrInvalidRecord rejects a record that violates the contract.
	ErrInvalidRecord = errors.New(ReasonInvalidRecord)
	// ErrCloseoutNotAllowed rejects a closeout on a non-semantic stop.
	ErrCloseoutNotAllowed = errors.New(ReasonCloseoutNotAllowed)
)

// BoundaryError is the inner-loop refusal: the effect layer tried to
// cross a boundary and must stop instead of borrowing outer authority.
type BoundaryError struct {
	Reason   string
	Boundary string
}

func (e *BoundaryError) Error() string {
	return fmt.Sprintf("%s at %s: inner effect layer may not borrow outer authority", e.Reason, e.Boundary)
}

// Record is one turn's stride fact for one layer.
type Record struct {
	TurnID     string     `json:"turn_id"`
	Layer      Layer      `json:"layer"`
	Boundary   string     `json:"boundary"`
	StopReason StopReason `json:"stop_reason"`
	Closeout   string     `json:"closeout,omitempty"`
	Outcome    Outcome    `json:"outcome"`
}

// Turn is the read-only stride fact of one turn.
type Turn struct {
	TurnID     string   `json:"turn_id"`
	Records    []Record `json:"records"`
	Settlement string   `json:"settlement,omitempty"`
	Progress   int      `json:"progress"`
	Complete   bool     `json:"complete"`
}

// Settled reports whether the turn's state advance was settled.
func (t Turn) Settled() bool { return strings.TrimSpace(t.Settlement) != "" }

// Layers returns the stride layers present in the turn.
func (t Turn) Layers() []Layer {
	seen := map[Layer]bool{}
	out := make([]Layer, 0, 3)
	for _, r := range t.Records {
		if !seen[r.Layer] {
			seen[r.Layer] = true
			out = append(out, r.Layer)
		}
	}
	return out
}

// StopReasons reports the controlled vocabulary.
func StopReasons() []StopReason {
	return []StopReason{
		StopSemanticCloseout, StopNumericCeiling, StopAuthorityBoundary,
		StopAcceptanceBoundary, StopSettlementRequired, StopSchedulerWake, StopBlocked,
	}
}

// Valid reports whether the layer is part of the contract.
func (l Layer) Valid() bool {
	return l == LayerEffect || l == LayerDelivery || l == LayerAuthority
}

// Valid reports whether the stop reason is part of the vocabulary.
func (s StopReason) Valid() bool {
	for _, known := range StopReasons() {
		if s == known {
			return true
		}
	}
	return false
}

// Evaluate validates a record and derives its outcome. Numeric ceilings
// and scheduler wakes are never completion; completion requires a
// semantic closeout with a non-empty proof.
func Evaluate(rec Record) (Record, error) {
	rec.TurnID = strings.TrimSpace(rec.TurnID)
	if rec.TurnID == "" {
		return rec, fmt.Errorf("%w: turn_id is empty", ErrInvalidRecord)
	}
	if !rec.Layer.Valid() {
		return rec, fmt.Errorf("%w: unknown layer %q", ErrInvalidRecord, rec.Layer)
	}
	rec.Boundary = strings.TrimSpace(rec.Boundary)
	if rec.Boundary == "" {
		return rec, fmt.Errorf("%w: boundary is empty", ErrInvalidRecord)
	}
	if !rec.StopReason.Valid() {
		return rec, fmt.Errorf("%w: unknown stop_reason %q", ErrInvalidRecord, rec.StopReason)
	}
	rec.Closeout = strings.TrimSpace(rec.Closeout)
	if rec.StopReason != StopSemanticCloseout && rec.Closeout != "" {
		return rec, fmt.Errorf("%w: closeout is only allowed with %s", ErrCloseoutNotAllowed, StopSemanticCloseout)
	}
	switch rec.StopReason {
	case StopSemanticCloseout:
		if rec.Closeout == "" {
			rec.Outcome = OutcomeIncomplete
		} else {
			rec.Outcome = OutcomeComplete
		}
	case StopNumericCeiling, StopSchedulerWake:
		rec.Outcome = OutcomeIncomplete
	default:
		rec.Outcome = OutcomeBlocked
	}
	return rec, nil
}

// BuildTurn evaluates every record of one turn and derives the turn's
// progress and completion facts. A semantic closeout on the effect
// layer never counts as outer progress; progress requires a settled
// delivery record.
func BuildTurn(turnID, settlement string, records []Record) (Turn, error) {
	turnID = strings.TrimSpace(turnID)
	if turnID == "" {
		return Turn{}, fmt.Errorf("%w: turn_id is empty", ErrInvalidRecord)
	}
	turn := Turn{TurnID: turnID, Settlement: strings.TrimSpace(settlement), Records: make([]Record, 0, len(records))}
	for _, rec := range records {
		if strings.TrimSpace(rec.TurnID) == "" {
			rec.TurnID = turnID
		}
		if strings.TrimSpace(rec.TurnID) != turnID {
			return Turn{}, fmt.Errorf("%w: record turn_id %q does not match turn %q", ErrInvalidRecord, rec.TurnID, turnID)
		}
		ev, err := Evaluate(rec)
		if err != nil {
			return Turn{}, err
		}
		turn.Records = append(turn.Records, ev)
	}
	turn.recompute()
	return turn, nil
}

func (t *Turn) recompute() {
	t.Progress = 0
	t.Complete = false
	settled := t.Settled()
	for _, rec := range t.Records {
		if rec.Outcome != OutcomeComplete {
			continue
		}
		t.Complete = true
		if settled && rec.Layer == LayerDelivery {
			t.Progress++
		}
	}
}

// GuardRequest is one inner-loop request at the next step.
type GuardRequest struct {
	Layer                Layer
	Boundary             string
	CrossesAuthority     bool
	CrossesAcceptance    bool
	BorrowOuterAuthority bool
}

// Guard judges an inner (effect-layer) request. Crossing the permission
// or acceptance boundary, or trying to borrow the outer authority, stops
// the turn with a blocked record and an error; the caller must not
// continue. A request that crosses nothing returns an empty record and
// a nil error.
func Guard(turnID string, req GuardRequest) (Record, error) {
	if req.Layer != LayerEffect {
		return Record{}, fmt.Errorf("%w: guard applies to the %s layer, got %q", ErrInvalidRecord, LayerEffect, req.Layer)
	}
	rec := Record{TurnID: strings.TrimSpace(turnID), Layer: req.Layer, Boundary: strings.TrimSpace(req.Boundary)}
	switch {
	case req.CrossesAuthority || req.BorrowOuterAuthority:
		rec.StopReason = StopAuthorityBoundary
	case req.CrossesAcceptance:
		rec.StopReason = StopAcceptanceBoundary
	default:
		return Record{}, nil
	}
	ev, err := Evaluate(rec)
	if err != nil {
		return Record{}, err
	}
	reason := ReasonAuthorityBoundary
	if ev.StopReason == StopAcceptanceBoundary {
		reason = ReasonAcceptanceBoundary
	}
	return ev, &BoundaryError{Reason: reason, Boundary: ev.Boundary}
}
