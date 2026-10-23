// Version-level release check (R2b, parity audit H7): the pure
// half of Python commands/release.py `check` + prod_ops_v168
// .select_release_profile. Python's check is a VERSION verdict —
// "can this version ship" — over ~25 checks; the Go release GATE
// (EvaluateRelease) is a per-feature composition and stays as it
// is. This file judges the version: per-feature facts in, a
// checklist with pass/warn/fail per item out, with the release
// profile's downgrade applied.
//
// Profile semantics (select_release_profile, verbatim lists):
// full -> gates [full_suite]; hotfix -> HOTFIX_REQUIRED_GATES;
// docs_only -> gates [docs_consistency, human_release]. An unknown
// profile is refused with release_profile_invalid.
//
// Downgrade semantics (release.py check, the v1.6.8 wiring): under
// hotfix, the soft set's failures are rewritten to warn — the
// source's soft set is {"implement prompt boundary verdict pass",
// "semantic contract ready", "project metadata contract"}; of
// these only the metadata contract has a Go counterpart item, so
// the Go soft set is {project_metadata_contract} (the other two
// Python checks have no Go fact source — see the r2b report's
// unmapped list). Under docs_only the source's check never consumed
// the profile's gate list; the Go check DOES (the audit task's
// requirement): every item outside the docs-consistency analogue
// set + human_release is downgraded. core safety stays hard under
// every profile: package state, evidence and the human release
// are never downgraded by hotfix in the source, and the same line
// is kept here.
package gate

import (
	"strings"

	"beacon-go/internal/core"
)

// ReleaseProfiles is prod_ops_v168.RELEASE_PROFILES (verbatim).
var ReleaseProfiles = []string{"full", "hotfix", "docs_only"}

// HotfixRequiredGates is prod_ops_v168.HOTFIX_REQUIRED_GATES
// (verbatim): the gates a hotfix release must still satisfy.
var HotfixRequiredGates = []string{
	"evidence_manifest", "hotfix_baseline", "forward_port_plan",
	"rollback_notes", "minimal_qa", "human_release",
}

// DocsOnlyGates is select_release_profile's docs_only gate list
// (verbatim).
var DocsOnlyGates = []string{"docs_consistency", "human_release"}

// FullGates is select_release_profile's full gate list (verbatim).
var FullGates = []string{"full_suite"}

// ReleaseProfileSelection is select_release_profile's outcome.
type ReleaseProfileSelection struct {
	OK         bool     `json:"ok"`
	Blocked    bool     `json:"blocked"`
	ReasonCode string   `json:"reason_code,omitempty"`
	Profile    string   `json:"profile"`
	Gates      []string `json:"gates,omitempty"`
	Allowed    []string `json:"allowed,omitempty"`
}

// SelectReleaseProfile mirrors select_release_profile: the name is
// normalised (trim/lower, empty = full); an unknown name is
// blocked with release_profile_invalid.
func SelectReleaseProfile(name string) ReleaseProfileSelection {
	profile := strings.ToLower(strings.TrimSpace(name))
	if profile == "" {
		profile = "full"
	}
	known := false
	for _, p := range ReleaseProfiles {
		if p == profile {
			known = true
			break
		}
	}
	if !known {
		return ReleaseProfileSelection{
			Blocked: true, ReasonCode: core.ReasonReleaseProfileInvalid,
			Profile: profile, Allowed: append([]string{}, ReleaseProfiles...),
		}
	}
	sel := ReleaseProfileSelection{OK: true, Profile: profile}
	switch profile {
	case "hotfix":
		sel.Gates = append([]string{}, HotfixRequiredGates...)
	case "docs_only":
		sel.Gates = append([]string{}, DocsOnlyGates...)
	default:
		sel.Gates = append([]string{}, FullGates...)
	}
	return sel
}

// Version check item names (the Go checklist; each item's Python
// counterpart is named in EvaluateVersionRelease's assembly and in
// the r2b report's mapping table).
const (
	VersionCheckRegistered = "version_registered"
	VersionCheckFrozen     = "requirement_packages_frozen"
	VersionCheckNoPending  = "requirement_packages_no_pending_change"
	VersionCheckIntegrity  = "integrity_verdicts_pass"
	VersionCheckGates      = "feature_gates_pass"
	VersionCheckEvidence   = "qa_evidence_present"
	VersionCheckCoverage   = "coverage_complete"
	VersionCheckMaterials  = "materials_fresh"
	VersionCheckHuman      = "human_release_confirmed"
	VersionCheckMetadata   = "project_metadata_contract"
	VersionCheckSurfaceGov = "version_surface_governance"
)

