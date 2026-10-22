// Shared helpers for the spine/projects command groups (P1): store
// opening, JSON output and reason-coded CLI failures. JSON mode writes
// exactly one JSON document to stdout and nothing else; failures always
// return a typed core.Error so stderr carries a stable reason_code and
// the process exits non-zero (see cmd/beacon/main.go).
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// CLI reason codes (stable contract strings, AC-065 error surface).
const (
	ReasonProjectNotRegistered = "project_not_registered"
	ReasonFeatureNotFound      = "feature_not_found"
	ReasonRunNotFound          = "run_not_found"
)

// openReadStore opens the global DB read-only (AC-061 path resolution).
// exists=false (with a nil store) means the DB file has not been
// created yet — callers translate that into empty lists or not-found
// errors instead of leaking a driver error.
func openReadStore() (st *store.Store, exists bool, err error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, false, err
	}
	if path != ":memory:" {
		if _, statErr := os.Stat(path); statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, false, nil
			}
			return nil, false, statErr
		}
	}
	st, err = store.OpenWithOptions(path, store.OpenOptions{ReadOnly: true})
	if err != nil {
		return nil, false, err
	}
	return st, true, nil
}

// openWriteStore opens the global DB for writing (projects register).
func openWriteStore() (*store.Store, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

// printJSON writes exactly one JSON document to the command's stdout.
func printJSON(cmd *cobra.Command, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(raw))
	return err
}

// fail builds a reason-coded CLI error (typed, never a panic).
func fail(reasonCode, op, detail string) error {
	return core.NewError(reasonCode, op, detail)
}
