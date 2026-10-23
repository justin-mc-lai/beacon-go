// Standard flow S2 (AC-142): the admission evidence bundle — the
// SUPERSET verdict at the implement entry. Five verification
// classes, each with its own verdict and a named fact source:
//
//  1. truth_map_verify        — the package is registered in
//     spine/projects AND its on-disk layout matches the
//     registration. DERIVED source, stated honestly on the item:
//     the kernel has no standalone truth-map artifact, so the
//     verdict is derived live from the registration rows + the
//     package files' frontmatter/canonical_refs (spine.go /
//     packageFiles) — never presented as a recorded artifact.
//  2. verify_materials_strict — the four-piece set + baseline
//     fingerprints under STRICT integrity (gate.CheckIntegrity in
//     block mode — the same block pattern the integrity gate
//     uses), plus the S1 surface fingerprint on record matching
//     the live package.
//  3. git_diff_check          — the bound worktree's diff carries
//     no requirement-body or protected-path violation (P4c
//     denylist + S1 package fingerprints via the dirty policy of
//     baseline_check.go) and `git diff --check` is clean (R11
//     governance_verify's named check).
//  4. branch_governance_verify — canonical discipline + committed
//     baseline (S1): branch fact on record, baseline committed,
//     AC-141 ancestor leg, no stale quarantine (AC-140 scan); plus,
//     since S3 (AC-173), the topology_gate sub-result (AC-171/172)
//     judged live by the same shared function admission item 8
//     uses.
//  5. workspace_admission     — the shared admission checker
//     (admission.go, now seven items incl. AC-141) as a whole.
//
// Missing/failing any applicable item blocks implement. The
// headline code for a failure in items 1–4 is the frozen TC-142
// code standard_flow_verification_incomplete (the per-item codes
// ride along); when ONLY item 5 fails, the headline stays the
// admission checker's own first failing code, so tick/delegate
// blocks keep their precise admission semantics.
//
// Persistence (argued in the S2 report): a bundle evaluation at
// the implement entry lands ONE gate_results row
// (gate_type=admission_bundle — the store's gate_type column is a
// free ledger axis, no schema change) plus ONE
// admission_bundle_evaluated trail event via the gate service's
// persistGateResult, exactly like every formal gate (AC-013).
// `goal admit` stays a read-only preview of item 5's subset and
// produces no bundle — the bundle exists only where implement is
// actually entered (Tick, delegate packing, flow advance).
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// GateTypeAdmissionBundle is the gate_results axis of the bundle.
const GateTypeAdmissionBundle = "admission_bundle"

// EventAdmissionBundleEvaluated is appended per bundle evaluation.
const EventAdmissionBundleEvaluated = "admission_bundle_evaluated"

// BundleEvaluator is the recorded evaluator identity of bundle
// rows (the kernel itself judges; no external evaluator exists).
const BundleEvaluator = "beacon-goal-admission"

// Bundle item names (AC-142's five classes, frozen wording
// normalised to the kernel's check-name style).
const (
	BundleTruthMap       = "truth_map_verify"
	BundleMaterials      = "verify_materials_strict"
	BundleGitDiff        = "git_diff_check"
	BundleBranchGov      = "branch_governance_verify"
	BundleWorkspaceAdmin = "workspace_admission"
)

// BundleItem is one of the five verification verdicts.
type BundleItem struct {
	Check       string   `json:"check"`
	OK          bool     `json:"ok"`
	Applicable  bool     `json:"applicable"`
	Source      string   `json:"source"` // recorded | derived | live
	ReasonCode  string   `json:"reason_code,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	// Topology is the AC-173 topology_gate sub-result; only the
	// branch_governance_verify item carries it (AC-GSG-007: the
	// standard verification bundle includes a topology_gate check).
	Topology *TopologySubResult `json:"topology_gate,omitempty"`
}

// TopologySubResult is the topology_gate check inside the
// branch-governance bundle item (AC-173 / AC-GSG-007): status is
// pass | blocked | missing | not_applicable, ref names the
// BranchContext the verdict was taken over.
type TopologySubResult struct {
	Status      string   `json:"status"`
	Ref         string   `json:"topology_gate_ref,omitempty"`
	Kind        string   `json:"topology_kind,omitempty"`
	ReasonCode  string   `json:"reason_code,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	Detail      string   `json:"detail,omitempty"`
}

