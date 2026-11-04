// Implement work-gate (implement-work-gate AC-001..006): the write
// side of the implement stage's work evidence.
//
// The judgement itself lives in internal/gate (implementWorkItem) and
// its facts are gathered read-only in gates_facts.go. This file owns
// the two things the exit needs beyond reading git:
//
//   - the explicit empty-stage declaration — the
//     `implement_empty_allowed` lifecycle event (AC-003). An empty
//     implement stage is allowed, but only when it is DECLARED; a
//     silent no-op is exactly the beacon-go#4 hole.
//   - the `implement_work_gate` config switch's Go-side value
//     (default on; off records the item not_applicable).
//
// Both are trail/snapshot writes, never worktree writes: the gate
// evaluation itself stays read-only (AC-006).
package orchestrator

import (
	"database/sql"
	"encoding/json"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// EventImplementEmptyAllowed records that a run deliberately ships an
// implement stage with no work (AC-003). It is the ONLY way the
// implement_work_recorded item becomes not_applicable, and it is
// per-run: a new run does not inherit the previous run's declaration.
const EventImplementEmptyAllowed = "implement_empty_allowed"

// implement_work_gate config values (truth 配置项: default on).
const (
	ImplementWorkGateOn  = "on"
	ImplementWorkGateOff = "off"
)

// SetImplementWorkGate sets the implement_work_gate switch: "on"
// (default; an empty value is on) or "off". Any other value is
// refused rather than guessed — a typo must never silently disable a
// gate (implement_work_gate_invalid).
func (e *Engine) SetImplementWorkGate(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ImplementWorkGateOn:
		e.implementWorkGate = ImplementWorkGateOn
	case ImplementWorkGateOff:
		e.implementWorkGate = ImplementWorkGateOff
	default:
		return core.NewError(core.ReasonImplementWorkGateInvalid, "gate.implement_work",
			"implement_work_gate must be on or off, got "+mode)
	}
	return nil
}

// implementWorkGateEnabled reports the switch (zero value = on).
func (e *Engine) implementWorkGateEnabled() bool {
	return e.implementWorkGate != ImplementWorkGateOff
}

// AllowEmptyResult reports one `--allow-empty` declaration.
type AllowEmptyResult struct {
	Schema      string `json:"schema"`
	RunID       string `json:"run_id"`
	FeatureSlug string `json:"feature_slug,omitempty"`
	EventID     string `json:"event_id,omitempty"`
	Appended    bool   `json:"appended"`
	Detail      string `json:"detail,omitempty"`
}

// implementEmptyAllowedRecorded reports whether the run already
// carries the declaration event (idempotency read).
func (e *Engine) implementEmptyAllowedRecorded(runID string) (bool, error) {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return false, err
	}
	for _, ev := range events {
		if ev.EventType == EventImplementEmptyAllowed {
			return true, nil
		}
	}
	return false, nil
}

// AllowEmptyImplement records the implement_empty_allowed event on one
// run (AC-003; the CLI's `gate evaluate implement --allow-empty`). The
// run resolves exactly like a gate evaluation's (explicit --run or the
// feature's newest goal run). Idempotent: an already-declared run is
// not re-appended, so repeated invocations produce the same state.
func (e *Engine) AllowEmptyImplement(ctx store.FeatureContext, runID, by, note string) (AllowEmptyResult, error) {
	run, _, has, err := e.latestGoalRun(ctx, runID)
	if err != nil {
		return AllowEmptyResult{}, err
	}
	res := AllowEmptyResult{Schema: "implement-empty-allowed.v1", FeatureSlug: ctx.Feature.Slug}
	if !has {
		return res, core.NewError(core.ReasonGateRunRequired, "gate.AllowEmptyImplement",
			"implement_empty_allowed is a per-run declaration; no goal run exists for this feature (create one or pass --run)")
	}
	res.RunID = run.ID
	recorded, err := e.implementEmptyAllowedRecorded(run.ID)
	if err != nil {
		return AllowEmptyResult{}, err
	}
	if recorded {
		res.Detail = "the run already declares an empty implement stage; nothing appended"
		return res, nil
	}
	payload, err := json.Marshal(map[string]any{
		"schema": res.Schema, "by": by, "note": note, "at": nowISO(),
	})
	if err != nil {
		return AllowEmptyResult{}, err
	}
	ev, err := e.store.AppendEvent(store.LifecycleEvent{
		FeatureID: ctx.Feature.ID,
		RunID:     sql.NullString{String: run.ID, Valid: true},
		EventType: EventImplementEmptyAllowed,
		Payload:   payload,
	})
	if err != nil {
		return AllowEmptyResult{}, err
	}
	res.Appended, res.EventID = true, ev.ID
	res.Detail = "recorded: the implement_work_recorded check reports not_applicable for this run"
	return res, nil
}
