// External / manual stage evidence (kernel-external-stage-evidence).
//
// A manual or external run has no stage executor: the tick identity is
// the completion record. This file writes the same stage_executed
// evidence a real executor writes, authored by the kernel. Only the
// identity (by) can come from the caller, so an external room cannot
// forge the executor/source/recording fields that the qa gate trusts.
package orchestrator

import (
	"strings"

	"beacon-go/internal/store"
)

// recordManualStageExecuted appends the kernel-authored stage_executed
// event for one completed manual/external stage. It runs inside the
// same transaction as the stage_advanced event, so a completion never
// leaves one without the other.
//
// The payload deliberately carries the recording marker (placeholder
// and recording) as explicit false values: the qa gate treats a
// missing marker as recording state and refuses it (AC-002).
func (e *Engine) recordManualStageExecuted(w writeSet, run store.Run, stage, by string) error {
	return e.appendEventOn(w, run, "stage_executed", map[string]any{
		"stage":       stage,
		"executor":    ExecutorModeExternal,
		"source":      ExecutorModeExternal,
		"by":          strings.TrimSpace(by),
		"placeholder": false,
		"recording":   false,
		"outcome":     "done",
	})
}
