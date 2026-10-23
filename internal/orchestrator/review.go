// Implementation review face (R2b, parity audit M15): Python
// commands/review.py runs three reviews over a delivered package —
// spec (does the package still say what was agreed), quality (did
// verification actually close) and design (does the design hold) —
// each emitting issues with critical/warn severities, a status
// (fail when any critical, else warn when any issue, else pass)
// and gate_ready = no critical. This file ports the CORE subset of
// each checklist onto Go facts; the unported items are listed in
// the r2b report. The verdict is written back through the H5
// review-decision channel (recordReviewDecisionOn): critical
// findings arrive as P0 (drift findings recommend the
// beacon-gen-change route), warnings as P2 advisories — so a
// review here has the same consequences as a planner review.
package orchestrator

import (
	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Review kinds (review.py's three subcommands).
const (
	ReviewSpec    = "spec"
	ReviewQuality = "quality"
	ReviewDesign  = "design"
)

// ReviewIssue is one review finding (review.py's issue shape:
// check name as code, severity critical|warn).
type ReviewIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // critical | warn
	Detail   string `json:"detail"`
}

// ReviewView is one review's outcome packet.
type ReviewView struct {
	Kind            string        `json:"kind"`
	FeatureSlug     string        `json:"feature_slug"`
	Status          string        `json:"status"` // pass | warn | fail
	GateReady       bool          `json:"gate_ready"`
	Issues          []ReviewIssue `json:"issues,omitempty"`
	RunID           string        `json:"run_id,omitempty"`
	ReviewRecorded  bool          `json:"review_recorded"`
	RouteStage      string        `json:"route_stage,omitempty"`
	RouteReasonCode string        `json:"route_reason_code,omitempty"`
}

// reviewFacts are the gathered facts the three checklists read.
type reviewFacts struct {
	pkg            gate.PackageFiles
	frozen         bool
	pendingChange  bool
	materialsStale bool
	drift          bool
	coverage       float64 // -1 = no ACs
	qaCount        int
	qaStale        int
	implementSeen  bool
	designSeen     bool
	qaGateVerdict  string
	designVerdict  string
	planBlocked    bool
	hasRun         bool
	runID          string
	run            store.Run
	meta           *runMeta
}

func (e *Engine) gatherReviewFacts(fc store.FeatureContext) (reviewFacts, error) {
	var f reviewFacts
	f.pkg = e.packageFiles(fc)
	f.coverage = -1
	frozen, err := e.FrozenFact(fc)
	if err != nil {
		return f, err
	}
	f.frozen, f.pendingChange = frozen.Frozen, frozen.ChangePending
	f.materialsStale = len(e.staleMaterialSurfaces(fc)) > 0
	// Drift (R2a caliber): a frozen package whose live surface
	// fingerprint no longer matches the freeze record.
	if frozen.Frozen {
		if recs, rerr := e.trailRecords(fc.Feature.ID); rerr == nil && recs.freeze != nil && recs.freeze.SurfaceFingerprint != "" {
			live := core.PackageSurfaceFingerprint([]byte(f.pkg.Truth.Text), []byte(f.pkg.Tests.Text),
				[]byte(f.pkg.Tasks.Text), []byte(f.pkg.Evidence.Text))
			f.drift = live != recs.freeze.SurfaceFingerprint
		}
	}
	if f.pendingChange {
		f.drift = true
	}
	rep, _, _, err := e.integrityReportLive(fc, "")
	if err != nil {
		return f, err
	}
	if f.pkg.Truth.Present {
		acs := gate.ExtractACDescriptions(f.pkg.Truth.Text)
		if len(acs) > 0 {
			uncovered := 0
			for _, fd := range rep.Findings {
				if fd.ReasonCode == core.ReasonIntegrityACWithoutTC {
					uncovered++
				}
			}
			f.coverage = float64(len(acs)-uncovered) / float64(len(acs))
		}
	}
	latest, err := e.store.Gates().LatestByType(fc.Feature.ID)
	if err != nil {
		return f, err
	}
	for _, g := range latest {
		switch g.GateType {
		case string(gate.GateQA):
			f.qaGateVerdict = g.Verdict
		case string(gate.GateDesign):
			f.designVerdict = g.Verdict
		case string(gate.GateImplement):
			f.implementSeen = true
		}
	}
	run, m, has, err := e.latestGoalRun(fc, "")
	if err != nil {
		return f, err
	}
	f.hasRun, f.run, f.meta = has, run, m
	if has {
		f.runID = run.ID
		f.planBlocked = m.Blocked
	}
	runs, err := e.store.Runs().ListByFeature(fc.Feature.ID)
	if err != nil {
		return f, err
	}
	for _, r := range runs {
		arts, err := e.store.Artifacts().ListByRun(r.ID)
		if err != nil {
			return f, err
		}
		for _, a := range arts {
			switch {
			case qaKind(a.Kind):
				f.qaCount++
				if stale, _ := store.ArtifactStaleness(a); stale {
					f.qaStale++
				}
			case isDesignKind(a.Kind):
				f.designSeen = true
			case isImplementKind(a.Kind):
				f.implementSeen = true
			}
		}
	}
	return f, nil
}

func isImplementKind(kind string) bool {
	switch kind {
	case "implement", "implementation", "code", "patch":
		return true
	}
	return false
}

func isDesignKind(kind string) bool {
	switch kind {
	case "design", "design_html", "prototype":
		return true
	}
	return false
}

// specIssues ports run_spec_review's core: required documents,
// the implementation marker, drift and material closure.
func specIssues(f reviewFacts) []ReviewIssue {
	var out []ReviewIssue
	crit := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "critical", Detail: detail})
	}
	warn := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "warn", Detail: detail})
	}
	// Required documents: in the Go four-piece package the truth
	// document carries the user story + PRD + AC matrix, tests.md
	// carries the test cases (see the r2b report's mapping note).
	if !f.pkg.Truth.Present {
		crit("user_story_missing", "truth.md (carrying the user story) is missing")
		crit("prd_missing", "truth.md (carrying the PRD) is missing")
		crit("ac_matrix_missing", "truth.md (carrying the AC matrix) is missing")
	} else if f.coverage < 0 {
		crit("ac_matrix_missing", "truth.md declares no acceptance criteria")
	}
	if !f.pkg.Tests.Present {
		crit("test_case_missing", "tests.md is missing")
	}
	if !f.implementSeen {
		warn("implementation_review_marker_missing", "no implement gate verdict or implement artifact on record")
	}
	if f.drift {
		crit(core.ReasonReviewSpecDrift, "the package drifted from its frozen record (pending change or fingerprint drift)")
	}
	if f.materialsStale {
		warn(core.ReasonMaterialsStale, "package materials are marked stale")
	}
	if f.planBlocked {
		warn("plan_blocked", "the newest run is blocked")
	}
	return out
}

