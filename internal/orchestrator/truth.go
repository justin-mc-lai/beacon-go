// Truth package service (P4a): the native feature-package lifecycle
// of the truth ability — create / freeze / change / refreeze over
// real packages on disk plus the store ledger.
//
// Until this slice a feature package could only appear via the P3b
// v1 migration or hand-seeded rows; the truth ability itself was
// empty. The frozen rule stands: requirement truth lives ONLY in
// the package markdown (docs/beacon/<version>/features/<slug>/);
// the store carries pointers, lifecycle snapshots and the event
// trail, never the truth text.
//
// The one sanctioned CLI write into a truth document is the freeze
// status line: freeze/refreeze rewrite ONLY the frontmatter
// `status:` scalar (draft -> frozen), after hashing the original
// bytes into the trail event. `change` never touches the body at
// all — it records a change document under the package's changes/
// directory plus a truth_change_requested event (the gen-change ->
// gen-refreeze governance of charter section 5, item 4).
package orchestrator

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// Truth service reason codes (CLI-stable contract strings).
const (
	ReasonTruthSlugInvalid        = "truth_slug_invalid"
	ReasonTruthPackageExists      = "truth_package_already_exists"
	ReasonTruthPackageDirExists   = "truth_package_dir_exists"
	ReasonTruthVersionMissing     = "truth_version_not_recorded"
	ReasonTruthNotFrozen          = "truth_not_frozen"
	ReasonTruthChangePending      = "truth_change_already_pending"
	ReasonTruthNoChangePending    = "truth_no_change_pending"
	ReasonTruthChangeSummaryReq   = "truth_change_summary_required"
	ReasonTruthFreezeBlocked      = "truth_freeze_blocked"
	ReasonTruthFrontmatterMissing = "truth_frontmatter_missing"
)

// Event types written by the truth service (feature-scoped trail).
const (
	EventTruthCreated          = "truth_created"
	EventTruthFrozen           = "truth_frozen"
	EventTruthChangeRequested  = "truth_change_requested"
	EventTruthRefrozen         = "truth_refrozen"
	EventTruthFreezeReconciled = "truth_freeze_reconciled"
)

