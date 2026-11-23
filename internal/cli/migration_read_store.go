package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"modernc.org/sqlite"

	"beacon-go/internal/store"
)

// Migration queries an SQLite online backup, never an independently copied DB
// and WAL pair. SQLite's read-only source connection supplies a consistent view
// of committed data while excluding unfinished transactions. It does not mutate
// authored packages or ledger data, but SQLite may create/update WAL/SHM locking
// and index sidecars beside the source. This is not a zero-filesystem-write API.
func openMigrationReadStore() (*store.Store, func(), error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, nil, err
	}
	// Resolve the ledger itself before SQLite discovers the corresponding WAL;
	// sidecars belong beside the real database, not beside a symlink alias.
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return nil, nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "beacon-migrate-store-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	target := filepath.Join(dir, "snapshot.db")
	if err = backupMigrationLedger(path, target); err != nil {
		cleanup()
		return nil, nil, err
	}
	st, err := store.OpenWithOptions(target, store.OpenOptions{ReadOnly: true})
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return st, func() { _ = st.Close(); cleanup() }, nil
}

func backupMigrationLedger(source, target string) error {
	uri := url.URL{Scheme: "file", Path: source}
	query := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(5000)"}}
	uri.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		backuper, ok := raw.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("migration_store_backup: SQLite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(target)
		if err != nil {
			return err
		}
		// One SQLite step holds the source read lock for the entire copy. Busy
		// or locked sources fail rather than reverting to an inconsistent copy.
		more, stepErr := backup.Step(-1)
		finishErr := backup.Finish()
		if stepErr != nil || finishErr != nil {
			return errors.Join(stepErr, finishErr)
		}
		if more {
			return fmt.Errorf("migration_store_backup: incomplete SQLite online backup")
		}
		return nil
	})
}
