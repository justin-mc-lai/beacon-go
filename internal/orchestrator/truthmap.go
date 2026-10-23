// Truth-map standalone face (R2b, parity audit M16): Python
// commands/truth_map.py exposes resolve/verify as their own
// surface over "the static resolver + the markdown feature
// package" (authority_source, verbatim). In Go the same judgement
// previously existed ONLY inside the admission bundle
// (bundleTruthMap). This file holds the ONE shared judgement —
// layout problem enumeration — consumed by the bundle item and by
// the standalone Engine.TruthMapResolve / TruthMapVerify faces,
// plus the verify face's freshness leg (the frozen fingerprint vs
// the package's current fingerprint, reported with the source's
// machine_json_source_hash_stale diagnostic code).
package orchestrator

import (
	"os"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// TruthMapContract is truth_map.py's contract id (verbatim).
const TruthMapContract = "truth_map"

// TruthMapAuthoritySource is truth_map.py's resolve payload field
// (verbatim): the map is derived from the CLI's static resolver
// plus the markdown package; no machine JSON is authoritative.
const TruthMapAuthoritySource = "cli_static_resolver_plus_markdown_feature_package"

// TruthMapDiagnostic is one verify diagnostic (the source's
// {code, severity, message} shape).
type TruthMapDiagnostic struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error | info
	Message  string `json:"message"`
}

// TruthMapView is the resolve/verify payload. Resolve fields
// mirror resolve_feature_truth_map's payload; verify adds Status
// (pass | blocked, the source's verify vocabulary), diagnostics
// and the machine-JSON freshness leg.
type TruthMapView struct {
	Contract        string `json:"contract"`
	Mode            string `json:"mode"` // resolve | verify
	Status          string `json:"status,omitempty"`
	FeatureID       string `json:"feature_id"`
	FeatureSlug     string `json:"feature_slug"`
	TruthPackageDir string `json:"truth_package_dir"`
	TruthDoc        string `json:"truth_doc"`
	TestsDoc        string `json:"tests_doc,omitempty"`
	TasksDoc        string `json:"tasks_doc,omitempty"`
	EvidenceDoc     string `json:"evidence_doc,omitempty"`
	Frozen          bool   `json:"frozen"`
	AuthoritySource string `json:"authority_source"`
	JSONAuthority   bool   `json:"json_authority"`
	// verify legs
	PackagePresent    bool                 `json:"package_present"`
	LayoutOK          bool                 `json:"layout_ok"`
	FingerprintStatus string               `json:"fingerprint_status,omitempty"` // fresh | stale | no_freeze_recorded
	Diagnostics       []TruthMapDiagnostic `json:"diagnostics,omitempty"`
	ReasonCodes       []string             `json:"reason_codes,omitempty"`
}

// truthMapLayoutProblems is the ONE layout judgement shared by
// the bundle item and the standalone faces: it enumerates the
// package's layout problems exactly as the bundle always did
// (missing members, unparseable frontmatter, slug mismatch,
// dangling canonical_refs).
func (e *Engine) truthMapLayoutProblems(fc store.FeatureContext) (gate.PackageFiles, bool, []string) {
	pkg := e.packageFiles(fc)
	if !packagePresent(pkg) {
		return pkg, false, nil
	}
	var problems []string
	for _, f := range []struct {
		name string
		pf   gate.PackageFile
	}{
		{"truth", pkg.Truth}, {"tests", pkg.Tests}, {"tasks", pkg.Tasks}, {"evidence", pkg.Evidence},
	} {
		if !f.pf.Present {
			problems = append(problems, "registered "+f.name+" member missing on disk: "+f.pf.Path)
		}
	}
	if pkg.Truth.Present {
		scalars, refs, parsed := gate.ParseFrontmatter(pkg.Truth.Text)
		if !parsed {
			problems = append(problems, "truth.md frontmatter unparseable; layout cannot be verified")
		} else {
			if scalars["slug"] != "" && scalars["slug"] != fc.Feature.Slug {
				problems = append(problems, "truth.md slug "+scalars["slug"]+" != registered slug "+fc.Feature.Slug)
			}
			for name, ref := range refs {
				if _, err := os.Stat(resolvePkgPath(fc.Project.RootPath, ref)); err != nil {
					problems = append(problems, "canonical_refs."+name+" points at missing file "+ref)
				}
			}
		}
	}
	return pkg, true, problems
}

func (e *Engine) truthMapContext(projectRoot, slug string) (store.FeatureContext, error) {
	root := store.ResolveRootPath(projectRoot)
	return e.store.ResolveFeature(root, slug, "")
}

