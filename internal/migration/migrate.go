// The v1 migration importer (AC-077..079 + AC-016): one-way, read-only
// on the v1 project, idempotent on natural keys, and honest about what
// v1 never recorded. See scan.go for the probed v1 on-disk format.
package migration

import (
	"database/sql"
	"errors"
	"fmt"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// Stable contract strings for the migration surface.
const (
	// MigrationRunKind tags the synthetic runs that imported artifact
	// pointers hang off (artifact_refs.run_id is NOT NULL; AC-014).
	MigrationRunKind = "v1_migration"
	// TruthReviewGateType is the v1 per-feature gate's name, kept
	// source-faithful; it is not one of the five P3a evaluation gates.
	TruthReviewGateType = "truth_review"
	// DerivedReason marks every lifecycle snapshot the migration derived
	// from v1 records (v1 has no lifecycle ledger; see MapLifecycle).
	DerivedReason = "migration_v1_derived"

	ReasonHasV2Data       = "v1_migration_project_has_v2_data"
	ReasonProjectNotFound = "v1_project_root_not_found"
)

// Category names, in report order.
var Categories = []string{
	"projects", "project_versions", "feature_packages",
	"feature_lifecycle", "gate_results", "artifact_refs",
}

// CategoryCount tallies one import category.
type CategoryCount struct {
	Imported       int `json:"imported"`
	AlreadyPresent int `json:"already_present"`
	Skipped        int `json:"skipped"`
}

// SkipEntry is one item that was not imported, with the reason.
type SkipEntry struct {
	Category string `json:"category"`
	Item     string `json:"item"`
	Reason   string `json:"reason"`
}

// Report is the migration outcome (CLI text + --json share this shape).
type Report struct {
	ProjectRoot string                    `json:"project_root"`
	ProjectID   string                    `json:"project_id,omitempty"`
	DryRun      bool                      `json:"dry_run"`
	Forced      bool                      `json:"forced,omitempty"`
	Counts      map[string]*CategoryCount `json:"counts"`
	Skipped     []SkipEntry               `json:"skipped"`
	Uncertain   []string                  `json:"uncertain,omitempty"`
	Ledgers     []LedgerSource            `json:"ledger_sources"`
	GateReports []ProjectGateReport       `json:"project_gate_reports,omitempty"`
}

// Options controls one ImportV1 run.
type Options struct {
	DryRun bool
	Force  bool
}

func newReport(root string, opts Options) *Report {
	rep := &Report{
		ProjectRoot: root,
		DryRun:      opts.DryRun,
		Forced:      opts.Force,
		Counts:      map[string]*CategoryCount{},
		Skipped:     []SkipEntry{},
	}
	for _, c := range Categories {
		rep.Counts[c] = &CategoryCount{}
	}
	return rep
}

func (r *Report) skip(category, item, reason string) {
	r.Counts[category].Skipped++
	r.Skipped = append(r.Skipped, SkipEntry{Category: category, Item: item, Reason: reason})
}

// ImportV1 imports one v1 project into the global store. The v1 project
// directory is only ever read. Re-running it is idempotent: every
// category dedupes on its natural key (project root_path; version
// string; (version, slug); lifecycle snapshot presence; gate
// (type, evaluated_at, verdict); artifact (migration run, abs_path)).
func ImportV1(st *store.Store, root string, opts Options) (*Report, error) {
	scan, err := ScanProject(root)
	if err != nil {
		return nil, core.NewError(ReasonProjectNotFound, "migrate.ScanProject", err.Error())
	}
	resolved := store.ResolveRootPath(root)
	rep := newReport(resolved, opts)
	rep.Ledgers = scan.Ledgers
	rep.GateReports = scan.GateReports

	for _, l := range scan.Ledgers {
		if l.Present {
			rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
				"ledger %s present at %s: no such database existed in the probed v1 sample, its schema is unverified, so it was NOT imported (manual confirmation needed)",
				l.Kind, l.Path))
		}
	}
	for _, loser := range scan.LosingRoots {
		rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
			"version %s exists at both docs roots; imported from the root with more feature packages, %s ignored",
			loser.Version, loser.DocsRoot))
	}

	// Project row (P1 register semantics: root_path idempotent).
	proj, err := st.Projects().GetByRootPath(resolved)
	exists := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return rep, err
	}
	if exists {
		rep.ProjectID = proj.ID
		rep.Counts["projects"].AlreadyPresent = 1
		v2, err := hasV2Data(st, proj.ID)
		if err != nil {
			return rep, err
		}
		if v2 && !opts.Force {
			rep.Uncertain = append(rep.Uncertain,
				"project already holds v2 runtime data; migration refuses to touch it without --force (with --force only missing items are filled, nothing is overwritten or deleted)")
			return rep, core.NewError(ReasonHasV2Data, "migrate.ImportV1",
				"project "+resolved+" already holds v2 data; re-run with --force to fill missing items only")
		}
	} else {
		rep.Counts["projects"].Imported = 1
		if !opts.DryRun {
			proj, err = st.Projects().Register(store.Project{RootPath: resolved, Name: scan.Name})
			if err != nil {
				return rep, err
			}
			rep.ProjectID = proj.ID
		}
	}

	// Determine the current version: version-truth declaration wins;
	// otherwise the highest scanned version (derived, reported).
	current := scan.CurrentVersion
	if current != "" && !hasVersion(scan, current) {
		rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
			"%s declares current version %s but no docs root for it was found; current version left unset",
			scan.CurrentSource, current))
		current = ""
	}
	if current == "" && len(scan.Versions) > 0 {
		current = scan.Versions[len(scan.Versions)-1].Version
		rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
			"no current version declared by version-truth.json; highest scanned version %s marked current (derived)", current))
	}

	// Version rows.
	versionIDs := map[string]string{} // version -> row id ("planned" in dry-run for new rows)
	if exists {
		versions, err := st.Projects().ListVersions(proj.ID)
		if err != nil {
			return rep, err
		}
		for _, v := range versions {
			versionIDs[v.Version] = v.ID
		}
	}
	for _, vs := range scan.Versions {
		if _, ok := versionIDs[vs.Version]; ok {
			rep.Counts["project_versions"].AlreadyPresent++
			continue
		}
		rep.Counts["project_versions"].Imported++
		if opts.DryRun {
			versionIDs[vs.Version] = "planned"
			continue
		}
		row, err := st.Projects().AddVersion(store.ProjectVersion{
			ProjectID: proj.ID, Version: vs.Version, DocsRoot: vs.DocsRoot, IsCurrent: false,
		})
		if err != nil {
			return rep, err
		}
		versionIDs[vs.Version] = row.ID
	}
	if !opts.DryRun && rep.ProjectID != "" && current != "" {
		// Mark the current version if none is current yet.
		versions, err := st.Projects().ListVersions(rep.ProjectID)
		if err != nil {
			return rep, err
		}
		anyCurrent := false
		for _, v := range versions {
			if v.IsCurrent {
				anyCurrent = true
			}
		}
		if !anyCurrent {
			for _, v := range versions {
				if v.Version == current {
					if err := st.Projects().SetCurrent(v.ID); err != nil {
						return rep, err
					}
				}
			}
		}
	}

	// Presence model for (version, slug) -> package row.
	type pkgKey struct{ version, slug string }
	pkgs := map[pkgKey]store.FeaturePackage{}
	if rep.ProjectID != "" {
		ctxs, err := st.ListFeatureContexts(resolved)
		if err != nil {
			return rep, err
		}
		for _, ctx := range ctxs {
			pkgs[pkgKey{ctx.Version.Version, ctx.Feature.Slug}] = ctx.Feature
		}
	}

	for _, vs := range scan.Versions {
		for _, f := range vs.Features {
			key := pkgKey{vs.Version, f.Slug}
			item := vs.Version + "/" + f.Slug
			pkg, present := pkgs[key]
			if f.SlugMismatch {
				rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
					"feature %s: frontmatter slug %q differs from directory name %q; frontmatter slug used", item, f.Slug, f.DirName))
			}
			if !f.HasTests {
				rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
					"feature %s: tests.md missing on disk; tests_path recorded by convention", item))
			}
			if !present {
				status := f.PkgStatus
				if status == "" {
					status = "unknown"
					rep.Uncertain = append(rep.Uncertain, fmt.Sprintf(
						"feature %s: no status in truth frontmatter or revision record; package status recorded as \"unknown\"", item))
				}
				rep.Counts["feature_packages"].Imported++
				if opts.DryRun {
					pkg = store.FeaturePackage{ID: "planned", Slug: f.Slug, Status: status}
				} else {
					pkg, err = st.Features().CreatePackage(store.FeaturePackage{
						ProjectVersionID: versionIDs[vs.Version],
						Slug:             f.Slug,
						TruthPath:        f.TruthPath,
						TestsPath:        f.TestsPath,
						Status:           status,
					})
					if err != nil {
						return rep, err
					}
				}
				pkgs[key] = pkg
			} else {
				rep.Counts["feature_packages"].AlreadyPresent++
			}

			if err := importLifecycle(st, rep, pkg, f, item, opts); err != nil {
				return rep, err
			}
			if err := importGate(st, rep, pkg, f, item, opts); err != nil {
				return rep, err
			}
			if err := importArtifacts(st, rep, scan, proj.ID, vs, pkg, f, item, opts); err != nil {
				return rep, err
			}
		}
	}

	// Project-level gate reports: version-scoped, no feature — counted
	// as skipped, never forced into a feature's gate_results.
	for _, gr := range scan.GateReports {
		rep.skip("gate_results", gr.File, fmt.Sprintf(
			"version-scoped v1 gate report (stage=%s status=%s version=%s) has no feature attribution; not imported into feature gate_results",
			gr.Stage, gr.Status, gr.Version))
	}
	return rep, nil
}

func hasVersion(scan *ProjectScan, version string) bool {
	for _, v := range scan.Versions {
		if v.Version == version {
			return true
		}
	}
	return false
}
