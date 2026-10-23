// Operator Spine full snapshot (AC-017..021, AC-081/082): the frozen
// SpineSnapshot read model, its DAO builder and the feature/project
// resolution joins. The snapshot is a DB-first read model: requirement
// truth stays in Markdown; every field comes from the nine runtime
// tables, from run metadata_json, or from a documented derivation over
// them (spine_derive.go) — never from blob files as authority (AC-015).
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"beacon-go/internal/core"
)

// SpineSnapshot is the frozen Operator Spine read model (truth AC-017
// code block + AC-081). Field sources are documented in
// docs/p1-implementation.md; Go field names Stage/Status/ReasonCodes/
// Slug/LatestGates keep the P0 DAO surface working unchanged.
type SpineSnapshot struct {
	RunID           string             `json:"run_id"`
	ProjectKey      string             `json:"project_key"`
	ProjectRoot     string             `json:"project_root"`
	Version         string             `json:"version"`
	FeatureID       string             `json:"feature_id"`
	FeatureSlug     string             `json:"feature_slug"`
	Slug            string             `json:"slug"`
	Stage           core.WorkflowStage `json:"lifecycle_stage"`
	Status          core.StageStatus   `json:"lifecycle_status"`
	Phase           string             `json:"phase"`
	PhasesCompleted []string           `json:"phases_completed"`
	PhasePromoted   bool               `json:"phase_promoted"`
	NextHarness     string             `json:"next_harness"`
	NextAction      string             `json:"next_action"`
	ProgressStatus  string             `json:"progress_status"`
	ActiveRunID     string             `json:"active_run_id"`
	RunStatus       core.RunStatus     `json:"run_status"`
	Iteration       int                `json:"iteration"`
	ReasonCodes     []string           `json:"blocking_reason_codes"`
	UpdatedAt       string             `json:"updated_at"`
	RunCount        int                `json:"run_count"`
	LatestRun       *Run               `json:"latest_run,omitempty"`
	LatestGates     []GateResult       `json:"gate_results"`
	ArtifactRefs    []ArtifactView     `json:"artifact_refs"`
	ArtifactCount   int                `json:"artifact_count"`
}

// SpineDao implements the Operator Spine read aggregation (AC-081).
type SpineDao struct{ s *Store }

// Spine returns the SpineDao.
func (s *Store) Spine() *SpineDao { return &SpineDao{s: s} }

// GetVersionByID looks a project_versions row up by id.
func (d *ProjectDao) GetVersionByID(id string) (ProjectVersion, error) {
	var v ProjectVersion
	var cur int
	err := d.q.QueryRow(`SELECT id, project_id, version, docs_root, is_current FROM project_versions WHERE id = ?`, id).
		Scan(&v.ID, &v.ProjectID, &v.Version, &v.DocsRoot, &cur)
	v.IsCurrent = cur == 1
	return v, err
}