// TruthPackageView is the truth ability's read/write surface for one
// feature package: where it lives, what state it is in, and — for
// blocked freeze attempts — exactly what stopped it.
type TruthPackageView struct {
	Slug             string         `json:"slug"`
	FeatureID        string         `json:"feature_id"`
	ProjectRoot      string         `json:"project_root"`
	Version          string         `json:"version"`
	Status           string         `json:"status"` // draft | frozen | change_pending
	Frozen           bool           `json:"frozen"`
	ChangePending    bool           `json:"change_pending"`
	PendingChangeIDs []string       `json:"pending_change_ids,omitempty"`
	PackageDir       string         `json:"package_dir"`
	TruthPath        string         `json:"truth_path"`
	TestsPath        string         `json:"tests_path"`
	TasksPath        string         `json:"tasks_path"`
	EvidencePath     string         `json:"evidence_path"`
	LifecycleStage   string         `json:"lifecycle_stage,omitempty"`
	LifecycleStatus  string         `json:"lifecycle_status,omitempty"`
	AlreadyFrozen    bool           `json:"already_frozen,omitempty"`
	ChangeID         string         `json:"change_id,omitempty"`
	ChangeDoc        string         `json:"change_doc,omitempty"`
	TruthSHA256      string         `json:"truth_sha256,omitempty"`
	Blocked          bool           `json:"blocked,omitempty"`
	BlockReasons     []string       `json:"block_reasons,omitempty"`
	Findings         []gate.Finding `json:"integrity_findings,omitempty"`
	// S1 (AC-136/139): the branch fact of the package's project
	// and, for frozen packages, the committed-baseline state.
	BranchCheck      string `json:"branch_check,omitempty"`
	CurrentBranch    string `json:"current_branch,omitempty"`
	CanonicalBranch  string `json:"canonical_branch,omitempty"`
	BaselineStatus   string `json:"baseline_status,omitempty"`
	BaselineCommit   string `json:"baseline_commit,omitempty"`
	BaselineRecorded bool   `json:"baseline_recorded,omitempty"`
	BaselineAlready  bool   `json:"baseline_already_recorded,omitempty"`
	BaselineNote     string `json:"baseline_note,omitempty"`
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

// appendFeatureEvent writes a feature-scoped trail entry (no run).
func (e *Engine) appendFeatureEvent(featureID, eventType string, payload map[string]any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = e.store.AppendEvent(store.LifecycleEvent{
		FeatureID: featureID, EventType: eventType, Payload: raw,
	})
	return err
}

// pendingChangeIDs scans the feature trail: change requests since
// the last freeze/refreeze clearing event, in request order (the
// same event vocabulary evalTruthFreeze consults).
func (e *Engine) pendingChangeIDs(featureID string) ([]string, error) {
	events, err := e.store.ListEvents(featureID)
	if err != nil {
		return nil, err
	}
	var pending []string
	for _, ev := range events {
		switch ev.EventType {
		case EventTruthChangeRequested, "change_requested", "gen_change":
			var p struct {
				ChangeID string `json:"change_id"`
			}
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			pending = append(pending, p.ChangeID)
		case EventTruthRefrozen, EventTruthFrozen, "refreeze", "gen_refreeze":
			pending = nil
		}
	}
	return pending, nil
}

// TruthPackage assembles the current view of one package from fresh
// store rows + the truth frontmatter (read-only).
func (e *Engine) TruthPackage(ctx store.FeatureContext) (TruthPackageView, error) {
	feat, err := e.store.Features().GetByID(ctx.Feature.ID)
	if err != nil {
		return TruthPackageView{}, err
	}
	ctx.Feature = feat
	pkg := e.packageFiles(ctx)
	pending, err := e.pendingChangeIDs(feat.ID)
	if err != nil {
		return TruthPackageView{}, err
	}
	fmStatus := ""
	if pkg.Truth.Present {
		scalars, _, ok := gate.ParseFrontmatter(pkg.Truth.Text)
		if ok {
			fmStatus = scalars["status"]
		}
	}
	v := TruthPackageView{
		Slug: feat.Slug, FeatureID: feat.ID, ProjectRoot: ctx.Project.RootPath,
		Version:      ctx.Version.Version,
		PackageDir:   filepath.Dir(feat.TruthPath),
		TruthPath:    feat.TruthPath,
		TestsPath:    pkg.Tests.Path,
		TasksPath:    pkg.Tasks.Path,
		EvidencePath: pkg.Evidence.Path,
	}
	if lc, err := e.store.Features().GetLifecycle(feat.ID); err == nil {
		v.LifecycleStage = string(lc.Stage)
		v.LifecycleStatus = string(lc.Status)
	}
	switch {
	case len(pending) > 0:
		v.Status = "change_pending"
		v.ChangePending = true
		v.PendingChangeIDs = pending
	case fmStatus != "":
		v.Status = fmStatus
	default:
		v.Status = feat.Status
	}
	v.Frozen = fmStatus == "frozen" && !v.ChangePending
	// S1 facts: the branch discipline state of the project
	// (always reported — a non-git project says so instead of
	// silently skipping) and, for a frozen package, whether the
	// frozen body is a committed baseline (AC-139).
	bi := e.branchInfo(ctx.Project.RootPath)
	v.BranchCheck, v.CurrentBranch, v.CanonicalBranch = bi.BranchCheck, bi.CurrentBranch, bi.CanonicalBranch
	if v.Frozen {
		bl := e.factBaseline(ctx, pkg.Truth)
		v.BaselineStatus, v.BaselineCommit = bl.Status, bl.Commit
	}
	return v, nil
}

// ---- create (write half) ----

// CreateTruthPackage creates a new draft package under a registered
// project's recorded version: the four-piece skeleton on disk, the
// feature_packages row, the Think/NotStarted lifecycle snapshot and
// the truth_created trail event.
func (e *Engine) CreateTruthPackage(projectRoot, version, slug string) (TruthPackageView, error) {
	if !slugRe.MatchString(slug) {
		return TruthPackageView{}, core.NewError(ReasonTruthSlugInvalid, "truth.Create",
			"slug "+slug+" is not a valid package slug (lowercase letters, digits, - and _)")
	}
	root := store.ResolveRootPath(projectRoot)
	proj, err := e.store.Projects().GetByRootPath(root)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TruthPackageView{}, core.NewError(ReasonProjectNotRegistered, "truth.Create",
				"project not registered: "+root+" (run `beacon projects register` first)")
		}
		return TruthPackageView{}, err
	}
	// AC-136: creating a package writes requirement truth — on a
	// git project that is canonical-branch work only.
	if _, err := e.requireCanonicalBranch(root, "truth.Create"); err != nil {
		return TruthPackageView{}, err
	}
	versions, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return TruthPackageView{}, err
	}
	var ver *store.ProjectVersion
	for i := range versions {
		if version != "" && versions[i].Version == version {
			ver = &versions[i]
			break
		}
		if version == "" && versions[i].IsCurrent {
			ver = &versions[i]
		}
	}
	if ver == nil {
		return TruthPackageView{}, core.NewError(ReasonTruthVersionMissing, "truth.Create",
			"no usable docs version recorded for project "+root+" (register one with `beacon projects register --version`)")
	}
	if _, err := e.store.Features().GetBySlug(ver.ID, slug); err == nil {
		return TruthPackageView{}, core.NewError(ReasonTruthPackageExists, "truth.Create",
			"package "+slug+" already exists in version "+ver.Version)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return TruthPackageView{}, err
	}
	pkgRel := filepath.Join(ver.DocsRoot, "features", slug)
	absDir := resolvePkgPath(root, pkgRel)
	if _, err := os.Stat(absDir); err == nil {
		return TruthPackageView{}, core.NewError(ReasonTruthPackageDirExists, "truth.Create",
			"directory "+pkgRel+" already exists on disk but is not registered (migrate it or move it away)")
	} else if !os.IsNotExist(err) {
		return TruthPackageView{}, err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return TruthPackageView{}, err
	}
	files := truthSkeleton(slug, ver.Version, filepath.ToSlash(pkgRel))
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(absDir, name), []byte(text), 0o644); err != nil {
			return TruthPackageView{}, err
		}
	}
	feat, err := e.store.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: ver.ID, Slug: slug,
		TruthPath: filepath.Join(pkgRel, "truth.md"),
		TestsPath: filepath.Join(pkgRel, "tests.md"),
		Status:    "draft",
	})
	if err != nil {
		return TruthPackageView{}, err
	}
	if err := e.store.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: feat.ID, Stage: core.StageThink, Status: core.StatusNotStarted,
	}); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.appendFeatureEvent(feat.ID, EventTruthCreated, map[string]any{
		"slug": slug, "version": ver.Version, "package_dir": filepath.ToSlash(pkgRel),
	}); err != nil {
		return TruthPackageView{}, err
	}
	ctx := store.FeatureContext{Project: proj, Version: *ver, Feature: feat}
	return e.TruthPackage(ctx)
}

