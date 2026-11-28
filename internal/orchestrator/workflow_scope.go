package orchestrator

import (
	"encoding/json"
	"strings"

	"beacon-go/internal/intenttree"
)

// Only scoped nodes and explicitly declared sources belong in the review
// artifact. Ancestors are navigation shells, not future delivery contracts.
func workflowRenderScope(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) (intenttree.Index, map[string]intenttree.Node, string, error) {
	scoped, err := workflowScopeAll(c, idx, bodies)
	if err != nil {
		return idx, nil, "", err
	}
	for _, id := range c.SourceNodes {
		scoped[id] = true
	}
	selected := map[string]intenttree.Node{}
	for id := range scoped {
		n := bodies[id]
		// This is a contract review projection. Runtime lifecycle and evidence
		// remain canonical facts but cannot invalidate a human contract review.
		n.Status = ""
		if workflowDeprecated(id, bodies) {
			n.Status = intenttree.StatusDeprecated
		}
		n.Implementation = nil
		n.VerificationEvidence = nil
		n.ChildFingerprints = nil
		n.Fingerprint = ""
		n.FingerprintAlg = ""
		var stampErr error
		n, stampErr = intenttree.Stamp(n)
		if stampErr != nil {
			return idx, nil, "", stampErr
		}
		selected[id] = n
		parent := n.ParentID
		for parent != "" {
			p := bodies[parent]
			if p.ID == "" {
				return idx, nil, "", qaError("workflow_contract_invalid", "missing render ancestor")
			}
			if !scoped[parent] {
				selected[parent] = intenttree.Node{ID: p.ID, ParentID: p.ParentID, Kind: p.Kind, Path: p.Path, Title: p.Title}
			}
			parent = p.ParentID
		}
	}
	view := intenttree.Index{Schema: idx.Schema, RootID: idx.RootID, Version: idx.Version}
	for _, entry := range idx.Nodes {
		if _, ok := selected[entry.ID]; !ok {
			continue
		}
		if !scoped[entry.ID] {
			entry = intenttree.IndexEntry{ID: entry.ID, ParentID: entry.ParentID, Kind: entry.Kind, Title: entry.Title, Path: entry.Path}
		}
		if scoped[entry.ID] {
			n := selected[entry.ID]
			entry.Status = n.Status
			entry.Fingerprint, entry.FingerprintAlg = n.Fingerprint, n.FingerprintAlg
		}
		view.Nodes = append(view.Nodes, entry)
	}
	// Source binding includes bodies as well as the renderer's index source.
	raw, err := json.Marshal(struct {
		Index  intenttree.Index
		Bodies map[string]intenttree.Node
	}{intenttree.SortedIndex(view), selected})
	if err != nil {
		return idx, nil, "", err
	}
	return view, selected, digestOf(string(raw)), nil
}

func validateWorkflowDraft(c WorkflowContract) error {
	if c.Schema != "beacon-workflow.v1" || strings.TrimSpace(c.Goal) == "" || strings.TrimSpace(c.Scope) == "" || len(c.ScopeNodes) == 0 {
		return qaError("workflow_contract_invalid", "schema, goal, scope and explicit scope_nodes required")
	}
	seen := map[string]bool{}
	for _, id := range c.ScopeNodes {
		if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) || seen[id] {
			return qaError("workflow_contract_invalid", "blank/duplicate scope node")
		}
		seen[id] = true
	}
	return nil
}

// Scope is independently declared; scheduling and coverage may not silently
// narrow it by omitting a canonical feature or touchpoint.
func workflowScope(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) (map[string]bool, error) {
	all, err := workflowScopeAll(c, idx, bodies)
	if err != nil {
		return nil, err
	}
	for id := range all {
		if workflowDeprecated(id, bodies) {
			delete(all, id)
		}
	}
	return all, nil
}

func workflowScopeAll(c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) (map[string]bool, error) {
	result := map[string]bool{}
	for _, root := range c.ScopeNodes {
		if bodies[root].ID == "" {
			return nil, qaError("workflow_contract_invalid", "unknown scope node")
		}
		for _, entry := range idx.Nodes {
			id := entry.ID
			seen := map[string]bool{}
			for id != "" {
				if seen[id] {
					return nil, qaError("workflow_contract_invalid", "ancestor cycle")
				}
				seen[id] = true
				if id == root {
					if result[entry.ID] {
						return nil, qaError("workflow_contract_invalid", "overlapping scope roots")
					}
					result[entry.ID] = true
					break
				}
				id = bodies[id].ParentID
			}
		}
	}
	return result, nil
}

func (e *Engine) workflowRunFeature(runID string, c WorkflowContract, idx intenttree.Index, bodies map[string]intenttree.Node) error {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return err
	}
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return err
	}
	var matches []string
	for _, n := range bodies {
		if n.Kind != intenttree.KindFeature || workflowDeprecated(n.ID, bodies) {
			continue
		}
		for _, slug := range intenttree.FeaturePackageSlugs(idx, n.ID, n.EffectiveTruthPath()) {
			if slug == fc.Feature.Slug {
				matches = append(matches, n.ID)
				break
			}
		}
	}
	if len(matches) != 1 {
		return qaError("workflow_feature_ambiguous", "run feature must resolve to exactly one canonical feature")
	}
	for _, module := range c.Modules {
		for _, id := range module.Features {
			if id == matches[0] {
				return nil
			}
		}
	}
	return qaError("workflow_feature_ambiguous", "run feature is outside declared delivery scope")
}