// BundleResult is the structured AC-142 outcome.
type BundleResult struct {
	Schema         string          `json:"schema"`
	RunID          string          `json:"run_id"`
	FeatureSlug    string          `json:"feature_slug"`
	Status         string          `json:"status"` // pass | blocked
	AllowImplement bool            `json:"allow_implement"`
	ReasonCode     string          `json:"reason_code"`
	ReasonCodes    []string        `json:"reason_codes,omitempty"`
	Items          []BundleItem    `json:"items"`
	Admission      AdmissionResult `json:"workspace_admission_result"`
	BaselineCommit string          `json:"baseline_commit,omitempty"`
	WorktreeDir    string          `json:"worktree_dir,omitempty"`
	GateResultID   string          `json:"gate_result_id,omitempty"`
	EvaluatedAt    string          `json:"evaluated_at,omitempty"`
}

// packagePresent reports whether any package file exists on disk
// (the S1 no_document caliber's trigger: seeded registrations with
// no on-disk package at all are not painted red by file checks).
func packagePresent(pkg gate.PackageFiles) bool {
	return pkg.Truth.Present || pkg.Tests.Present || pkg.Tasks.Present || pkg.Evidence.Present
}

// bundleTruthMap verifies registration + layout consistency. The
// layout judgement itself is the shared truthMapLayoutProblems
// (R2b/M16): the standalone `beacon truth map verify` face and
// this bundle item read the same function, so the two can never
// disagree.
func (e *Engine) bundleTruthMap(run store.Run, slug string) BundleItem {
	item := BundleItem{Check: BundleTruthMap, Source: "derived"}
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil || !ok {
		item.Applicable = true
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "feature is not registered in spine/projects; no truth-map can be derived"
		return item
	}
	_, present, problems := e.truthMapLayoutProblems(fc)
	if !present {
		item.OK, item.Applicable = true, false
		item.Detail = "no on-disk package (seeded registration): truth-map not applicable — S1 no_document caliber; " +
			"no standalone truth-map artifact exists in the kernel, this verdict is derived from registration + layout when a package exists"
		return item
	}
	item.Applicable = true
	if len(problems) > 0 {
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "truth-map verify failed (derived from spine/projects registration + on-disk layout; " +
			"the kernel has no standalone truth-map artifact): " + strings.Join(problems, "; ")
		return item
	}
	item.OK = true
	item.Detail = "registered slug " + fc.Feature.Slug + " matches the on-disk four-piece layout " +
		"(derived: spine/projects registration + frontmatter/canonical_refs — no standalone truth-map artifact exists)"
	return item
}

// bundleMaterials runs strict integrity + the S1 fingerprint leg.
func (e *Engine) bundleMaterials(run store.Run) BundleItem {
	item := BundleItem{Check: BundleMaterials, Source: "live"}
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil || !ok {
		item.Applicable = true
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "feature context unresolvable; strict materials cannot be verified"
		return item
	}
	pkg := e.packageFiles(fc)
	if !packagePresent(pkg) {
		item.OK, item.Applicable = true, false
		item.Detail = "no on-disk package (seeded registration): strict materials not applicable — S1 no_document caliber"
		return item
	}
	root := fc.Project.RootPath
	var artifacts []gate.ArtifactFact
	if runs, err := e.store.Runs().ListByFeature(fc.Feature.ID); err == nil {
		for _, r := range runs {
			if arts, err := e.store.Artifacts().ListByRun(r.ID); err == nil {
				for _, a := range arts {
					artifacts = append(artifacts, gate.ArtifactFact{AbsPath: a.AbsPath})
				}
			}
		}
	}
	rep := gate.CheckIntegrity(gate.IntegrityInput{
		Pkg: pkg, Artifacts: artifacts,
		ResolveRef: func(ref string) string { return resolvePkgPath(root, ref) },
		FileExists: func(abs string) bool { _, err := os.Stat(abs); return err == nil },
	}, gate.IntegrityModeBlock)
	item.Applicable = true
	if rep.Verdict != gate.VerdictPass {
		item.ReasonCodes = rep.ReasonCodes
		if len(rep.ReasonCodes) > 0 {
			item.ReasonCode = rep.ReasonCodes[0]
		}
		first := ""
		if len(rep.Findings) > 0 {
			first = rep.Findings[0].Kind + ": " + rep.Findings[0].Detail
		}
		item.Detail = "verify-materials strict (integrity block mode) found " +
			strconv.Itoa(len(rep.Findings)) + " fracture(s); first: " + first
		return item
	}
	// Fingerprint leg: the recorded freeze fingerprint (S1 trail)
	// must still describe the live package — a body that moved
	// after its freeze without a refreeze is not strict-clean.
	if recs, err := e.trailRecords(fc.Feature.ID); err == nil && recs.freeze != nil && recs.freeze.SurfaceFingerprint != "" {
		live := core.PackageSurfaceFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text),
			[]byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text))
		if live != recs.freeze.SurfaceFingerprint {
			item.ReasonCode = core.ReasonRevisionAuthorityDrift
			item.ReasonCodes = []string{core.ReasonRevisionAuthorityDrift}
			item.Detail = "package surface fingerprint drifted from the frozen record (S1 pkgsurface-v1); " +
				"the live package is not the frozen baseline body — refreeze before implementing"
			return item
		}
		item.Detail = "four-piece strict integrity pass; surface fingerprint matches the frozen record"
	} else {
		item.Detail = "four-piece strict integrity pass (no freeze fingerprint on record to compare)"
	}
	item.OK = true
	return item
}

