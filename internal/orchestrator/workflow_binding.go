package orchestrator

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"beacon-go/internal/store"
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
	var delivery *WorkflowDelivery
	// Plan accepts a draft contract before the tree exists. Once modules are
	// declared, persist the deterministic plan with its binding in one write.
	if len(c.Modules) > 0 {
		_, _, idx, bodies, err := e.workflowInput(runID)
		if err != nil {
			return WorkflowView{}, err
		}
		d, err := resolveWorkflowDelivery(c, idx, bodies)
		if err != nil {
			return WorkflowView{}, err
		}
		fc, err := e.featureContext(run.FeatureID)
		if err != nil {
			return WorkflowView{}, err
		}
		d = namespaceWorkflowDelivery(d, fc.Project.ID, fc.Version.ID)
		delivery = &d
		binding.ScopeFingerprint = d.Fingerprint
		payload["scope_fingerprint"] = d.Fingerprint
	}
	previous, _, err := e.workflowBinding(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	if previous != nil && previous.Hash == binding.Hash && previous.FileHash == binding.FileHash && previous.Path == binding.Path && previous.ScopeFingerprint == binding.ScopeFingerprint {
		return e.WorkflowCheck(runID)
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "workflow_contract_bound", payload); err != nil {
			return err
		}
		if delivery != nil {
			raw, err := json.Marshal(delivery)
			if err != nil {
				return err
			}
			var taskPayload map[string]any
			if err := json.Unmarshal(raw, &taskPayload); err != nil {
				return err
			}
			taskPayload["binding_hash"] = binding.Hash
			return e.appendEventOn(tx, run, "workflow_tasks_planned", taskPayload)
		}
		return nil
	}); err != nil {
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
