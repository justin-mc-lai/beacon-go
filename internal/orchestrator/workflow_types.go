package orchestrator

import "beacon-go/internal/intenttree"

type WorkflowModule struct {
	ID       string   `json:"id"`
	NodeID   string   `json:"node_id"`
	Depends  []string `json:"depends_on"`
	Features []string `json:"features"`
}
type WorkflowCoverage struct {
	Source string `json:"source_node"`
	AC     string `json:"ac"`
	Target string `json:"target_node"`
}
type WorkflowContract struct {
	Schema      string             `json:"schema"`
	Goal        string             `json:"goal"`
	Scope       string             `json:"scope"`
	ScopeNodes  []string           `json:"scope_nodes"`
	SourceNodes []string           `json:"source_nodes"`
	RunAnchor   string             `json:"run_anchor,omitempty"`
	Acceptance  []string           `json:"acceptance_nodes,omitempty"`
	Modules     []WorkflowModule   `json:"modules"`
	Coverage    []WorkflowCoverage `json:"coverage"`
}
type workflowBinding struct {
	Contract         WorkflowContract `json:"contract"`
	Hash             string           `json:"hash"`
	By               string           `json:"by"`
	Path             string           `json:"path"`
	FileHash         string           `json:"file_hash"`
	ScopeFingerprint string           `json:"scope_fingerprint,omitempty"`
}
type workflowArtifacts struct {
	PreparationID string `json:"preparation_id"`
	Source        string `json:"source"`
	BindingHash   string `json:"binding_hash"`
	HTML          string `json:"html"`
	HTMLHash      string `json:"html_hash"`
	Manifest      string `json:"manifest"`
	ManifestHash  string `json:"manifest_hash"`
}
type WorkflowView struct {
	SourceHash   string   `json:"source_hash,omitempty"`
	PreparedHash string   `json:"prepared_hash,omitempty"`
	RunID        string   `json:"run_id"`
	Mode         string   `json:"mode"`
	Stage        string   `json:"current_stage"`
	Ready        bool     `json:"ready"`
	Missing      []string `json:"missing"`
	Next         string   `json:"next_action"`
	Owner        string   `json:"owner"`
	HTML         string   `json:"html_path,omitempty"`
	Manifest     string   `json:"coverage_manifest,omitempty"`
	ModuleCount  int      `json:"module_count"`
}

func (e *Engine) workflowInput(runID string) (WorkflowContract, string, intenttree.Index, map[string]intenttree.Node, error) {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return WorkflowContract{}, "", intenttree.Index{}, nil, err
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return WorkflowContract{}, "", intenttree.Index{}, nil, err
	}
	dir, err := intenttree.ResolveTreeLocation(fc.Project.RootPath, fc.Version.DocsRoot)
	if err != nil {
		return WorkflowContract{}, "", intenttree.Index{}, nil, err
	}
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		return WorkflowContract{}, dir, idx, nil, err
	}
	bodies := map[string]intenttree.Node{}
	for _, entry := range idx.Nodes {
		n, err := intenttree.ReadNode(dir, entry.ID)
		if err != nil {
			return WorkflowContract{}, dir, idx, nil, err
		}
		bodies[n.ID] = n
	}
	return WorkflowContract{}, dir, idx, bodies, nil
}
