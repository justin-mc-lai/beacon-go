// Package progressreview carries the progress review receipt contract
// (kernel-progress-drift): a fixed-schema fact about whether a periodic
// self-review saw the run drift away from its plan.
//
// A receipt is evidence, not a completion claim. It never advances a
// run on its own. Only consecutive completed receipts that report drift
// raise the existing replan obligation, and only in assist mode; shadow
// mode records receipts without driving a decision, and off mode (the
// default) produces no drivable receipt at all.
package progressreview

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SchemaVersion is the fixed receipt schema identifier.
const SchemaVersion = "progress_review_receipt_v0"

// DefaultDriftThreshold is the default number of consecutive completed
// drift receipts that raises the replan obligation.
const DefaultDriftThreshold = 2

// Reason codes surfaced by this package.
const (
	// ReasonRevisionUnpinned marks an assist decision that cannot count
	// because the goal has no pinned contract revision.
	ReasonRevisionUnpinned = "contract_revision_unpinned"
	// ReasonRevisionMismatch marks a receipt bound to a revision other
	// than the pinned one.
	ReasonRevisionMismatch = "contract_revision_mismatch"
	// ReasonDuplicateEvent marks a receipt whose event was already
	// recorded; a duplicate never counts twice.
	ReasonDuplicateEvent = "progress_review_duplicate_event"
)

// RebindHintNewerUnderUnpinned is the summary hint raised when receipts
// were produced under a revision that is not the pinned one.
const RebindHintNewerUnderUnpinned = "newer_receipts_under_unpinned_revision"

// Mode is the progress review mode vocabulary.
type Mode string

const (
	// ModeOff is the default: no drivable receipt is produced.
	ModeOff Mode = "off"
	// ModeShadow records receipts without driving a decision.
	ModeShadow Mode = "shadow"
	// ModeAssist lets qualifying receipts raise the replan obligation.
	ModeAssist Mode = "assist"
)

// DefaultMode is the mode used when none is configured.
const DefaultMode = ModeOff

// Modes returns the mode vocabulary in contract order.
func Modes() []Mode { return []Mode{ModeOff, ModeShadow, ModeAssist} }

// ParseMode maps a configured word onto the vocabulary. An empty word
// means the default; anything else outside the vocabulary is rejected.
func ParseMode(raw string) (Mode, error) {
	word := strings.ToLower(strings.TrimSpace(raw))
	if word == "" {
		return DefaultMode, nil
	}
	for _, m := range Modes() {
		if string(m) == word {
			return m, nil
		}
	}
	return "", fmt.Errorf("progress review: unknown mode %q", raw)
}

// Status is one evaluation's outcome word.
type Status string

const (
	StatusCompleted    Status = "completed"
	StatusAbstained    Status = "abstained"
	StatusFailed       Status = "failed"
	StatusNotEvaluated Status = "not_evaluated"
	StatusStale        Status = "stale"
	// StatusUnknown covers the literal "unknown" and every unrecognized
	// value; it is never treated as handled.
	StatusUnknown Status = "unknown"
)

// Statuses returns the frozen status vocabulary.
func Statuses() []Status {
	return []Status{StatusCompleted, StatusAbstained, StatusFailed, StatusNotEvaluated, StatusStale}
}

// NormalizeStatus maps a raw word onto the vocabulary. Anything outside
// the frozen five words becomes StatusUnknown.
func NormalizeStatus(raw string) Status {
	word := strings.ToLower(strings.TrimSpace(raw))
	for _, s := range Statuses() {
		if string(s) == word {
			return s
		}
	}
	return StatusUnknown
}

// Handled reports whether a status counts as a handled evaluation. Only
// completed does; unknown, abstained, failed, not_evaluated and stale
// are never handled.
func (s Status) Handled() bool { return s == StatusCompleted }

// Run is the run snapshot fact a receipt is about.
type Run struct {
	RunID  string `json:"run_id"`
	Stage  string `json:"stage,omitempty"`
	GoalID string `json:"goal_id,omitempty"`
}

// Judgment is one item of the review's per-question judging.
type Judgment struct {
	QuestionID string `json:"question_id"`
	Verdict    string `json:"verdict"`
	Detail     string `json:"detail,omitempty"`
}

// Receipt is one progress review fact in the frozen v0 schema.
type Receipt struct {
	SchemaVersion    string     `json:"schema_version"`
	EventID          string     `json:"event_id"`
	EvidenceID       string     `json:"evidence_id"`
	Status           Status     `json:"status"`
	Run              Run        `json:"run"`
	Judgments        []Judgment `json:"judgments"`
	DriftSignal      bool       `json:"drift_signal"`
	QuestionVersion  string     `json:"question_version"`
	ContractRevision string     `json:"contract_revision"`
}

