package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"

	"beacon-go/internal/intenttree"
)

func (e *Engine) PrepareWorkflow(runID string) (WorkflowView, error) {
	unlock, err := e.lockProgram(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	defer unlock()
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	bound, _, err := e.workflowBinding(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	if bound == nil {
		return WorkflowView{}, qaError("workflow_contract_missing", "bind contract first")
	}
	fileHash, fileErr := qaHash(bound.Path)
	if fileErr != nil || fileHash != bound.FileHash {
		return WorkflowView{}, qaError("workflow_contract_changed", "explicitly rebind modified contract")
	}
	_, _, idx, bodies, err := e.workflowInput(runID)
	if err != nil {
		return WorkflowView{}, err
	}
	if err := validateWorkflow(bound.Contract, idx, bodies); err != nil {
		return WorkflowView{}, err
	}
	if err := e.workflowRunFeature(runID, bound.Contract, idx, bodies); err != nil {
		return WorkflowView{}, err
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return WorkflowView{}, err
	}
	base := filepath.Join(fc.Project.RootPath, ".beacon", "workflows", runID)
	if err := os.MkdirAll(base, 0700); err != nil {
		return WorkflowView{}, err
	}
	base, err = os.MkdirTemp(base, "prepared-")
	if err != nil {
		return WorkflowView{}, err
	}
	viewIndex, viewBodies, source, err := workflowRenderScope(bound.Contract, idx, bodies)
	if err != nil {
		return WorkflowView{}, err
	}
	a := workflowArtifacts{PreparationID: filepath.Base(base), Source: source, BindingHash: bound.Hash, HTML: filepath.Join(base, "index.html"), Manifest: filepath.Join(base, "coverage.json")}
	if err := os.WriteFile(a.HTML, intenttree.RenderHTMLTree(viewIndex, viewBodies), 0600); err != nil {
		return WorkflowView{}, err
	}
	raw, _ := json.MarshalIndent(bound.Contract, "", "  ")
	if err := os.WriteFile(a.Manifest, raw, 0600); err != nil {
		return WorkflowView{}, err
	}
	a.HTMLHash, err = qaHash(a.HTML)
	if err != nil {
		return WorkflowView{}, err
	}
	a.ManifestHash, err = qaHash(a.Manifest)
	if err != nil {
		return WorkflowView{}, err
	}
	payload := map[string]any{}
	encoded, _ := json.Marshal(a)
	_ = json.Unmarshal(encoded, &payload)
	if err := e.appendEvent(run, "workflow_prepared", payload); err != nil {
		return WorkflowView{}, err
	}
	return e.WorkflowCheck(runID)
}
