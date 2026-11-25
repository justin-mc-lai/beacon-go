package store

import (
	"database/sql"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func openV1ArtifactDB(t *testing.T, withRun bool) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE runs (id TEXT PRIMARY KEY, feature_id TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE artifact_refs (
		id TEXT PRIMARY KEY, run_id TEXT NOT NULL, kind TEXT NOT NULL, abs_path TEXT NOT NULL,
		content_hash TEXT NOT NULL, size_bytes INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if withRun {
		if _, err := db.Exec(`INSERT INTO runs(id, feature_id) VALUES('run_1','feat_1')`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO artifact_refs(id, run_id, kind, abs_path, content_hash, created_at)
		VALUES('art_1','run_1','qa_report','/tmp/a','h','now')`); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrateArtifactScopeBackfillsFeature(t *testing.T) {
	db := openV1ArtifactDB(t, true)
	if err := migrateArtifactScope(db); err != nil {
		t.Fatal(err)
	}
	var kind, id string
	if err := db.QueryRow(`SELECT scope_kind, scope_id FROM artifact_refs WHERE id='art_1'`).Scan(&kind, &id); err != nil {
		t.Fatal(err)
	}
	if kind != "feature" || id != "feat_1" {
		t.Fatalf("scope = %s/%s, want feature/feat_1", kind, id)
	}
	// Idempotent.
	if err := migrateArtifactScope(db); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateArtifactScopeFailsOnDanglingRun(t *testing.T) {
	db := openV1ArtifactDB(t, false)
	err := migrateArtifactScope(db)
	if err == nil || !strings.Contains(err.Error(), "artifact_scope_backfill_failed") {
		t.Fatalf("dangling run must fail closed, got %v", err)
	}
}

func TestArtifactAddFillsFeatureScope(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	run, err := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning})
	if err != nil {
		t.Fatal(err)
	}
	art, err := s.Artifacts().Add(ArtifactRef{RunID: run.ID, Kind: "qa_report", AbsPath: "/tmp/scope-proof", ContentHash: "h"})
	if err != nil {
		t.Fatal(err)
	}
	var kind, id string
	if err := s.DB().QueryRow(`SELECT scope_kind, scope_id FROM artifact_refs WHERE id = ?`, art.ID).Scan(&kind, &id); err != nil {
		t.Fatal(err)
	}
	if kind != "feature" || id != f.ID {
		t.Fatalf("scope = %s/%s, want feature/%s", kind, id, f.ID)
	}
}

func TestNormalizeArtifactScope(t *testing.T) {
	if _, _, err := NormalizeArtifactScope("", "x"); err == nil {
		t.Error("empty kind must fail")
	}
	if _, _, err := NormalizeArtifactScope("node", " "); err == nil {
		t.Error("empty id must fail")
	}
	if _, _, err := NormalizeArtifactScope("team", "x"); err == nil {
		t.Error("unknown kind must fail")
	}
	if k, id, err := NormalizeArtifactScope(" NODE ", " n1 "); err != nil || k != "node" || id != "n1" {
		t.Fatalf("normalize = %s/%s %v", k, id, err)
	}
}

func TestArtifactNodeScopeIsExplicitAndIdempotent(t *testing.T) {
	s, _ := openTestStore(t)
	ref := ArtifactRef{RunID: ScopeRunID("node", "n1"), ScopeKind: "node", ScopeID: "n1",
		Kind: "evidence", AbsPath: "/tmp/node-proof", ContentHash: "h1"}
	if _, err := s.Artifacts().Add(ref); err != nil {
		t.Fatal(err)
	}
	ref.ContentHash = "h2"
	if _, err := s.Artifacts().Add(ref); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Artifacts().ListByScope("node", "n1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ContentHash != "h2" {
		t.Fatalf("node scope rows = %+v", rows)
	}
	empty, err := s.Artifacts().ListByScope("feature", "n1")
	if err != nil || len(empty) != 0 {
		t.Fatalf("feature scope leaked: %+v %v", empty, err)
	}
	if _, err := s.Artifacts().Add(ArtifactRef{RunID: ScopeRunID("node", "n2"), Kind: "evidence", AbsPath: "relative", ContentHash: "h"}); err == nil {
		t.Fatal("relative path must fail")
	}
}