// Validate checks the structural contract of a receipt and normalizes
// its status word. It does not decide whether the receipt counts; that
// is the reviewer's job.
func (r *Receipt) Validate() error {
	if strings.TrimSpace(r.SchemaVersion) != SchemaVersion {
		return fmt.Errorf("progress review: schema_version must be %q", SchemaVersion)
	}
	if strings.TrimSpace(r.EventID) == "" {
		return fmt.Errorf("progress review: event_id is required")
	}
	if strings.TrimSpace(r.EvidenceID) == "" {
		return fmt.Errorf("progress review: evidence_id is required")
	}
	if strings.TrimSpace(r.QuestionVersion) == "" {
		return fmt.Errorf("progress review: question_version is required")
	}
	if strings.TrimSpace(r.Run.RunID) == "" {
		return fmt.Errorf("progress review: run.run_id is required")
	}
	if r.Judgments == nil {
		r.Judgments = []Judgment{}
	}
	if r.ContractRevision != "" {
		rev, err := NormalizeContractRevision(r.ContractRevision)
		if err != nil {
			return err
		}
		r.ContractRevision = rev
	}
	r.Status = NormalizeStatus(string(r.Status))
	return nil
}

// MarshalJSON keeps the normalized status in the encoded form.
func (r Receipt) MarshalJSON() ([]byte, error) {
	type plain Receipt
	if r.Status == "" {
		r.Status = StatusUnknown
	}
	return json.Marshal(plain(r))
}

// ObligationKind is the kind of decision a raised obligation feeds. The
// kernel already has a re-plan path; this record is its input, not a
// second plan engine.
const ObligationKind = "replan"

// ReplanObligation is the existing replan obligation raised by a streak
// of completed drift receipts.
type ReplanObligation struct {
	Kind             string   `json:"kind"`
	Reason           string   `json:"reason"`
	Consecutive      int      `json:"consecutive"`
	Threshold        int      `json:"threshold"`
	ContractRevision string   `json:"contract_revision"`
	RunID            string   `json:"run_id"`
	EvidenceIDs      []string `json:"evidence_ids"`
}

// Decision is what one Review call decided about one receipt.
type Decision struct {
	SchemaVersion       string            `json:"schema_version"`
	Mode                Mode              `json:"mode"`
	Recorded            bool              `json:"recorded"`
	Drivable            bool              `json:"drivable"`
	Counted             bool              `json:"counted"`
	Handled             bool              `json:"handled"`
	Status              Status            `json:"status"`
	Consecutive         int               `json:"consecutive"`
	Threshold           int               `json:"threshold"`
	ContractRevision    string            `json:"contract_revision,omitempty"`
	AssistBlockedReason string            `json:"assist_blocked_reason,omitempty"`
	RebindHint          string            `json:"rebind_hint,omitempty"`
	Obligation          *ReplanObligation `json:"obligation,omitempty"`
}

// SummaryKeyRebindHint is the summary key named by the frozen contract.
const SummaryKeyRebindHint = "rebind_hint"

// Options configures a Reviewer.
type Options struct {
	// Mode defaults to off.
	Mode Mode
	// Threshold defaults to DefaultDriftThreshold when not positive.
	Threshold int
	// PinnedRevision is the goal's pinned contract revision. Empty means
	// the goal has not pinned one yet; counting stays blocked.
	PinnedRevision string
}

// Reviewer applies the receipt policy to a stream of receipts.
type Reviewer struct {
	mode          Mode
	threshold     int
	pinned        string
	receipts      []Receipt
	seenEvents    map[string]struct{}
	streak        int
	driftEvidence []string
	obligation    *ReplanObligation
	rebindHint    string
	lastBlocked   string
}

// New builds a Reviewer.
func New(opts Options) (*Reviewer, error) {
	mode, err := ParseMode(string(opts.Mode))
	if err != nil {
		return nil, err
	}
	threshold := opts.Threshold
	if threshold < 0 {
		return nil, fmt.Errorf("progress review: threshold must not be negative")
	}
	if threshold == 0 {
		threshold = DefaultDriftThreshold
	}
	pinned := ""
	if strings.TrimSpace(opts.PinnedRevision) != "" {
		pinned, err = NormalizeContractRevision(opts.PinnedRevision)
		if err != nil {
			return nil, err
		}
	}
	return &Reviewer{
		mode:       mode,
		threshold:  threshold,
		pinned:     pinned,
		seenEvents: map[string]struct{}{},
	}, nil
}

