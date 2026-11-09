// Package effectjournal carries the implement-stage effect ledger
// contract (kernel-effect-journal): every side effect carries a
// non-empty idempotency key and appends one journal record; replay
// deduplicates on (run_id, idempotency_key); resume lists only proven
// stages; a missing, unparseable or unsupported journal fails closed
// instead of silently re-running.
package effectjournal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// SchemaVersion is the only journal schema this kernel understands.
const SchemaVersion = 1

// Status is the recorded outcome of one effect.
type Status string

const (
	// StatusSucceeded records a completed side effect.
	StatusSucceeded Status = "succeeded"
	// StatusFailed records an attempted side effect that failed.
	StatusFailed Status = "failed"
	// StatusSkippedReplay records a replay that reused a succeeded effect.
	StatusSkippedReplay Status = "skipped_replay"
)

// Reason codes surfaced by this package.
const (
	ReasonMissingKey    = "effect_missing_idempotency_key"
	ReasonDuplicateKey  = "effect_duplicate_idempotency_key"
	ReasonJournalAbsent = "effect_journal_missing"
	ReasonJournalBroken = "effect_journal_corrupt"
	ReasonSchemaVersion = "effect_journal_schema_unsupported"
	ReasonRecordBroken  = "effect_journal_record_incomplete"
	ReasonInvalidStatus = "effect_status_invalid"
)

var (
	// ErrMissingIdempotencyKey refuses an effect without a key.
	ErrMissingIdempotencyKey = errors.New(ReasonMissingKey)
	// ErrDuplicateIdempotencyKey refuses a key already recorded in the run.
	ErrDuplicateIdempotencyKey = errors.New(ReasonDuplicateKey)
	// ErrJournalMissing refuses resume without a journal file.
	ErrJournalMissing = errors.New(ReasonJournalAbsent)
	// ErrJournalCorrupt refuses an unparseable journal.
	ErrJournalCorrupt = errors.New(ReasonJournalBroken)
	// ErrSchemaUnsupported refuses an unknown journal schema version.
	ErrSchemaUnsupported = errors.New(ReasonSchemaVersion)
	// ErrIncompleteRecord refuses to append or trust a partial record.
	ErrIncompleteRecord = errors.New(ReasonRecordBroken)
)

// Valid reports whether s is in the status vocabulary.
func (s Status) Valid() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusSkippedReplay:
		return true
	default:
		return false
	}
}

// Record is one append-only journal entry. The field set is fixed by
// the frozen contract; completeness is checked before a record can be
// appended or counted as proof.
type Record struct {
	SchemaVersion  int    `json:"schema_version"`
	RunID          string `json:"run_id"`
	Stage          string `json:"stage"`
	IdempotencyKey string `json:"idempotency_key"`
	EffectKind     string `json:"effect_kind"`
	Status         Status `json:"status"`
	RecordedAt     string `json:"recorded_at"`
}

// Complete reports whether the record carries every required field in
// a usable shape at the current schema version.
func (r Record) Complete() bool {
	if r.SchemaVersion != SchemaVersion {
		return false
	}
	if strings.TrimSpace(r.RunID) == "" || strings.TrimSpace(r.Stage) == "" {
		return false
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" || strings.TrimSpace(r.EffectKind) == "" {
		return false
	}
	if !r.Status.Valid() {
		return false
	}
	_, err := time.Parse(time.RFC3339, r.RecordedAt)
	return err == nil
}

// Journal is an append-only JSONL effect journal for one path.
type Journal struct {
	path    string
	mu      sync.Mutex
	records []Record
}

// NewJournal returns an empty in-memory journal that appends to path.
func NewJournal(path string) *Journal {
	return &Journal{path: path}
}

// Path returns the backing journal path.
func (j *Journal) Path() string { return j.path }

// Records returns a copy of the loaded records.
func (j *Journal) Records() []Record {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Record, len(j.records))
	copy(out, j.records)
	return out
}

// LoadJournal reads a journal from disk. A missing file, an unparseable
// line and an unknown schema version are distinct fail-closed errors.
func LoadJournal(path string) (*Journal, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrJournalMissing, path)
		}
		return nil, fmt.Errorf("%w: %v", ErrJournalCorrupt, err)
	}
	defer f.Close()

	j := &Journal{path: path}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var rec Record
		if err := json.Unmarshal([]byte(raw), &rec); err != nil {
			return nil, fmt.Errorf("%w: line %d: %v", ErrJournalCorrupt, line, err)
		}
		if rec.SchemaVersion != SchemaVersion {
			return nil, fmt.Errorf("%w: line %d: schema_version %d, supported %d", ErrSchemaUnsupported, line, rec.SchemaVersion, SchemaVersion)
		}
		j.records = append(j.records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJournalCorrupt, err)
	}
	return j, nil
}

func (j *Journal) appendLocked(rec Record) error {
	if !rec.Complete() {
		return fmt.Errorf("%w: %+v", ErrIncompleteRecord, rec)
	}
	if dir := filepath.Dir(j.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create journal dir: %w", err)
		}
	}
	f, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open journal: %w", err)
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		f.Close()
		return fmt.Errorf("encode journal record: %w", err)
	}
	if _, err := f.Write(append(payload, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("append journal record: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close journal: %w", err)
	}
	j.records = append(j.records, rec)
	return nil
}

