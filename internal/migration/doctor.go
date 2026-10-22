// Doctor projection rebuild (AC-011): reconcile the global store's
// project index against the on-disk facts of a registered project —
// the feature-package MD under docs/beacon/<version>/features/ plus
// the legacy v1 JSON records (.machine/execution/*.revision.json).
//
// Rules (AC-011 + task contract):
//   - Default is report-only: three drift kinds, nothing is written.
//   - --apply only fills or corrects the package index: missing
//     packages are registered, package status mismatches are corrected,
//     missing lifecycle snapshots are filled with the same documented
//     derivation the migration uses. Every correction appends a
//     lifecycle_events row (append-only trail, AC-012).
//   - Existing lifecycle snapshots and gate results are NEVER
//     overwritten: after a rebuild, spine show must agree with the
//     pre-rebuild snapshot on lifecycle/gate fields (AC-011).
//   - Store rows with no on-disk counterpart are reported
//     (missing_on_disk) and never deleted.
package migration

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// DoctorDerivedReason marks lifecycle snapshots filled by doctor
// rebuild (as opposed to migration-derived or genuine v2 runtime data).
const DoctorDerivedReason = "doctor_rebuild_derived"

// Drift kinds (stable contract strings).
const (
	DriftMissingInStore = "missing_in_store"
	DriftMissingOnDisk  = "missing_on_disk"
	DriftStatusMismatch = "status_mismatch"
	// DriftTruthBodyDrift (P4c): a frozen package's on-disk body
	// no longer matches the fingerprint its freeze/refreeze event
	// recorded — the body moved outside the change/refreeze flow
	// (the Go landing of the v1.6.3 spec-fingerprint repair).
	// Report-only under every mode: rebuild never rewrites truth.
	DriftTruthBodyDrift = "truth_body_drift"
)

