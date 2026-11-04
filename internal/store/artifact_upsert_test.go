// artifact-upsert TC-AU-001..002, TC-AU-006: registration by
// (run_id, abs_path) is an upsert. Re-running a stage rewrites the same
// artifact file, so re-registering it must refresh the stored pointer
// in place — one row, same id, new hash/size/time — instead of leaving a
// second row whose hash can never match disk again.
package store

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// seedRun creates a project -> version -> feature -> run chain.
func seedRun(t *testing.T, s *Store) Run {
	t.Helper()
	_, _, f := seedFeature(t, s)
	r, err := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning})
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return r
}

// TC-AU-001 (AC-001): two Adds of the same (run, path) leave ONE row
// carrying the newest content_hash / size_bytes / created_at.
func TestAddUpsertsByPath(t *testing.T) {
	s, _ := openTestStore(t)
	r := seedRun(t, s)
	path := filepath.Join(t.TempDir(), "qa", "report.json")

	first, err := s.Artifacts().Add(ArtifactRef{
		RunID: r.ID, Kind: "qa_report", AbsPath: path,
		ContentHash: "hash-v1", SizeBytes: 10, CreatedAt: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}
	second, err := s.Artifacts().Add(ArtifactRef{
		RunID: r.ID, Kind: "qa_report", AbsPath: path,
		ContentHash: "hash-v2", SizeBytes: 99, CreatedAt: "2026-01-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("second Add: %v", err)
	}

	rows, err := s.Artifacts().ListByRun(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows after re-registering the same path = %d, want 1 (%v)", len(rows), rows)
	}
	got := rows[0]
	if got.ContentHash != "hash-v2" || got.SizeBytes != 99 || got.CreatedAt != "2026-01-02T00:00:00Z" {
		t.Fatalf("row not refreshed: %+v, want hash-v2/99/2026-01-02T00:00:00Z", got)
	}
	if got.Kind != "qa_report" || got.AbsPath != path || got.RunID != r.ID {
		t.Fatalf("upsert changed identity columns: %+v", got)
	}
	if second.ContentHash != got.ContentHash || second.SizeBytes != got.SizeBytes {
		t.Fatalf("Add result = %+v, want the stored row %+v", second, got)
	}

	// No leftovers keyed by the old hash (the stale-hash trap that made
	// the qa gate permanently stale).
	old, err := s.Artifacts().ListByHash("hash-v1")
	if err != nil {
		t.Fatal(err)
	}
	if len(old) != 0 {
		t.Fatalf("old hash still referenced by %d row(s): %v", len(old), old)
	}
	if first.ID == "" {
		t.Fatal("first Add must return an id")
	}

	// Two different paths in the same run are two rows: the key is
	// (run_id, abs_path), not the run or the file name alone.
	other := filepath.Join(t.TempDir(), "qa", "report.json")
	if _, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "qa_report", AbsPath: other, ContentHash: "hash-x", SizeBytes: 1}); err != nil {
		t.Fatalf("Add other path: %v", err)
	}
	// Same path in another run is its own row too.
	r2, err := s.Runs().CreateRun(Run{FeatureID: r.FeatureID, RunKind: "goal", Status: core.RunRunning})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Artifacts().Add(ArtifactRef{RunID: r2.ID, Kind: "qa_report", AbsPath: path, ContentHash: "hash-v3", SizeBytes: 2}); err != nil {
		t.Fatalf("Add same path on run2: %v", err)
	}
	if rows, err := s.Artifacts().ListByRun(r.ID); err != nil || len(rows) != 2 {
		t.Fatalf("run1 rows = %d err=%v, want 2", len(rows), err)
	}
	if rows, err := s.Artifacts().ListByRun(r2.ID); err != nil || len(rows) != 1 || rows[0].ContentHash != "hash-v3" {
		t.Fatalf("run2 rows = %v err=%v, want 1 row hash-v3", rows, err)
	}

	// TC-AU-006 (AC-006): the fix rides on the frozen schema — no new
	// column, so the schema contract check keeps passing untouched.
	if err := s.VerifySchemaContract(); err != nil {
		t.Fatalf("schema contract drifted: %v", err)
	}
	cols, err := columnsOf(t, s, "artifact_refs")
	if err != nil {
		t.Fatal(err)
	}
	want := "abs_path,content_hash,created_at,id,kind,run_id,size_bytes"
	if strings.Join(cols, ",") != want {
		t.Fatalf("artifact_refs columns = %v, want %s", cols, want)
	}
}

// TC-AU-002 (AC-002): the in-place update keeps the original row id, so
// ids held by events, evidence and external references stay valid.
func TestAddKeepsIdentity(t *testing.T) {
	s, _ := openTestStore(t)
	r := seedRun(t, s)
	path := filepath.Join(t.TempDir(), "evidence.json")

	first, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: path, ContentHash: "h1", SizeBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	// A caller-supplied id must not hijack the stored row's identity
	// either: the row on record wins.
	second, err := s.Artifacts().Add(ArtifactRef{ID: "art_caller_supplied", RunID: r.ID, Kind: "evidence", AbsPath: path, ContentHash: "h2", SizeBytes: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("id after refresh = %q, want the original %q", second.ID, first.ID)
	}
	if second.ID == "art_caller_supplied" {
		t.Fatal("a caller-supplied id must not replace the stored row id")
	}
	third, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: path, ContentHash: "h3", SizeBytes: 3})
	if err != nil {
		t.Fatal(err)
	}
	if third.ID != first.ID {
		t.Fatalf("id after second refresh = %q, want %q", third.ID, first.ID)
	}
	rows, err := s.Artifacts().ListByRun(r.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows = %v err=%v, want 1", rows, err)
	}
	if rows[0].ID != first.ID || rows[0].ContentHash != "h3" {
		t.Fatalf("stored row = %+v, want id %q with hash h3", rows[0], first.ID)
	}
	// Re-registering a fresh path still mints a new id.
	fresh, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: filepath.Join(t.TempDir(), "b.json"), ContentHash: "h4"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == first.ID || fresh.ID == "" {
		t.Fatalf("a new path must get a new id, got %q", fresh.ID)
	}
}

// columnsOf reads the live column names of one table, sorted.
func columnsOf(t *testing.T, s *Store, table string) ([]string, error) {
	t.Helper()
	rows, err := s.DB().Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, rows.Err()
}
