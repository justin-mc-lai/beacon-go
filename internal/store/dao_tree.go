// Intent-tree index cache DAO. The canonical truth is the repo's JSON
// under <docs_root>/tree/; these rows are a rebuildable cache used for
// queries and joins. See docs/adr/ADR-0002-intent-tree-truth-model.md.
package store

import (
	"database/sql"
	"errors"
	"time"
)

// TreeNode is one cached tree node (no body/AC columns).
type TreeNode struct {
	ID               string `json:"id"`
	ProjectVersionID string `json:"project_version_id"`
	ParentID         string `json:"parent_id,omitempty"`
	Kind             string `json:"kind"`
	Path             string `json:"path"`
	Status           string `json:"status"`
	Title            string `json:"title"`
	Fingerprint      string `json:"fingerprint"`
	FingerprintAlg   string `json:"fingerprint_alg"`
	DocPath          string `json:"doc_path"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// TreeMigration records the one-time import from the flat feature
// table into the intent tree.
type TreeMigration struct {
	ID               string `json:"id"`
	ProjectVersionID string `json:"project_version_id"`
	Source           string `json:"source"`
	SnapshotPath     string `json:"snapshot_path"`
	NodeCount        int    `json:"node_count"`
	CreatedAt        string `json:"created_at"`
}

// Tree returns the tree DAO.
func (s *Store) Tree() *TreeDao { return &TreeDao{s: s, q: s.db} }

// TreeDao reads/writes the tree cache tables.
type TreeDao struct {
	s  *Store
	q  dbtx
	in bool
}

// insertTreeNodes inserts cached rows inside an open transaction. It is
// the single write shape shared by SaveTree and ReplaceTreeNodes.
func insertTreeNodes(tx *sql.Tx, versionID string, nodes []TreeNode) error {
	for _, n := range nodes {
		if _, err := tx.Exec(`INSERT INTO tree_nodes
			(id, project_version_id, parent_id, kind, path, status, title, fingerprint, fingerprint_alg, doc_path, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, versionID, nullIfEmpty(n.ParentID), n.Kind, n.Path, n.Status, n.Title,
			n.Fingerprint, n.FingerprintAlg, n.DocPath, n.CreatedAt, n.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

// SaveTree replaces the cached nodes for one version and records the
// migration, all in one transaction.
func (s *Store) SaveTree(versionID string, nodes []TreeNode, mig TreeMigration) error {
	if err := s.checkWritable(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM tree_nodes WHERE project_version_id = ?`, versionID); err != nil {
		return err
	}
	if err := insertTreeNodes(tx, versionID, nodes); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO tree_migrations
		(id, project_version_id, source, snapshot_path, node_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		mig.ID, mig.ProjectVersionID, mig.Source, mig.SnapshotPath, mig.NodeCount, mig.CreatedAt); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceTreeNodes refreshes only the node cache, leaving the migration
// record untouched. Used by `doctor rebuild-tree` after a cache loss.
func (s *Store) ReplaceTreeNodes(versionID string, nodes []TreeNode) error {
	if err := s.checkWritable(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM tree_nodes WHERE project_version_id = ?`, versionID); err != nil {
		return err
	}
	if err := insertTreeNodes(tx, versionID, nodes); err != nil {
		return err
	}
	return tx.Commit()
}

// GetTreeMigration returns the recorded migration for a version.
func (d *TreeDao) GetTreeMigration(versionID string) (TreeMigration, error) {
	var m TreeMigration
	err := d.q.QueryRow(`SELECT id, project_version_id, source, snapshot_path, node_count, created_at
		FROM tree_migrations WHERE project_version_id = ?`, versionID).
		Scan(&m.ID, &m.ProjectVersionID, &m.Source, &m.SnapshotPath, &m.NodeCount, &m.CreatedAt)
	return m, err
}

// ListTreeNodes returns all cached nodes for a version, ordered by id.
func (d *TreeDao) ListTreeNodes(versionID string) ([]TreeNode, error) {
	rows, err := d.q.Query(`SELECT id, project_version_id, COALESCE(parent_id,''), kind, path, status,
		title, fingerprint, fingerprint_alg, doc_path, created_at, updated_at
		FROM tree_nodes WHERE project_version_id = ? ORDER BY id`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TreeNode
	for rows.Next() {
		var n TreeNode
		if err := rows.Scan(&n.ID, &n.ProjectVersionID, &n.ParentID, &n.Kind, &n.Path, &n.Status,
			&n.Title, &n.Fingerprint, &n.FingerprintAlg, &n.DocPath, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetTreeNode returns one cached node.
func (d *TreeDao) GetTreeNode(id string) (TreeNode, error) {
	var n TreeNode
	err := d.q.QueryRow(`SELECT id, project_version_id, COALESCE(parent_id,''), kind, path, status,
		title, fingerprint, fingerprint_alg, doc_path, created_at, updated_at
		FROM tree_nodes WHERE id = ?`, id).
		Scan(&n.ID, &n.ProjectVersionID, &n.ParentID, &n.Kind, &n.Path, &n.Status,
			&n.Title, &n.Fingerprint, &n.FingerprintAlg, &n.DocPath, &n.CreatedAt, &n.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return n, sql.ErrNoRows
	}
	return n, err
}

// DeleteTree removes the cache rows for one version (rollback = rerun
// migrate after the repo JSON is deleted; the old flat tables stay).
func (s *Store) DeleteTree(versionID string) error {
	if err := s.checkWritable(); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM tree_nodes WHERE project_version_id = ?`, versionID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM tree_migrations WHERE project_version_id = ?`, versionID); err != nil {
		return err
	}
	return tx.Commit()
}

// NowUTC is the timestamp shape used by tree rows.
func NowUTC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
