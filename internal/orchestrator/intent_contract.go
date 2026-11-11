// Intent contract bridge (AC-001/002/003): the aligned briefing is a
// package file plus a run event, and the kernel refuses to auto-drive
// a run whose package carries no confirmed intent.
//
// intent.json is a machine-readable copy of the intent: truth.md stays
// the single source of requirement truth. Nothing here weakens truth
// freeze or the release human gate.
package orchestrator

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

const (
	// IntentDocSchema is the schema tag of the recorded intent document.
	IntentDocSchema = "beacon-intent-v1"
	// IntentFileName is the package-sibling file the intent lands in.
	IntentFileName = "intent.json"
	// EventIntentRecorded is the run event appended after the file lands.
	EventIntentRecorded = "intent_recorded"
	// EventGoalResumed is the run event appended on every Resume call.
	EventGoalResumed = "goal_resumed"
)

// IntentDoc is the recorded intent: package metadata plus the briefing
// AlignIntentStream returned, preserved as-is.
type IntentDoc struct {
	Schema      string   `json:"schema"`
	FeatureSlug string   `json:"feature_slug"`
	Version     string   `json:"version"`
	TruthPath   string   `json:"truth_path"`
	RecordedAt  string   `json:"recorded_at"`
	RecordedBy  string   `json:"recorded_by"`
	Confirmed   bool     `json:"confirmed"`
	Briefing    Briefing `json:"briefing"`
}

// IntentRecordResult is the command's report: the file that was written
// and the run the event went to (empty when the feature has no run).
type IntentRecordResult struct {
	Schema        string `json:"schema"`
	FeatureSlug   string `json:"feature_slug"`
	Version       string `json:"version"`
	RunID         string `json:"run_id"`
	IntentPath    string `json:"intent_path"`
	IntentSHA256  string `json:"intent_sha256"`
	Confirmed     bool   `json:"confirmed"`
	RecordedAt    string `json:"recorded_at"`
	EventAppended bool   `json:"event_appended"`
}

// intentRelPath resolves intent.json next to truth.md. Deriving it from
// the package's own truth path keeps the file resolvable after a
// package (or a whole docs version) moves.
func intentRelPath(featureTruthPath string) string {
	truth := strings.TrimSpace(featureTruthPath)
	if truth == "" {
		return IntentFileName
	}
	return filepath.ToSlash(filepath.Join(filepath.Dir(truth), IntentFileName))
}