// latestFreezeFingerprint returns the package fingerprint
// recorded by the feature's most recent freeze/refreeze event, or
// "" when no baseline exists (packages frozen before fingerprints
// were recorded, or never frozen through the Go truth face).
func latestFreezeFingerprint(st *store.Store, featureID string) string {
	events, err := st.ListEvents(featureID)
	if err != nil {
		return ""
	}
	baseline := ""
	for _, ev := range events {
		if ev.EventType != "truth_frozen" && ev.EventType != "truth_refrozen" {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			continue
		}
		if fp, ok := p["package_fingerprint"].(string); ok && fp != "" {
			baseline = fp
		}
	}
	return baseline
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// DriftEntry is one file-vs-store disagreement.
type DriftEntry struct {
	Kind    string `json:"kind"`
	Version string `json:"version"`
	Slug    string `json:"slug"`
	Detail  string `json:"detail"`
}

// RebuildReport is the doctor rebuild outcome.
type RebuildReport struct {
	ProjectRoot   string       `json:"project_root"`
	ProjectID     string       `json:"project_id"`
	Applied       bool         `json:"applied"`
	Drift         []DriftEntry `json:"drift"`
	Corrected     []string     `json:"corrected,omitempty"`
	EventsWritten int          `json:"events_written"`
}

// Rebuild compares one registered project's disk facts with the store
// and, when apply is set, fills/corrects the package index.
func Rebuild(st *store.Store, projectRoot string, apply bool) (*RebuildReport, error) {
	resolved := store.ResolveRootPath(projectRoot)
	proj, err := st.Projects().GetByRootPath(resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, core.NewError("project_not_registered", "doctor.Rebuild",
			"no project registered at "+resolved)
	}
	if err != nil {
		return nil, err
	}
	scan, err := ScanProject(resolved)
	if err != nil {
		return nil, err
	}
	rep := &RebuildReport{ProjectRoot: resolved, ProjectID: proj.ID, Applied: apply, Drift: []DriftEntry{}}

	ctxs, err := st.ListFeatureContexts(resolved)
	if err != nil {
		return nil, err
	}
	type key struct{ version, slug string }
	storePkgs := map[key]store.FeatureContext{}
	for _, ctx := range ctxs {
		storePkgs[key{ctx.Version.Version, ctx.Feature.Slug}] = ctx
	}
	versions, err := st.Projects().ListVersions(proj.ID)
	if err != nil {
		return nil, err
	}
	versionIDs := map[string]string{}
	for _, v := range versions {
		versionIDs[v.Version] = v.ID
	}

	ensureVersion := func(vs VersionScan) (string, error) {
		if id, ok := versionIDs[vs.Version]; ok {
			return id, nil
		}
		row, err := st.Projects().AddVersion(store.ProjectVersion{
			ProjectID: proj.ID, Version: vs.Version, DocsRoot: vs.DocsRoot, IsCurrent: false,
		})
		if err != nil {
			return "", err
		}
		versionIDs[vs.Version] = row.ID
		return row.ID, nil
	}
	appendEvent := func(featureID, eventType string, payload map[string]string) error {
		raw, _ := json.Marshal(payload)
		if _, err := st.AppendEvent(store.LifecycleEvent{
			FeatureID: featureID, EventType: eventType, Payload: raw,
		}); err != nil {
			return err
		}
		rep.EventsWritten++
		return nil
	}

	diskKeys := map[key]bool{}
	for _, vs := range scan.Versions {
		for _, f := range vs.Features {
			k := key{vs.Version, f.Slug}
			diskKeys[k] = true
			item := vs.Version + "/" + f.Slug
			ctx, inStore := storePkgs[k]
			if !inStore {
				rep.Drift = append(rep.Drift, DriftEntry{
					Kind: DriftMissingInStore, Version: vs.Version, Slug: f.Slug,
					Detail: "feature package on disk but not in store",
				})
				if apply {
					vid, err := ensureVersion(vs)
					if err != nil {
						return rep, err
					}
					status := f.PkgStatus
					if status == "" {
						status = "unknown"
					}
					pkg, err := st.Features().CreatePackage(store.FeaturePackage{
						ProjectVersionID: vid, Slug: f.Slug,
						TruthPath: f.TruthPath, TestsPath: f.TestsPath, Status: status,
					})
					if err != nil {
						return rep, err
					}
					rep.Corrected = append(rep.Corrected, "registered missing package "+item)
					if err := appendEvent(pkg.ID, "doctor_rebuild_registered", map[string]string{
						"source": "doctor_rebuild", "version": vs.Version, "slug": f.Slug,
					}); err != nil {
						return rep, err
					}
					if stage, status, ok, _ := MapLifecycle(f); ok {
						if err := st.Features().UpdateLifecycle(store.Lifecycle{
							FeatureID: pkg.ID, Stage: stage, Status: status,
							ReasonCodes: []string{DoctorDerivedReason},
						}); err != nil {
							return rep, err
						}
						rep.Corrected = append(rep.Corrected, "filled derived lifecycle for "+item)
					}
				}
				continue
			}

			// Package status mismatch (file fact wins for the index).
			if f.PkgStatus != "" && ctx.Feature.Status != f.PkgStatus {
				rep.Drift = append(rep.Drift, DriftEntry{
					Kind: DriftStatusMismatch, Version: vs.Version, Slug: f.Slug,
					Detail: fmt.Sprintf("package status: store=%q disk=%q", ctx.Feature.Status, f.PkgStatus),
				})
				if apply {
					if err := st.Features().UpdatePackageStatus(ctx.Feature.ID, f.PkgStatus); err != nil {
						return rep, err
					}
					rep.Corrected = append(rep.Corrected, fmt.Sprintf(
						"corrected package status of %s: %q -> %q", item, ctx.Feature.Status, f.PkgStatus))
					if err := appendEvent(ctx.Feature.ID, "doctor_rebuild_corrected", map[string]string{
						"source": "doctor_rebuild", "field": "feature_packages.status",
						"from": ctx.Feature.Status, "to": f.PkgStatus,
					}); err != nil {
						return rep, err
					}
				}
			}

			// Lifecycle: missing snapshots may be filled; existing
			// snapshots are runtime state and are never overwritten
			// (AC-011 rebuild consistency), only reported.
			stage, status, mappable, _ := MapLifecycle(f)
			lc, lerr := st.Features().GetLifecycle(ctx.Feature.ID)
			switch {
			case errors.Is(lerr, sql.ErrNoRows):
				detail := "lifecycle snapshot missing in store"
				if !mappable {
					detail += " and no lifecycle is derivable from the v1 records"
				}
				rep.Drift = append(rep.Drift, DriftEntry{
					Kind: DriftMissingInStore, Version: vs.Version, Slug: f.Slug, Detail: detail,
				})
				if apply && mappable {
					if err := st.Features().UpdateLifecycle(store.Lifecycle{
						FeatureID: ctx.Feature.ID, Stage: stage, Status: status,
						ReasonCodes: []string{DoctorDerivedReason},
					}); err != nil {
						return rep, err
					}
					rep.Corrected = append(rep.Corrected, "filled derived lifecycle for "+item)
					if err := appendEvent(ctx.Feature.ID, "doctor_rebuild_lifecycle_filled", map[string]string{
						"source": "doctor_rebuild", "version": vs.Version, "slug": f.Slug,
					}); err != nil {
						return rep, err
					}
				}
			case lerr != nil:
				return rep, lerr
			case mappable && (lc.Stage != stage || lc.Status != status):
				rep.Drift = append(rep.Drift, DriftEntry{
					Kind: DriftStatusMismatch, Version: vs.Version, Slug: f.Slug,
					Detail: fmt.Sprintf(
						"lifecycle: store=%s/%s, derived-from-disk=%s/%s — reported only; rebuild never overwrites lifecycle snapshots (AC-011)",
						lc.Stage, lc.Status, stage, status),
				})
			}

			// Frozen-body fingerprint (P4c): when the feature's
			// freeze/refreeze trail carries a body fingerprint,
			// recompute it from the on-disk package. A mismatch
			// means the frozen body changed outside the
			// change/refreeze flow. Report-only in every mode —
			// rebuild never rewrites truth documents.
			if baseline := latestFreezeFingerprint(st, ctx.Feature.ID); baseline != "" {
				truthRaw, terr := os.ReadFile(filepath.Join(resolved, filepath.FromSlash(ctx.Feature.TruthPath)))
				if terr == nil {
					testsRaw, _ := os.ReadFile(filepath.Join(resolved, filepath.FromSlash(ctx.Feature.TestsPath)))
					if got := core.PackageFingerprint(truthRaw, testsRaw); got != baseline {
						rep.Drift = append(rep.Drift, DriftEntry{
							Kind: DriftTruthBodyDrift, Version: vs.Version, Slug: f.Slug,
							Detail: fmt.Sprintf(
								"frozen package body fingerprint mismatch: freeze recorded %s, disk computes %s — truth/tests changed after freeze without change/refreeze; reported only, rebuild never rewrites truth",
								shortHash(baseline), shortHash(got)),
						})
					}
				}
			}
		}
	}
	for k, ctx := range storePkgs {
		if !diskKeys[k] {
			rep.Drift = append(rep.Drift, DriftEntry{
				Kind: DriftMissingOnDisk, Version: ctx.Version.Version, Slug: ctx.Feature.Slug,
				Detail: "feature package in store but not on disk; reported only, never auto-deleted",
			})
		}
	}
	return rep, nil
}
