package orchestrator

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func (e *Engine) validateWorkflowDelivery(dir string, scoped map[string]bool, bodies map[string]intenttree.Node) error {
	for id := range scoped {
		if bodies[id].Kind == intenttree.KindTouchpoint && !workflowDeprecated(id, bodies) {
			if err := intenttree.ValidateVerifiedDelivery(e.store, dir, bodies[id]); err != nil {
				return err
			}
		}
	}
	return nil
}

func workflowDeprecated(id string, bodies map[string]intenttree.Node) bool {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		n := bodies[id]
		if n.Status == intenttree.StatusDeprecated {
			return true
		}
		id = n.ParentID
	}
	return false
}

// This combines only read-only contract/artifact and canonical facts. It must
// never call WorkflowCheck, admission, or stage exit evaluation.
func (e *Engine) workflowStageFacts(run store.Run, stage string) (bool, string, error) {
	v, err := e.workflowCheckStage(run.ID, stage)
	if err != nil {
		return false, "", err
	}
	if v.Mode == "legacy" {
		return true, "", nil
	}
	if !v.Ready {
		return false, v.Missing[0], nil
	}
	return e.treeStageFacts(run, stage, true)
}
