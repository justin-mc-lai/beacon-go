// P2b store support: queue scans over the runs table (G-09 stores
// discovery items as runs rows — no new table, see the p2b report for
// the argument) and a compare-and-swap run state update used for the
// cross-process driver lease (G-06). (GetVersionByID already lives in
// spine.go since P1.)
package store

import (
	"database/sql"
	"encoding/json"

	"beacon-go/internal/core"
)

// ListByKind returns runs of one run_kind in FIFO order (oldest first:
// started_at, then insertion order). The discovery queue (G-09) is the
// runs rows with run_kind="discovery_queue".
func (d *RunDao) ListByKind(kind string) ([]Run, error) {
	rows, err := d.q.Query(`SELECT `+runCols+` FROM runs WHERE run_kind = ? ORDER BY started_at, rowid`, kind)
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

// UpdateRunStateCAS atomically replaces a run's state only when the
// stored metadata_json still equals expectMetadata (compare-and-swap
// on the snapshot blob). Returns true when the update landed. This is
// the primitive that makes the driver lease (G-06) safe across
// processes: two starters racing the same run cannot both swap.
func (d *RunDao) UpdateRunStateCAS(id string, expectMetadata json.RawMessage, status core.RunStatus, metadata json.RawMessage, endedAt *string) (bool, error) {
	if err := writable(d.s, d.inTx); err != nil {
		return false, err
	}
	if !status.Valid() {
		return false, core.NewError(core.ReasonInvalidRunStatus, "RunDao.UpdateRunStateCAS", "invalid run status "+string(status))
	}
	meta := metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	expect := expectMetadata
	if len(expect) == 0 {
		expect = json.RawMessage("{}")
	}
	var res sql.Result
	var err error
	if endedAt != nil {
		res, err = d.q.Exec(`UPDATE runs SET status = ?, metadata_json = ?, ended_at = ? WHERE id = ? AND metadata_json = ?`,
			string(status), string(meta), *endedAt, id, string(expect))
	} else {
		res, err = d.q.Exec(`UPDATE runs SET status = ?, metadata_json = ? WHERE id = ? AND metadata_json = ?`,
			string(status), string(meta), id, string(expect))
	}
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}