// ---- freeze / refreeze shared precheck ----

// freezePrecheck runs the freeze gate's factual half: the four-piece
// set complete and non-empty, then the Truth Integrity Gate in
// block mode (AC-101 judgement, reused — which also covers the
// no-undecided-deferral requirement via its deferral_pending
// findings). It returns the blocked view when anything fails.
func (e *Engine) freezePrecheck(ctx store.FeatureContext, by string) (TruthPackageView, bool, error) {
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, false, err
	}
	pkg := e.packageFiles(ctx)
	members := []struct {
		name string
		pf   gate.PackageFile
	}{
		{"truth", pkg.Truth}, {"tests", pkg.Tests}, {"tasks", pkg.Tasks}, {"evidence", pkg.Evidence},
	}
	for _, m := range members {
		if !m.pf.Present || strings.TrimSpace(m.pf.Text) == "" {
			view.Blocked = true
			view.BlockReasons = []string{core.ReasonIntegrityPackageIncomplete}
			view.Findings = []gate.Finding{{
				Kind: "missing_or_empty_" + m.name, ReasonCode: core.ReasonIntegrityPackageIncomplete,
				File: m.pf.Path, Detail: "package member " + m.name + " is missing or empty",
				RepairRoute: gate.DefaultRepairRoute,
			}}
			return view, false, nil
		}
	}
	iv, err := e.EvaluateIntegrity(ctx, by, gate.IntegrityModeBlock)
	if err != nil {
		return TruthPackageView{}, false, err
	}
	if iv.Verdict == string(gate.VerdictBlocked) {
		view.Blocked = true
		view.BlockReasons = iv.ReasonCodes
		view.Findings = iv.Findings
		return view, false, nil
	}
	return view, true, nil
}

