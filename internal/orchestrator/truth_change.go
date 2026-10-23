// Truth change + refreeze (P4a): the gen-change -> gen-refreeze
// governance half of the truth package service in truth.go.
// change registers a change document + trail event against a
// frozen package without touching its body; refreeze re-runs
// the freeze precheck and closes the pending change ids.
package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// ---- change ----

// RequestTruthChange registers a change against a FROZEN package
// (gen-change): it writes the change document under the package's
// changes/ directory, marks the package change_pending and appends
// truth_change_requested. The truth body is never touched — its
// hash before and after is identical by construction (the file is
// only read, for the frozen check).
func (e *Engine) RequestTruthChange(ctx store.FeatureContext, summary, by string) (TruthPackageView, error) {
	// AC-136: registering a change writes into the package —
	// canonical-only.
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Change"); err != nil {
		return TruthPackageView{}, err
	}
	if strings.TrimSpace(summary) == "" {
		return TruthPackageView{}, core.NewError(ReasonTruthChangeSummaryReq, "truth.Change",
			"a change summary is required (--summary)")
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if view.ChangePending {
		return view, core.NewError(ReasonTruthChangePending, "truth.Change",
			"package "+view.Slug+" already has a pending change ("+strings.Join(view.PendingChangeIDs, ", ")+"); refreeze it first")
	}
	if !view.Frozen {
		return view, core.NewError(ReasonTruthNotFrozen, "truth.Change",
			"package "+view.Slug+" is not frozen (status "+view.Status+"); draft packages are edited directly, change registration is for frozen truth")
	}
	var rnd [6]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return TruthPackageView{}, err
	}
	changeID := "chg_" + hex.EncodeToString(rnd[:])
	pkgDir := filepath.Dir(ctx.Feature.TruthPath)
	changeRel := filepath.Join(pkgDir, "changes", changeID+".md")
	// R1: record the frozen body the change was authored against.
	// A later refreeze compares this base with the then-current
	// freeze record; a mismatch means a parallel change landed and
	// the author must rebase explicitly (never silent overwrite).
	recs, _ := e.trailRecords(ctx.Feature.ID)
	baseTruthSHA := ""
	basePackageFP := ""
	if recs.freeze != nil {
		baseTruthSHA = recs.freeze.TruthSHA256After
		basePackageFP = recs.freeze.Fingerprint
	}
	if baseTruthSHA == "" {
		rawTruth, _ := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath))
		baseTruthSHA = sha256HexOf(rawTruth)
	}
	doc := fmt.Sprintf(`---
change_id: %s
slug: %s
status: pending
created_by: %s
created_at: %s
base_truth_sha256: %s
base_package_fingerprint: %s
---

# Change Request: %s

%s

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `+"`beacon truth refreeze`"+`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
`, changeID, view.Slug, by, nowISO(), baseTruthSHA, basePackageFP, summary, summary)
	absChange := resolvePkgPath(ctx.Project.RootPath, changeRel)
	if err := os.MkdirAll(filepath.Dir(absChange), 0o755); err != nil {
		return TruthPackageView{}, err
	}
	if err := os.WriteFile(absChange, []byte(doc), 0o644); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.store.Features().UpdatePackageStatus(ctx.Feature.ID, "change_pending"); err != nil {
		return TruthPackageView{}, err
	}
	// S1 (AC-137): the package fingerprint AT CHANGE-OPEN is the
	// middle link of the freeze -> change -> refreeze proof
	// chain: it must still equal the frozen body's fingerprint,
	// or the edit began outside the change flow (drift).
	pkg := e.packageFiles(ctx)
	openFP := core.PackageFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text))
	// M7 (R2a): opening a change marks the tests/tasks materials
	// stale — they describe the pre-change truth until the
	// refreeze refreshes them. This runs AFTER openFP is taken:
	// the fingerprint-at-open must describe the package as the
	// change found it, not as the marking left it.
	staleMarked, err := e.markPackageMaterialsStale(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthChangeRequested, map[string]any{
		"change_id": changeID, "summary": summary, "by": by,
		"change_doc":          filepath.ToSlash(changeRel),
		"package_fingerprint": openFP, "fingerprint_alg": core.FingerprintAlg,
		"base_truth_sha256": baseTruthSHA, "base_package_fingerprint": basePackageFP,
		"materials_marked_stale": staleMarked,
	}); err != nil {
		return TruthPackageView{}, err
	}
	out, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	out.ChangeID = changeID
	out.ChangeDoc = filepath.ToSlash(changeRel)
	return out, nil
}

// ---- refreeze ----

// RefreezeTruthPackage closes a pending change (gen-refreeze): the
// same precheck as freeze, then the status line is set back to
// frozen, the pending state cleared, and a truth_refrozen event
// records exactly which change ids it closed.
func (e *Engine) RefreezeTruthPackage(ctx store.FeatureContext, by string) (TruthPackageView, error) {
	// AC-136: refreezing writes requirement truth — canonical-only.
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Refreeze"); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !view.ChangePending {
		return view, core.NewError(ReasonTruthNoChangePending, "truth.Refreeze",
			"package "+view.Slug+" has no pending change to refreeze")
	}
	// R1: a change authored against an older frozen body (because a
	// parallel change landed first) must be rebased explicitly.
	if err := e.checkChangeBase(ctx); err != nil {
		return view, err
	}
	// AC-137: the refreeze transaction checks (content moved,
	// revision id agrees, freeze->change fingerprint chain
	// intact) run before the freeze precheck — a drifted or
	// empty refreeze must not even reach the package checks.
	if err := e.refreezeTxGuard(ctx); err != nil {
		return view, err
	}
	pre, ok, err := e.freezePrecheck(ctx, by)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !ok {
		return pre, freezeBlockedError("truth.Refreeze", pre)
	}
	changeIDs := append([]string{}, view.PendingChangeIDs...)
	// applyFreeze's truth_refrozen event clears the pending list in
	// the trail scan, so the returned view already reads frozen with
	// no pending change; changeIDs rides along in the event payload.
	return e.applyFreeze(ctx, EventTruthRefrozen, map[string]any{
		"by": by, "change_ids": changeIDs,
	})
}

