// Row types and the Project/Feature DAOs (AC-076). All DAO writes go
// through Store.checkWritable (read-only + track domain, AC-158).
package store

import (
	"encoding/json"

	"beacon-go/internal/core"
)

// Project is a projects row (AC-067).
type Project struct {
	ID           string `json:"id"`
	RootPath     string `json:"root_path"`
	Name         string `json:"name"`
	RegisteredAt string `json:"registered_at"`
}

// ProjectVersion is a project_versions row (AC-068).
type ProjectVersion struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Version   string `json:"version"`
	DocsRoot  string `json:"docs_root"`
	IsCurrent bool   `json:"is_current"`
}

// FeaturePackage is a feature_packages row (AC-069).
type FeaturePackage struct {
	ID               string `json:"id"`
	ProjectVersionID string `json:"project_version_id"`
	Slug             string `json:"slug"`
	TruthPath        string `json:"truth_path"`
	TestsPath        string `json:"tests_path"`
	Status           string `json:"status"`
}

// Lifecycle is a feature_lifecycle snapshot row (AC-070).
type Lifecycle struct {
	FeatureID   string             `json:"feature_id"`
	Stage       core.WorkflowStage `json:"stage"`
	Status      core.StageStatus   `json:"status"`
	ReasonCodes []string           `json:"reason_codes,omitempty"`
	UpdatedAt   string             `json:"updated_at"`
}

// ProjectDao implements project + version access (AC-067/068).
type ProjectDao struct {
	s    *Store
	q    dbtx
	inTx bool
}

// Projects returns the ProjectDao.
func (s *Store) Projects() *ProjectDao { return &ProjectDao{s: s, q: s.db} }

// Register inserts a project (root_path is UNIQUE). An empty ID and
// RegisteredAt are filled in.
func (d *ProjectDao) Register(p Project) (Project, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return Project{}, err
	}
	if p.ID == "" {
		p.ID = newID("proj_")
	}
	// Canonicalize at the write boundary: every read path resolves
	// the root (ResolveRootPath), so storing an unresolved path would
	// make the row unfindable whenever the two spellings differ
	// (macOS /var -> /private/var symlink is the common case).
	p.RootPath = ResolveRootPath(p.RootPath)
	p.RegisteredAt = orNow(p.RegisteredAt)
	_, err := d.q.Exec(`INSERT INTO projects(id, root_path, name, registered_at) VALUES(?,?,?,?)`,
		p.ID, p.RootPath, p.Name, p.RegisteredAt)
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

func scanProject(row interface{ Scan(...any) error }) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.RootPath, &p.Name, &p.RegisteredAt)
	return p, err
}

// GetByRootPath looks a project up by its unique root path.
func (d *ProjectDao) GetByRootPath(rootPath string) (Project, error) {
	return scanProject(d.q.QueryRow(
		`SELECT id, root_path, name, registered_at FROM projects WHERE root_path = ?`, ResolveRootPath(rootPath)))
}

// GetByID looks a project up by id.
func (d *ProjectDao) GetByID(id string) (Project, error) {
	return scanProject(d.q.QueryRow(
		`SELECT id, root_path, name, registered_at FROM projects WHERE id = ?`, id))
}

