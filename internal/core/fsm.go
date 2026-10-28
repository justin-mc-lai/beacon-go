// FSM validation and transition rules (AC-026..031).
//
// DERIVED (推导): the frozen truth.md enumerates the WorkflowStage set and
// mandates an explicit transition table (AC-027) but does not enumerate the
// allowed (from,to) pairs one by one. The table below is derived from the
// pipeline order stated in the truth (Think→UserStory→Prd→TestCase→
// Prototype→Freeze→Implement→Qa→Release→Archive): only a forward move to
// the immediate next stage is legal. This derivation is recorded in
// docs/p0-implementation.md.
package core

// AllWorkflowStages returns the stages in pipeline order.
func AllWorkflowStages() []WorkflowStage {
	return []WorkflowStage{
		StageThink, StageUserStory, StagePrd, StageTestCase, StagePrototype,
		StageFreeze, StageImplement, StageQa, StageRelease, StageArchive,
	}
}

// AllStageStatuses returns every legal StageStatus.
func AllStageStatuses() []StageStatus {
	return []StageStatus{
		StatusNotStarted, StatusInProgress, StatusBlocked, StatusCompleted,
		StatusFailed, StatusSkipped, StatusCrashed, StatusTimedOut,
		StatusInterrupted, StatusRepairing,
	}
}

// AllRunStatuses returns the 12 delivery run statuses in contract order.
// The store layer builds the runs.status CHECK constraint from this list,
// so the two can never drift apart silently (AC-031).
func AllRunStatuses() []RunStatus {
	return []RunStatus{
		RunCreated, RunResolving, RunRunning, RunPaused, RunStopping,
		RunStopped, RunSucceeded, RunFailed, RunRepairing, RunImporting,
		RunAwaitingQa, RunClosed,
	}
}

func (s WorkflowStage) String() string { return string(s) }
func (s StageStatus) String() string   { return string(s) }
func (s RunStatus) String() string     { return string(s) }

// Valid reports whether s is a known WorkflowStage (AC-026).
func (s WorkflowStage) Valid() bool {
	for _, v := range AllWorkflowStages() {
		if s == v {
			return true
		}
	}
	return false
}

// ParseWorkflowStage parses a stage name, rejecting unknown values with a
// typed error (AC-026: no bare strings).
func ParseWorkflowStage(v string) (WorkflowStage, error) {
	s := WorkflowStage(v)
	if !s.Valid() {
		return "", NewError(ReasonInvalidStage, "ParseWorkflowStage", "unknown stage "+v)
	}
	return s, nil
}

// Valid reports whether s is a known StageStatus.
func (s StageStatus) Valid() bool {
	for _, v := range AllStageStatuses() {
		if s == v {
			return true
		}
	}
	return false
}

// ParseStageStatus parses a status name, rejecting unknown values.
func ParseStageStatus(v string) (StageStatus, error) {
	s := StageStatus(v)
	if !s.Valid() {
		return "", NewError(ReasonInvalidStageStatus, "ParseStageStatus", "unknown stage status "+v)
	}
	return s, nil
}

// Valid reports whether s is one of the 12 run statuses (AC-031).
func (s RunStatus) Valid() bool {
	for _, v := range AllRunStatuses() {
		if s == v {
			return true
		}
	}
	return false
}

// ParseRunStatus parses a run status name, rejecting unknown values.
func ParseRunStatus(v string) (RunStatus, error) {
	s := RunStatus(v)
	if !s.Valid() {
		return "", NewError(ReasonInvalidRunStatus, "ParseRunStatus", "unknown run status "+v)
	}
	return s, nil
}

// NextStage returns the immediate pipeline successor (derived table).
func NextStage(s WorkflowStage) (WorkflowStage, bool) {
	stages := AllWorkflowStages()
	for i, v := range stages {
		if v == s && i+1 < len(stages) {
			return stages[i+1], true
		}
	}
	return "", false
}

// CanTransition reports whether from→to is a legal stage transition
// (derived: forward to the immediate next stage only).
func CanTransition(from, to WorkflowStage) bool {
	next, ok := NextStage(from)
	return ok && next == to
}

// ValidateTransition returns a typed error with
// reason_code=illegal_stage_transition for illegal moves (AC-027).
func ValidateTransition(from, to WorkflowStage) error {
	if !from.Valid() {
		return NewError(ReasonInvalidStage, "ValidateTransition", "unknown from-stage "+string(from))
	}
	if !to.Valid() {
		return NewError(ReasonInvalidStage, "ValidateTransition", "unknown to-stage "+string(to))
	}
	if !CanTransition(from, to) {
		return NewError(ReasonIllegalStageTransition, "ValidateTransition",
			string(from)+" -> "+string(to)+" is not an allowed forward transition")
	}
	return nil
}

