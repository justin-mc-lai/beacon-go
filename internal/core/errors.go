// Typed errors for the core layer (AC-027/028 translation of the Rust
// thiserror design): every failure carries a machine-readable ReasonCode as
// a struct field, never smuggled inside the message string.
package core

import (
	"errors"
	"fmt"
)

// Reason codes owned by the core layer. Values are stable contract strings.
const (
	ReasonIllegalStageTransition = "illegal_stage_transition"
	ReasonInvalidStage           = "invalid_stage"
	ReasonInvalidStageStatus     = "invalid_stage_status"
	ReasonInvalidRunStatus       = "invalid_run_status"
	ReasonBlockedMissingReasons  = "blocked_missing_reason_codes"
	ReasonSkippedMissingDeferral = "skipped_missing_deferral_ref"
	// ReasonEvidenceChainBroken is the AC-038 fail-closed code (uppercase in
	// the frozen truth, kept verbatim).
	ReasonEvidenceChainBroken = "EVIDENCE_CHAIN_BROKEN"
)

// Error is the unified typed error for core (and, by import, store/gate).
type Error struct {
	ReasonCode string
	Op         string
	Detail     string
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Op, e.Detail, e.ReasonCode)
	}
	return fmt.Sprintf("%s (%s)", e.Op, e.ReasonCode)
}

// NewError builds a typed error.
func NewError(reasonCode, op, detail string) *Error {
	return &Error{ReasonCode: reasonCode, Op: op, Detail: detail}
}

// ReasonCodeOf extracts the reason code from an error chain, if present.
func ReasonCodeOf(err error) (string, bool) {
	var ce *Error
	if AsError(err, &ce) {
		return ce.ReasonCode, true
	}
	return "", false
}

// AsError is a small wrapper over errors.As for *Error.
func AsError(err error, target **Error) bool {
	return errors.As(err, target)
}