// readConfirmedIntent reports whether the package holds an intent the
// kernel accepts: parseable, confirmed, with a non-empty restatement.
// Any unreadable or malformed file is simply "no intent".
func (e *Engine) readConfirmedIntent(ctx store.FeatureContext) bool {
	abs := resolvePkgPath(ctx.Project.RootPath, intentRelPath(ctx.Feature.TruthPath))
	raw, err := os.ReadFile(abs)
	if err != nil {
		return false
	}
	var doc struct {
		Confirmed bool `json:"confirmed"`
		Briefing  struct {
			RestatedIntent string `json:"restated_intent"`
		} `json:"briefing"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	return doc.Confirmed && strings.TrimSpace(doc.Briefing.RestatedIntent) != ""
}

// IsKernelDrivenExecutor reports whether the kernel drives the run
// itself: recording, subprocess, subprocess:<profile>, ralph, or any
// run that carries an Exec command configuration. manual / external are
// advanced by an explicit --by tick, so they are not in this set.
func IsKernelDrivenExecutor(mode string, exec *ExecutorConfig) bool {
	switch mode {
	case ExecutorModeRecording, ExecutorModeSubprocess, ExecutorModeRalph:
		return true
	}
	if strings.HasPrefix(mode, ExecutorModeSubprocess+":") {
		return true
	}
	return exec != nil
}

// RecordIntent writes the aligned briefing into <pkg>/intent.json
// (temp file + rename, so a reader never sees a half-written doc) and,
// when the feature has a run, appends the intent_recorded event.
func (e *Engine) RecordIntent(ctx store.FeatureContext, briefingPath, runID, by string) (IntentRecordResult, error) {
	by = strings.TrimSpace(by)
	if by == "" {
		by = "cli"
	}
	if strings.TrimSpace(briefingPath) == "" {
		return IntentRecordResult{}, fmt.Errorf("truth intent record: --briefing is required")
	}
	briefAbs := briefingPath
	if !filepath.IsAbs(briefAbs) {
		briefAbs = filepath.Join(ctx.Project.RootPath, briefAbs)
	}
	briefRaw, err := os.ReadFile(briefAbs)
	if err != nil {
		return IntentRecordResult{}, fmt.Errorf("truth intent record: read briefing %s: %w", briefingPath, err)
	}
	var brief Briefing
	if err := json.Unmarshal(briefRaw, &brief); err != nil {
		return IntentRecordResult{}, fmt.Errorf("truth intent record: briefing %s is not valid JSON: %w", briefingPath, err)
	}

	recordedAt := time.Now().UTC().Format(time.RFC3339)
	doc := IntentDoc{
		Schema:      IntentDocSchema,
		FeatureSlug: ctx.Feature.Slug,
		Version:     ctx.Version.Version,
		TruthPath:   filepath.ToSlash(ctx.Feature.TruthPath),
		RecordedAt:  recordedAt,
		RecordedBy:  by,
		Confirmed:   brief.Confirmed,
		Briefing:    brief,
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return IntentRecordResult{}, err
	}
	raw = append(raw, '\n')

	rel := intentRelPath(ctx.Feature.TruthPath)
	// Resolve the target run before touching the package: a bad
	// --run-id must refuse without writing a file the caller cannot
	// attach to a run.
	target, err := e.intentTargetRun(ctx.Feature.ID, runID)
	if err != nil {
		return IntentRecordResult{}, err
	}
	abs := resolvePkgPath(ctx.Project.RootPath, rel)
	if err := writeFileAtomic(abs, raw, 0o644); err != nil {
		return IntentRecordResult{}, err
	}

	intentSum := sha256.Sum256(raw)
	briefSum := sha256.Sum256(briefRaw)
	res := IntentRecordResult{
		Schema: IntentDocSchema, FeatureSlug: ctx.Feature.Slug, Version: ctx.Version.Version,
		RunID: "", IntentPath: rel, IntentSHA256: hex.EncodeToString(intentSum[:]),
		Confirmed: brief.Confirmed, RecordedAt: recordedAt,
	}
	if target == nil {
		return res, nil
	}
	if err := e.appendEvent(*target, EventIntentRecorded, map[string]any{
		"intent_path":     rel,
		"intent_sha256":   res.IntentSHA256,
		"briefing_sha256": hex.EncodeToString(briefSum[:]),
		"restated_intent": brief.RestatedIntent,
		"risks":           brief.IdentifiedRisks,
		"boundaries":      brief.Boundaries,
		"proposed_slugs":  brief.ProposedSlugs,
		"confirmed":       brief.Confirmed,
		"recorded_by":     by,
	}); err != nil {
		return res, err
	}
	res.RunID = target.ID
	res.EventAppended = true
	return res, nil
}

// intentTargetRun picks the run the event lands on: an explicit
// --run-id wins (and must belong to the feature); otherwise the most
// recently created run of the feature; otherwise none.
func (e *Engine) intentTargetRun(featureID, runID string) (*store.Run, error) {
	if strings.TrimSpace(runID) != "" {
		run, err := e.store.Runs().GetByID(runID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, core.NewError(ReasonRunNotFound, "truth.IntentRecord", "no such goal run: "+runID)
			}
			return nil, err
		}
		if run.FeatureID != featureID {
			return nil, core.NewError(ReasonRunNotFound, "truth.IntentRecord",
				"run "+runID+" belongs to another feature; pass a run of the target package")
		}
		return &run, nil
	}
	runs, err := e.store.Runs().ListByFeature(featureID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[len(runs)-1], nil
}

// writeFileAtomic writes data to a temp file in the destination
// directory, then renames it into place.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".intent-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
