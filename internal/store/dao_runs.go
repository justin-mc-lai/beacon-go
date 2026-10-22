// RunDao: runs, lifecycle_events (append-only) and checkpoints (AC-071,
// AC-072, AC-075, AC-012).
package store

import (
	"database/sql"
	"encoding/json"

	"beacon-go/internal/core"
)

// Run is a runs row (AC-071).
type Run struct {
	ID        string          `json:"id"`
	FeatureID string          `json:"feature_id"`
	RunKind   string          `json:"run_kind"`
	Status    core.RunStatus  `json:"status"`
	Iteration int             `json:"iteration"`
	StartedAt string          `json:"started_at"`
	EndedAt   sql.NullString  `json:"ended_at,omitempty"`
	Metadata  json.RawMessage `json:"metadata_json,omitempty"`
}

// MarshalJSON renders ended_at as a plain string-or-null instead of
// the sql.NullString object shape (AC-065 stable JSON schema).
func (r Run) MarshalJSON() ([]byte, error) {
	type runJSON struct {
		ID        string          `json:"id"`
		FeatureID string          `json:"feature_id"`
		RunKind   string          `json:"run_kind"`
		Status    core.RunStatus  `json:"status"`
		Iteration int             `json:"iteration"`
		StartedAt string          `json:"started_at"`
		EndedAt   *string         `json:"ended_at"`
		Metadata  json.RawMessage `json:"metadata_json,omitempty"`
	}
	out := runJSON{ID: r.ID, FeatureID: r.FeatureID, RunKind: r.RunKind,
		Status: r.Status, Iteration: r.Iteration, StartedAt: r.StartedAt, Metadata: r.Metadata}
	if r.EndedAt.Valid {
		out.EndedAt = &r.EndedAt.String
	}
	return json.Marshal(out)
}

// LifecycleEvent is a lifecycle_events row (AC-072, append-only AC-012).
type LifecycleEvent struct {
	ID        string          `json:"id"`
	FeatureID string          `json:"feature_id"`
	RunID     sql.NullString  `json:"run_id,omitempty"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload_json,omitempty"`
	CreatedAt string          `json:"created_at"`
}

// Checkpoint is a checkpoints row (AC-075).
type Checkpoint struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Iteration int             `json:"iteration"`
	State     json.RawMessage `json:"state_json,omitempty"`
	CreatedAt string          `json:"created_at"`
}

// RunDao implements run + event + checkpoint access (AC-076).
type RunDao struct {
	s    *Store
	q    dbtx
	inTx bool
}

// Runs returns the RunDao.
func (s *Store) Runs() *RunDao { return &RunDao{s: s, q: s.db} }

// CreateRun inserts a run. Status must be a valid core RunStatus (AC-031);
// the DB CHECK enforces the same set as a second line of defense.
func (d *RunDao) CreateRun(r Run) (Run, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return Run{}, err
	}
	if !r.Status.Valid() {
		return Run{}, core.NewError(core.ReasonInvalidRunStatus, "RunDao.CreateRun", "invalid run status "+string(r.Status))
	}
	if r.ID == "" {
		r.ID = newID("run_")
	}
	r.StartedAt = orNow(r.StartedAt)
	meta := r.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	_, err := d.q.Exec(`INSERT INTO runs(id, feature_id, run_kind, status, iteration, started_at, ended_at, metadata_json) VALUES(?,?,?,?,?,?,?,?)`,
		r.ID, r.FeatureID, r.RunKind, string(r.Status), r.Iteration, r.StartedAt, r.EndedAt, string(meta))
	if err != nil {
		return Run{}, err
	}
	r.Metadata = meta
	return r, nil
}

func scanRun(row interface{ Scan(...any) error }) (Run, error) {
	var r Run
	var status, meta string
	err := row.Scan(&r.ID, &r.FeatureID, &r.RunKind, &status, &r.Iteration, &r.StartedAt, &r.EndedAt, &meta)
	r.Status = core.RunStatus(status)
	if meta != "" {
		r.Metadata = json.RawMessage(meta)
	}
	return r, err
}

const runCols = `id, feature_id, run_kind, status, iteration, started_at, ended_at, metadata_json`

// GetByID returns one run.
func (d *RunDao) GetByID(id string) (Run, error) {
	return scanRun(d.q.QueryRow(`SELECT `+runCols+` FROM runs WHERE id = ?`, id))
}

// UpdateStatus changes a run's status after core validation (AC-031).
// It does not stamp ended_at; callers that close a run pass a terminal
// status and manage timestamps at the orchestrator layer (P2).
func (d *RunDao) UpdateStatus(id string, status core.RunStatus) error {
	if err := writable(d.s, d.inTx); err != nil {
		return err
	}
	if !status.Valid() {
		return core.NewError(core.ReasonInvalidRunStatus, "RunDao.UpdateStatus", "invalid run status "+string(status))
	}
	_, err := d.q.Exec(`UPDATE runs SET status = ? WHERE id = ?`, string(status), id)
	return err
}

