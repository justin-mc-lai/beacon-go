package cli

import (
	"bytes"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func TestMigrationReadStoreSnapshotsCommittedWALThroughSymlink(t *testing.T) {
	// Keep the writer open and disable auto-checkpoint so committed project
	// rows remain in the WAL. The large unfinished update also spills frames
	// into WAL, exercising SQLite's commit boundary rather than file stability.
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	project, err := st.Projects().Register(store.Project{RootPath: t.TempDir(), Name: "committed"})
	if err != nil {
		_ = st.Close()
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	uri := url.URL{Scheme: "file", Path: path}
	writer, err := sql.Open("sqlite", uri.String())
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	writer.SetMaxOpenConns(1)
	for _, statement := range []string{"PRAGMA journal_mode=WAL", "PRAGMA wal_autocheckpoint=0", "PRAGMA cache_size=1", "PRAGMA cache_spill=ON"} {
		if _, err = writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = writer.Exec("UPDATE projects SET name = 'committed-in-wal' WHERE id = ?", project.ID); err != nil {
		t.Fatal(err)
	}
	wal, err := os.Stat(path + "-wal")
	if err != nil || wal.Size() == 0 {
		t.Fatalf("fixture has no committed WAL: %v", err)
	}
	tx, err := writer.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE projects SET name = ? WHERE id = ?", strings.Repeat("unfinished", 100000), project.ID); err != nil {
		t.Fatal(err)
	}
	linkDir := t.TempDir()
	alias := filepath.Join(linkDir, "alias.db")
	if err = os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv(store.EnvStorePath, alias)
	snapshot, cleanup, err := openMigrationReadStore()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	got, err := snapshot.Projects().GetByID(project.ID)
	if err != nil || got.Name != "committed-in-wal" {
		t.Fatalf("snapshot exposed unfinished data or missed committed WAL: %+v %v", got, err)
	}
	if _, err = snapshot.Projects().Register(store.Project{RootPath: t.TempDir(), Name: "forbidden"}); err == nil {
		t.Fatal("snapshot store is writable")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err = writer.Exec("UPDATE projects SET name = 'committed-after-snapshot' WHERE id = ?", project.ID); err != nil {
		t.Fatal(err)
	}
	got, err = snapshot.Projects().GetByID(project.ID)
	if err != nil || got.Name != "committed-in-wal" {
		t.Fatalf("snapshot changed after a later commit: %+v %v", got, err)
	}
	var sourceName string
	if err = writer.QueryRow("SELECT name FROM projects WHERE id = ?", project.ID).Scan(&sourceName); err != nil || sourceName != "committed-after-snapshot" {
		t.Fatalf("source ledger changed unexpectedly: %q %v", sourceName, err)
	}
	entries, err := os.ReadDir(linkDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "alias.db" {
		t.Fatalf("SQLite used sidecars beside symlink instead of real database: %+v %v", entries, err)
	}
	snapshotDir := filepath.Dir(snapshot.Path)
	cleanup()
	if _, err = os.Stat(snapshotDir); !os.IsNotExist(err) {
		t.Fatalf("snapshot cleanup left its directory: %v", err)
	}
}

func TestMigrationReadStoreLeavesClosedLedgerDataUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	project, err := st.Projects().Register(store.Project{RootPath: t.TempDir(), Name: "original"})
	if err != nil {
		_ = st.Close()
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(store.EnvStorePath, path)
	snapshot, cleanup, err := openMigrationReadStore()
	if err != nil {
		t.Fatal(err)
	}
	got, err := snapshot.Projects().GetByID(project.ID)
	cleanup()
	if err != nil || got.Name != "original" {
		t.Fatalf("snapshot read: %+v %v", got, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("online backup mutated source database bytes: %v", err)
	}
}

func TestMigrationReadStoreMissingLedgerCreatesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(store.EnvStorePath, filepath.Join(dir, "missing.db"))
	if snapshot, cleanup, err := openMigrationReadStore(); err == nil {
		cleanup()
		t.Fatalf("missing ledger opened: %+v", snapshot)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing source open created files: %+v %v", entries, err)
	}
}
