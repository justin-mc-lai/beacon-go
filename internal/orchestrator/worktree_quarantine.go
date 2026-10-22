// Standard flow S1 (AC-140): stale feature-local refreeze
// quarantine.
//
// The failure this guards: a satellite worktree locally flips a
// package's frontmatter to frozen (a "feature-local refreeze")
// while the canonical package is not frozen, or freezes a body
// that diverges from the canonical baseline. If that worktree's
// output were then merged and consumed as requirement truth, an
// ungoverned local freeze would silently become the baseline.
//
// The Go kernel's isolation is MARKER-BASED, deliberately: the
// frozen text says "worktree 中的 stale refreeze 必须被隔离
// (quarantine/stash 或等价隔离)" — the "or equivalent isolation"
// clause. The kernel never runs git stash or moves files inside
// a user's worktree (auto-stash can strand the author's only
// copy of in-flight edits and races with their editor); instead
// the stale state is recorded with its evidence (file hash,
// frontmatter status, reason) on the run's worktree book and the
// feature trail, surfaced on worktree status, and the merge of
// that worktree is REFUSED with stale_feature_local_refreeze —
// which is the isolation's enforcement point: quarantined truth
// can never become the canonical baseline through the kernel.
package orchestrator

import (
	"os"
	"path/filepath"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// EventWorktreeTruthQuarantined is appended when a scan records a
// new quarantine against a run's worktree.
const EventWorktreeTruthQuarantined = "worktree_truth_quarantined"

// Quarantine reasons (the two stale shapes of AC-140).
const (
	QuarantineLocalFrozenCanonicalUnfrozen = "local_frozen_canonical_unfrozen"
	QuarantineFingerprintMismatch          = "fingerprint_mismatch"
)

// scanWorktreeQuarantines inspects every active satellite record
// of the run (skipping records bound at the project root itself —
// that IS the canonical location) for stale feature-local
// freezes: a package copy whose frontmatter declares frozen
// while (a) the canonical package is not frozen, or (b) its body
// fingerprint differs from the canonical frozen baseline.
// Read-only over disk; recording is recordQuarantines' job.
func (e *Engine) scanWorktreeQuarantines(run store.Run, m *runMeta, root string) []TruthQuarantine {
	book := worktreeBook(m)
	if len(book.Records) == 0 {
		return nil
	}
	fc, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil || !ok {
		return nil
	}
	features, err := e.store.Projects().ListFeatures(fc.Project.ID)
	if err != nil {
		return nil
	}
	type canonicalPkg struct {
		status string
		fp     string
	}
	canonicals := map[string]canonicalPkg{}
	for _, feat := range features {
		if feat.TruthPath == "" {
			continue
		}
		truthRaw, _ := os.ReadFile(resolvePkgPath(root, feat.TruthPath))
		status, _ := frontmatterStatusOf(gate.PackageFile{Present: len(truthRaw) > 0, Text: string(truthRaw)})
		var fp string
		if len(truthRaw) > 0 {
			testsRaw, _ := os.ReadFile(resolvePkgPath(root, feat.TestsPath))
			fp = core.PackageFingerprint(truthRaw, testsRaw)
		}
		canonicals[feat.ID] = canonicalPkg{status: status, fp: fp}
	}
	var found []TruthQuarantine
	for _, rec := range book.Records {
		if rec.State != WorktreeActive || rec.Path == "" {
			continue
		}
		if filepath.Clean(rec.Path) == filepath.Clean(root) {
			continue // the canonical location itself
		}
		for _, feat := range features {
			if feat.TruthPath == "" {
				continue
			}
			wtTruth, err := os.ReadFile(resolvePkgPath(rec.Path, feat.TruthPath))
			if err != nil || len(wtTruth) == 0 {
				continue
			}
			wtStatus, _ := frontmatterStatusOf(gate.PackageFile{Present: true, Text: string(wtTruth)})
			if wtStatus != "frozen" {
				continue // drafts and local edits are normal satellite work
			}
			canon := canonicals[feat.ID]
			q := TruthQuarantine{
				FeatureID: feat.ID, FeatureSlug: feat.Slug,
				WorktreePath: rec.Path, TruthRelPath: feat.TruthPath,
				FrontmatterStatus: wtStatus, TruthSHA256: sha256HexOf(wtTruth),
				RecordedAt: nowISO(),
			}
			switch {
			case canon.status != "frozen":
				q.Reason = QuarantineLocalFrozenCanonicalUnfrozen
			default:
				wtTests, _ := os.ReadFile(resolvePkgPath(rec.Path, feat.TestsPath))
				if core.PackageFingerprint(wtTruth, wtTests) != canon.fp {
					q.Reason = QuarantineFingerprintMismatch
				} else {
					continue // identical frozen copy: a faithful mirror, not stale
				}
			}
			found = append(found, q)
		}
	}
	return found
}

// recordQuarantines persists fresh scan findings (deduped by
// feature + worktree + file hash) onto the run's worktree book
// and appends one trail event per new record. Returns whether
// anything new was recorded (the caller then saves the run).
func (e *Engine) recordQuarantines(run store.Run, m *runMeta, found []TruthQuarantine) (bool, error) {
	book := worktreeBook(m)
	changed := false
	for _, q := range found {
		dup := false
		for _, old := range book.Quarantines {
			if old.FeatureID == q.FeatureID && old.WorktreePath == q.WorktreePath && old.TruthSHA256 == q.TruthSHA256 {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		book.Quarantines = append(book.Quarantines, q)
		changed = true
		if err := e.appendEvent(run, EventWorktreeTruthQuarantined, map[string]any{
			"feature_id": q.FeatureID, "feature_slug": q.FeatureSlug,
			"worktree_path": q.WorktreePath, "truth_rel_path": q.TruthRelPath,
			"frontmatter_status": q.FrontmatterStatus, "truth_sha256": q.TruthSHA256,
			"quarantine_reason": q.Reason, "reason_code": core.ReasonStaleFeatureLocalRefreeze,
		}); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// scanAndRecordQuarantines is the status/create hook: scan the
// run's worktrees, persist any new finding, and return all
// currently-recorded quarantines for the view.
func (e *Engine) scanAndRecordQuarantines(run store.Run, m *runMeta, root string) ([]TruthQuarantine, error) {
	found := e.scanWorktreeQuarantines(run, m, root)
	changed, err := e.recordQuarantines(run, m, found)
	if err != nil {
		return nil, err
	}
	if changed {
		if err := e.saveRun(run, m); err != nil {
			return nil, err
		}
	}
	return worktreeBook(m).Quarantines, nil
}
