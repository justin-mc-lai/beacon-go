package orchestrator

import (
	"encoding/json"
	"strings"
)

type workflowConfirmation struct {
	By           string `json:"by"`
	PreparedHash string `json:"prepared_hash"`
	BindingHash  string `json:"binding_hash"`
	SourceHash   string `json:"source_hash"`
	HTMLHash     string `json:"html_hash"`
	ManifestHash string `json:"manifest_hash"`
}

func workflowPreparedHash(a workflowArtifacts) string {
	raw, _ := json.Marshal(a)
	return digestOf(string(raw))
}

func (e *Engine) workflowConfirmed(runID string, a workflowArtifacts) (bool, error) {
	events, err := e.store.ListEventsByRun(runID)
	if err != nil {
		return false, err
	}
	confirmed := false
	for _, event := range events {
		switch event.EventType {
		case "workflow_contract_bound", "workflow_prepared":
			confirmed = false
		case "workflow_confirmed":
			var c workflowConfirmation
			if err := json.Unmarshal(event.Payload, &c); err != nil {
				return false, err
			}
			confirmed = strings.TrimSpace(c.By) != "" && c.PreparedHash == workflowPreparedHash(a) && c.BindingHash == a.BindingHash && c.SourceHash == a.Source && c.HTMLHash == a.HTMLHash && c.ManifestHash == a.ManifestHash
		}
	}
	return confirmed, nil
}

// Confirmation is explicit and bound to exactly the prepared review artifact.
// The preflight shares the same read-only facts as check and stage advancement.
func (e *Engine) ConfirmWorkflow(runID, expectedPreparedHash, by string) (WorkflowView, error) {
	unlock, err := e.lockProgram(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	defer unlock()
	if strings.TrimSpace(by) == "" || strings.TrimSpace(expectedPreparedHash) == "" {
		return WorkflowView{}, qaError("workflow_confirmation_invalid", "identity and expected prepared hash required")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	v, err := e.workflowCheckStage(runID, m.currentStage())
	if err != nil {
		return v, err
	}
	if v.Mode != "stable" || v.PreparedHash != expectedPreparedHash || (len(v.Missing) > 0 && v.Missing[0] != "workflow_confirmation_required") {
		return v, qaError("workflow_confirmation_invalid", "review artifact missing, stale or expected hash differs")
	}
	_, a, err := e.workflowBinding(runID)
	if err != nil {
		return v, err
	}
	if a == nil {
		return v, qaError("workflow_confirmation_invalid", "prepare first")
	}
	if workflowPreparedHash(*a) != expectedPreparedHash {
		return v, qaError("workflow_confirmation_invalid", "prepared generation changed")
	}
	if e.workflowConfirmCaptured != nil {
		e.workflowConfirmCaptured()
	}
	// Re-read under the writer lock, compare again, and only write captured A.
	_, current, err := e.workflowBinding(runID)
	if err != nil {
		return v, err
	}
	if current == nil || workflowPreparedHash(*current) != expectedPreparedHash {
		return v, qaError("workflow_confirmation_invalid", "prepared generation changed")
	}
	c := workflowConfirmation{By: strings.TrimSpace(by), PreparedHash: workflowPreparedHash(*a), BindingHash: a.BindingHash, SourceHash: a.Source, HTMLHash: a.HTMLHash, ManifestHash: a.ManifestHash}
	raw, _ := json.Marshal(c)
	payload := map[string]any{}
	_ = json.Unmarshal(raw, &payload)
	if err := e.appendEvent(run, "workflow_confirmed", payload); err != nil {
		return v, err
	}
	return e.WorkflowCheck(runID)
}
