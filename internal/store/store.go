// Package store is the Go counterpart of the frozen v2 design's
// beacon-store crate: the machine-level global SQLite database
// ~/.beacon/beacon-go.db (WAL) plus blob pointers (AC-007..016, AC-067..083).
//
// P0 scope: Open + migration skeleton (user_version), the nine-table DDL,
// six DAOs, append-only lifecycle events, the per-track write domain
// (AC-158) and the schema-contract drift check (AC-161). The DB stores
// runtime state only — never requirement truth bodies (AC-005).
package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"

	"beacon-go/internal/core"
)

// EnvStorePath overrides the global DB location (test isolation, AC-061).
const EnvStorePath = "BEACON_STORE_PATH"

// TrackV2Go is this implementation's writer-track identity (AC-158).
const TrackV2Go = "track_v2_go"

// Reason codes owned by the store layer.
const (
	ReasonSchemaContractDrift   = "schema_contract_drift"
	ReasonDualTrackWriter       = "dual_track_writer_conflict"
	ReasonDowngradeForbidden    = "schema_downgrade_forbidden"
	ReasonStoreReadOnly         = "store_read_only"
	ReasonArtifactPathNotAbs    = "artifact_path_not_absolute"
	ReasonCheckpointRegression  = "checkpoint_iteration_regression"
	ReasonLifecycleBlockedCodes = core.ReasonBlockedMissingReasons
)

// Tables is the frozen nine-table contract (AC-067..075), kept for callers
// that only need the names; the column contract lives in schema.go.
var Tables = []string{
	"projects", "project_versions", "feature_packages", "feature_lifecycle",
	"runs", "lifecycle_events", "gate_results", "artifact_refs", "checkpoints",
}

// DefaultPath resolves the global DB path: $BEACON_STORE_PATH when set,
// otherwise ~/.beacon/beacon-go.db. Per-project beacon.db files are explicitly
// forbidden as runtime authority (AC-008).
func DefaultPath() (string, error) {
	if p := os.Getenv(EnvStorePath); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".beacon", "beacon-go.db"), nil
}

// OpenOptions controls Open. Track binds the write domain (AC-158);
// ReadOnly opens the DB without write permission and skips the track check.
type OpenOptions struct {
	Track    string
	ReadOnly bool
}

// Store is an open handle to the global runtime DB.
type Store struct {
	db       *sql.DB
	Path     string
	Track    string
	ReadOnly bool
	// txOpen guards WithTx against nesting (see tx.go).
	txOpen atomic.Bool
}

// Open opens (creating if needed) the DB at path with the default track.
func Open(path string) (*Store, error) {
	return OpenWithOptions(path, OpenOptions{})
}

// OpenWithOptions opens the DB, running the migration skeleton when the
// file is new. Behaviour:
//   - WAL journal mode, foreign keys on.
//   - PRAGMA user_version > SchemaVersion → refuse to open (no downgrade,
//     AC-080, reason_code=schema_downgrade_forbidden).
//   - user_version == 0 → create schema, stamp meta + user_version.
//   - Write opens bind the writer track: a DB already owned by another
//     track fails closed (AC-158, reason_code=dual_track_writer_conflict).
//     Read-only opens are always allowed (cross-track read/compare).
func OpenWithOptions(path string, opts OpenOptions) (*Store, error) {
	track := opts.Track
	if track == "" {
		track = TrackV2Go
	}
	if !opts.ReadOnly && path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, err
			}
		}
	}
	dsn := sqliteDSN(path, opts.ReadOnly)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single-writer local store (AC-093)
	s := &Store{db: db, Path: path, Track: track, ReadOnly: opts.ReadOnly}
	if err := s.init(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func sqliteDSN(path string, readOnly bool) string {
	if path == ":memory:" {
		return "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	if readOnly {
		// No journal_mode pragma: a read-only connection cannot change it.
		return fmt.Sprintf("file:%s?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	}
	// synchronous=NORMAL (P6 write-path fix, docs/p6-fix-write-perf.md):
	// under WAL, NORMAL can never corrupt the database and a process
	// crash loses nothing; only an OS crash / power cut can roll back
	// the last committed transaction(s). beacon.db holds runtime
	// state — gate verdicts are re-evaluable and `doctor rebuild`
	// reconstructs the store from the Markdown truth — so the trade
	// buys ~200x cheaper commits (measured on the reference volume:
	// ~23ms per commit at the FULL default vs ~0.1ms at NORMAL) at a
	// bounded, rebuildable risk. The pragma is per-connection, so it
	// is pinned here in the DSN and asserted by a store test.
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
}

func (s *Store) init() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version > SchemaVersion {
		return core.NewError(ReasonDowngradeForbidden, "store.Open",
			fmt.Sprintf("db user_version %d is newer than supported %d; downgrade open refused", version, SchemaVersion))
	}
	if s.ReadOnly {
		return nil // no migration, no track binding on read-only opens
	}
	if version == 0 {
		for _, stmt := range schemaDDL() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("schema init: %w", err)
			}
		}
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, SchemaVersion)); err != nil {
			return err
		}
	} else {
		// Idempotent re-open: IF NOT EXISTS DDL is a no-op, then verify.
		for _, stmt := range schemaDDL() {
			if _, err := s.db.Exec(stmt); err != nil {
				return fmt.Errorf("schema ensure: %w", err)
			}
		}
		if err := s.VerifySchemaContract(); err != nil {
			return err
		}
	}
	return s.bindTrack()
}

