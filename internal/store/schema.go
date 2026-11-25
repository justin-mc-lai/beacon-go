// Runtime-table schema contract (AC-067..075 + intent-tree tables) and its drift check (AC-161).
package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// SchemaVersion is the current PRAGMA user_version (migration skeleton,
// AC-080). Bumping it requires a migration step in Open.
const SchemaVersion = 2

// SchemaContract is the frozen column contract per table, in DDL order.
var SchemaContract = map[string][]string{
	"projects":          {"id", "root_path", "name", "registered_at"},
	"project_versions":  {"id", "project_id", "version", "docs_root", "is_current"},
	"feature_packages":  {"id", "project_version_id", "slug", "truth_path", "tests_path", "status"},
	"feature_lifecycle": {"feature_id", "stage", "status", "reason_codes_json", "updated_at"},
	"runs":              {"id", "feature_id", "run_kind", "status", "iteration", "started_at", "ended_at", "metadata_json"},
	"lifecycle_events":  {"id", "feature_id", "run_id", "event_type", "payload_json", "created_at"},
	"gate_results":      {"id", "feature_id", "gate_type", "verdict", "reason_codes_json", "evaluated_at"},
	"artifact_refs":     {"id", "run_id", "kind", "abs_path", "content_hash", "size_bytes", "created_at", "scope_kind", "scope_id"},
	"checkpoints":       {"id", "run_id", "iteration", "state_json", "created_at"},
	"tree_nodes": {"id", "project_version_id", "parent_id", "kind", "path", "status",
		"title", "fingerprint", "fingerprint_alg", "doc_path", "created_at", "updated_at"},
	"tree_migrations": {"id", "project_version_id", "source", "snapshot_path", "node_count", "created_at"},
}

// internalTables are bookkeeping tables excluded from the nine-table
// contract comparison (writer-track metadata + SQLite internals).
var internalTables = map[string]bool{"store_meta": true, "sqlite_sequence": true}

func runStatusCheckList() string {
	parts := make([]string, 0, 12)
	for _, s := range core.AllRunStatuses() {
		parts = append(parts, "'"+s.String()+"'")
	}
	return strings.Join(parts, ", ")
}