func truthMapBaseView(fc store.FeatureContext, pkg gate.PackageFiles, mode string) TruthMapView {
	v := TruthMapView{
		Contract: TruthMapContract, Mode: mode,
		FeatureID: fc.Feature.ID, FeatureSlug: fc.Feature.Slug,
		AuthoritySource: TruthMapAuthoritySource, JSONAuthority: false,
	}
	if pkg.Truth.Present {
		v.TruthPackageDir = pkgDirOf(pkg.Truth.Path)
		v.TruthDoc = pkg.Truth.Path
	}
	if pkg.Tests.Present {
		v.TestsDoc = pkg.Tests.Path
	}
	if pkg.Tasks.Present {
		v.TasksDoc = pkg.Tasks.Path
	}
	if pkg.Evidence.Present {
		v.EvidenceDoc = pkg.Evidence.Path
	}
	return v
}

func pkgDirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return ""
}

// TruthMapResolve is the standalone resolve face: the package's
// registration + document pointers, derived — never read from a
// machine JSON (json_authority=false, as in the source).
func (e *Engine) TruthMapResolve(projectRoot, slug string) (TruthMapView, error) {
	fc, err := e.truthMapContext(projectRoot, slug)
	if err != nil {
		return TruthMapView{}, err
	}
	pkg, present, _ := e.truthMapLayoutProblems(fc)
	v := truthMapBaseView(fc, pkg, "resolve")
	v.PackagePresent = present
	v.LayoutOK = present
	if frozen, ferr := e.FrozenFact(fc); ferr == nil {
		v.Frozen = frozen.Frozen
	}
	if !present {
		v.Status = "blocked"
		v.ReasonCodes = []string{core.ReasonTruthMapNotResolved}
		v.Diagnostics = []TruthMapDiagnostic{{
			Code: core.ReasonTruthMapNotResolved, Severity: "error",
			Message: "no on-disk package resolves for the registered feature",
		}}
		return v, nil
	}
	v.Status = "resolved"
	return v, nil
}

// TruthMapVerify is the standalone verify face: layout judgement
// (shared with the bundle) + the machine-JSON freshness leg — a
// frozen package whose current fingerprint no longer matches the
// recorded freeze fingerprint is reported with the source's
// machine_json_source_hash_stale diagnostic. Status follows the
// source: blocked when any error diagnostic exists, else pass.
func (e *Engine) TruthMapVerify(projectRoot, slug string) (TruthMapView, error) {
	fc, err := e.truthMapContext(projectRoot, slug)
	if err != nil {
		return TruthMapView{}, err
	}
	pkg, present, problems := e.truthMapLayoutProblems(fc)
	v := truthMapBaseView(fc, pkg, "verify")
	v.PackagePresent = present
	v.LayoutOK = present && len(problems) == 0
	if !present {
		v.Diagnostics = append(v.Diagnostics, TruthMapDiagnostic{
			Code: core.ReasonTruthMapNotResolved, Severity: "error",
			Message: "feature package did not resolve to an on-disk package",
		})
	}
	for _, p := range problems {
		v.Diagnostics = append(v.Diagnostics, TruthMapDiagnostic{
			Code: core.ReasonStandardFlowVerificationIncm, Severity: "error", Message: p,
		})
	}
	frozen, ferr := e.FrozenFact(fc)
	if ferr != nil {
		return TruthMapView{}, ferr
	}
	v.Frozen = frozen.Frozen
	// Freshness leg (the source's machine-JSON check, Go caliber):
	// the freeze record's surface fingerprint (S1 trail) vs the
	// live package's — the same comparison bundleMaterials makes.
	recs, rerr := e.trailRecords(fc.Feature.ID)
	switch {
	case !frozen.Frozen:
		v.FingerprintStatus = "no_freeze_recorded"
	case rerr != nil || recs.freeze == nil || recs.freeze.SurfaceFingerprint == "":
		v.FingerprintStatus = "no_freeze_recorded"
		v.Diagnostics = append(v.Diagnostics, TruthMapDiagnostic{
			Code: core.ReasonTruthMapStale, Severity: "info",
			Message: "package is frozen but no freeze fingerprint is recorded; freshness cannot be verified",
		})
	default:
		live := core.PackageSurfaceFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text),
			[]byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text))
		if live != recs.freeze.SurfaceFingerprint {
			v.FingerprintStatus = "stale"
			v.Diagnostics = append(v.Diagnostics, TruthMapDiagnostic{
				Code: core.ReasonTruthMapStale, Severity: "error",
				Message: "package changed since the freeze fingerprint was recorded (machine JSON would be stale)",
			})
		} else {
			v.FingerprintStatus = "fresh"
		}
	}
	v.Status = "pass"
	seen := map[string]bool{}
	for _, d := range v.Diagnostics {
		if d.Severity == "error" {
			v.Status = "blocked"
		}
		if !seen[d.Code] {
			seen[d.Code] = true
			v.ReasonCodes = append(v.ReasonCodes, d.Code)
		}
	}
	return v, nil
}
