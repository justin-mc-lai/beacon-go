// Truth Integrity Gate service (P3a, AC-101): gathers one feature
// package (contract-required members) + the feature's registered artifact
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
	ReasonCode   string         `json:"reason_code,omitempty"`
	Findings     []gate.Finding `json:"findings,omitempty"`
	Evaluator    string         `json:"evaluator"`
	FeatureID    string         `json:"feature_id"`
	FeatureSlug  string         `json:"feature_slug,omitempty"`
	GateResultID string         `json:"gate_result_id,omitempty"`
	EvaluatedAt  string         `json:"evaluated_at,omitempty"`
	RepairRoute  string         `json:"repair_route,omitempty"`
}

// packageFiles gathers package members from registered paths and canonical_refs.
// SDD coverage always comes from truth.md, regardless of a legacy TestsPath.
func (e *Engine) packageFiles(ctx store.FeatureContext) gate.PackageFiles {
	root := ctx.Project.RootPath
	feat := ctx.Feature
	pkg := gate.PackageFiles{Slug: feat.Slug, Truth: readPkgFile(root, feat.TruthPath)}
	pkg.Contract = packageContract(pkg.Truth.Text)
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
	if pkg.Contract == SDDContract {
		pkg.Tests = pkg.CoverageFile()
	} else {
		pkg.Tests = readPkgFile(root, testsPath)
	}
	pkg.Tasks = readPkgFile(root, tasksPath)
	pkg.Evidence = readPkgFile(root, evidencePath)
	pkg.Restatement = readPkgFile(root, siblingPkgPath(feat.TruthPath, gate.RestatementFile))
	return pkg
}

// surfaceDrifted reports whether the live package's contract face moved
// off the recorded freeze fingerprint. Evidence edits are the byproduct
// of implementation and do not drift a frozen package (AC-001); legacy
// four-surface records still compare fresh through their own hash.
func surfaceDrifted(recorded string, pkg gate.PackageFiles) bool {
	return core.PackageSurfaceDrifted(recorded,
		[]byte(pkg.Truth.Text), []byte(pkg.Tests.Text), []byte(pkg.Tasks.Text),
		[]byte(pkg.Evidence.Text), []byte(pkg.Restatement.Text))
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
	// Feature-scoped registrations (AC-002) count as evidence too: a
	// package can register its artifacts with no run in existence.
	if arts, ferr := e.store.Artifacts().ListByFeatureScope(ctx.Feature.ID); ferr == nil {
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
		FileNonTrivial: func(abs string) bool {
			st, err := os.Stat(abs)
			return err == nil && st.Size() >= 2
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
	reasonCode := ""
	if len(rep.ReasonCodes) > 0 {
		reasonCode = rep.ReasonCodes[0]
	}
	return IntegrityView{
		GateType: string(gate.GateTruthIntegrity), Verdict: string(rep.Verdict),
		Mode: mode, ReasonCodes: rep.ReasonCodes, ReasonCode: reasonCode, Findings: rep.Findings,
		Evaluator: evaluator, FeatureID: ctx.Feature.ID, FeatureSlug: ctx.Feature.Slug,
		GateResultID: ev.GateResultID, EvaluatedAt: ev.EvaluatedAt,
		RepairRoute: gate.DefaultRepairRoute,
	}, nil
}
