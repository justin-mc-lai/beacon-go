package orchestrator

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func (e *Engine) BindWorkflow(runID, path, by string) (WorkflowView, error) {
	unlock, err := e.lockProgram(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	defer unlock()
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	if (m.currentStage() != StagePlan && m.currentStage() != StageTruth) || strings.TrimSpace(by) == "" {
		return WorkflowView{}, qaError("workflow_bind_invalid", "bind only at plan/truth with owner")
	}
	var c WorkflowContract
	if err := qaRead(path, &c); err != nil {
		return WorkflowView{}, err
	}
	if err := validateWorkflowDraft(c); err != nil {
		return WorkflowView{}, err
	}
	raw, _ := json.Marshal(c)
	path, err = filepath.Abs(path)
	if err != nil {
		return WorkflowView{}, err
	}
	fileHash, err := qaHash(path)
	if err != nil {
		return WorkflowView{}, err
	}
	binding := workflowBinding{Contract: c, Hash: digestOf(string(raw)), By: by, Path: path, FileHash: fileHash}
	payload := map[string]any{}
	encoded, _ := json.Marshal(binding)
	_ = json.Unmarshal(encoded, &payload)
	if err := e.appendEvent(run, "workflow_contract_bound", payload); err != nil {
		return WorkflowView{}, err
	}
	return e.WorkflowCheck(runID)
}

func (e *Engine) workflowBinding(runID string) (*workflowBinding, *workflowArtifacts, error) {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return nil, nil, err
	}
	var bound *workflowBinding
	var artifacts *workflowArtifacts
	for _, event := range events {
		switch event.EventType {
		case "workflow_contract_bound":
			var b workflowBinding
			if err := json.Unmarshal(event.Payload, &b); err != nil {
				return nil, nil, err
			}
			raw, _ := json.Marshal(b.Contract)
			if b.Hash != digestOf(string(raw)) {
				return nil, nil, qaError("workflow_contract_invalid", "binding hash mismatch")
			}
			bound = &b
			artifacts = nil
		case "workflow_prepared":
			var a workflowArtifacts
			if err := json.Unmarshal(event.Payload, &a); err != nil {
				return nil, nil, err
			}
			artifacts = &a
		}
	}
	return bound, artifacts, nil
}