// applyFreeze performs the freeze write half shared by freeze and
// refreeze: hash the current truth bytes, flip the frontmatter
// status line to frozen (the one sanctioned body write), move the
// package status + lifecycle, and append the trail event.
func (e *Engine) applyFreeze(ctx store.FeatureContext, eventType string, payload map[string]any) (TruthPackageView, error) {
	root := ctx.Project.RootPath
	abs := resolvePkgPath(root, ctx.Feature.TruthPath)
	raw, err := os.ReadFile(abs)
	if err != nil {
		return TruthPackageView{}, err
	}
	sum := sha256.Sum256(raw)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["slug"] = ctx.Feature.Slug
	payload["truth_sha256_before"] = hex.EncodeToString(sum[:])
	// Package body fingerprint (P4c): the drift baseline doctor
	// rebuild checks later. Recorded from the pre-flip bytes — the
	// fingerprint normalizes the status line out, so it describes
	// the frozen BODY, identical before and after the flip.
	testsRaw, _ := os.ReadFile(resolvePkgPath(root, ctx.Feature.TestsPath))
	payload["package_fingerprint"] = core.PackageFingerprint(raw, testsRaw)
	payload["fingerprint_alg"] = core.FingerprintAlg
	// S1 (AC-137): the four-surface fingerprint (the refreeze
	// transaction's proof base — at least one requirement surface
	// must change) and the document's declared revision id (the
	// drift comparison base), recorded from the same bytes.
	pkg := e.packageFiles(ctx)
	payload["surface_fingerprint"] = core.PackageSurfaceFingerprint(
		[]byte(pkg.Truth.Text), []byte(pkg.Tests.Text), []byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text))
	payload["surface_fingerprint_alg"] = core.SurfaceFingerprintAlg
	if scalars, _, ok := gate.ParseFrontmatter(string(raw)); ok {
		if rev := scalars["revision_id"]; rev != "" {
			payload["revision_id"] = rev
		} else if rev := scalars["revision"]; rev != "" {
			payload["revision_id"] = rev
		}
	}
	if updated, changed := setFrontmatterStatus(string(raw), "frozen"); changed {
		if err := os.WriteFile(abs, []byte(updated), 0o644); err != nil {
			return TruthPackageView{}, err
		}
	}
	if err := e.store.Features().UpdatePackageStatus(ctx.Feature.ID, "frozen"); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.store.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: ctx.Feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted,
	}); err != nil {
		return TruthPackageView{}, err
	}
	// S1 (AC-139): the post-flip body hash plus a baseline
	// attempt. The flip is normally still uncommitted at this
	// instant, so the event usually records baseline_status=
	// uncommitted and the baseline lands later via commit +
	// `truth baseline`; when the frozen body is already committed
	// the baseline_commit rides in this very event.
	if finalRaw, ferr := os.ReadFile(abs); ferr == nil {
		payload["truth_sha256_after"] = sha256HexOf(finalRaw)
	}
	bl := e.probeBaseline(ctx)
	payload["baseline_status"] = bl.Status
	if bl.Status == BaselineCommitted {
		payload["baseline_commit"] = bl.Commit
	}
	if err := e.appendFeatureEvent(ctx.Feature.ID, eventType, payload); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	view.TruthSHA256 = hex.EncodeToString(sum[:])
	return view, nil
}