// schemaDDL returns the CREATE statements. The runs.status CHECK list is
// generated from core.AllRunStatuses() so code and DB can never drift
// (AC-031); a cross-package test asserts the same equality from the DB side.
func schemaDDL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS store_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			root_path TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			registered_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_versions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id),
			version TEXT NOT NULL,
			docs_root TEXT NOT NULL,
			is_current INTEGER NOT NULL DEFAULT 0
		)`,
		// AC-068: at most one current version per project, enforced by DB.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_project_versions_one_current
			ON project_versions(project_id) WHERE is_current = 1`,
		`CREATE TABLE IF NOT EXISTS feature_packages (
			id TEXT PRIMARY KEY,
			project_version_id TEXT NOT NULL REFERENCES project_versions(id),
			slug TEXT NOT NULL,
			truth_path TEXT NOT NULL,
			tests_path TEXT NOT NULL,
			status TEXT NOT NULL,
			UNIQUE (project_version_id, slug)
		)`,
		`CREATE TABLE IF NOT EXISTS feature_lifecycle (
			feature_id TEXT PRIMARY KEY REFERENCES feature_packages(id),
			stage TEXT NOT NULL,
			status TEXT NOT NULL,
			reason_codes_json TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL
		)`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			feature_id TEXT NOT NULL REFERENCES feature_packages(id),
			run_kind TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN (%s)),
			iteration INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			metadata_json TEXT NOT NULL DEFAULT '{}'
		)`, runStatusCheckList()),
		`CREATE TABLE IF NOT EXISTS lifecycle_events (
			id TEXT PRIMARY KEY,
			feature_id TEXT NOT NULL,
			run_id TEXT,
			event_type TEXT NOT NULL,
			payload_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL
		)`,
		// AC-012/072: append-only enforced at the DB layer, not by convention.
		`CREATE TRIGGER IF NOT EXISTS trg_lifecycle_events_no_update
			BEFORE UPDATE ON lifecycle_events
			BEGIN SELECT RAISE(ABORT, 'lifecycle_events_append_only'); END`,
		`CREATE TRIGGER IF NOT EXISTS trg_lifecycle_events_no_delete
			BEFORE DELETE ON lifecycle_events
			BEGIN SELECT RAISE(ABORT, 'lifecycle_events_append_only'); END`,
		`CREATE TABLE IF NOT EXISTS gate_results (
			id TEXT PRIMARY KEY,
			feature_id TEXT NOT NULL REFERENCES feature_packages(id),
			gate_type TEXT NOT NULL,
			verdict TEXT NOT NULL,
			reason_codes_json TEXT NOT NULL DEFAULT '[]',
			evaluated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS artifact_refs (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			abs_path TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			size_bytes INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			scope_kind TEXT NOT NULL DEFAULT 'feature',
			scope_id TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_artifact_refs_scope ON artifact_refs(scope_kind, scope_id, abs_path)`,
		`CREATE TABLE IF NOT EXISTS checkpoints (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL REFERENCES runs(id),
			iteration INTEGER NOT NULL,
			state_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			UNIQUE (run_id, iteration)
		)`,
		// Intent-tree index cache (mirrors the repo's canonical JSON; a
		// cache, never the truth source). Bodies stay in Markdown/JSON
		// files, so no truth-body column lives here (AC-005).
		`CREATE TABLE IF NOT EXISTS tree_nodes (
			id TEXT PRIMARY KEY,
			project_version_id TEXT NOT NULL REFERENCES project_versions(id),
			parent_id TEXT,
			kind TEXT NOT NULL,
			path TEXT NOT NULL,
			status TEXT NOT NULL,
			title TEXT NOT NULL,
			fingerprint TEXT NOT NULL,
			fingerprint_alg TEXT NOT NULL,
			doc_path TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tree_nodes_version ON tree_nodes(project_version_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tree_nodes_version_path ON tree_nodes(project_version_id, path)`,
		`CREATE TABLE IF NOT EXISTS tree_migrations (
			id TEXT PRIMARY KEY,
			project_version_id TEXT NOT NULL UNIQUE REFERENCES project_versions(id),
			source TEXT NOT NULL,
			snapshot_path TEXT NOT NULL,
			node_count INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
}

// VerifySchemaContract compares the live schema against the frozen
// nine-table contract (AC-161). Any missing/extra table or column returns
// a typed error with reason_code=schema_contract_drift.
func (s *Store) VerifySchemaContract() error {
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	live := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		live[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	var drift []string
	for table := range live {
		if internalTables[table] || strings.HasPrefix(table, "sqlite_") {
			continue
		}
		if _, ok := SchemaContract[table]; !ok {
			drift = append(drift, "unexpected table "+table)
		}
	}
	for table, wantCols := range SchemaContract {
		if !live[table] {
			drift = append(drift, "missing table "+table)
			continue
		}
		gotCols, err := s.tableColumns(table)
		if err != nil {
			return err
		}
		if !sameStringSet(gotCols, wantCols) {
			drift = append(drift, fmt.Sprintf("table %s columns live=%v want=%v", table, gotCols, wantCols))
		}
	}
	if len(drift) > 0 {
		sort.Strings(drift)
		return core.NewError(ReasonSchemaContractDrift, "VerifySchemaContract", strings.Join(drift, "; "))
	}
	return nil
}

// migrateArtifactScope upgrades a v1 artifact table to explicit scope.
// Rows whose run has no feature are refused instead of silently scoped.
func migrateArtifactScope(db *sql.DB) error {
	var hasKind int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('artifact_refs') WHERE name='scope_kind'`).Scan(&hasKind); err != nil {
		return err
	}
	if hasKind == 0 {
		if _, err := db.Exec(`ALTER TABLE artifact_refs ADD COLUMN scope_kind TEXT NOT NULL DEFAULT 'feature'`); err != nil {
			return err
		}
		if _, err := db.Exec(`ALTER TABLE artifact_refs ADD COLUMN scope_id TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`UPDATE artifact_refs SET scope_id = COALESCE((SELECT feature_id FROM runs WHERE runs.id = artifact_refs.run_id), '')
		WHERE scope_id = ''`); err != nil {
		return err
	}
	var missing int
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifact_refs WHERE scope_id = ''`).Scan(&missing); err != nil {
		return err
	}
	if missing > 0 {
		return core.NewError("artifact_scope_backfill_failed", "migrateArtifactScope",
			fmt.Sprintf("%d artifact rows have no resolvable feature scope", missing))
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_artifact_refs_scope ON artifact_refs(scope_kind, scope_id, abs_path)`); err != nil {
		return err
	}
	return nil
}

func (s *Store) tableColumns(table string) ([]string, error) {
	rows, err := s.db.Query(fmt.Sprintf(`SELECT name FROM pragma_table_info('%s') ORDER BY cid`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if !set[v] {
			return false
		}
	}
	return true
}
