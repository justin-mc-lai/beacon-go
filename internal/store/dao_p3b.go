// P3b additions for the v1 migration importer and the doctor projection
// rebuild (AC-077..079, AC-011): project-wide feature listing, package
// status correction and a v2-data probe. No schema changes — these are
// plain queries over the frozen nine tables.
package store

// ListFeatures returns every feature package of a project across all of
// its versions, ordered by version then slug.
func (d *ProjectDao) ListFeatures(projectID string) ([]FeaturePackage, error) {
	rows, err := d.q.Query(`SELECT f.id, f.project_version_id, f.slug, f.truth_path, f.tests_path, f.status
		FROM feature_packages f
		JOIN project_versions v ON f.project_version_id = v.id
		WHERE v.project_id = ?
		ORDER BY v.version, f.slug`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeaturePackage
	for rows.Next() {
		f, err := scanFeature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// CountRunsByKind counts runs of a project grouped by run_kind. The
// migration guard uses it to distinguish migration-produced runs
// (run_kind = "v1_migration") from genuine v2 runtime data.
func (d *ProjectDao) CountRunsByKind(projectID string) (map[string]int, error) {
	rows, err := d.q.Query(`SELECT r.run_kind, COUNT(*)
		FROM runs r
		JOIN feature_packages f ON r.feature_id = f.id
		JOIN project_versions v ON f.project_version_id = v.id
		WHERE v.project_id = ?
		GROUP BY r.run_kind`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, err
		}
		out[kind] = n
	}
	return out, rows.Err()
}

// UpdatePackageStatus corrects a feature package's status string. It is
// the doctor rebuild's only in-place correction (AC-011): lifecycle
// snapshots and gate results are never rewritten by rebuild.
func (d *FeatureDao) UpdatePackageStatus(id, status string) error {
	if err := writable(d.s, d.inTx); err != nil {
		return err
	}
	_, err := d.q.Exec(`UPDATE feature_packages SET status = ? WHERE id = ?`, status, id)
	return err
}
