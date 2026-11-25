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
	return intenttree.StageGate(dir, stage, fc.Feature.Slug)
}
