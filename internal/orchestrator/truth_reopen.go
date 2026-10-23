// Truth reopen (R2d, parity audit M14): the Go port of Python
// commands/reopen.py + utils/revision_state.py's
// reopen_revision_state. Reopen opens a NEW REVISION of a
// frozen/active package:
//
//   - the revision id increments (R1 -> R2 -> ...), the new
//     revision records its source revision;
//   - the old frozen revision stays on the trail (the reopen
//     event carries its ids, its frozen body hash and its
//     package fingerprint) — nothing about the old freeze is
//     erased or rewritten;
//   - the working document returns to draft, derived from the
//     old body (the body text itself is NOT forked into a new
//     file: Go packages carry one working truth.md per feature,
//     and the revision chain lives in its frontmatter —
//     revision_id / source_revision_id — plus the trail);
//   - the intent signature is voided and the tests/tasks
//     materials are marked stale (M7): the new revision must
//     walk ask -> sign -> freeze again, exactly like Python's
//     new draft revision.
//
// Reopen is NOT change/refreeze: change registers an intent to
// edit a frozen package and refreeze closes it in place (same
// revision); reopen starts a new revision cycle. It is also
// NOT rebase: rebase only re-bases a pending change onto a
// newer frozen body.
package orchestrator

import (
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// EventTruthReopened trails one reopen (feature-scoped).
const EventTruthReopened = "truth_reopened"

// currentRevisionID reads the package's revision id: the
// document's own revision_id/revision scalar wins, then the
// last freeze record's, then Python's default R1.
func (e *Engine) currentRevisionID(ctx store.FeatureContext, truthText string) string {
	if scalars, _, ok := gate.ParseFrontmatter(truthText); ok {
		if rev := strings.TrimSpace(scalars["revision_id"]); rev != "" {
			return rev
		}
		if rev := strings.TrimSpace(scalars["revision"]); rev != "" {
			return rev
		}
	}
	if recs, err := e.trailRecords(ctx.Feature.ID); err == nil && recs.freeze != nil && recs.freeze.RevisionID != "" {
		return recs.freeze.RevisionID
	}
	return "R1"
}

// nextRevisionID ports reopen_revision_state's increment:
// parse the numeric suffix (a malformed id counts as R1).
func nextRevisionID(current string) string {
	num, err := strconv.Atoi(strings.TrimLeft(current, "R"))
	if err != nil {
		num = 1
	}
	return "R" + strconv.Itoa(num+1)
}

// ReopenTruthPackage opens a new revision of a frozen package.
func (e *Engine) ReopenTruthPackage(ctx store.FeatureContext, reason, by string) (TruthPackageView, error) {
	// Python checks the truth-authoring branch first (reopen
	// writes requirement truth).
	if _, err := e.requireCanonicalBranch(ctx.Project.RootPath, "truth.Reopen"); err != nil {
		return TruthPackageView{}, err
	}
	view, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	if view.ChangePending {
		return view, core.NewError(core.ReasonTruthChangePending, "truth.Reopen",
			"package "+view.Slug+" has a pending change; close it with refreeze before reopening a new revision")
	}
	if !view.Frozen {
		return view, core.NewError(core.ReasonTruthReopenNotFrozen, "truth.Reopen",
			"package "+view.Slug+" is not frozen (status "+view.Status+"); reopen opens a new revision of a frozen package — a draft is simply edited")
	}
	if strings.TrimSpace(reason) == "" {
		reason = "change-request" // commands/reopen.py's default
	}
	text, err := readTruthText(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	prevRevision := e.currentRevisionID(ctx, text)
	nextRevision := nextRevisionID(prevRevision)

	// Old-revision trail facts BEFORE any write: the frozen
	// body hash + package fingerprint of the revision being
	// superseded.
	recs, _ := e.trailRecords(ctx.Feature.ID)
	prevTruthSHA, prevFingerprint := "", ""
	if recs.freeze != nil {
		prevTruthSHA = recs.freeze.TruthSHA256After
		prevFingerprint = recs.freeze.Fingerprint
	}

	// The new revision derives from the old body: only the
	// sanctioned frontmatter lines move (status back to draft,
	// the revision chain, the voided signature).
	text, _ = setFrontmatterStatus(text, "draft")
	text, _ = setFrontmatterScalar(text, "revision_id", nextRevision)
	text, _ = setFrontmatterScalar(text, "source_revision_id", prevRevision)
	text, _ = setFrontmatterScalar(text, "intent_confirmed", "false")
	if err := writeTruthText(ctx, text); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.store.Features().UpdatePackageStatus(ctx.Feature.ID, "draft"); err != nil {
		return TruthPackageView{}, err
	}
	// The new revision starts the lifecycle over (create's
	// snapshot), like Python's fresh draft revision state
	// (doc_status draft, delivery idle).
	if err := e.store.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: ctx.Feature.ID, Stage: core.StageThink, Status: core.StatusNotStarted,
	}); err != nil {
		return TruthPackageView{}, err
	}
	// Python marks the materials stale on reopen: tests/tasks
	// still describe the OLD revision.
	if _, err := e.markPackageMaterialsStale(ctx); err != nil {
		return TruthPackageView{}, err
	}
	if err := e.appendFeatureEvent(ctx.Feature.ID, EventTruthReopened, map[string]any{
		"slug":                  view.Slug,
		"previous_revision_id":  prevRevision,
		"new_revision_id":       nextRevision,
		"reason":                reason,
		"by":                    by,
		"previous_truth_sha256": prevTruthSHA,
		"previous_fingerprint":  prevFingerprint,
		"supersedes_status":     "frozen",
	}); err != nil {
		return TruthPackageView{}, err
	}
	out, err := e.TruthPackage(ctx)
	if err != nil {
		return TruthPackageView{}, err
	}
	out.PreviousRevisionID = prevRevision
	out.NewRevisionID = nextRevision
	out.ReopenReason = reason
	return out, nil
}
