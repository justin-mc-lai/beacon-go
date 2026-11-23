package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

func evidenceDigest(root string, refs []string) (string, error) {
	if len(refs) == 0 {
		return "", fmt.Errorf("QA evidence is empty")
	}
	paths := append([]string{}, refs...)
	sort.Strings(paths)
	var parts []string
	for _, ref := range paths {
		path := resolvePkgPath(root, ref)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("QA evidence missing or not a regular file: %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		parts = append(parts, path, digestOf(string(raw)))
	}
	return digestOf(parts...), nil
}

func (e *Engine) programRequirementFP(root, runID string) (string, error) {
	run, _, _, err := e.loadRun(runID)
	if err != nil {
		return "", err
	}
	feat, err := e.store.Features().GetByID(run.FeatureID)
	if err != nil {
		return "", err
	}
	pkg := e.packageFiles(store.FeatureContext{Project: store.Project{RootPath: root}, Feature: feat})
	coverage := pkg.CoverageFile()
	if !pkg.Truth.Present {
		return "", fmt.Errorf("requirement truth missing: %s", pkg.Truth.Path)
	}
	if !coverage.Present {
		return "", fmt.Errorf("requirement coverage missing: %s", coverage.Path)
	}
	return core.PackageFingerprint([]byte(pkg.Truth.Text), []byte(coverage.Text)), nil
}

func (e *Engine) lakeQADigest(root, runID string, refs []string) (string, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return "", err
	}
	if err := e.requireCurrentFormalQA(run, m); err != nil {
		return "", err
	}
	if m.Verifier == nil || !m.Verifier.Present || len(m.Verifier.Refs) == 0 ||
		strings.TrimSpace(m.Verifier.Source) == "" || m.Verifier.Source == m.ExecutorID {
		return "", fmt.Errorf("run %s lacks independent verifier", runID)
	}
	facts, _, err := e.runGateFacts(runID)
	if err != nil {
		return "", err
	}
	qa := facts["qa"]
	g, err := e.store.Gates().GetByID(qa.GateResultID)
	if err != nil || qa.Verdict != "pass" || g.Verdict != "pass" || g.FeatureID != run.FeatureID || g.GateType != "qa" {
		return "", fmt.Errorf("run %s lacks its own passing QA gate", runID)
	}
	arts, err := e.store.Artifacts().ListByRun(runID)
	if err != nil {
		return "", err
	}
	all := append(append([]string{}, refs...), m.Verifier.Refs...)
	for _, ref := range all {
		found := false
		for _, a := range arts {
			if a.AbsPath == resolvePkgPath(root, ref) && qaKind(a.Kind) {
				if stale, _ := store.ArtifactStaleness(a); !stale {
					found = true
				}
			}
		}
		if !found {
			return "", fmt.Errorf("run %s has no live registered QA artifact for %s", runID, ref)
		}
	}
	content, err := evidenceDigest(root, all)
	if err != nil {
		return "", err
	}
	return digestOf(runID, m.Verifier.Source, qa.GateResultID, content), nil
}

func (e *Engine) programCandidateFP(root, host string, plan *ProgramState) (string, error) {
	fp, err := e.programRequirementFP(root, host)
	if err != nil {
		return "", err
	}
	parts := []string{host, plan.Slug, fp}
	order, _ := TopoOrder(plan.Lakes)
	for _, slug := range order {
		l := plan.lake(slug)
		fp, err := e.programRequirementFP(root, l.RunID)
		if err != nil {
			return "", err
		}
		h := plan.handoffFor(slug)
		if h == nil {
			return "", fmt.Errorf("missing handoff: %s", slug)
		}
		parts = append(parts, slug, l.RunID, fp, h.ContractFP)
	}
	return digestOf(parts...), nil
}

func (e *Engine) verifyProgramCandidate(ctx context.Context, root, host string, plan *ProgramState) error {
	c := plan.Candidate
	fp, err := e.programCandidateFP(root, host, plan)
	if err != nil {
		return err
	}
	content, err := evidenceDigest(root, c.Evidence)
	if err != nil {
		return err
	}
	order, _ := TopoOrder(plan.Lakes)
	if fp != c.ContractFP || c.EvidenceDigest != digestOf(c.SHA, fp, content) || strings.Join(order, "\x00") != strings.Join(c.Lakes, "\x00") || len(c.Runs) != len(order) {
		return core.NewError(core.ReasonProgramCandidateChanged, "program.candidate", "candidate evidence or requirement binding changed; re-run total QA")
	}
	for i, slug := range order {
		if c.Runs[i] != plan.lake(slug).RunID || !e.liveHandoffValidity(ctx, root, plan, plan.handoffFor(slug), c.SHA) {
			return core.NewError(core.ReasonProgramHandoffInvalid, "program.candidate", "lake handoff is no longer live: "+slug)
		}
	}
	return nil
}

// Reuse the normal completion and formal release evaluators. Confirmation is
// persisted once, so resuming after one lake closed never signs that lake twice.
func (e *Engine) completeProgramRun(runID string, approval *ProgramApproval) error {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return err
	}
	if m.LoopStage == string(core.LoopDone) {
		return nil
	}
	if m.LoopStage != string(core.LoopReleaseGate) {
		return core.NewError(core.ReasonNotAtReleaseGate, "program.finalize", "run "+runID+" is not at release gate")
	}
	by := "program:" + approval.SignedBy
	if !m.ReleaseHumanConfirmed {
		m.ReleaseHumanConfirmed, m.HumanConfirmedBy = true, by
		if err := e.store.WithTx(func(tx *store.Tx) error {
			if err := e.appendEventOn(tx, run, "release_confirmed", map[string]any{"by": by, "candidate_sha": approval.CandidateSHA, "evidence_digest": approval.EvidenceDigest}); err != nil {
				return err
			}
			return e.saveRunOn(tx, run, m)
		}); err != nil {
			return err
		}
	} else if m.HumanConfirmedBy != by {
		return fmt.Errorf("run %s has a different human binding", runID)
	}
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("missing feature context: %s", runID)
	}
	v, err := e.EvaluateGate(fc, "release", runID, by)
	if err != nil {
		return err
	}
	if v.Verdict != gate.VerdictPass {
		return core.NewError(core.ReasonProgramCandidateNotReady, "program.finalize", "formal release gate blocked for "+runID+": "+strings.Join(v.ReasonCodes, ","))
	}
	_, err = e.Complete(runID, CompleteOptions{})
	return err
}