// FreezeTruthPackage freezes a draft package: precheck (four-piece
// completeness + integrity block-mode clean), then the status-line
// flip + Freeze/Completed lifecycle + truth_frozen event carrying
// the pre-flip truth hash. Re-freezing an already frozen package
// (no change pending) is idempotent.
func (e *Engine) FreezeTruthPackage(ctx store.FeatureContext, by string) (TruthPackageView, error) {
	// AC-136: freezing writes requirement truth — canonical-only.
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Freeze"); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if view.ChangePending {
		return view, core.NewError(core.ReasonTruthChangePending, "truth.Freeze",
			"package "+view.Slug+" has a pending change; close it with `beacon truth refreeze`")
	}
	if view.Frozen {
		// Idempotent re-freeze: reconcile the lifecycle snapshot
		// if it drifted, but never rewrite the document or stack
		// duplicate freeze events when nothing changed.
		if view.LifecycleStage != string(core.StageFreeze) || view.LifecycleStatus != string(core.StatusCompleted) {
			if err := e.store.Features().UpdateLifecycle(store.Lifecycle{
				FeatureID: ctx.Feature.ID, Stage: core.StageFreeze, Status: core.StatusCompleted,
			}); err != nil {
				return TruthPackageView{}, err
			}
			raw, rerr := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath))
			if rerr != nil {
				return TruthPackageView{}, rerr
			}
			sum := sha256.Sum256(raw)
			if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthFreezeReconciled, map[string]any{
				"slug": view.Slug, "by": by, "truth_sha256": hex.EncodeToString(sum[:]),
			}); err != nil {
				return TruthPackageView{}, err
			}
			view, err = e.TruthPackage(ctx)
			if err != nil {
				return TruthPackageView{}, err
			}
		}
		// AC-139 auto-record: an already-frozen package whose
		// body is committed but carries no baseline record yet
		// gets the record in passing (the same recording
		// `truth baseline` performs explicitly).
		if recs, rerr := e.trailRecords(ctx.Feature.ID); rerr == nil && recs.baseline == nil {
			if bl := e.probeBaseline(ctx); bl.Status == BaselineCommitted {
				if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthBaselineRecorded, map[string]any{
					"baseline_commit": bl.Commit, "truth_sha256": bl.TruthSHA256,
					"canonical_branch": view.CanonicalBranch, "by": by, "auto": true,
				}); err != nil {
					return TruthPackageView{}, err
				}
				view.BaselineStatus, view.BaselineCommit = bl.Status, bl.Commit
				view.BaselineRecorded = true
			}
		}
		view.AlreadyFrozen = true
		return view, nil
	}
	pre, ok, err := e.freezePrecheck(ctx, by)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !ok {
		return pre, freezeBlockedError("truth.Freeze", pre)
	}
	return e.applyFreeze(ctx, EventTruthFrozen, map[string]any{"by": by})
}

// freezeBlockedError builds the typed failure for a blocked
// freeze/refreeze: the headline reason is the first block code.
func freezeBlockedError(op string, view TruthPackageView) error {
	code := ReasonTruthFreezeBlocked
	if len(view.BlockReasons) > 0 {
		code = view.BlockReasons[0]
	}
	return core.NewError(code, op, fmt.Sprintf(
		"package %s is blocked by the freeze precheck (%d finding(s)); first reasons: %s",
		view.Slug, len(view.Findings), strings.Join(view.BlockReasons, ", ")))
}