// checkChangeBase is the R1 parallel-change guard: every pending
// change names the frozen body it was authored against; if the latest
// freeze record moved past that base, another change landed and this
// refreeze would silently overwrite it. Fail closed with a rebase
// instruction instead.
func (e *Engine) checkChangeBase(ctx store.FeatureContext) error {
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return err
	}
	if recs.freeze == nil || recs.freeze.TruthSHA256After == "" {
		return nil // pre-R1 history: nothing to prove against
	}
	for _, ch := range recs.pendingChanges {
		if ch.BaseTruthSHA256 == "" {
			continue
		}
		if ch.BaseTruthSHA256 != recs.freeze.TruthSHA256After {
			return core.NewError(core.ReasonTruthBaseDiverged, "truth.Refreeze",
				"change "+ch.ChangeID+" was opened on frozen body "+short(ch.BaseTruthSHA256)+
					" but the latest freeze is "+short(recs.freeze.TruthSHA256After)+
					"; another change landed first. Merge it, then run `beacon truth rebase --change "+
					ch.ChangeID+" --accept-current` before refreezing")
		}
	}
	return nil
}

// refreezeTxGuard enforces the AC-137 refreeze transaction checks
// (all fail-closed, all fingerprint-based — no second hash system,
// the P4c/S1 fingerprint family is the only evidence):
//
//  1. Content moved: the four-surface fingerprint now must
//     differ from the one recorded at the last freeze. A
//     refreeze whose only payload is the stated reason or the
//     regenerated machine projection changes no surface and is
//     refused (empty_refreeze_reason_only — the frozen reason
//     names both halves of that case). Pre-S1 freeze records
//     carry only the truth+tests fingerprint; the comparison
//     then uses that pair.
//  2. Revision id: when both the document's frontmatter
//     revision/revision_id and the last freeze record declare an
//     id, they must agree (revision_authority_drift).
//  3. Chain: the package fingerprint recorded when the pending
//     change was OPENED must equal the fingerprint recorded at
//     the last freeze. v1 compared three revision representations
//     (narrative document / machine projection / revision record);
//     the Go kernel's equivalent chain is freeze fingerprint ->
//     change-open fingerprint — a mismatch proves the narrative
//     body moved between the freeze and the change registration,
//     i.e. the edit began outside the governed flow
//     (revision_authority_drift).
func (e *Engine) refreezeTxGuard(ctx store.FeatureContext) error {
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return err
	}
	if recs.freeze == nil {
		return nil // no freeze record to prove against (pre-kernel history): nothing to check
	}
	pkg := e.packageFiles(ctx)
	if recs.freeze.SurfaceFingerprint != "" {
		cur := core.PackageSurfaceFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text), []byte(pkg.Tasks.Text), []byte(pkg.Evidence.Text))
		if cur == recs.freeze.SurfaceFingerprint {
			return core.NewError(core.ReasonEmptyRefreezeReasonOnly, "truth.Refreeze",
				"no requirement surface (truth/tests/tasks/evidence) changed since the last freeze; a refreeze that only restates the reason or the machine projection is refused (AC-137)")
		}
	} else if recs.freeze.Fingerprint != "" {
		cur := core.PackageFingerprint([]byte(pkg.Truth.Text), []byte(pkg.Tests.Text))
		if cur == recs.freeze.Fingerprint {
			return core.NewError(core.ReasonEmptyRefreezeReasonOnly, "truth.Refreeze",
				"the package body (truth+tests) is unchanged since the last freeze; a refreeze that only restates the reason or the machine projection is refused (AC-137)")
		}
	}
	if recs.freeze.RevisionID != "" {
		if scalars, _, ok := gate.ParseFrontmatter(pkg.Truth.Text); ok {
			curRev := scalars["revision_id"]
			if curRev == "" {
				curRev = scalars["revision"]
			}
			if curRev != "" && curRev != recs.freeze.RevisionID {
				return core.NewError(core.ReasonRevisionAuthorityDrift, "truth.Refreeze",
					"truth frontmatter revision "+curRev+" disagrees with revision "+
						recs.freeze.RevisionID+" recorded at the last freeze (AC-137)")
			}
		}
	}
	if recs.freeze.Fingerprint != "" && len(recs.pendingChanges) > 0 {
		first := recs.pendingChanges[0]
		if first.FingerprintAtOpen != "" && first.FingerprintAtOpen != recs.freeze.Fingerprint {
			return core.NewError(core.ReasonRevisionAuthorityDrift, "truth.Refreeze",
				"the package body had already moved when change "+first.ChangeID+
					" was opened (fingerprint at open != frozen body); the edit began outside the change flow (AC-137)")
		}
	}
	return nil
}
