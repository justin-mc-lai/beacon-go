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
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// ACDiff is the requirement-level difference one refreeze closes:
// which ACs the change added, changed and removed. The same value is
// rendered into the change document's `## AC Diff` section and stored
// in the truth_refrozen event payload, so `truth show --json` can
// answer which ACs a change covered without re-reading the document.
type ACDiff struct {
	Added   []string       `json:"added"`
	Changed []ACDiffChange `json:"changed"`
	Removed []string       `json:"removed"`
}

// ACDiffChange is one AC present on both sides with a different
// requirement fingerprint (the first 12 hex chars are the stable
// comparison prefix).
type ACDiffChange struct {
	AC     string `json:"ac"`
	Before string `json:"before"`
	After  string `json:"after"`
}

const (
	ReasonTruthACDiffBaseMissing = "truth_change_ac_diff_base_missing"
	ReasonTruthACDiffWriteFailed = "truth_change_ac_diff_write_failed"
	acDiffSectionHeading         = "## AC Diff"
	acFingerprintPrefixLen       = 12
)

// fingerprintPrefix is the stable short form written into the change
// document and the JSON view.
func fingerprintPrefix(fp string) string {
	if len(fp) > acFingerprintPrefixLen {
		return fp[:acFingerprintPrefixLen]
	}
	return fp
}

// sortedFingerprintKeys returns the AC ids of a fingerprint map in
// stable order (the document must not depend on map iteration).
func sortedFingerprintKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// renderACDiff renders the change document section. The three groups
// are always present, so "removed: (none)" is explicit rather than an
// absent section.
func renderACDiff(diff ACDiff) string {
	var b strings.Builder
	b.WriteString("\n" + acDiffSectionHeading + "\n\n")
	b.WriteString("added:\n")
	if len(diff.Added) == 0 {
		b.WriteString("- (none)\n")
	}
	for _, id := range diff.Added {
		b.WriteString("- " + id + "\n")
	}
	b.WriteString("\nchanged:\n")
	if len(diff.Changed) == 0 {
		b.WriteString("- (none)\n")
	}
	for _, c := range diff.Changed {
		b.WriteString(fmt.Sprintf("- %s (before %s → after %s)\n", c.AC, c.Before, c.After))
	}
	b.WriteString("\nremoved:\n")
	if len(diff.Removed) == 0 {
		b.WriteString("- (none)\n")
	}
	for _, id := range diff.Removed {
		b.WriteString("- " + id + "\n")
	}
	return b.String()
}

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
	baseReqFP := map[string]string{}
	if recs.freeze != nil {
		for k, v := range recs.freeze.RequirementFingerprints {
			baseReqFP[k] = v
		}
	}
	if len(baseReqFP) == 0 {
		if rawTruth, rerr := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath)); rerr == nil {
			baseReqFP = core.RequirementFingerprints(rawTruth)
		}
	}
	var baseFPBlock strings.Builder
	if len(baseReqFP) > 0 {
		baseFPBlock.WriteString("base_requirement_fingerprints:\n")
		for _, id := range sortedFingerprintKeys(baseReqFP) {
			baseFPBlock.WriteString("  " + id + ": " + baseReqFP[id] + "\n")
		}
	}
	doc := fmt.Sprintf(`---
change_id: %s
slug: %s
status: pending
created_by: %s
created_at: %s
base_truth_sha256: %s
base_package_fingerprint: %s
%s
---

# Change Request: %s

%s

## Route

Edit the package truth (and its tests/tasks/evidence as needed), then close this change with `+"`beacon truth refreeze`"+`. The change stays pending — and the truth_freeze gate stays blocked — until the refreeze lands.
`, changeID, view.Slug, by, nowISO(), baseTruthSHA, basePackageFP, baseFPBlock.String(), summary, summary)
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
		"base_requirement_fingerprints": baseReqFP, "requirement_fingerprint_alg": core.RequirementFingerprintAlg,
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