// List returns all registered projects in registration order.
func (d *ProjectDao) List() ([]Project, error) {
	rows, err := d.q.Query(`SELECT id, root_path, name, registered_at FROM projects ORDER BY registered_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddVersion inserts a project version; when IsCurrent is set it atomically
// clears the previous current version (AC-068). Outside a batch it runs
// in its own transaction, exactly as before; inside a WithTx batch the
// statements join the caller's transaction.
func (d *ProjectDao) AddVersion(v ProjectVersion) (ProjectVersion, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return ProjectVersion{}, err
	}
	if v.ID == "" {
		v.ID = newID("ver_")
	}
	if d.inTx {
		return v, d.addVersionOn(d.q, v)
	}
	tx, err := d.s.db.Begin()
	if err != nil {
		return ProjectVersion{}, err
	}
	defer tx.Rollback()
	if err := d.addVersionOn(tx, v); err != nil {
		return ProjectVersion{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectVersion{}, err
	}
	return v, nil
}

func (d *ProjectDao) addVersionOn(q dbtx, v ProjectVersion) error {
	if v.IsCurrent {
		if _, err := q.Exec(`UPDATE project_versions SET is_current = 0 WHERE project_id = ?`, v.ProjectID); err != nil {
			return err
		}
	}
	_, err := q.Exec(`INSERT INTO project_versions(id, project_id, version, docs_root, is_current) VALUES(?,?,?,?,?)`,
		v.ID, v.ProjectID, v.Version, v.DocsRoot, boolToInt(v.IsCurrent))
	return err
}

// SetCurrent moves the current marker to the given version in one
// transaction (AC-068: exactly one current version per project).
// Inside a WithTx batch the statements join the caller's transaction.
func (d *ProjectDao) SetCurrent(versionID string) error {
	if err := writable(d.s, d.inTx); err != nil {
		return err
	}
	if d.inTx {
		return d.setCurrentOn(d.q, versionID)
	}
	tx, err := d.s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := d.setCurrentOn(tx, versionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *ProjectDao) setCurrentOn(q dbtx, versionID string) error {
	var projectID string
	if err := q.QueryRow(`SELECT project_id FROM project_versions WHERE id = ?`, versionID).Scan(&projectID); err != nil {
		return err
	}
	if _, err := q.Exec(`UPDATE project_versions SET is_current = 0 WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	_, err := q.Exec(`UPDATE project_versions SET is_current = 1 WHERE id = ?`, versionID)
	return err
}

// CurrentVersion returns the project's current version row.
func (d *ProjectDao) CurrentVersion(projectID string) (ProjectVersion, error) {
	var v ProjectVersion
	var cur int
	err := d.q.QueryRow(`SELECT id, project_id, version, docs_root, is_current FROM project_versions WHERE project_id = ? AND is_current = 1`, projectID).
		Scan(&v.ID, &v.ProjectID, &v.Version, &v.DocsRoot, &cur)
	v.IsCurrent = cur == 1
	return v, err
}

// ListVersions returns all versions of a project.
func (d *ProjectDao) ListVersions(projectID string) ([]ProjectVersion, error) {
	rows, err := d.q.Query(`SELECT id, project_id, version, docs_root, is_current FROM project_versions WHERE project_id = ? ORDER BY version`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectVersion
	for rows.Next() {
		var v ProjectVersion
		var cur int
		if err := rows.Scan(&v.ID, &v.ProjectID, &v.Version, &v.DocsRoot, &cur); err != nil {
			return nil, err
		}
		v.IsCurrent = cur == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// FeatureDao implements feature package + lifecycle access (AC-069/070).
type FeatureDao struct {
	s    *Store
	q    dbtx
	inTx bool
}

// Features returns the FeatureDao.
func (s *Store) Features() *FeatureDao { return &FeatureDao{s: s, q: s.db} }

// CreatePackage inserts a feature package. Slug must be unique within the
// project version (DB UNIQUE constraint, AC-069).
func (d *FeatureDao) CreatePackage(f FeaturePackage) (FeaturePackage, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return FeaturePackage{}, err
	}
	if f.ID == "" {
		f.ID = newID("feat_")
	}
	if f.Status == "" {
		f.Status = "active"
	}
	_, err := d.q.Exec(`INSERT INTO feature_packages(id, project_version_id, slug, truth_path, tests_path, status) VALUES(?,?,?,?,?,?)`,
		f.ID, f.ProjectVersionID, f.Slug, f.TruthPath, f.TestsPath, f.Status)
	if err != nil {
		return FeaturePackage{}, err
	}
	return f, nil
}

func scanFeature(row interface{ Scan(...any) error }) (FeaturePackage, error) {
	var f FeaturePackage
	err := row.Scan(&f.ID, &f.ProjectVersionID, &f.Slug, &f.TruthPath, &f.TestsPath, &f.Status)
	return f, err
}

// GetBySlug looks a package up by (project_version, slug).
func (d *FeatureDao) GetBySlug(projectVersionID, slug string) (FeaturePackage, error) {
	return scanFeature(d.q.QueryRow(
		`SELECT id, project_version_id, slug, truth_path, tests_path, status FROM feature_packages WHERE project_version_id = ? AND slug = ?`,
		projectVersionID, slug))
}

// GetByID looks a package up by id.
func (d *FeatureDao) GetByID(id string) (FeaturePackage, error) {
	return scanFeature(d.q.QueryRow(
		`SELECT id, project_version_id, slug, truth_path, tests_path, status FROM feature_packages WHERE id = ?`, id))
}

// UpdateLifecycle upserts the current lifecycle snapshot for a feature
// (AC-070 + AC-012 snapshot semantics). Stage/status are validated through
// core, including the Blocked/Skipped obligations (AC-028/029).
func (d *FeatureDao) UpdateLifecycle(l Lifecycle) error {
	if err := writable(d.s, d.inTx); err != nil {
		return err
	}
	st := core.StageState{Stage: l.Stage, Status: l.Status, ReasonCodes: l.ReasonCodes}
	if err := st.Validate(); err != nil {
		return err
	}
	codes, err := json.Marshal(orEmptyCodes(l.ReasonCodes))
	if err != nil {
		return err
	}
	l.UpdatedAt = orNow(l.UpdatedAt)
	_, err = d.q.Exec(`INSERT INTO feature_lifecycle(feature_id, stage, status, reason_codes_json, updated_at)
		VALUES(?,?,?,?,?)
		ON CONFLICT(feature_id) DO UPDATE SET stage=excluded.stage, status=excluded.status,
			reason_codes_json=excluded.reason_codes_json, updated_at=excluded.updated_at`,
		l.FeatureID, string(l.Stage), string(l.Status), string(codes), l.UpdatedAt)
	return err
}

// GetLifecycle returns the current snapshot for a feature.
func (d *FeatureDao) GetLifecycle(featureID string) (Lifecycle, error) {
	var l Lifecycle
	var stage, status, codes string
	err := d.q.QueryRow(`SELECT feature_id, stage, status, reason_codes_json, updated_at FROM feature_lifecycle WHERE feature_id = ?`, featureID).
		Scan(&l.FeatureID, &stage, &status, &codes, &l.UpdatedAt)
	if err != nil {
		return Lifecycle{}, err
	}
	l.Stage = core.WorkflowStage(stage)
	l.Status = core.StageStatus(status)
	if codes != "" {
		_ = json.Unmarshal([]byte(codes), &l.ReasonCodes)
	}
	return l, nil
}

func orEmptyCodes(c []string) []string {
	if c == nil {
		return []string{}
	}
	return c
}
