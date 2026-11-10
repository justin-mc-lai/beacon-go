package effectjournal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tempJournal(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), dir, "journal.jsonl")
}

func runJournal(t *testing.T, path string, runID string, stages []string) *Journal {
	t.Helper()
	j := NewJournal(path)
	for _, stage := range stages {
		if _, err := j.Execute(Effect{
			RunID:          runID,
			Stage:          stage,
			IdempotencyKey: runID + ":" + stage,
			EffectKind:     "write_file",
			Apply:          func() error { return nil },
		}); err != nil {
			t.Fatalf("execute %s: %v", stage, err)
		}
	}
	return j
}

// TC-001 / AC-001: every implement-stage effect writes one journal record
// with a non-empty idempotency key that is unique within the run.
func TestImplementEffectCarriesIdempotencyKeyAndWritesJournal(t *testing.T) {
	path := tempJournal(t, "run")
	j := NewJournal(path)
	runs := 0
	out, err := j.Execute(Effect{
		RunID:          "run-001",
		Stage:          "apply",
		IdempotencyKey: "run-001:apply:a1",
		EffectKind:     "write_file",
		Apply:          func() error { runs++; return nil },
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Executed || out.Status != StatusSucceeded {
		t.Fatalf("outcome = %+v, want an executed success", out)
	}
	if runs != 1 {
		t.Fatalf("apply calls = %d, want 1", runs)
	}
	if _, err := j.Execute(Effect{
		RunID:          "run-001",
		Stage:          "deliver",
		IdempotencyKey: "run-001:deliver:a1",
		EffectKind:     "copy_artifact",
		Apply:          func() error { return nil },
	}); err != nil {
		t.Fatalf("execute second effect: %v", err)
	}

	loaded, err := LoadJournal(path)
	if err != nil {
		t.Fatalf("load journal: %v", err)
	}
	records := loaded.Records()
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	keys := map[string]bool{}
	for _, rec := range records {
		if rec.IdempotencyKey == "" {
			t.Fatalf("record has an empty idempotency key: %+v", rec)
		}
		if keys[rec.IdempotencyKey] {
			t.Fatalf("duplicate idempotency key %q in one run", rec.IdempotencyKey)
		}
		keys[rec.IdempotencyKey] = true
		if rec.Status != StatusSucceeded || rec.Stage == "" || rec.EffectKind == "" {
			t.Fatalf("incomplete success record: %+v", rec)
		}
		if _, err := time.Parse(time.RFC3339, rec.RecordedAt); err != nil {
			t.Fatalf("recorded_at %q is not RFC3339: %v", rec.RecordedAt, err)
		}
	}
}

// TC-002 / AC-001: an effect without an idempotency key is refused
// before it runs and leaves no journal success record.
func TestEffectWithoutIdempotencyKeyIsRefused(t *testing.T) {
	path := tempJournal(t, "run")
	j := NewJournal(path)
	runs := 0
	for _, key := range []string{"", "   "} {
		_, err := j.Execute(Effect{
			RunID:          "run-002",
			Stage:          "apply",
			IdempotencyKey: key,
			EffectKind:     "write_file",
			Apply:          func() error { runs++; return nil },
		})
		if !errors.Is(err, ErrMissingIdempotencyKey) {
			t.Fatalf("key %q error = %v, want ErrMissingIdempotencyKey", key, err)
		}
	}
	if runs != 0 {
		t.Fatalf("effect ran %d times without a key, want 0", runs)
	}
	if len(j.Records()) != 0 {
		t.Fatalf("journal has %d records, want 0", len(j.Records()))
	}
	if _, err := LoadJournal(path); !errors.Is(err, ErrJournalMissing) {
		t.Fatalf("load after refusal error = %v, want ErrJournalMissing", err)
	}
}

// TC-003 / AC-002: replaying a succeeded key returns the recorded
// outcome, marks skipped_replay and never re-runs the effect; resume
// lists only proven stages.
func TestReplaySkipsSucceededEffectAndResumeListsProvenStages(t *testing.T) {
	path := tempJournal(t, "run")
	j := NewJournal(path)
	runs := 0
	effect := Effect{
		RunID:          "run-003",
		Stage:          "apply",
		IdempotencyKey: "run-003:apply:1",
		EffectKind:     "write_file",
		Apply:          func() error { runs++; return nil },
	}
	first, err := j.Execute(effect)
	if err != nil {
		t.Fatalf("first execute: %v", err)
	}
	if !first.Executed || first.Replayed {
		t.Fatalf("first outcome = %+v, want executed and not replayed", first)
	}
	second, err := j.Execute(effect)
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if runs != 1 {
		t.Fatalf("apply calls = %d, want 1", runs)
	}
	if second.Executed || !second.Replayed {
		t.Fatalf("replay outcome = %+v, want replayed only", second)
	}
	if second.Status != StatusSucceeded || second.Stage != first.Stage {
		t.Fatalf("replay outcome = %+v, want recorded status/stage", second)
	}

	records := j.Records()
	if len(records) != 2 || records[1].Status != StatusSkippedReplay {
		t.Fatalf("records = %+v, want a skipped_replay record", records)
	}

	plan, err := Resume(path, "run-003", []string{"apply", "deliver"})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if plan.FailClosed {
		t.Fatalf("resume failed closed: %s", plan.Reason)
	}
	if len(plan.ProvenStages) != 1 || plan.ProvenStages[0] != "apply" {
		t.Fatalf("proven stages = %v, want [apply]", plan.ProvenStages)
	}
	if len(plan.RerunStages) != 1 || plan.RerunStages[0] != "deliver" {
		t.Fatalf("rerun stages = %v, want [deliver]", plan.RerunStages)
	}
}

// TC-004 / AC-002: failed and incomplete records are never proven, and
// replay never fabricates a success for them.
func TestFailedOrIncompleteRecordsAreNotProvenStages(t *testing.T) {
	path := tempJournal(t, "run")
	j := runJournal(t, path, "run-004", []string{"apply"})
	runs := 0
	if _, err := j.Execute(Effect{
		RunID:          "run-004",
		Stage:          "deploy",
		IdempotencyKey: "run-004:deploy:1",
		EffectKind:     "run_command",
		Apply:          func() error { runs++; return errors.New("boom") },
	}); err == nil {
		t.Fatal("failed effect returned no error")
	}
	if runs != 1 {
		t.Fatalf("apply calls = %d, want 1", runs)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	if _, err := f.WriteString(`{"schema_version":1,"run_id":"run-004","stage":"verify","effect_kind":"check","status":"succeeded"}` + "\n"); err != nil {
		t.Fatalf("append incomplete record: %v", err)
	}
	f.Close()

	loaded, err := LoadJournal(path)
	if err != nil {
		t.Fatalf("load journal: %v", err)
	}
	proven := loaded.ProvenStages("run-004")
	if len(proven) != 1 || proven[0] != "apply" {
		t.Fatalf("proven stages = %v, want [apply]", proven)
	}
	plan, err := Resume(path, "run-004", []string{"apply", "deploy", "verify"})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(plan.ProvenStages) != 1 || plan.ProvenStages[0] != "apply" {
		t.Fatalf("resume proven = %v, want [apply]", plan.ProvenStages)
	}
	if len(plan.RerunStages) != 2 || plan.RerunStages[0] != "deploy" || plan.RerunStages[1] != "verify" {
		t.Fatalf("resume rerun = %v, want [deploy verify]", plan.RerunStages)
	}

	before := len(loaded.Records())
	replay, err := loaded.Execute(Effect{
		RunID:          "run-004",
		Stage:          "deploy",
		IdempotencyKey: "run-004:deploy:1",
		EffectKind:     "run_command",
		Apply:          func() error { runs++; return nil },
	})
	if !errors.Is(err, ErrDuplicateIdempotencyKey) {
		t.Fatalf("failed-key replay error = %v, want ErrDuplicateIdempotencyKey", err)
	}
	if runs != 1 {
		t.Fatalf("failed-key replay re-ran the effect: %d calls", runs)
	}
	if replay.Executed || replay.Replayed || replay.Status == StatusSucceeded {
		t.Fatalf("failed-key replay fabricated success: %+v", replay)
	}
	if got := len(loaded.Records()); got != before {
		t.Fatalf("failed-key replay wrote %d records, want %d", got-before, 0)
	}
}

// TC-005 / AC-003: a valid journal resumes from proof without re-running
// proven stages.
func TestResumeWithValidJournalProceedsFromProof(t *testing.T) {
	path := tempJournal(t, "run")
	runJournal(t, path, "run-005", []string{"plan", "apply"})

	plan, err := Resume(path, "run-005", []string{"plan", "apply", "deliver"})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if plan.FailClosed {
		t.Fatalf("resume failed closed: %s", plan.Reason)
	}
	if len(plan.ProvenStages) != 2 || plan.ProvenStages[0] != "plan" || plan.ProvenStages[1] != "apply" {
		t.Fatalf("proven = %v, want plan and apply in stage order", plan.ProvenStages)
	}
	if len(plan.RerunStages) != 1 || plan.RerunStages[0] != "deliver" {
		t.Fatalf("rerun = %v, want [deliver]", plan.RerunStages)
	}

	j, err := LoadJournal(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	runs := 0
	for _, stage := range plan.RerunStages {
		if _, err := j.Execute(Effect{
			RunID:          "run-005",
			Stage:          stage,
			IdempotencyKey: "run-005:" + stage,
			EffectKind:     "write_file",
			Apply:          func() error { runs++; return nil },
		}); err != nil {
			t.Fatalf("execute %s: %v", stage, err)
		}
	}
	if runs != 1 {
		t.Fatalf("resume executed %d stages, want exactly the unproven one", runs)
	}
	if again, err := Resume(path, "run-005", []string{"plan", "apply", "deliver"}); err != nil {
		t.Fatalf("resume after delivery: %v", err)
	} else if len(again.RerunStages) != 0 {
		t.Fatalf("rerun after delivery = %v, want none", again.RerunStages)
	}
}

// TC-006 / AC-003: missing, corrupt and unsupported journals fail closed
// with an explicit reason and zero stages re-run.
func TestResumeWithMissingOrCorruptJournalFailsClosed(t *testing.T) {
	dir := t.TempDir()
	stages := []string{"plan", "apply"}

	missing := filepath.Join(dir, "absent.jsonl")
	plan, err := Resume(missing, "run-006", stages)
	if !errors.Is(err, ErrJournalMissing) {
		t.Fatalf("missing journal error = %v, want ErrJournalMissing", err)
	}
	if !plan.FailClosed || len(plan.RerunStages) != 0 || len(plan.ProvenStages) != 0 || plan.Reason == "" {
		t.Fatalf("missing journal plan = %+v, want fail-closed and no rerun", plan)
	}

	corrupt := filepath.Join(dir, "corrupt.jsonl")
	if err := os.WriteFile(corrupt, []byte("{not json\n"), 0o644); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}
	plan, err = Resume(corrupt, "run-006", stages)
	if !errors.Is(err, ErrJournalCorrupt) {
		t.Fatalf("corrupt journal error = %v, want ErrJournalCorrupt", err)
	}
	if !plan.FailClosed || len(plan.RerunStages) != 0 {
		t.Fatalf("corrupt journal plan = %+v, want fail-closed and no rerun", plan)
	}

	unsupported := filepath.Join(dir, "future.jsonl")
	if err := os.WriteFile(unsupported, []byte(`{"schema_version":99,"run_id":"run-006","stage":"plan","idempotency_key":"k","effect_kind":"x","status":"succeeded","recorded_at":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write future journal: %v", err)
	}
	plan, err = Resume(unsupported, "run-006", stages)
	if !errors.Is(err, ErrSchemaUnsupported) {
		t.Fatalf("unsupported journal error = %v, want ErrSchemaUnsupported", err)
	}
	if !plan.FailClosed || len(plan.RerunStages) != 0 || len(plan.ProvenStages) != 0 {
		t.Fatalf("unsupported journal plan = %+v, want fail-closed and no rerun", plan)
	}
}
