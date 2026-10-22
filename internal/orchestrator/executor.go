// RecordingExecutor (P2a placeholder): records calls and writes one
// stage_executed evidence event per call. It performs NO real stage
// work and must never be presented as a real agent backend.
package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"

	"beacon-go/internal/store"
)

// ---- RecordingExecutor (placeholder, honest) ----

// RecordedCall is one invocation of the RecordingExecutor.
type RecordedCall struct {
	RunID string
	Stage string
}

// RecordingExecutor is the P2a placeholder executor: it records every
// call and appends ONE stage_executed evidence event per call, then
// reports done (or a configured failure). It performs no real stage
// work and must never be presented as a real backend.
type RecordingExecutor struct {
	st         *store.Store
	id         string
	mu         sync.Mutex
	Calls      []RecordedCall
	FailStages map[string]string // stage -> reason_code to fail with
}

// NewRecordingExecutor builds a RecordingExecutor with the given id
// (default "recording").
func NewRecordingExecutor(st *store.Store, id string) *RecordingExecutor {
	if id == "" {
		id = "recording"
	}
	return &RecordingExecutor{st: st, id: id, FailStages: map[string]string{}}
}

// ID returns the executor identity.
func (r *RecordingExecutor) ID() string { return r.id }

// ExecuteStage records the call, writes the evidence event and reports
// the configured outcome.
func (r *RecordingExecutor) ExecuteStage(ctx context.Context, run RunView, stage string) (StageOutcome, error) {
	r.mu.Lock()
	r.Calls = append(r.Calls, RecordedCall{RunID: run.RunID, Stage: stage})
	failReason := r.FailStages[stage]
	r.mu.Unlock()
	status := "done"
	if failReason != "" {
		status = "failed"
	}
	raw, _ := json.Marshal(map[string]any{
		"stage": stage, "executor": r.id, "outcome": status, "placeholder": true,
	})
	if _, err := r.st.AppendEvent(store.LifecycleEvent{
		FeatureID: run.FeatureID,
		RunID:     sql.NullString{String: run.RunID, Valid: true},
		EventType: "stage_executed",
		Payload:   raw,
	}); err != nil {
		return StageOutcome{}, err
	}
	if failReason != "" {
		return StageOutcome{Status: "failed", ReasonCode: failReason, Summary: "recording executor configured failure"}, nil
	}
	return StageOutcome{Status: "done", Summary: "recording executor placeholder (no real stage work performed)"}, nil
}

// CallCount returns how many times the executor was invoked.
func (r *RecordingExecutor) CallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Calls)
}
