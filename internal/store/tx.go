// Transaction batching (P6 write-path fix): one SQLite transaction
// per logical write unit instead of one autocommit (one WAL fsync)
// per row. Before this change every DAO write was its own implicit
// transaction, so a single gate evaluation paid 2 fsyncs, one engine
// tick 4, one driver turn ~10 — at ~23ms per commit on the reference
// volume that, not computation, was the write-path latency
// (docs/p6-fix-write-perf.md has the probe numbers).
//
// WithTx opens ONE transaction and hands fn a *Tx whose DAO
// accessors mirror Store's (Projects/Features/Runs/Gates/Artifacts
// + AppendEvent) but execute inside the transaction. The existing
// single-write DAO API is unchanged: DAOs constructed from a Store
// run in autocommit exactly as before, so no other caller had to
// change. Inside a batch, the writable check (read-only + writer
// track, AC-158) is performed once by WithTx instead of per write.
//
// Atomicity: any error from fn rolls the whole batch back — no
// half-written gate verdict without its trail event, no run state
// advanced without its lifecycle snapshot.
package store

import (
	"database/sql"

	"beacon-go/internal/core"
)

// ReasonTransactionNested rejects a WithTx call made while another
// batch is open on the same Store handle. With MaxOpenConns(1) an
// unguarded nested Begin would deadlock waiting for the connection
// the outer transaction holds, so nesting fails fast and typed.
const ReasonTransactionNested = "store_transaction_nested"

// dbtx is the statement surface the DAOs run against: *sql.DB in
// autocommit mode, *sql.Tx inside a WithTx batch.
type dbtx interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// writable is the per-write gate every DAO write passes: skipped
// inside a batch (WithTx already checked once for the whole unit).
func writable(s *Store, inTx bool) error {
	if inTx {
		return nil
	}
	return s.checkWritable()
}

// Tx is one open batch transaction. Its DAO accessors return the
// same DAO types as Store's, bound to the transaction.
type Tx struct {
	s  *Store
	tx *sql.Tx
}

// Projects returns the ProjectDao bound to this transaction.
func (t *Tx) Projects() *ProjectDao { return &ProjectDao{s: t.s, q: t.tx, inTx: true} }

// Features returns the FeatureDao bound to this transaction.
func (t *Tx) Features() *FeatureDao { return &FeatureDao{s: t.s, q: t.tx, inTx: true} }

// Runs returns the RunDao bound to this transaction.
func (t *Tx) Runs() *RunDao { return &RunDao{s: t.s, q: t.tx, inTx: true} }

// Gates returns the GateDao bound to this transaction.
func (t *Tx) Gates() *GateDao { return &GateDao{s: t.s, q: t.tx, inTx: true} }

// Artifacts returns the ArtifactDao bound to this transaction.
func (t *Tx) Artifacts() *ArtifactDao { return &ArtifactDao{s: t.s, q: t.tx, inTx: true} }

// AppendEvent appends one lifecycle event inside this transaction
// (same append-only semantics as Store.AppendEvent, AC-012/072).
func (t *Tx) AppendEvent(e LifecycleEvent) (LifecycleEvent, error) {
	return t.s.appendEvent(t.tx, true, e)
}

// WithTx runs fn inside a single SQLite transaction and commits it
// as one unit (one WAL sync at the configured synchronous level).
// Any error from fn — or a failed commit — leaves none of the
// batch's writes behind. Batches do not nest (see
// ReasonTransactionNested).
//
// Inside fn, callers must use ONLY the Tx's accessors: the store is
// single-connection (MaxOpenConns(1), AC-093), the transaction holds
// that connection, and any Store-level DAO call inside fn would
// block waiting for it. Do all reads and external work (file I/O,
// executor runs) before opening the batch.
func (s *Store) WithTx(fn func(tx *Tx) error) error {
	// The nesting guard runs FIRST, before any statement on s.db:
	// with MaxOpenConns(1) the outer transaction holds the only
	// connection, so even the writable check's SELECT would block
	// forever if it ran before this guard.
	if !s.txOpen.CompareAndSwap(false, true) {
		return core.NewError(ReasonTransactionNested, "store.WithTx",
			"a batch transaction is already open on this store handle; batches do not nest")
	}
	defer s.txOpen.Store(false)
	if err := s.checkWritable(); err != nil {
		return err
	}
	sqlTx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(&Tx{s: s, tx: sqlTx}); err != nil {
		_ = sqlTx.Rollback()
		return err
	}
	return sqlTx.Commit()
}