// Append writes one record. Incomplete records are refused and never
// reach disk.
func (j *Journal) Append(rec Record) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.appendLocked(rec)
}

// Effect is one side effect to run under an idempotency key.
type Effect struct {
	RunID          string
	Stage          string
	IdempotencyKey string
	EffectKind     string
	Apply          func() error
}

// Outcome reports what happened: a fresh execution, a replay of a
// succeeded effect, or nothing when the request was refused.
type Outcome struct {
	RunID          string
	Stage          string
	IdempotencyKey string
	EffectKind     string
	Status         Status
	Executed       bool
	Replayed       bool
}

// Execute runs an effect at most once per (run_id, idempotency_key).
// A succeeded record replays without calling Apply; a failed record is
// not turned into success, and a key already present is refused rather
// than silently re-executed.
func (j *Journal) Execute(e Effect) (Outcome, error) {
	out := Outcome{
		RunID:          e.RunID,
		Stage:          e.Stage,
		IdempotencyKey: e.IdempotencyKey,
		EffectKind:     e.EffectKind,
	}
	if strings.TrimSpace(e.IdempotencyKey) == "" {
		return out, ErrMissingIdempotencyKey
	}
	if strings.TrimSpace(e.RunID) == "" {
		return out, fmt.Errorf("%w: run_id is empty", ErrIncompleteRecord)
	}
	if e.Apply == nil {
		return out, fmt.Errorf("%w: nil apply function", ErrIncompleteRecord)
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	for _, rec := range j.records {
		if rec.RunID != e.RunID || rec.IdempotencyKey != e.IdempotencyKey {
			continue
		}
		if rec.Status == StatusSucceeded && rec.Complete() {
			replay := j.newRecord(e, StatusSkippedReplay)
			replay.Stage = rec.Stage
			if err := j.appendLocked(replay); err != nil {
				return out, err
			}
			out.Stage = rec.Stage
			out.Status = rec.Status
			out.Executed = false
			out.Replayed = true
			return out, nil
		}
		return out, fmt.Errorf("%w: run %s key %s already recorded as %s", ErrDuplicateIdempotencyKey, e.RunID, e.IdempotencyKey, rec.Status)
	}

	err := e.Apply()
	status := StatusSucceeded
	if err != nil {
		status = StatusFailed
	}
	if recordErr := j.appendLocked(j.newRecord(e, status)); recordErr != nil {
		if err != nil {
			return out, fmt.Errorf("effect failed (%v) and journal append failed: %w", err, recordErr)
		}
		return out, recordErr
	}
	out.Status = status
	if err != nil {
		return out, fmt.Errorf("effect %s failed: %w", e.Stage, err)
	}
	out.Executed = true
	return out, nil
}

func (j *Journal) newRecord(e Effect, status Status) Record {
	return Record{
		SchemaVersion:  SchemaVersion,
		RunID:          e.RunID,
		Stage:          e.Stage,
		IdempotencyKey: e.IdempotencyKey,
		EffectKind:     e.EffectKind,
		Status:         status,
		RecordedAt:     time.Now().UTC().Format(time.RFC3339),
	}
}

// ProvenStages lists the stages of one run that the journal proves:
// a complete succeeded record. Failed, replay-only and incomplete
// records never count.
func (j *Journal) ProvenStages(runID string) []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return provenStagesLocked(j.records, runID)
}

func provenStagesLocked(records []Record, runID string) []string {
	seen := map[string]bool{}
	var out []string
	for _, rec := range records {
		if rec.RunID != runID || rec.Status != StatusSucceeded || !rec.Complete() {
			continue
		}
		if seen[rec.Stage] {
			continue
		}
		seen[rec.Stage] = true
		out = append(out, rec.Stage)
	}
	sort.Strings(out)
	return out
}

// ResumePlan is what resume may do next. FailClosed means no stage may
// run: the caller must repair or discard the journal.
type ResumePlan struct {
	RunID        string
	ProvenStages []string
	RerunStages  []string
	FailClosed   bool
	Reason       string
}

// Resume reads the journal and splits the requested stages into proven
// and rerun. A missing, unparseable or unsupported journal fails closed
// with an explicit reason and an empty rerun list.
func Resume(path, runID string, stages []string) (ResumePlan, error) {
	plan := ResumePlan{RunID: runID, ProvenStages: []string{}, RerunStages: []string{}}
	j, err := LoadJournal(path)
	if err != nil {
		plan.FailClosed = true
		plan.Reason = err.Error()
		return plan, err
	}
	proven := map[string]bool{}
	for _, stage := range provenStagesLocked(j.Records(), runID) {
		proven[stage] = true
	}
	for _, stage := range stages {
		if proven[stage] {
			plan.ProvenStages = append(plan.ProvenStages, stage)
			continue
		}
		plan.RerunStages = append(plan.RerunStages, stage)
	}
	return plan, nil
}
