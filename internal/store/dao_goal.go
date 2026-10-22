// Goal-engine store support (P2a): atomic run state updates (status +
// metadata + ended_at in one statement) and per-run event listing. The
// goal engine keeps ALL of its runtime state in the nine tables — runs
// (status/metadata_json), lifecycle_events (append-only trail) and
// checkpoints — never in its own JSON state files (charter rule 7,
// frozen AC-016).
package store

import (
	"database/sql"
	"encoding/json"

	"beacon-go/internal/core"
)

// UpdateRunState atomically updates a run's status, metadata_json and
// (when endedAt is non-nil) ended_at. Status is validated through core
// (AC-031). Returns sql.ErrNoRows when the run does not exist.
func (d *RunDao) UpdateRunState(id string, status core.RunStatus, metadata json.RawMessage, endedAt *string) error {
	if err := writable(d.s, d.inTx); err != nil {
		return err
	}
	if !status.Valid() {
		return core.NewError(core.ReasonInvalidRunStatus, "RunDao.UpdateRunState", "invalid run status "+string(status))
	}
	meta := metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	var res sql.Result
	var err error
	if endedAt != nil {
		res, err = d.q.Exec(`UPDATE runs SET status = ?, metadata_json = ?, ended_at = ? WHERE id = ?`,
			string(status), string(meta), *endedAt, id)
	} else {
		res, err = d.q.Exec(`UPDATE runs SET status = ?, metadata_json = ? WHERE id = ?`,
			string(status), string(meta), id)
	}
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListEventsByRun returns the lifecycle events of one run in append
// order (created_at, then insertion order) — the trace read (G-17).
func (s *Store) ListEventsByRun(runID string) ([]LifecycleEvent, error) {
	rows, err := s.db.Query(`SELECT id, feature_id, run_id, event_type, payload_json, created_at FROM lifecycle_events WHERE run_id = ? ORDER BY created_at, rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LifecycleEvent
	for rows.Next() {
		var e LifecycleEvent
		var payload string
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.RunID, &e.EventType, &payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		if payload != "" {
			e.Payload = json.RawMessage(payload)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
