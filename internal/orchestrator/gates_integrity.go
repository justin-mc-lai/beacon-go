// Truth Integrity Gate service (P3a, AC-101): gathers one feature
// package (four-piece set) + the feature's registered artifact
// pointers, runs gate.CheckIntegrity, and persists the verdict under
// gate_type=truth_integrity.
package orchestrator

import (
	"os"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// IntegrityView is the persisted Truth Integrity Gate outcome.
type IntegrityView struct {
	GateType     string         `json:"gate_type"`
	Verdict      string         `json:"verdict"`
	Mode         string         `json:"mode"`
	ReasonCodes  []string       `json:"reason_codes,omitempty"`
	Findings     []gate.Finding `json:"findings,omitempty"`
	Evaluator    string         `json:"evaluator"`
	FeatureID    string         `json:"feature_id"`
	FeatureSlug  string         `json:"feature_slug,omitempty"`
	GateResultID string         `json:"gate_result_id,omitempty"`
	EvaluatedAt  string         `json:"evaluated_at,omitempty"`
	RepairRoute  string         `json:"repair_route,omitempty"`
}

// packageFiles gathers the four-piece package for the integrity
// check: truth/tests paths come from the feature row (tests falling
// back to the sibling file), tasks/evidence from the truth
// frontmatter's canonical_refs when declared, else siblings.
func (e *Engine) packageFiles(ctx store.FeatureContext) gate.PackageFiles {
	root := ctx.Project.RootPath
	feat := ctx.Feature
	pkg := gate.PackageFiles{Slug: feat.Slug, Truth: readPkgFile(root, feat.TruthPath)}
	testsPath, tasksPath, evidencePath := feat.TestsPath, siblingPkgPath(feat.TruthPath, "tasks.md"), siblingPkgPath(feat.TruthPath, "evidence.md")
	if pkg.Truth.Present {
		_, refs, ok := gate.ParseFrontmatter(pkg.Truth.Text)
		if ok {
			if v := refs["tests"]; v != "" && testsPath == "" {
				testsPath = v
			}
			if v := refs["tasks"]; v != "" {
				tasksPath = v
			}
			if v := refs["evidence"]; v != "" {
				evidencePath = v
			}
		}
	}
	if testsPath == "" {
		testsPath = siblingPkgPath(feat.TruthPath, "tests.md")
	}
	pkg.Tests = readPkgFile(root, testsPath)
	pkg.Tasks = readPkgFile(root, tasksPath)
	pkg.Evidence = readPkgFile(root, evidencePath)
	return pkg
}

// integrityReportLive runs the integrity check WITHOUT persisting
// (the fact gathering + mode default + CheckIntegrity call that
// EvaluateIntegrity persists, and the version release check /
// truth-map face read live). Returns the report, the gathered
// package and the effective mode.
func (e *Engine) integrityReportLive(ctx store.FeatureContext, mode string) (gate.IntegrityReport, gate.PackageFiles, string, error) {
	pkg := e.packageFiles(ctx)
	if mode == "" {
		mode = gate.IntegrityModeBlock
		scalars, _, ok := gate.ParseFrontmatter(pkg.Truth.Text)
		if (ok && scalars["status"] == "frozen") || ctx.Feature.Status == "frozen" {
			mode = gate.IntegrityModeAuditOnly
		}
	}
	root := ctx.Project.RootPath
	var artifacts []gate.ArtifactFact
	runs, err := e.store.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil {
		return gate.IntegrityReport{}, pkg, mode, err
	}
	for _, run := range runs {
		arts, err := e.store.Artifacts().ListByRun(run.ID)
		if err != nil {
			return gate.IntegrityReport{}, pkg, mode, err
		}
		for _, a := range arts {
			artifacts = append(artifacts, gate.ArtifactFact{AbsPath: a.AbsPath})
		}
	}
	rep := gate.CheckIntegrity(gate.IntegrityInput{
		Pkg: pkg, Artifacts: artifacts,
		ResolveRef: func(ref string) string { return resolvePkgPath(root, ref) },
		FileExists: func(abs string) bool {
			_, err := os.Stat(abs)
			return err == nil
		},
	}, mode)
	return rep, pkg, mode, nil
}

// EvaluateIntegrity runs the Truth Integrity Gate (AC-101) and
// persists the verdict under gate_type=truth_integrity. mode is
// "block", "audit_only", or "" for the AC-101 default: an
// already-frozen package is audited without blocking, anything else
// blocks on fractures.
func (e *Engine) EvaluateIntegrity(ctx store.FeatureContext, evaluator, mode string) (IntegrityView, error) {
	if strings.TrimSpace(evaluator) == "" {
		return IntegrityView{}, core.NewError(core.ReasonGateEvaluatorRequired, "gate.EvaluateIntegrity",
			"an evaluator identity is required (--evaluator); unattributed verdicts are not recorded")
	}
	if mode != "" && mode != gate.IntegrityModeBlock && mode != gate.IntegrityModeAuditOnly {
		return IntegrityView{}, core.NewError(core.ReasonGateTypeUnknown, "gate.EvaluateIntegrity",
			"unknown integrity mode "+mode+" (block | audit_only)")
	}
	rep, _, mode, err := e.integrityReportLive(ctx, mode)
	if err != nil {
		return IntegrityView{}, err
	}
	ev := gate.Evaluation{
		GateType: gate.GateTruthIntegrity, Verdict: rep.Verdict,
		ReasonCodes: rep.ReasonCodes, Evaluator: evaluator,
	}
	ev, err = e.persistGateResult(ctx, ev, "", EventTruthIntegrityEvaluated, map[string]any{
		"mode": mode, "findings": len(rep.Findings),
	})
	if err != nil {
		return IntegrityView{}, err
	}
	return IntegrityView{
		GateType: string(gate.GateTruthIntegrity), Verdict: string(rep.Verdict),
		Mode: mode, ReasonCodes: rep.ReasonCodes, Findings: rep.Findings,
		Evaluator: evaluator, FeatureID: ctx.Feature.ID, FeatureSlug: ctx.Feature.Slug,
		GateResultID: ev.GateResultID, EvaluatedAt: ev.EvaluatedAt,
		RepairRoute: gate.DefaultRepairRoute,
	}, nil
}
