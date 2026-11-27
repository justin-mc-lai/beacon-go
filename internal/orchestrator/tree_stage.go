package orchestrator

import (
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// treeStageGate delegates a goal stage's readiness to the canonical
// tree. Projects without a tree keep the legacy behaviour.
func (e *Engine) treeStageGate(run store.Run, stage string) (bool, string, error) {
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return true, "", nil
	}
	dir := intenttree.TreeDir(fc.Project.RootPath, fc.Version.DocsRoot)
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
	if !intenttree.HasIndex(intenttree.TreeDir(fc.Project.RootPath, fc.Version.DocsRoot)) {
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
	dir := intenttree.TreeDir(fc.Project.RootPath, fc.Version.DocsRoot)
	if !intenttree.HasIndex(dir) {
		return nil
	}
	return intenttree.ValidateFeatureDelivery(e.store, dir, fc.Feature.Slug)
}