// CoverageThreshold is release.py's coverage bar: a coverage rate
// of 0.8 or better counts the coverage artifact ready (verbatim).
const CoverageThreshold = 0.8

// VersionFeatureFacts are one feature's facts for the version
// check, gathered by the orchestrator from the store + package.
type VersionFeatureFacts struct {
	Slug             string
	Frozen           bool
	PendingChange    bool
	MaterialsStale   bool
	IntegrityVerdict string // pass | blocked | audit | "" (unevaluated)
	IntegrityBlocked bool
	GatesPass        bool
	GatesDetail      string
	QAArtifactCount  int
	QAStaleCount     int
	ACCoverage       float64 // -1 = no ACs / unknown
	HumanConfirmed   bool
}

// VersionReleaseFacts are the version check's inputs.
type VersionReleaseFacts struct {
	Version            string
	VersionRegistered  bool
	Features           []VersionFeatureFacts
	MetadataOK         bool
	MetadataDetail     string
	TopologyApplicable bool
	TopologyPass       bool
	TopologyDetail     string
}

// VersionCheckItem is one checklist line. Status is pass | warn |
// fail | na; a downgraded failure reads warn with Downgraded=true
// and the downgraded_warning reason code, a hard failure keeps its
// specific code (or release_check_hard_block).
type VersionCheckItem struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"` // pass | warn | fail | na
	Applicable bool     `json:"applicable"`
	Downgraded bool     `json:"downgraded,omitempty"`
	ReasonCode string   `json:"reason_code,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	Features   []string `json:"features,omitempty"` // slugs failing this item
}

// VersionReleaseResult is the version check's verdict packet
// (release.py check's payload shape: status / release_ready /
// checks / failed_checks / warning_checks / release_profile).
type VersionReleaseResult struct {
	Version       string             `json:"version"`
	Status        string             `json:"status"` // pass | blocked
	ReleaseReady  bool               `json:"release_ready"`
	ReasonCode    string             `json:"reason_code,omitempty"`
	Profile       string             `json:"release_profile"`
	ProfileGates  []string           `json:"profile_gates,omitempty"`
	FeatureCount  int                `json:"feature_count"`
	Checks        []VersionCheckItem `json:"checks"`
	FailedChecks  []string           `json:"failed_checks,omitempty"`
	WarningChecks []string           `json:"warning_checks,omitempty"`
}

// hotfixSoftChecks is the Go soft set under profile=hotfix (see
// the file comment for the mapping from the source's set).
var hotfixSoftChecks = map[string]bool{
	VersionCheckMetadata: true,
}

// docsOnlyHardChecks is the docs-consistency analogue set +
// human_release: the items that stay hard under docs_only.
var docsOnlyHardChecks = map[string]bool{
	VersionCheckRegistered: true,
	VersionCheckFrozen:     true,
	VersionCheckMaterials:  true,
	VersionCheckMetadata:   true,
	VersionCheckHuman:      true,
}

// EvaluateVersionRelease assembles the checklist from the facts
// and applies the profile downgrade. Blocked iff any applicable
// item hard-fails after downgrade; the headline code of a blocked
// version is release_check_failed (the source's trace code), with
// each item keeping its own code.
func EvaluateVersionRelease(f VersionReleaseFacts, sel ReleaseProfileSelection) VersionReleaseResult {
	res := VersionReleaseResult{
		Version: f.Version, Profile: sel.Profile, ProfileGates: sel.Gates,
		FeatureCount: len(f.Features), Status: "pass", ReleaseReady: true,
	}
	failures := func(pred func(VersionFeatureFacts) bool) []string {
		var slugs []string
		for _, feat := range f.Features {
			if pred(feat) {
				slugs = append(slugs, feat.Slug)
			}
		}
		return slugs
	}
	item := func(name, code string, failing []string, detail string) VersionCheckItem {
		it := VersionCheckItem{Name: name, Applicable: true, Features: failing, Detail: detail}
		if len(failing) == 0 {
			it.Status = "pass"
		} else {
			it.Status = "fail"
			it.ReasonCode = code
		}
		return it
	}
	var checks []VersionCheckItem

	// version_registered (analogue: the version + its packages
	// exist — release.py resolves the version before any check).
	reg := VersionCheckItem{Name: VersionCheckRegistered, Applicable: true}
	if f.VersionRegistered && len(f.Features) > 0 {
		reg.Status = "pass"
		reg.Detail = "version registered with features"
	} else {
		reg.Status = "fail"
		reg.ReasonCode = core.ReasonRequirementContractBlocked
		reg.Detail = "version not registered or carries no feature packages"
	}
	checks = append(checks, reg)

	checks = append(checks, item(VersionCheckFrozen, core.ReasonStageExitTruthNotFrozen,
		failures(func(x VersionFeatureFacts) bool { return !x.Frozen }),
		"every feature package of the version is frozen"))
	checks = append(checks, item(VersionCheckNoPending, core.ReasonTruthChangePending,
		failures(func(x VersionFeatureFacts) bool { return x.PendingChange }),
		"no feature carries an unrefrozen truth change"))
	checks = append(checks, item(VersionCheckIntegrity, core.ReasonIntegrityPackageValid,
		failures(func(x VersionFeatureFacts) bool { return x.IntegrityBlocked }),
		"every feature's integrity verdict passes (audit counts for frozen packages)"))
	checks = append(checks, item(VersionCheckGates, core.ReasonGatePrereqNotPassed,
		failures(func(x VersionFeatureFacts) bool { return !x.GatesPass }),
		"every feature's four gates hold a latest pass verdict"))
	checks = append(checks, item(VersionCheckEvidence, core.ReasonStageExitQAEvidenceMiss,
		failures(func(x VersionFeatureFacts) bool { return x.QAArtifactCount == 0 || x.QAStaleCount > 0 }),
		"every feature has live QA evidence registered"))
	// coverage_complete: release.py's 0.8 bar over the AC->TC
	// coverage the package itself declares; features without ACs
	// carry no coverage obligation (unknown is not painted red —
	// a broken package already fails the integrity item).
	covFail := failures(func(x VersionFeatureFacts) bool {
		return x.ACCoverage >= 0 && x.ACCoverage < CoverageThreshold
	})
	cov := item(VersionCheckCoverage, core.ReasonReleaseCheckHardBlock, covFail,
		"every feature's AC->TC coverage meets the 0.8 bar")
	checks = append(checks, cov)
	checks = append(checks, item(VersionCheckMaterials, core.ReasonMaterialsStale,
		failures(func(x VersionFeatureFacts) bool { return x.MaterialsStale }),
		"no package material is marked stale"))
	checks = append(checks, item(VersionCheckHuman, core.ReasonReleaseRequiresHuman,
		failures(func(x VersionFeatureFacts) bool { return !x.HumanConfirmed }),
		"every feature holds a human release confirmation"))
	md := VersionCheckItem{Name: VersionCheckMetadata, Applicable: true}
	if f.MetadataOK {
		md.Status = "pass"
	} else {
		md.Status = "fail"
		md.ReasonCode = core.ReasonProjectMetadataBlocked
	}
	md.Detail = f.MetadataDetail
	checks = append(checks, md)
	topo := VersionCheckItem{Name: VersionCheckSurfaceGov, Applicable: f.TopologyApplicable}
	if !f.TopologyApplicable {
		topo.Status = "na"
		topo.Detail = f.TopologyDetail
	} else if f.TopologyPass {
		topo.Status = "pass"
		topo.Detail = f.TopologyDetail
	} else {
		topo.Status = "fail"
		topo.ReasonCode = core.ReasonTopologyGateBlocked
		topo.Detail = f.TopologyDetail
	}
	checks = append(checks, topo)

	// Profile downgrade (release.py's hotfix rewrite + the
	// docs_only gate list): a downgraded failure becomes a warning
	// and stops blocking; its code becomes the downgraded marker.
	for i := range checks {
		it := &checks[i]
		if it.Status != "fail" {
			continue
		}
		downgrade := false
		switch sel.Profile {
		case "hotfix":
			downgrade = hotfixSoftChecks[it.Name]
		case "docs_only":
			downgrade = !docsOnlyHardChecks[it.Name]
		}
		if downgrade {
			it.Status = "warn"
			it.Downgraded = true
			it.ReasonCode = core.ReasonReleaseCheckDowngraded
			it.Detail += "; profile=" + sel.Profile + " soft-warn"
		}
	}
	for _, it := range checks {
		switch it.Status {
		case "fail":
			res.FailedChecks = append(res.FailedChecks, it.Name)
		case "warn":
			res.WarningChecks = append(res.WarningChecks, it.Name)
		}
	}
	if len(res.FailedChecks) > 0 {
		res.Status = "blocked"
		res.ReleaseReady = false
		res.ReasonCode = core.ReasonReleaseCheckFailed
	}
	res.Checks = checks
	return res
}
