// Backend contracts kept by the harness after the built-in agent
// backends (subprocess loop / ACP full session / ralph) were removed.
//
// The kernel no longer executes agents itself: the host agent drives
// stages and records work. What remains here is the result/status
// vocabulary shared by the host-facing surfaces and the subprocess
// helper used by autotest, hooks and task repair.
package harness

import "strings"

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

// HealthStatus is one backend health outcome.
type HealthStatus struct {
	Backend string `json:"backend"`
	OK      bool   `json:"ok"`
	Detail  string `json:"detail,omitempty"`
}

// DoneMarker is the ralph loop's completion marker word. Kept as a
// vocabulary constant: subprocess output parsing still recognises it.
const DoneMarker = "RALPH_DONE"

// ParseArgvFlag splits a --exec-cmd style value. The value is a
// SINGLE program path, never a shell string: arguments travel as
// separate --exec-arg flags. This is the one supported config shape
// so no caller can smuggle a shell pipeline into an executor.
func ParseArgvFlag(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	return []string{cmd}
}