// qualityIssues ports run_quality_review's core: verification
// closure, evidence closure, pending artifact updates, package
// closure, coverage, plan state.
func qualityIssues(f reviewFacts) []ReviewIssue {
	var out []ReviewIssue
	crit := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "critical", Detail: detail})
	}
	warn := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "warn", Detail: detail})
	}
	if f.qaGateVerdict != string(gate.VerdictPass) {
		crit("verify_not_passed", "the latest QA gate verdict is not pass")
	}
	if f.qaCount == 0 {
		crit("evidence_not_closed", "no QA evidence artifacts are registered")
	}
	if f.qaStale > 0 {
		crit("artifact_updates_pending", "registered QA artifacts are stale on disk")
	}
	if !f.frozen {
		crit("formal_package_closure_missing", "the package is not frozen; the formal package is not closed")
	}
	if f.coverage >= 0 && f.coverage < CoverageThresholdQ {
		warn("coverage_incomplete", "AC->TC coverage is below the 0.8 bar")
	}
	if f.planBlocked {
		crit("plan_blocked", "the newest run is blocked")
	}
	return out
}

// CoverageThresholdQ re-exports the release coverage bar for the
// quality checklist (one number, one home: gate.CoverageThreshold).
const CoverageThresholdQ = gate.CoverageThreshold