// Mode reports the effective mode.
func (r *Reviewer) Mode() Mode { return r.mode }

// Threshold reports the effective consecutive-drift threshold.
func (r *Reviewer) Threshold() int { return r.threshold }

// PinnedRevision reports the normalized pinned contract revision.
func (r *Reviewer) PinnedRevision() string { return r.pinned }

// Receipts returns a copy of the recorded receipts in arrival order.
func (r *Reviewer) Receipts() []Receipt {
	out := make([]Receipt, len(r.receipts))
	copy(out, r.receipts)
	return out
}

// ConsecutiveDrift reports the current completed-drift streak.
func (r *Reviewer) ConsecutiveDrift() int { return r.streak }

// Obligation returns the outstanding replan obligation, if any.
func (r *Reviewer) Obligation() *ReplanObligation {
	if r.obligation == nil {
		return nil
	}
	cp := *r.obligation
	cp.EvidenceIDs = append([]string(nil), r.obligation.EvidenceIDs...)
	return &cp
}

// Summary is the contract read model, including rebind_hint when a
// receipt arrived under a revision that is not the pinned one.
func (r *Reviewer) Summary() map[string]any {
	out := map[string]any{
		"mode":        string(r.mode),
		"threshold":   r.threshold,
		"consecutive": r.streak,
		"receipts":    len(r.receipts),
		"pinned":      r.pinned != "",
	}
	if r.rebindHint != "" {
		out[SummaryKeyRebindHint] = r.rebindHint
	}
	return out
}

// Review validates and applies one receipt.
func (r *Reviewer) Review(receipt Receipt) (Decision, error) {
	if err := receipt.Validate(); err != nil {
		return Decision{}, err
	}
	decision := Decision{
		SchemaVersion:    SchemaVersion,
		Mode:             r.mode,
		Status:           receipt.Status,
		Threshold:        r.threshold,
		ContractRevision: r.pinned,
	}

	// Off mode produces no drivable receipt at all.
	if r.mode == ModeOff {
		return decision, nil
	}

	// A duplicate event never counts twice, and is not recorded again.
	if _, dup := r.seenEvents[receipt.EventID]; dup {
		r.streak = 0
		r.driftEvidence = nil
		decision.Consecutive = r.streak
		if r.mode == ModeAssist {
			r.lastBlocked = ReasonDuplicateEvent
			decision.Drivable = true
			decision.AssistBlockedReason = ReasonDuplicateEvent
		}
		return decision, nil
	}
	r.seenEvents[receipt.EventID] = struct{}{}
	decision.Recorded = true
	r.receipts = append(r.receipts, receipt)

	// Bind the receipt to the pinned goal revision before counting. A
	// foreign or unpinned receipt breaks the streak instead of being
	// counted under the wrong contract.
	binding := ""
	switch {
	case r.pinned == "":
		binding = ReasonRevisionUnpinned
	case receipt.ContractRevision == "":
		binding = ReasonRevisionUnpinned
	case receipt.ContractRevision != r.pinned:
		binding = ReasonRevisionMismatch
		r.rebindHint = RebindHintNewerUnderUnpinned
	}
	if binding != "" {
		r.streak = 0
		r.driftEvidence = nil
	}
	decision.RebindHint = r.rebindHint
	decision.Consecutive = r.streak

	// Shadow records the fact and stops: it never drives a decision.
	if r.mode == ModeShadow {
		decision.Handled = receipt.Status.Handled()
		return decision, nil
	}

	decision.Drivable = true
	if binding != "" {
		r.lastBlocked = binding
		decision.AssistBlockedReason = binding
		return decision, nil
	}

	// Only completed receipts are handled; every other status breaks the
	// consecutive streak and is never counted as handled.
	if !receipt.Status.Handled() {
		r.streak = 0
		r.driftEvidence = nil
		decision.Consecutive = r.streak
		return decision, nil
	}
	decision.Handled = true

	if !receipt.DriftSignal {
		r.streak = 0
		r.driftEvidence = nil
		decision.Consecutive = r.streak
		return decision, nil
	}

	r.streak++
	r.driftEvidence = append(r.driftEvidence, receipt.EvidenceID)
	decision.Counted = true
	decision.Consecutive = r.streak
	if r.streak >= r.threshold {
		r.obligation = &ReplanObligation{
			Kind:             ObligationKind,
			Reason:           "progress_drift",
			Consecutive:      r.streak,
			Threshold:        r.threshold,
			ContractRevision: r.pinned,
			RunID:            receipt.Run.RunID,
			EvidenceIDs:      append([]string(nil), r.driftEvidence...),
		}
		decision.Obligation = r.Obligation()
	}
	return decision, nil
}