// NeedsRecoveryEvent reports whether a stage status is a recovery-class
// status that must be persisted to lifecycle_events and may trigger a
// recovery path (AC-030). The store/orchestrator layers consult this.
func NeedsRecoveryEvent(s StageStatus) bool {
	switch s {
	case StatusCrashed, StatusTimedOut, StatusInterrupted:
		return true
	default:
		return false
	}
}

// StageState is one stage's runtime snapshot (the in-memory counterpart of
// a feature_lifecycle row). Blocked/Skipped carry extra obligations
// (AC-028/029) enforced by Validate and the constructors.
type StageState struct {
	Stage             WorkflowStage `json:"stage"`
	Status            StageStatus   `json:"status"`
	ReasonCodes       []string      `json:"reason_codes,omitempty"`
	BlockingGate      string        `json:"blocking_gate,omitempty"`
	DeferralID        string        `json:"user_accepted_deferral_id,omitempty"`
	DeferralLedgerRef string        `json:"deferral_ledger_ref,omitempty"`
}

// Validate enforces the status obligations:
//   - Blocked must carry at least one non-empty reason code (AC-028).
//   - Skipped must carry a user-accepted deferral id or a deferral ledger
//     reference (AC-029).
func (st StageState) Validate() error {
	if !st.Stage.Valid() {
		return NewError(ReasonInvalidStage, "StageState.Validate", "unknown stage "+string(st.Stage))
	}
	if !st.Status.Valid() {
		return NewError(ReasonInvalidStageStatus, "StageState.Validate", "unknown status "+string(st.Status))
	}
	switch st.Status {
	case StatusBlocked:
		if !hasNonEmpty(st.ReasonCodes) {
			return NewError(ReasonBlockedMissingReasons, "StageState.Validate",
				"Blocked requires at least one reason_code")
		}
	case StatusSkipped:
		if st.DeferralID == "" && st.DeferralLedgerRef == "" {
			return NewError(ReasonSkippedMissingDeferral, "StageState.Validate",
				"Skipped requires user_accepted_deferral_id or deferral ledger ref")
		}
	}
	return nil
}

// NewBlockedState constructs a valid Blocked snapshot (AC-028).
func NewBlockedState(stage WorkflowStage, reasonCodes []string, blockingGate string) (StageState, error) {
	st := StageState{Stage: stage, Status: StatusBlocked, ReasonCodes: reasonCodes, BlockingGate: blockingGate}
	if err := st.Validate(); err != nil {
		return StageState{}, err
	}
	return st, nil
}

// NewSkippedState constructs a valid Skipped snapshot (AC-029). Pass the
// deferral id and/or the ledger reference; at least one is required.
func NewSkippedState(stage WorkflowStage, deferralID, ledgerRef string) (StageState, error) {
	st := StageState{Stage: stage, Status: StatusSkipped, DeferralID: deferralID, DeferralLedgerRef: ledgerRef}
	if err := st.Validate(); err != nil {
		return StageState{}, err
	}
	return st, nil
}

func hasNonEmpty(vs []string) bool {
	for _, v := range vs {
		if v != "" {
			return true
		}
	}
	return false
}

// EvidencePayload represents the machine-verifiable evidence generated by QA Harness.
type EvidencePayload struct {
	AC           string   `json:"ac"`
	Status       string   `json:"status"` // passed / failed
	VerifiedAt   string   `json:"verified_at"`
	ExitCode     int      `json:"exit_code"`
	EvidenceHash string   `json:"evidence_hash"`
	TestCases    []string `json:"test_cases"`
}

// ValidateEvidenceTransition enforces that transitioning to QA / Release
// requires valid non-empty machine evidence with exit_code == 0 and status == "passed" (AC-AGENT-006).
func ValidateEvidenceTransition(from, to WorkflowStage, ev *EvidencePayload) error {
	if err := ValidateTransition(from, to); err != nil {
		return err
	}
	// Gate transition from Implement to Qa or Qa to Release
	if to == StageQa || to == StageRelease {
		if ev == nil {
			return NewError(ReasonEvidenceMissing, "ValidateEvidenceTransition",
				"transition to "+string(to)+" rejected: machine evidence is missing")
		}
		if ev.Status != "passed" || ev.ExitCode != 0 || ev.EvidenceHash == "" {
			return NewError(ReasonEvidenceVerificationFailed, "ValidateEvidenceTransition",
				"transition to "+string(to)+" rejected: verification failed or invalid evidence hash")
		}
	}
	return nil
}
