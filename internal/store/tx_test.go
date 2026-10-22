package store

// WithTx batch semantics (P6 write-path fix): one transaction per
// logical write unit, all-or-nothing. These tests pin the atomicity
// the orchestrator's merged write paths rely on — a failure mid-batch
// must leave NONE of the batch's rows behind.

import (
	"database/sql"
	"errors"
	"testing"

	"beacon-go/internal/core"
)

var errProbeAbort = errors.New("probe abort")

func TestWithTxCommitsAllWrites(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	err := s.WithTx(func(tx *Tx) error {
		if _, err := tx.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "pass"}); err != nil {
			return err
		}
		if _, err := tx.AppendEvent(LifecycleEvent{FeatureID: f.ID, EventType: "gate_evaluated"}); err != nil {
			return err
		}
		return tx.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusCompleted})
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}
	if _, err := s.Gates().Latest(f.ID, "qa"); err != nil {
		t.Fatalf("gate row missing after commit: %v", err)
	}
	events, err := s.ListEvents(f.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("events after commit = %d, %v want 1", len(events), err)
	}
	lc, err := s.Features().GetLifecycle(f.ID)
	if err != nil || lc.Stage != core.StageQa {
		t.Fatalf("lifecycle after commit = %+v, %v", lc, err)
	}
}

func TestWithTxRollsBackEverythingOnError(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	err := s.WithTx(func(tx *Tx) error {
		if _, err := tx.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "pass"}); err != nil {
			return err
		}
		if _, err := tx.AppendEvent(LifecycleEvent{FeatureID: f.ID, EventType: "gate_evaluated"}); err != nil {
			return err
		}
		if err := tx.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusCompleted}); err != nil {
			return err
		}
		return errProbeAbort // abort AFTER all three writes landed in the tx
	})
	if !errors.Is(err, errProbeAbort) {
		t.Fatalf("WithTx error = %v want the fn error", err)
	}
	if _, err := s.Gates().Latest(f.ID, "qa"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("gate row survived rollback (err=%v)", err)
	}
	events, err := s.ListEvents(f.ID)
	if err != nil || len(events) != 0 {
		t.Fatalf("events survived rollback: %d, %v want 0", len(events), err)
	}
	if _, err := s.Features().GetLifecycle(f.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("lifecycle snapshot survived rollback (err=%v)", err)
	}
}

func TestWithTxRollsBackOnConstraintFailure(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	// Second write violates the runs.status CHECK: the first write
	// (gate row) must be rolled back with it — no half-written unit.
	err := s.WithTx(func(tx *Tx) error {
		if _, err := tx.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "pass"}); err != nil {
			return err
		}
		_, err := tx.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunStatus("bogus_status")})
		return err
	})
	if err == nil {
		t.Fatal("WithTx with invalid run status returned nil error")
	}
	if _, gerr := s.Gates().Latest(f.ID, "qa"); !errors.Is(gerr, sql.ErrNoRows) {
		t.Fatalf("gate row survived constraint rollback (err=%v)", gerr)
	}
}

func TestWithTxNestedRejected(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	var nestedErr error
	err := s.WithTx(func(tx *Tx) error {
		nestedErr = s.WithTx(func(inner *Tx) error { return nil })
		if _, err := tx.AppendEvent(LifecycleEvent{FeatureID: f.ID, EventType: "outer_event"}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("outer WithTx: %v", err)
	}
	if nestedErr == nil {
		t.Fatal("nested WithTx was not rejected")
	}
	if code, ok := core.ReasonCodeOf(nestedErr); !ok || code != ReasonTransactionNested {
		t.Fatalf("nested error reason = %v want %s", nestedErr, ReasonTransactionNested)
	}
	events, err := s.ListEvents(f.ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("outer batch events = %d, %v want 1 (outer still commits)", len(events), err)
	}
}

// The writable DSN pins synchronous=NORMAL (P6 write-path fix) —
// the choice is deliberate, so pin it: if the DSN ever drifts back
// to the FULL default, this test says so loudly. 1 == NORMAL,
// 2 == FULL in SQLite's pragma encoding.
func TestWritableStoreUsesSynchronousNormal(t *testing.T) {
	s, _ := openTestStore(t)
	var v int
	if err := s.db.QueryRow(`PRAGMA synchronous`).Scan(&v); err != nil {
		t.Fatalf("pragma synchronous: %v", err)
	}
	if v != 1 {
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", v)
	}
}

func TestWithTxReadOnlyStoreRejected(t *testing.T) {
	s, path := openTestStore(t)
	ro, err := OpenWithOptions(path, OpenOptions{ReadOnly: true})
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer ro.Close()
	err = ro.WithTx(func(tx *Tx) error { return nil })
	if code, ok := core.ReasonCodeOf(err); !ok || code != ReasonStoreReadOnly {
		t.Fatalf("WithTx on read-only store = %v want %s", err, ReasonStoreReadOnly)
	}
	_ = s
}
