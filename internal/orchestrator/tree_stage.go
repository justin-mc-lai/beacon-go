package orchestrator

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// treeStageGate delegates a goal stage's readiness to the canonical
// tree. Projects without a tree keep the legacy behaviour.
func (e *Engine) treeStageGate(run store.Run, stage string) (bool, string, error) {
	view, err := e.workflowCheckStage(run.ID, stage)
	if err != nil {
		return false, "", err
	}
	if !view.Ready {
		return false, view.Missing[0], nil
	}
	return e.treeStageFacts(run, stage, view.Mode == "stable")
}

// Facts never call workflow/status/tick; all entry points can share them.
func (e *Engine) treeStageFacts(run store.Run, stage string, stable bool) (bool, string, error) {
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return true, "", nil
	}
	dir, err := intenttree.ResolveTreeLocation(fc.Project.RootPath, fc.Version.DocsRoot)
	if err != nil {
		return false, "", err
	}
	if stage == StagePlan {
		return true, "", nil
	}
	if stable {
		if !intenttree.HasIndex(dir) {
			return false, "workflow_tree_missing", nil
		}
		bound, _, err := e.workflowBinding(run.ID)
		if err != nil {
			return false, "", err
		}
		_, _, idx, bodies, err := e.workflowInput(run.ID)
		if err != nil {
			return false, "", err
		}
		scoped, err := workflowScope(bound.Contract, idx, bodies)
		if err != nil {
			return false, "workflow_contract_invalid", err
		}
		for id := range scoped {
			n := bodies[id]
			switch stage {
			case StageTruth, StageImplement:
				if n.Status != intenttree.StatusFrozen && n.Status != intenttree.StatusImplemented && n.Status != intenttree.StatusVerified && n.Status != intenttree.StatusDelivered {
					return false, "tree_scope_not_frozen", nil
				}
			case StageQA, StageRelease:
				if workflowDeprecated(id, bodies) {
					continue
				}
				if n.Kind != intenttree.KindFeature && n.Kind != intenttree.KindTouchpoint {
					continue
				}
				if n.Status != intenttree.StatusVerified && n.Status != intenttree.StatusDelivered {
					return false, "tree_scope_not_verified", nil
				}
			}
		}
		// Stable delivery covers the declared current scope, not future root work.
		if stage == StageQA || stage == StageRelease {
			if err := e.validateWorkflowDelivery(dir, scoped, bodies); err != nil {
				return false, "delivery_evidence_invalid", nil
			}
			if err := e.validateDeliveryTasks(run); err != nil {
				return false, "delivery_ledger_open", err
			}
		}
		return true, "", nil
	}
	ready, reason, err := intenttree.StageGate(dir, stage, fc.Feature.Slug)
	if err != nil || !ready {
		return ready, reason, err
	}
	if (stage == "qa" || stage == "release") && intenttree.HasIndex(dir) {
		if err := e.validateTreeDelivery(fc); err != nil {
			return false, "delivery_evidence_invalid", err
		}
		if err := e.validateDeliveryTasks(run); err != nil {
			return false, "delivery_ledger_open", err
		}
	}
	return true, "", nil
}

func (e *Engine) validateDeliveryTasks(run store.Run) error {
	fc, err := e.featureContext(run.FeatureID)
	if err != nil {
		return err
	}
	dir, err := intenttree.ResolveTreeLocation(fc.Project.RootPath, fc.Version.DocsRoot)
	if err != nil {
		return err
	}
	if !intenttree.HasIndex(dir) {
		return nil
	}
	if e.packageFiles(fc).Contract != SDDContract {
		return nil
	}
	report, err := e.TaskStatus(run.ID)
	if err != nil {
		return qaError("delivery_ledger_open", err.Error())
	}
	if report.Status != "passed" || len(report.Results) == 0 {
		return qaError("delivery_ledger_open", "SDD task invocation must pass every task")
	}
	for _, result := range report.Results {
		if result.Status != "passed" {
			return qaError("delivery_ledger_open", result.TaskID)
		}
	}
	return nil
}

func (e *Engine) validateTreeDelivery(fc store.FeatureContext) error {
	dir, err := intenttree.ResolveTreeLocation(fc.Project.RootPath, fc.Version.DocsRoot)
	if err != nil {
		return err
	}
	if !intenttree.HasIndex(dir) {
		return nil
	}
	return intenttree.ValidateFeatureDelivery(e.store, dir, fc.Feature.Slug)
}
