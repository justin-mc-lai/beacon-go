// Per-category import steps for the v1 migration (split from
// migrate.go): the lifecycle mapping table, the v2-data guard and the
// lifecycle/gate/artifact importers. All dedupe on natural keys.
package migration

import (
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// MapLifecycle maps a v1 feature's records onto the frozen lifecycle
// vocabulary (core.WorkflowStage x core.StageStatus). v1 has no
// lifecycle ledger; the mapping is a documented derivation from
// .machine/execution/<slug>.revision.json (lifecycle_status first,
// doc_status as fallback) and the truth.md frontmatter status:
//
//	lifecycle_status accepted    -> Release   / Completed
//	lifecycle_status verified    -> Qa        / Completed
//	lifecycle_status implemented -> Implement / Completed
//	lifecycle_status frozen      -> Freeze    / Completed
//	lifecycle_status blocked     -> Implement / Blocked (doc frozen)
//	                                Freeze    / Blocked (doc not frozen)
//	lifecycle_status idle        -> Think     / NotStarted
//	(no lifecycle_status) doc frozen -> Freeze / Completed
//	(no lifecycle_status) doc draft  -> Freeze / InProgress
//	anything else                -> unmapped (skipped, never forced)
func MapLifecycle(f FeatureScan) (core.WorkflowStage, core.StageStatus, bool, string) {
	var ls, doc string
	if f.Revision != nil {
		ls = strings.ToLower(strings.TrimSpace(f.Revision.LifecycleStatus))
		doc = strings.ToLower(strings.TrimSpace(f.Revision.DocStatus))
	}
	if doc == "" {
		doc = strings.ToLower(strings.TrimSpace(f.PkgStatus))
	}
	switch ls {
	case "accepted":
		return core.StageRelease, core.StatusCompleted, true, ""
	case "verified":
		return core.StageQa, core.StatusCompleted, true, ""
	case "implemented":
		return core.StageImplement, core.StatusCompleted, true, ""
	case "frozen":
		return core.StageFreeze, core.StatusCompleted, true, ""
	case "blocked":
		if doc == "frozen" {
			return core.StageImplement, core.StatusBlocked, true, ""
		}
		return core.StageFreeze, core.StatusBlocked, true, ""
	case "idle":
		return core.StageThink, core.StatusNotStarted, true, ""
	case "":
		// fall through to the doc-status fallback below
	default:
		return "", "", false, fmt.Sprintf("unmapped v1 lifecycle_status %q", ls)
	}
	switch doc {
	case "frozen":
		return core.StageFreeze, core.StatusCompleted, true, ""
	case "draft":
		return core.StageFreeze, core.StatusInProgress, true, ""
	case "":
		return "", "", false, "no v1 lifecycle source (no revision.json record and no package status)"
	default:
		return "", "", false, fmt.Sprintf("unmapped v1 doc status %q", doc)
	}
}

// hasV2Data reports whether a registered project already holds genuine
// v2 runtime data: any run that is not a migration run, or any lifecycle
// snapshot the migration did not derive.
func hasV2Data(st *store.Store, projectID string) (bool, error) {
	kinds, err := st.Projects().CountRunsByKind(projectID)
	if err != nil {
		return false, err
	}
	for kind, n := range kinds {
		if kind != MigrationRunKind && n > 0 {
			return true, nil
		}
	}
	features, err := st.Projects().ListFeatures(projectID)
	if err != nil {
		return false, err
	}
	for _, f := range features {
		lc, err := st.Features().GetLifecycle(f.ID)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		derived := false
		for _, c := range lc.ReasonCodes {
			if c == DerivedReason {
				derived = true
			}
		}
		if !derived {
			return true, nil
		}
	}
	return false, nil
}

// migrationRunID is the deterministic dedupe key for a feature's
// migration run: sha1 over (project, version, slug).
func migrationRunID(projectID, version, slug string) string {
	sum := sha1.Sum([]byte(projectID + "|" + version + "|" + slug))
	return "run_v1mig_" + hex.EncodeToString(sum[:])[:24]
}

func importLifecycle(st *store.Store, rep *Report, pkg store.FeaturePackage, f FeatureScan, item string, opts Options) error {
	if pkg.ID != "" && pkg.ID != "planned" {
		_, err := st.Features().GetLifecycle(pkg.ID)
		if err == nil {
			rep.Counts["feature_lifecycle"].AlreadyPresent++
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	stage, status, ok, reason := MapLifecycle(f)
	if !ok {
		rep.skip("feature_lifecycle", item, reason)
		return nil
	}
	rep.Counts["feature_lifecycle"].Imported++
	if opts.DryRun || pkg.ID == "planned" {
		return nil
	}
	return st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID:   pkg.ID,
		Stage:       stage,
		Status:      status,
		ReasonCodes: []string{DerivedReason},
	})
}

func mapGateVerdict(g *GateInfo) (string, []string) {
	if g.Pass {
		return "pass", nil
	}
	if g.Blocked {
		codes := []string{}
		if g.ReasonCode != "" {
			codes = append(codes, g.ReasonCode)
		} else {
			codes = append(codes, "v1_truth_review_blocked")
		}
		return "blocked", codes
	}
	codes := []string{}
	if g.ReasonCode != "" {
		codes = append(codes, g.ReasonCode)
	}
	return "fail", codes
}

func importGate(st *store.Store, rep *Report, pkg store.FeaturePackage, f FeatureScan, item string, opts Options) error {
	if f.Gate == nil {
		return nil
	}
	verdict, codes := mapGateVerdict(f.Gate)
	if pkg.ID != "" && pkg.ID != "planned" {
		existing, err := st.Gates().ListByFeature(pkg.ID)
		if err != nil {
			return err
		}
		for _, g := range existing {
			if g.GateType == TruthReviewGateType && g.EvaluatedAt == f.Gate.EvaluatedAt && g.Verdict == verdict {
				rep.Counts["gate_results"].AlreadyPresent++
				return nil
			}
		}
	}
	rep.Counts["gate_results"].Imported++
	if opts.DryRun || pkg.ID == "planned" {
		return nil
	}
	_, err := st.Gates().Record(store.GateResult{
		FeatureID:   pkg.ID,
		GateType:    TruthReviewGateType,
		Verdict:     verdict,
		ReasonCodes: codes,
		EvaluatedAt: f.Gate.EvaluatedAt,
	})
	return err
}

func importArtifacts(st *store.Store, rep *Report, scan *ProjectScan, projectID string, vs VersionScan, pkg store.FeaturePackage, f FeatureScan, item string, opts Options) error {
	if len(f.EvidenceRefs) == 0 {
		return nil
	}
	runID := migrationRunID(projectID, vs.Version, f.Slug)
	var existing []store.ArtifactRef
	runKnown := false
	if pkg.ID != "" && pkg.ID != "planned" {
		if _, err := st.Runs().GetByID(runID); err == nil {
			runKnown = true
			arts, err := st.Artifacts().ListByRun(runID)
			if err != nil {
				return err
			}
			existing = arts
		}
	}
	plannedPaths := map[string]bool{}
	for _, ref := range f.EvidenceRefs {
		absPath := filepath.Join(scan.Root, filepath.FromSlash(ref.RelPath))
		info, err := os.Stat(absPath)
		if err != nil {
			rep.skip("artifact_refs", item+":"+ref.RelPath, "evidence file not on disk")
			continue
		}
		if info.IsDir() {
			rep.skip("artifact_refs", item+":"+ref.RelPath, "evidence reference is a directory, not a blob")
			continue
		}
		dup := plannedPaths[absPath]
		for _, a := range existing {
			if a.AbsPath == absPath {
				dup = true
			}
		}
		if dup {
			rep.Counts["artifact_refs"].AlreadyPresent++
			continue
		}
		rep.Counts["artifact_refs"].Imported++
		plannedPaths[absPath] = true
		if opts.DryRun || pkg.ID == "planned" {
			continue
		}
		if !runKnown {
			meta, _ := json.Marshal(map[string]string{
				"source": "v1_migration", "version": vs.Version, "slug": f.Slug,
			})
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if _, err := st.Runs().CreateRun(store.Run{
				ID: runID, FeatureID: pkg.ID, RunKind: MigrationRunKind,
				Status: core.RunSucceeded, StartedAt: now,
				EndedAt:  sql.NullString{String: now, Valid: true},
				Metadata: meta,
			}); err != nil {
				return err
			}
			runKnown = true
		}
		hash, size, err := hashFile(absPath)
		if err != nil {
			return err
		}
		kind := strings.ToLower(strings.TrimSpace(ref.Surface))
		if kind == "" {
			kind = "evidence"
		}
		if _, err := st.Artifacts().Add(store.ArtifactRef{
			RunID: runID, Kind: kind, AbsPath: absPath,
			ContentHash: hash, SizeBytes: size,
		}); err != nil {
			return err
		}
	}
	return nil
}

// hashFile returns the hex sha256 of a file (the content_hash format
// used across the store, cf. spine staleness) and its size.
func hashFile(path string) (string, int64, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer fh.Close()
	h := sha256.New()
	size, err := io.Copy(h, fh)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}