// designIssues ports run_design_review's core: required documents,
// prototype/design presence (severity depends on whether the
// spec itself exists, as in the source), and the design gate's
// own verdict.
func designIssues(f reviewFacts) []ReviewIssue {
	var out []ReviewIssue
	crit := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "critical", Detail: detail})
	}
	warn := func(code, detail string) {
		out = append(out, ReviewIssue{Code: code, Severity: "warn", Detail: detail})
	}
	if !f.pkg.Truth.Present {
		crit("prd_missing", "truth.md (the design's spec input) is missing")
	}
	if !f.designSeen && f.designVerdict == "" {
		// prototype_missing: critical when the truth/spec is also
		// missing, warn otherwise (the source's rule).
		if !f.pkg.Truth.Present {
			crit("prototype_missing", "no design artifact or design gate verdict, and the spec is missing too")
		} else {
			warn("prototype_missing", "no design artifact or design gate verdict on record")
		}
	}
	switch f.designVerdict {
	case string(gate.VerdictBlocked), "fail":
		crit("design_review_blocked", "the latest design gate verdict blocks")
	case "warn":
		warn("design_review_revise", "the latest design gate verdict asks for revision")
	}
	if f.drift {
		warn(core.ReasonReviewSpecDrift, "the package drifted from its frozen record since the design was made")
	}
	return out
}

// Review runs one of the three reviews over a feature package and
// writes the verdict back through the H5 channel when the feature
// has a goal run.
func (e *Engine) Review(projectRoot, slug, kind string) (ReviewView, error) {
	root := store.ResolveRootPath(projectRoot)
	fc, err := e.store.ResolveFeature(root, slug, "")
	if err != nil {
		return ReviewView{}, err
	}
	f, err := e.gatherReviewFacts(fc)
	if err != nil {
		return ReviewView{}, err
	}
	var issues []ReviewIssue
	switch kind {
	case ReviewSpec:
		issues = specIssues(f)
	case ReviewQuality:
		issues = qualityIssues(f)
	case ReviewDesign:
		issues = designIssues(f)
	default:
		return ReviewView{}, core.NewError(core.ReasonGateTypeUnknown, "review.Review",
			"unknown review kind "+kind+" (spec|quality|design)")
	}
	v := ReviewView{Kind: kind, FeatureSlug: fc.Feature.Slug, Issues: issues, GateReady: true, Status: "pass"}
	hasCritical := false
	for _, is := range issues {
		if is.Severity == "critical" {
			hasCritical = true
		}
	}
	if hasCritical {
		v.Status = "fail"
		v.GateReady = false
	} else if len(issues) > 0 {
		v.Status = "warn"
	}
	if f.hasRun {
		v.RunID = f.runID
		if len(issues) > 0 {
			stage, code, rerr := e.writeBackReview(fc, f, kind, issues)
			if rerr != nil {
				return ReviewView{}, rerr
			}
			// recordReviewDecisionOn mutates the snapshot; the
			// caller persists (its contract).
			if serr := e.saveRun(f.run, f.meta); serr != nil {
				return ReviewView{}, serr
			}
			v.ReviewRecorded = true
			v.RouteStage, v.RouteReasonCode = stage, code
		}
	}
	return v, nil
}

// writeBackReview translates review issues into a planner-review
// packet and routes it: critical -> P0, warn -> P2; any drift
// issue recommends the truth-change route (the source's drift
// handling), everything else takes the standard route family.
// Returns the routed stage + reason code.
func (e *Engine) writeBackReview(fc store.FeatureContext, f reviewFacts, kind string, issues []ReviewIssue) (string, string, error) {
	var findings []core.ReviewFinding
	route := ""
	for _, is := range issues {
		sev := "P2"
		if is.Severity == "critical" {
			sev = "P0"
		}
		findings = append(findings, core.ReviewFinding{
			Severity: sev, ReasonCode: is.Code, Message: is.Detail, Source: "review_" + kind,
		})
		if is.Code == core.ReasonReviewSpecDrift {
			route = "beacon-gen-change"
		}
	}
	count := len(findings)
	pkt := core.ReviewPacket{
		ReviewID:             "review-" + kind + "-" + fc.Feature.Slug,
		RecommendedRoute:     route,
		Findings:             findings,
		DeclaredFindingCount: &count,
		SynthesisMarker:      true,
	}
	rec, err := e.recordReviewDecisionOn(f.run, f.meta, pkt, "review_"+kind)
	if err != nil {
		return "", "", err
	}
	return rec.NextStage, rec.ReasonCode, nil
}