// ListByFeature returns runs of a feature, oldest first.
func (d *RunDao) ListByFeature(featureID string) ([]Run, error) {
	rows, err := d.q.Query(`SELECT `+runCols+` FROM runs WHERE feature_id = ? ORDER BY started_at, id`, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AppendEvent appends one lifecycle event (AC-012/072). There is
// deliberately no Update/Delete API for events; DB triggers also reject
// direct UPDATE/DELETE (see schema.go).
func (s *Store) AppendEvent(e LifecycleEvent) (LifecycleEvent, error) {
	return s.appendEvent(s.db, false, e)
}

// appendEvent is the shared body behind Store.AppendEvent and
// Tx.AppendEvent (tx.go): same validation and row shape, executed
// against the caller's statement surface.
func (s *Store) appendEvent(q dbtx, inTx bool, e LifecycleEvent) (LifecycleEvent, error) {
	if err := writable(s, inTx); err != nil {
		return LifecycleEvent{}, err
	}
	if e.ID == "" {
		e.ID = newID("evt_")
	}
	e.CreatedAt = orNow(e.CreatedAt)
	payload := e.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("{}")
	}
	_, err := q.Exec(`INSERT INTO lifecycle_events(id, feature_id, run_id, event_type, payload_json, created_at) VALUES(?,?,?,?,?,?)`,
		e.ID, e.FeatureID, e.RunID, e.EventType, string(payload), e.CreatedAt)
	if err != nil {
		return LifecycleEvent{}, err
	}
	e.Payload = payload
	return e, nil
}

// ListEvents returns events for a feature in append order.
func (s *Store) ListEvents(featureID string) ([]LifecycleEvent, error) {
	rows, err := s.db.Query(`SELECT id, feature_id, run_id, event_type, payload_json, created_at FROM lifecycle_events WHERE feature_id = ? ORDER BY created_at, rowid`, featureID)
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

// SaveCheckpoint persists a checkpoint. Iterations must strictly increase
// per run; a regression is rejected with
// reason_code=checkpoint_iteration_regression (AC-075).
func (d *RunDao) SaveCheckpoint(c Checkpoint) (Checkpoint, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return Checkpoint{}, err
	}
	var maxIter sql.NullInt64
	if err := d.q.QueryRow(`SELECT MAX(iteration) FROM checkpoints WHERE run_id = ?`, c.RunID).Scan(&maxIter); err != nil {
		return Checkpoint{}, err
	}
	if maxIter.Valid && c.Iteration <= int(maxIter.Int64) {
		return Checkpoint{}, core.NewError(ReasonCheckpointRegression, "RunDao.SaveCheckpoint",
			"iteration must strictly increase for a run")
	}
	if c.ID == "" {
		c.ID = newID("ckpt_")
	}
	c.CreatedAt = orNow(c.CreatedAt)
	state := c.State
	if len(state) == 0 {
		state = json.RawMessage("{}")
	}
	_, err := d.q.Exec(`INSERT INTO checkpoints(id, run_id, iteration, state_json, created_at) VALUES(?,?,?,?,?)`,
		c.ID, c.RunID, c.Iteration, string(state), c.CreatedAt)
	if err != nil {
		return Checkpoint{}, err
	}
	c.State = state
	return c, nil
}

// LatestCheckpoint returns the highest-iteration checkpoint of a run.
func (d *RunDao) LatestCheckpoint(runID string) (Checkpoint, error) {
	var c Checkpoint
	var state string
	err := d.q.QueryRow(`SELECT id, run_id, iteration, state_json, created_at FROM checkpoints WHERE run_id = ? ORDER BY iteration DESC LIMIT 1`, runID).
		Scan(&c.ID, &c.RunID, &c.Iteration, &state, &c.CreatedAt)
	if state != "" {
		c.State = json.RawMessage(state)
	}
	return c, err
}

// ListCheckpoints returns checkpoints of a run by ascending iteration.
func (d *RunDao) ListCheckpoints(runID string) ([]Checkpoint, error) {
	rows, err := d.q.Query(`SELECT id, run_id, iteration, state_json, created_at FROM checkpoints WHERE run_id = ? ORDER BY iteration`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var c Checkpoint
		var state string
		if err := rows.Scan(&c.ID, &c.RunID, &c.Iteration, &state, &c.CreatedAt); err != nil {
			return nil, err
		}
		if state != "" {
			c.State = json.RawMessage(state)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
