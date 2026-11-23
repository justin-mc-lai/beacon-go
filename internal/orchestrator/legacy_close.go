// Legacy closure (backfill) for features delivered before the strict
// qa_candidate binding existed.
//
// This is deliberately NOT the normal release gate. It never sets a qa
// verdict and never pretends the four complete conditions held. It records
// one explicit, human-confirmed exception: the old independent report, its
// hash, the old candidate hash when one exists, and a whitelisted structural
// reason. The run becomes Succeeded with a legacy_closure event so a later
// real-environment acceptance can find and revisit it.
package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

const (
	ReasonLegacyReportMissing     = "legacy_report_missing"
	ReasonLegacyReportHashDrift   = "legacy_report_hash_drift"
	ReasonLegacyReasonInvalid     = "legacy_reason_invalid"
	ReasonLegacyConfirmIDRequired = "legacy_confirm_id_required"
	ReasonLegacyCandidateInvalid  = "legacy_candidate_invalid"
)

// CompatibleLegacyReasons are the structural gaps this gate may cover. A new
// reason must be added here on purpose; this is not a free-form bypass.
var CompatibleLegacyReasons = map[string]bool{
	"legacy_source_drift":                  true,
	"legacy_report_without_candidate":      true,
	"legacy_contract_review_misclassified": true,
	"legacy_evidence_thin":                 true,
}

var legacySHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// LegacyCloseOptions is the evidence bundle a human signs for one old feature.
type LegacyCloseOptions struct {
	ConfirmedBy     string
	By              string
	ReportPath      string
	ReportSHA256    string
	CandidateSHA256 string
	ReasonCode      string
}

// LegacyClose is the one explicit backfill path. It writes legacy_closure and
// moves the run to Succeeded/Released without a qa verdict.
func (e *Engine) LegacyClose(runID string, opts LegacyCloseOptions) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	reject := func(code, detail string) (RunView, error) {
		_ = e.appendEvent(run, "legacy_close_rejected", map[string]any{"reason_code": code})
		v := viewOf(run, m, slug)
		v.ReasonCode = code
		return v, core.NewError(code, "goal.LegacyClose", detail)
	}
	confirmedBy := strings.TrimSpace(opts.ConfirmedBy)
	if confirmedBy == "" {
		return reject(ReasonLegacyConfirmIDRequired, "legacy closure requires the human identity (--confirmed-by)")
	}
	if !CompatibleLegacyReasons[strings.TrimSpace(opts.ReasonCode)] {
		return reject(ReasonLegacyReasonInvalid, "reason code is not a whitelisted legacy structural gap")
	}
	report := strings.TrimSpace(opts.ReportPath)
	if report == "" {
		return reject(ReasonLegacyReportMissing, "--report is required")
	}
	raw, err := os.ReadFile(report)
	if err != nil {
		return reject(ReasonLegacyReportMissing, "report is unreadable: "+err.Error())
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	want := strings.TrimSpace(opts.ReportSHA256)
	if want == "" || !legacySHA256.MatchString(want) {
		return reject(ReasonLegacyReportHashDrift, "--report-sha256 must be the exact 64-hex report hash")
	}
	if got != want {
		return reject(ReasonLegacyReportHashDrift, "report hash drift: got "+got)
	}
	candidate := strings.TrimSpace(opts.CandidateSHA256)
	if candidate != "" && !legacySHA256.MatchString(candidate) {
		return reject(ReasonLegacyCandidateInvalid, "--candidate-sha256 must be 64 lowercase hex or empty")
	}
	by := strings.TrimSpace(opts.By)
	if by == "" {
		by = confirmedBy
	}

	m.ReleaseHumanConfirmed = true
	m.HumanConfirmedBy = confirmedBy
	m.LoopStage = string(core.LoopDone)
	m.Blocked = false
	m.BlockedStreak = 0
	m.LastBlockReason = ""
	if idx := m.CurrentStageIdx; idx >= 0 && idx < len(m.StagePlan) {
		m.StagePlan[idx].Status = "done"
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "legacy_closure", map[string]any{
			"confirmed_by":       confirmedBy,
			"by":                 by,
			"reason_code":        strings.TrimSpace(opts.ReasonCode),
			"report_path":        report,
			"report_sha256":      got,
			"candidate_sha256":   candidate,
			"closure_kind":       "legacy_backfill",
			"formal_qa_bypassed": true,
			"note":               "recorded before strict qa_candidate binding; revisit in real-environment acceptance",
		}); err != nil {
			return err
		}
		if err := e.saveRunOn(tx, run, m); err != nil {
			return err
		}
		_ = updateLifecycleMonotonic(tx.Features(), store.Lifecycle{
			FeatureID: run.FeatureID, Stage: core.StageRelease, Status: core.StatusCompleted,
		})
		return nil
	}); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

var _ = errors.New
