// Package core is the Go counterpart of the frozen v2 design's
// beacon-core crate: pure types only (workflow/stage/run state machines,
// evidence chain, agent iteration result). No I/O, no storage, no CLI.
//
// Source contract: docs/beacon/archive/v2.0.0/features/
// beacon-v2-rust-kernel-redesign/truth.md (Diagram Truth Layer, AC-026..039).
package core

// WorkflowStage is the product-lifecycle axis (truth.md Diagram Truth Layer).
// Think/Freeze/Archive are v1.6 compatibility variants kept in the type layer.
type WorkflowStage string

const (
	StageThink     WorkflowStage = "Think"
	StageUserStory WorkflowStage = "UserStory"
	StagePrd       WorkflowStage = "Prd"
	StageTestCase  WorkflowStage = "TestCase"
	StagePrototype WorkflowStage = "Prototype"
	StageFreeze    WorkflowStage = "Freeze"
	StageImplement WorkflowStage = "Implement"
	StageQa        WorkflowStage = "Qa"
	StageRelease   WorkflowStage = "Release"
	StageArchive   WorkflowStage = "Archive"
)

// StageStatus is the per-stage status axis.
type StageStatus string

const (
	StatusNotStarted  StageStatus = "NotStarted"
	StatusInProgress  StageStatus = "InProgress"
	StatusBlocked     StageStatus = "Blocked"
	StatusCompleted   StageStatus = "Completed"
	StatusFailed      StageStatus = "Failed"
	StatusSkipped     StageStatus = "Skipped"
	StatusCrashed     StageStatus = "Crashed"
	StatusTimedOut    StageStatus = "TimedOut"
	StatusInterrupted StageStatus = "Interrupted"
	StatusRepairing   StageStatus = "Repairing"
)

// RunStatus is the delivery-FSM runtime-compressed axis stored in runs.status
// (12 states, AC-031). It is a different axis from WorkflowStage and from the
// phase controller; the three must never be merged into one enum.
type RunStatus string

const (
	RunCreated    RunStatus = "Created"
	RunResolving  RunStatus = "Resolving"
	RunRunning    RunStatus = "Running"
	RunPaused     RunStatus = "Paused"
	RunStopping   RunStatus = "Stopping"
	RunStopped    RunStatus = "Stopped"
	RunSucceeded  RunStatus = "Succeeded"
	RunFailed     RunStatus = "Failed"
	RunRepairing  RunStatus = "Repairing"
	RunImporting  RunStatus = "Importing"
	RunAwaitingQa RunStatus = "AwaitingQa"
	RunClosed     RunStatus = "Closed"
)

// IterationResult is the mandatory output schema of every AgentBackend
// (AC-042). Backends are implemented in internal/orchestrator in Phase P2.
type IterationResult struct {
	Success        bool   `json:"success"`
	Summary        string `json:"summary"`
	KeyChangesMade string `json:"key_changes_made"`
	KeyLearnings   string `json:"key_learnings"`
	StopSignal     string `json:"stop_signal,omitempty"`
	TokenUsage     int64  `json:"token_usage,omitempty"`
	AgentName      string `json:"agent_name"`
}

// EvidenceChain is the ordered, hash-linked evidence container (AC-037).
// Its verification semantics live in evidence.go.
type EvidenceChain struct {
	ManifestHash   string   `json:"manifest_hash"`
	SignatureChain []string `json:"signature_chain"`
	ArtifactRefs   []string `json:"artifact_refs"` // ordered artifact_ref ids
}
