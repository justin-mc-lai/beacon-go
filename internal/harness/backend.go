// Agent-backend contracts (P3c): the Go form of frozen AC-042.
//
// The frozen truth defines an AgentBackend with iterate /
// health_check / abort / name, and a strongly-typed IterationResult
// {success, summary, key_changes_made, key_learnings, stop_signal?,
// token_usage, agent_name}. AC-043..046 bind concrete backends
// (codex / subprocess / claude / acp) to real integration iterations
// with this result schema; AC-100 forbids a subprocess-only run from
// masquerading as the implement harness.
//
// This package holds only contracts and the subprocess machinery.
// Orchestration (events, artifacts, checkpoints, gates) stays in the
// orchestrator, which consumes these types — harness never touches
// the store.
package harness

import "context"

// IterationResult is the AC-042 result schema, field for field.
type IterationResult struct {
	Success        bool     `json:"success"`
	Summary        string   `json:"summary"`
	KeyChangesMade []string `json:"key_changes_made,omitempty"`
	KeyLearnings   []string `json:"key_learnings,omitempty"`
	StopSignal     string   `json:"stop_signal,omitempty"` // optional (AC-042)
	TokenUsage     int      `json:"token_usage"`
	AgentName      string   `json:"agent_name"`

	// Execution facts the schema does not carry but the caller needs
	// to write evidence: real exit code, timeout marker, and the
	// stdout/stderr capture (filled by the subprocess layer).
	ExitCode   int    `json:"-"`
	TimedOut   bool   `json:"-"`
	StdoutText string `json:"-"`
	StderrText string `json:"-"`
}

// HealthStatus is one backend health_check outcome.
type HealthStatus struct {
	Backend string `json:"backend"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
}

// AgentBackend is the AC-042 backend contract: iterate, health_check,
// abort, name. Implementations must run real work in iterate — a
// backend that fabricates an IterationResult without executing is a
// fixture, and AC-043 explicitly forbids counting those.
type AgentBackend interface {
	Name() string
	Iterate(ctx context.Context, req IterationRequest) (IterationResult, error)
	HealthCheck(ctx context.Context) HealthStatus
	Abort(ctx context.Context) error
}

// DoneMarker is the ralph loop's completion marker word. Convention
// (P3c, written in the report): one iteration is complete when the
// command exits 0 AND its stdout contains a line that is exactly
// this marker (surrounding whitespace tolerated).
const DoneMarker = "RALPH_DONE"