// LatestByType returns the newest gate result per gate_type for a
// feature, ordered by gate_type (spine gate read view, AC-020).
func (d *GateDao) LatestByType(featureID string) ([]GateResult, error) {
	all, err := d.ListByFeature(featureID) // newest first
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []GateResult{}
	for _, g := range all {
		if !seen[g.GateType] {
			seen[g.GateType] = true
			out = append(out, g)
		}
	}
	// Stable, operator-friendly order: by gate_type.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].GateType < out[i].GateType {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

// Snapshot builds the full SpineSnapshot for one feature (AC-081:
// lifecycle + runs + gates + artifacts, joined with project/version).
func (d *SpineDao) Snapshot(featureID string) (SpineSnapshot, error) {
	feat, err := d.s.Features().GetByID(featureID)
	if err != nil {
		return SpineSnapshot{}, err
	}
	return d.buildSnapshot(feat)
}

func (d *SpineDao) buildSnapshot(feat FeaturePackage) (SpineSnapshot, error) {
	snap := SpineSnapshot{
		FeatureID: feat.ID, FeatureSlug: feat.Slug, Slug: feat.Slug,
		PhasesCompleted: []string{}, ReasonCodes: []string{},
		LatestGates: []GateResult{}, ArtifactRefs: []ArtifactView{},
	}
	if ver, err := d.s.Projects().GetVersionByID(feat.ProjectVersionID); err == nil {
		snap.Version = ver.Version
		if proj, err := d.s.Projects().GetByID(ver.ProjectID); err == nil {
			snap.ProjectRoot = proj.RootPath
			snap.ProjectKey = ProjectKey(proj.RootPath)
		}
	}
	if lc, err := d.s.Features().GetLifecycle(feat.ID); err == nil {
		snap.Stage, snap.Status = lc.Stage, lc.Status
		snap.ReasonCodes = orEmptyCodes(lc.ReasonCodes)
		snap.UpdatedAt = lc.UpdatedAt
	} else if !errors.Is(err, sql.ErrNoRows) {
		return SpineSnapshot{}, err
	}
	runs, err := d.s.Runs().ListByFeature(feat.ID) // oldest first
	if err != nil {
		return SpineSnapshot{}, err
	}
	snap.RunCount = len(runs)
	var meta map[string]any
	if len(runs) > 0 {
		latest := runs[len(runs)-1]
		snap.LatestRun = &latest
		snap.RunID = latest.ID
		snap.RunStatus = latest.Status
		snap.Iteration = latest.Iteration
		if snap.UpdatedAt == "" {
			snap.UpdatedAt = latest.StartedAt
		}
		if len(latest.Metadata) > 0 {
			_ = json.Unmarshal(latest.Metadata, &meta)
		}
		for i := len(runs) - 1; i >= 0; i-- { // newest non-terminal run
			if !isTerminalRun(runs[i].Status) {
				snap.ActiveRunID = runs[i].ID
				break
			}
		}
		for _, r := range runs {
			arts, err := d.s.Artifacts().ListByRun(r.ID)
			if err != nil {
				return SpineSnapshot{}, err
			}
			for _, a := range arts {
				snap.ArtifactRefs = append(snap.ArtifactRefs, ViewOf(a))
			}
		}
	}
	snap.ArtifactCount = len(snap.ArtifactRefs)
	gates, err := d.s.Gates().LatestByType(feat.ID)
	if err != nil {
		return SpineSnapshot{}, err
	}
	snap.LatestGates = gates
	deriveSpineFields(&snap, meta)
	return snap, nil
}

// SpineListFilter filters SpineDao.List (AC-082). Status matches either
// the lifecycle status or the run status; the two specific fields match
// only their own axis.
type SpineListFilter struct {
	ProjectID       string
	ProjectRoot     string
	LifecycleStatus string
	RunStatus       string
	Status          string
}

// List returns snapshots for all features (optionally one project),
// oldest project first, slug order inside a project (AC-018/082).
func (d *SpineDao) List(f SpineListFilter) ([]SpineSnapshot, error) {
	query := `SELECT f.id FROM feature_packages f
		JOIN project_versions v ON f.project_version_id = v.id
		JOIN projects p ON v.project_id = p.id`
	var args []any
	var where []string
	if f.ProjectID != "" {
		where = append(where, "p.id = ?")
		args = append(args, f.ProjectID)
	}
	if f.ProjectRoot != "" {
		where = append(where, "p.root_path = ?")
		args = append(args, f.ProjectRoot)
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY p.registered_at, p.id, f.slug"
	rows, err := d.s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []SpineSnapshot{}
	for _, id := range ids {
		snap, err := d.Snapshot(id)
		if err != nil {
			return nil, err
		}
		if f.LifecycleStatus != "" && string(snap.Status) != f.LifecycleStatus {
			continue
		}
		if f.RunStatus != "" && string(snap.RunStatus) != f.RunStatus {
			continue
		}
		if f.Status != "" && string(snap.Status) != f.Status && string(snap.RunStatus) != f.Status {
			continue
		}
		out = append(out, snap)
	}
	return out, nil
}

// FeatureContext is a feature joined with its version and project.
type FeatureContext struct {
	Project Project
	Version ProjectVersion
	Feature FeaturePackage
}

// ResolveFeature finds one feature by optional root path, slug and
// version. Current versions win; ties break by registration then slug.
// sql.ErrNoRows is returned when nothing matches.
func (s *Store) ResolveFeature(projectRoot, slug, version string) (FeatureContext, error) {
	list, err := s.ListFeatureContexts(projectRoot)
	if err != nil {
		return FeatureContext{}, err
	}
	for _, ctx := range list {
		if slug != "" && ctx.Feature.Slug != slug {
			continue
		}
		if version != "" && ctx.Version.Version != version {
			continue
		}
		return ctx, nil
	}
	return FeatureContext{}, sql.ErrNoRows
}

// ListFeatureContexts returns all features (optionally of one project
// root), current versions first, then registration order, then slug.
func (s *Store) ListFeatureContexts(projectRoot string) ([]FeatureContext, error) {
	query := `SELECT p.id, p.root_path, p.name, p.registered_at,
		v.id, v.project_id, v.version, v.docs_root, v.is_current,
		f.id, f.project_version_id, f.slug, f.truth_path, f.tests_path, f.status
		FROM feature_packages f
		JOIN project_versions v ON f.project_version_id = v.id
		JOIN projects p ON v.project_id = p.id`
	var args []any
	if projectRoot != "" {
		query += " WHERE p.root_path = ?"
		args = append(args, ResolveRootPath(projectRoot))
	}
	query += " ORDER BY v.is_current DESC, p.registered_at, p.id, f.slug"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FeatureContext{}
	for rows.Next() {
		var ctx FeatureContext
		var cur int
		if err := rows.Scan(&ctx.Project.ID, &ctx.Project.RootPath, &ctx.Project.Name, &ctx.Project.RegisteredAt,
			&ctx.Version.ID, &ctx.Version.ProjectID, &ctx.Version.Version, &ctx.Version.DocsRoot, &cur,
			&ctx.Feature.ID, &ctx.Feature.ProjectVersionID, &ctx.Feature.Slug, &ctx.Feature.TruthPath, &ctx.Feature.TestsPath, &ctx.Feature.Status); err != nil {
			return nil, err
		}
		ctx.Version.IsCurrent = cur == 1
		out = append(out, ctx)
	}
	return out, rows.Err()
}