// bindTrack records or enforces the writer-track ownership (AC-158).
func (s *Store) bindTrack() error {
	var owner string
	err := s.db.QueryRow(`SELECT value FROM store_meta WHERE key='writer_track'`).Scan(&owner)
	if err == sql.ErrNoRows {
		_, err = s.db.Exec(`INSERT INTO store_meta(key, value) VALUES('writer_track', ?)`, s.Track)
		return err
	}
	if err != nil {
		return err
	}
	if owner != s.Track {
		return core.NewError(ReasonDualTrackWriter, "store.Open",
			"db writer track is "+owner+", refusing write open as "+s.Track)
	}
	return nil
}

// WriterTrack returns the track that owns this DB's write domain.
func (s *Store) WriterTrack() (string, error) {
	var owner string
	err := s.db.QueryRow(`SELECT value FROM store_meta WHERE key='writer_track'`).Scan(&owner)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return owner, err
}

// checkWritable is the defense-in-depth gate used by every DAO write.
func (s *Store) checkWritable() error {
	if s.ReadOnly {
		return core.NewError(ReasonStoreReadOnly, "store.write", "store opened read-only")
	}
	owner, err := s.WriterTrack()
	if err != nil {
		return err
	}
	if owner != "" && owner != s.Track {
		return core.NewError(ReasonDualTrackWriter, "store.write",
			"db writer track is "+owner+", this handle is "+s.Track)
	}
	return nil
}

// Close closes the underlying DB.
func (s *Store) Close() error { return s.db.Close() }

// BackupTo writes a consistent snapshot of the database to dest
// (AC-092). VACUUM INTO gives a self-contained file that includes the
// WAL content, so a crash during a later write cannot lose the pre-write
// state. It refuses to overwrite: every backup is a new file.
func (s *Store) BackupTo(dest string) error {
	if s.ReadOnly {
		return core.NewError(ReasonStoreReadOnly, "store.BackupTo", "cannot back up through a read-only handle")
	}
	if s.Path == ":memory:" {
		return core.NewError(ReasonStoreReadOnly, "store.BackupTo", "in-memory store has no file to back up")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("backup target already exists: %s", dest)
	}
	if dir := filepath.Dir(dest); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, dest); err != nil {
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	return nil
}

// DB exposes the raw handle for tests and diagnostics.
func (s *Store) DB() *sql.DB { return s.db }

// UserVersion returns PRAGMA user_version.
func (s *Store) UserVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v)
	return v, err
}

// JournalMode returns the active journal mode (WAL expected for files).
func (s *Store) JournalMode() (string, error) {
	var m string
	err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&m)
	return m, err
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is vanishingly rare; fall back to time-based.
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

// tsLayout is RFC3339 in UTC with a fixed nine-digit fraction. Readers
// order events by the text value (`ORDER BY created_at, rowid`), so this
// layout must sort lexicographically in chronological order.
// time.RFC3339Nano trims trailing zeros, which breaks that: ".1Z" sorts
// after ".15Z", and a whole-second "Z" sorts after both — an earlier
// event can then surface behind a later one (TestTraceOrderAndBundle).
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func tsFormat(t time.Time) string { return t.UTC().Format(tsLayout) }

func nowUTC() string { return tsFormat(time.Now()) }

// orNow keeps caller-supplied stamps verbatim (fixtures and replayed
// history rely on exact values); only generated stamps get tsLayout.
func orNow(v string) string {
	if strings.TrimSpace(v) == "" {
		return nowUTC()
	}
	return v
}