// CancelTruthChange closes a pending change when its body never moved:
// the package returns to frozen without pretending a refreeze happened.
// A body that actually changed must go through refreeze; a change whose
// base record is missing needs the explicit acceptCurrent acknowledgement.
func (e *Engine) CancelTruthChange(ctx store.FeatureContext, by string, acceptCurrent bool) (TruthPackageView, error) {
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Cancel"); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !view.ChangePending {
		return view, nil // idempotent: nothing to cancel
	}
	pkgDir := filepath.Dir(ctx.Feature.TruthPath)
	rawTruth, err := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath))
	if err != nil {
		return view, err
	}
	liveSHA := sha256HexOf(rawTruth)
	baseMissing := false
	bases := make(map[string]string, len(view.PendingChangeIDs))
	for _, id := range view.PendingChangeIDs {
		docAbs := filepath.Join(resolvePkgPath(ctx.Project.RootPath, pkgDir), "changes", id+".md")
		baseSHA := ""
		if raw, rerr := os.ReadFile(docAbs); rerr == nil {
			if scalars, _, ok := gate.ParseFrontmatter(string(raw)); ok {
				baseSHA = scalars["base_truth_sha256"]
			}
		}
		bases[id] = baseSHA
		if baseSHA == "" {
			baseMissing = true
			if !acceptCurrent {
				return view, core.NewError(core.ReasonTruthRebaseNotAllowed, "truth.Cancel",
					"change "+id+" has no base fingerprint; re-run with --accept-current only after confirming the body was never edited")
			}
			continue
		}
		if baseSHA != liveSHA {
			return view, core.NewError(ReasonTruthChangePending, "truth.Cancel",
				"change "+id+" has a modified body (base "+fingerprintPrefix(baseSHA)+" vs live "+fingerprintPrefix(liveSHA)+"); run `beacon truth refreeze` instead of cancelling")
		}
	}
	ids := append([]string{}, view.PendingChangeIDs...)
	// AC-001/AC-005: the cancel event names the decision and the
	// evidence behind it. AC-005 keeps changes/<id>.md byte-identical,
	// so the doc status is never rewritten; pending state is derived
	// from events, and the cancel event only ever appends.
	payload := map[string]any{
		"by": by, "change_ids": ids, "decision": "cancel",
		"accepted_current": acceptCurrent, "baseline_missing": baseMissing,
		"truth_sha256": liveSHA,
	}
	if len(ids) == 1 {
		id := ids[0]
		payload["change_id"] = id
		payload["change_doc"] = filepath.ToSlash(filepath.Join(pkgDir, "changes", id+".md"))
		payload["base_truth_sha256"] = bases[id]
	} else if len(ids) > 1 {
		changes := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			changes = append(changes, map[string]any{
				"change_id":         id,
				"change_doc":        filepath.ToSlash(filepath.Join(pkgDir, "changes", id+".md")),
				"base_truth_sha256": bases[id],
			})
		}
		payload["changes"] = changes
	}
	if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthChangeCancelled, payload); err != nil {
		return view, err
	}
	return e.TruthPackage(ctx)
}

// RefreezeTruthPackage closes pending changes with the strict base
// checks; RefreezeTruthPackageAcceptCurrent is the explicit escape for
// legacy freeze records that carry no base fingerprints.
func (e *Engine) RefreezeTruthPackage(ctx store.FeatureContext, by string) (TruthPackageView, error) {
	return e.RefreezeTruthPackageAcceptCurrent(ctx, by, false)
}

func (e *Engine) RefreezeTruthPackageAcceptCurrent(ctx store.FeatureContext, by string, acceptCurrent bool) (TruthPackageView, error) {
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
	// parallel change landed first) must be rebased explicitly. With
	// --accept-current the owner states the current body is the base.
	if !acceptCurrent {
		if err := e.checkChangeBase(ctx); err != nil {
			return view, err
		}
	}
	// AC-137: the refreeze transaction checks (content moved,
	// revision id agrees, freeze->change fingerprint chain
	// intact) run before the freeze precheck — a drifted or
	// empty refreeze must not even reach the package checks.
	if !acceptCurrent {
		if err := e.refreezeTxGuard(ctx); err != nil {
			return view, err
		}
	}
	pre, ok, err := e.freezePrecheck(ctx, by)
	if err != nil {
		return TruthPackageView{}, err
	}
	if !ok {
		return pre, freezeBlockedError("truth.Refreeze", pre)
	}
	// K7 (AC-004): compute the base -> after AC difference BEFORE the
	// status flip. A missing base fingerprint or a change document
	// that cannot be written fails the refreeze closed: the package
	// stays change_pending and no truth_refrozen event is written.
	diff, err := e.changeACDiff(ctx)
	if err != nil {
		if !acceptCurrent {
			return view, err
		}
		// ponytail: no base fingerprints on record (legacy freeze) —
		// accept-current closes the change with an explicit empty diff
		// instead of leaving the package wedged in change_pending.
		diff = ACDiff{Added: []string{}, Changed: []ACDiffChange{}, Removed: []string{}}
	}
	if err := e.writeChangeACDiff(ctx, diff); err != nil {
		return view, err
	}
	changeIDs := append([]string{}, view.PendingChangeIDs...)
	// applyFreeze's truth_refrozen event clears the pending list in
	// the trail scan, so the returned view already reads frozen with
	// no pending change; changeIDs rides along in the event payload.
	return e.applyFreeze(ctx, EventTruthRefrozen, map[string]any{
		"by": by, "change_ids": changeIDs, "change_ac_diff": diff,
	})
}

