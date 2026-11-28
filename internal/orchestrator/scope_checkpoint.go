package orchestrator

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// Captured before opening a store transaction: reading Engine.store inside
// checkpointOn would re-enter the single SQLite connection and deadlock.
type workflowCheckpoint struct {
	Binding string `json:"workflow_binding_hash,omitempty"`
	Scope   string `json:"workflow_scope_fingerprint,omitempty"`
}

func (e *Engine) workflowCheckpointInput(runID string) (workflowCheckpoint, error) {
	b, _, err := e.workflowBinding(runID)
	if err != nil || b == nil {
		return workflowCheckpoint{}, err
	}
	h, err := qaHash(b.Path)
	if err != nil || h != b.FileHash {
		return workflowCheckpoint{}, qaError("workflow_contract_changed", "cannot checkpoint changed contract")
	}
	result := workflowCheckpoint{Binding: b.Hash}
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return workflowCheckpoint{}, err
	}
	if m.currentStage() != StagePlan || len(b.Contract.Modules) > 0 {
		d, err := e.WorkflowTasks(runID)
		if err != nil {
			return workflowCheckpoint{}, err
		}
		result.Scope = d.Fingerprint
	}
	return result, nil
}

func (e *Engine) workflowCheckpointCurrent(runID string) (bool, error) {
	c, err := e.store.Runs().LatestCheckpoint(runID)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	var previous workflowCheckpoint
	if err := json.Unmarshal(c.State, &previous); err != nil {
		return false, err
	}
	current, err := e.workflowCheckpointInput(runID)
	if err != nil {
		return false, err
	}
	// A stable run may resume without a checkpoint; an existing legacy
	// checkpoint cannot authorize a newly bound workflow.
	return previous.Binding == current.Binding && (previous.Scope == "" || previous.Scope == current.Scope), nil
}