// bundleGitDiff checks the bound worktree's diff: no package /
// protected violations (the dirty policy's violation classes) and
// a clean `git diff --check`.
func (e *Engine) bundleGitDiff(run store.Run, m *runMeta, slug string) BundleItem {
	item := BundleItem{Check: BundleGitDiff, Source: "live"}
	b, err := e.bindingOf(run, m, slug)
	if err != nil {
		item.Applicable, item.ReasonCode = true, core.ReasonStandardFlowVerificationIncm
		item.Detail = "binding resolution failed: " + err.Error()
		return item
	}
	if !b.Applicable {
		item.OK, item.Applicable = true, false
		item.Detail = b.Note
		return item
	}
	item.Applicable = true
	entries, err := harness.StatusEntries(context.Background(), b.Dir)
	if err != nil {
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "git status failed in the bound worktree: " + err.Error()
		return item
	}
	pkgRel := filepath.ToSlash(filepath.Dir(b.Feature.TruthPath))
	violations, _ := dirtyViolations(entries, pkgRel)
	if len(violations) > 0 {
		item.ReasonCode = core.ReasonUnscopedDirtyState
		item.ReasonCodes = []string{core.ReasonUnscopedDirtyState}
		item.Detail = "working-tree diff touches governed paths: " + strings.Join(violations, ", ")
		return item
	}
	findings, err := harness.DiffCheck(context.Background(), b.Dir)
	if err == nil && strings.TrimSpace(findings) != "" {
		item.ReasonCode = core.ReasonGitDiffCheckFailed
		item.ReasonCodes = []string{core.ReasonGitDiffCheckFailed}
		item.Detail = "git diff --check findings: " + strings.TrimSpace(firstLine(findings))
		return item
	}
	item.OK = true
	item.Detail = "no package/protected diff violations; git diff --check clean"
	return item
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// bundleBranchGovernance composes the S1 facts: branch check on
// record, committed baseline, the AC-141 ancestor leg and the
// AC-140 quarantine scan.
func (e *Engine) bundleBranchGovernance(run store.Run, m *runMeta, slug string) BundleItem {
	item := BundleItem{Check: BundleBranchGov, Source: "recorded"}
	b, err := e.bindingOf(run, m, slug)
	if err != nil {
		item.Applicable, item.ReasonCode = true, core.ReasonStandardFlowVerificationIncm
		item.Detail = "binding resolution failed: " + err.Error()
		return item
	}
	// AC-173 topology leg (S3): the branch-governance verdict must
	// carry the topology_gate sub-result. It is judged by the same
	// shared function as admission item 8 — one judgement, two
	// consumers. A recorded multi-git context makes this item
	// applicable even when the project root itself is not a repo.
	topoItem, topoVerdict, topoRec, topoErr := e.evalTopologyGateItem(run)
	if topoErr != nil {
		item.Applicable, item.ReasonCode = true, core.ReasonStandardFlowVerificationIncm
		item.Detail = "topology judgement failed: " + topoErr.Error()
		return item
	}
	sub := &TopologySubResult{Status: TopologyNotApplicable, Detail: topoItem.Detail}
	if topoRec != nil {
		sub.Ref = topoRec.Context.TopologyGateRef
		sub.Kind = topoRec.Context.TopologyKind
	}
	switch {
	case !topoItem.Applicable:
		// sub stays not_applicable.
	case topoItem.OK:
		sub.Status = TopologyGatePass
	default:
		sub.ReasonCode = topoItem.ReasonCode
		if topoItem.ReasonCode == core.ReasonTopologyGateMissing {
			sub.Status = TopologyGateMissing
		} else {
			sub.Status = TopologyGateBlocked
		}
		if topoVerdict != nil {
			sub.ReasonCodes = topoVerdict.ReasonCodes
		}
	}
	item.Topology = sub
	if !b.Applicable && !harness.IsGitRepo(context.Background(), b.Root) && !topoItem.Applicable {
		item.OK, item.Applicable = true, false
		item.Detail = b.Note
		return item
	}
	item.Applicable = true
	ff, err := e.frozenFactByID(run.FeatureID)
	if err != nil {
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "frozen fact unavailable: " + err.Error()
		return item
	}
	var codes []string
	var problems []string
	// S3 fold: the topology leg's failure joins this item's codes
	// verbatim (topology_gate_missing / topology_gate_blocked, with
	// the scope codes riding on the sub-result), so the bundle
	// headline keeps AC-173's own codes for a topology-only
	// failure instead of the generic incomplete code.
	if topoItem.Applicable && !topoItem.OK {
		codes = append(codes, topoItem.ReasonCode)
		if topoVerdict != nil {
			codes = append(codes, topoVerdict.ReasonCodes...)
		}
		problems = append(problems, "topology_gate: "+topoItem.Detail)
	}
	if ff.Frozen && ff.BaselineStatus == BaselineUncommitted {
		codes = append(codes, core.ReasonCanonicalRefreezeNotCommitted)
		problems = append(problems, "frozen body is not a committed baseline (AC-139)")
	}
	merged, _, err := e.evalBaselineMerged(run, m, slug)
	if err != nil {
		item.ReasonCode = core.ReasonStandardFlowVerificationIncm
		item.Detail = "baseline-merged judgement failed: " + err.Error()
		return item
	}
	if merged.Applicable && !merged.OK {
		codes = append(codes, merged.ReasonCode)
		problems = append(problems, merged.Detail)
	}
	if found := e.scanWorktreeQuarantines(run, m, b.Root); len(found) > 0 {
		codes = append(codes, core.ReasonStaleFeatureLocalRefreeze)
		problems = append(problems, "stale feature-local refreeze present in a satellite worktree (AC-140)")
	}
	if len(problems) > 0 {
		item.ReasonCodes = codes
		if len(codes) > 0 {
			item.ReasonCode = codes[0]
		}
		item.Detail = "branch-governance verify failed: " + strings.Join(problems, "; ")
		return item
	}
	item.OK = true
	item.Detail = "canonical discipline on record (branch_check=" + ff.BranchCheck +
		"), baseline committed and merged, no stale quarantine; topology_gate=" + sub.Status
	return item
}

// evaluateBundle computes the five verdicts (pure judgement; no
// persistence — persistBundle is the write half).
func (e *Engine) evaluateBundle(run store.Run, m *runMeta) (BundleResult, error) {
	slug := ""
	if feat, err := e.store.Features().GetByID(run.FeatureID); err == nil {
		slug = feat.Slug
	}
	adm, err := e.checkAdmission(run, m)
	if err != nil {
		return BundleResult{}, err
	}
	res := BundleResult{
		Schema: "admission-bundle.v1", RunID: run.ID, FeatureSlug: slug,
		Status: "pass", AllowImplement: true, Admission: adm,
	}
	if b, err := e.bindingOf(run, m, slug); err == nil && b.Applicable {
		res.WorktreeDir = b.Dir
	}
	if commit, _, _, err := e.baselineCommitOf(run.FeatureID); err == nil {
		res.BaselineCommit = commit
	}
	ws := BundleItem{Check: BundleWorkspaceAdmin, Source: "recorded", Applicable: true, OK: adm.AllowImplement}
	if adm.AllowImplement {
		ws.Detail = "shared admission checker passes all applicable items"
	} else {
		ws.ReasonCode = adm.ReasonCode
		ws.ReasonCodes = adm.ReasonCodes
		ws.Detail = "shared admission checker blocked: " + adm.ReasonCode
	}
	res.Items = []BundleItem{
		e.bundleTruthMap(run, slug),
		e.bundleMaterials(run),
		e.bundleGitDiff(run, m, slug),
		e.bundleBranchGovernance(run, m, slug),
		ws,
	}
	// Headline rule (see the file comment): items 1–4 failing ->
	// the frozen TC-142 code; item 5 alone -> its own headline.
	var codes []string
	structuralFail := false
	for _, it := range res.Items[:4] {
		if it.Applicable && !it.OK {
			structuralFail = true
			codes = append(codes, it.ReasonCodes...)
			if len(it.ReasonCodes) == 0 && it.ReasonCode != "" {
				codes = append(codes, it.ReasonCode)
			}
		}
	}
	if !ws.OK {
		codes = append(codes, ws.ReasonCodes...)
	}
	if structuralFail || !ws.OK {
		res.Status, res.AllowImplement = "blocked", false
		if structuralFail {
			res.ReasonCode = core.ReasonStandardFlowVerificationIncm
			codes = append([]string{core.ReasonStandardFlowVerificationIncm}, codes...)
		} else {
			res.ReasonCode = adm.ReasonCode
		}
		res.ReasonCodes = dedupStrs(codes)
	} else {
		res.ReasonCode = core.ReasonGoalImplementAdmissOK
	}
	return res, nil
}

func dedupStrs(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// persistBundle lands the bundle as one gate_results row
// (gate_type=admission_bundle) + one admission_bundle_evaluated
// event, through the gate service's one-transaction writer.
func (e *Engine) persistBundle(run store.Run, res BundleResult) (BundleResult, error) {
	ctx, err := e.featureContextByID(run.FeatureID)
	if err != nil {
		return res, err
	}
	verdict := gate.VerdictPass
	if !res.AllowImplement {
		verdict = gate.VerdictBlocked
	}
	ev, err := e.persistGateResult(ctx, gate.Evaluation{
		GateType: gate.GateType(GateTypeAdmissionBundle), Verdict: verdict,
		ReasonCodes: res.ReasonCodes, Evaluator: BundleEvaluator,
	}, run.ID, EventAdmissionBundleEvaluated, map[string]any{
		"schema": res.Schema, "status": res.Status,
		"baseline_commit": res.BaselineCommit, "worktree_dir": res.WorktreeDir,
		"items": bundleItemSummary(res.Items),
	})
	if err != nil {
		return res, err
	}
	res.GateResultID, res.EvaluatedAt = ev.GateResultID, ev.EvaluatedAt
	return res, nil
}

func bundleItemSummary(items []BundleItem) []map[string]any {
	out := []map[string]any{}
	for _, it := range items {
		out = append(out, map[string]any{
			"check": it.Check, "ok": it.OK, "applicable": it.Applicable,
			"source": it.Source, "reason_code": it.ReasonCode,
		})
	}
	return out
}

// EvaluateAdmissionBundle is the public bundle surface: evaluate +
// persist (the implement-entry evidence record).
func (e *Engine) EvaluateAdmissionBundle(runID string) (BundleResult, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return BundleResult{}, err
	}
	res, err := e.evaluateBundle(run, m)
	if err != nil {
		return BundleResult{}, err
	}
	return e.persistBundle(run, res)
}

// admissionBundleForEntry is the ONE implement-entry gate shared
// by Tick and the delegate packers: evaluate the bundle, persist
// it as evidence, and let its verdict decide. No second judgement
// exists on these paths anymore.
func (e *Engine) admissionBundleForEntry(run store.Run, m *runMeta) (BundleResult, error) {
	res, err := e.evaluateBundle(run, m)
	if err != nil {
		return BundleResult{}, err
	}
	return e.persistBundle(run, res)
}