// changeACDiff compares the base requirement fingerprints recorded at
// change-open with the truth about to be frozen. It fails closed when
// neither the change record nor the latest freeze record carries a
// base: without a base there is no answer to "which ACs did this
// change cover", and a silent empty diff would be a fake answer.
func (e *Engine) changeACDiff(ctx store.FeatureContext) (ACDiff, error) {
	empty := ACDiff{Added: []string{}, Changed: []ACDiffChange{}, Removed: []string{}}
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return empty, err
	}
	base := map[string]string{}
	if len(recs.pendingChanges) > 0 {
		for k, v := range recs.pendingChanges[0].BaseRequirementFingerprints {
			base[k] = v
		}
	}
	if len(base) == 0 && recs.freeze != nil {
		for k, v := range recs.freeze.RequirementFingerprints {
			base[k] = v
		}
	}
	if len(base) == 0 {
		return empty, core.NewError(ReasonTruthACDiffBaseMissing, "truth.Refreeze",
			"the pending change carries no base requirement fingerprints and the latest freeze record has none; AC diff cannot be computed (fail-closed)")
	}
	raw, err := os.ReadFile(resolvePkgPath(ctx.Project.RootPath, ctx.Feature.TruthPath))
	if err != nil {
		return empty, err
	}
	after := core.RequirementFingerprints(raw)
	union := map[string]bool{}
	for id := range base {
		union[id] = true
	}
	for id := range after {
		union[id] = true
	}
	ids := make([]string, 0, len(union))
	for id := range union {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		b, inBase := base[id]
		a, inAfter := after[id]
		switch {
		case !inBase && inAfter:
			empty.Added = append(empty.Added, id)
		case inBase && !inAfter:
			empty.Removed = append(empty.Removed, id)
		case inBase && inAfter && b != a:
			empty.Changed = append(empty.Changed, ACDiffChange{
				AC: id, Before: fingerprintPrefix(b), After: fingerprintPrefix(a),
			})
		}
	}
	return empty, nil
}

// writeChangeACDiff appends the AC Diff section to every change
// document this refreeze closes. The section is replaced when present,
// so retrying after a later failure cannot stack duplicate sections.
func (e *Engine) writeChangeACDiff(ctx store.FeatureContext, diff ACDiff) error {
	recs, err := e.trailRecords(ctx.Feature.ID)
	if err != nil {
		return err
	}
	pkgDir := filepath.Dir(ctx.Feature.TruthPath)
	paths := make([]string, 0, len(recs.pendingChanges))
	for _, ch := range recs.pendingChanges {
		if ch.ChangeID == "" {
			continue
		}
		paths = append(paths, filepath.Join(pkgDir, "changes", ch.ChangeID+".md"))
	}
	for _, rel := range paths {
		abs := resolvePkgPath(ctx.Project.RootPath, rel)
		text, rerr := os.ReadFile(abs)
		if rerr != nil {
			return core.NewError(ReasonTruthACDiffWriteFailed, "truth.Refreeze",
				"change document "+filepath.ToSlash(rel)+" cannot be read for the AC Diff: "+rerr.Error())
		}
		body := string(text)
		if idx := strings.Index(body, "\n"+acDiffSectionHeading); idx >= 0 {
			body = body[:idx]
		}
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if werr := os.WriteFile(abs, []byte(body+renderACDiff(diff)), 0o644); werr != nil {
			return core.NewError(ReasonTruthACDiffWriteFailed, "truth.Refreeze",
				"change document "+filepath.ToSlash(rel)+" cannot be written: "+werr.Error())
		}
	}
	return nil
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
