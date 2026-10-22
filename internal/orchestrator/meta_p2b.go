// P2b long-run care state types: driver (G-06), supervisor (G-07),
// queue provenance (G-09) and the guardrail group (G-21: budget /
// steering / todos). Every type here is persisted inside
// runs.metadata_json — the single store snapshot owned by the engine —
// never in package-owned JSON files (charter rule 7, frozen AC-016).
package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"beacon-go/internal/core"
)

// ---- Driver (G-06) ----

// Driver states, mirroring goal_agent_driver.DRIVER_STATES, plus
// awaiting_human: the driver stops at the release gate and waits for
// the human instead of ever crossing it.
const (
	DriverIdle          = "idle"
	DriverArming        = "arming"
	DriverTurnActive    = "turn_active"
	DriverTurnEnded     = "turn_ended"
	DriverRequeue       = "requeue"
	DriverHaltBudget    = "halt_budget"
	DriverHaltTerminal  = "halt_terminal"
	DriverAwaitingHuman = "awaiting_human"
)

// DriverDefaults mirror goal_agent_driver: max 32 agent turns, one
// hour per turn.
const (
	DefaultDriverMaxTurns       = 32
	DefaultDriverTurnTimeoutSec = 3600
	// DefaultDriverLeaseTTL bounds how long a started driver lease
	// stays valid without renewal activity (start / run-once refresh
	// it). See the p2b report for the lock-honesty note.
	DefaultDriverLeaseTTL = 10 * time.Minute
)

// DriverState is the driver record kept on the run snapshot. The lease
// fields (LeaseHolderDriverID + LeaseExpiresAt) are the store-level
// lock: DriverStart takes the lease with a compare-and-swap update on
// runs.metadata_json, so two processes cannot both hold a live lease.
type DriverState struct {
	DriverID       string `json:"driver_id"`
	State          string `json:"state"`
	Longrun        bool   `json:"longrun"`
	TurnN          int    `json:"turn_n"`
	MaxTurns       int    `json:"max_turns"`
	TurnTimeoutSec int    `json:"turn_timeout_sec"`
	StartedAt      string `json:"started_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	HaltReason     string `json:"halt_reason,omitempty"`
	LastError      string `json:"last_error,omitempty"`
	// NextPrompt is the digest summary handed to the next turn
	// (steering -> resume-pack digest -> next_action, condensed; the
	// full InjectBundle prompt rendering belongs to the host bridge,
	// which P2b does not implement).
	NextPrompt     string `json:"next_prompt,omitempty"`
	AutoRelease    bool   `json:"auto_release"` // always false, by design
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
}

// leaseActive reports whether the driver holds a live lease at now:
// state is one of the active states and the lease has not expired.
func (d *DriverState) leaseActive(now time.Time) bool {
	if d == nil || d.DriverID == "" {
		return false
	}
	switch d.State {
	case DriverArming, DriverTurnActive, DriverTurnEnded, DriverRequeue:
	default:
		return false
	}
	if d.LeaseExpiresAt == "" {
		return false
	}
	exp, err := time.Parse(time.RFC3339Nano, d.LeaseExpiresAt)
	return err == nil && now.Before(exp)
}

// terminal reports whether the driver has stopped for good.
func (d *DriverState) terminal() bool {
	if d == nil {
		return true
	}
	switch d.State {
	case DriverHaltBudget, DriverHaltTerminal, DriverAwaitingHuman:
		return true
	}
	return false
}

// ---- Supervisor (G-07) ----

// Supervisor timing mirrors goal_durable_supervisor: default heartbeat
// interval 120s, minimum 30s.
const (
	DefaultHeartbeatIntervalSec = 120
	MinHeartbeatIntervalSec     = 30
	// DefaultStallThresholdSec is the fallback stall threshold used
	// only when neither an explicit threshold nor any recorded
	// heartbeat interval exists on the run (see p2b report).
	DefaultStallThresholdSec = 1800 // 30 minutes
)

// SupervisorState is the opt-in supervisor record on the run snapshot.
type SupervisorState struct {
	Attached    bool   `json:"attached"`
	SessionID   string `json:"session_id,omitempty"`
	State       string `json:"state"` // active | polling | stopped
	IntervalSec int    `json:"interval_sec"`
	AttachedAt  string `json:"attached_at,omitempty"`
	LastPollAt  string `json:"last_poll_at,omitempty"`
	StallCount  int    `json:"stall_count"`
}

// normalizeIntervalSec mirrors normalize_interval (default 120, min 30).
func normalizeIntervalSec(v int) int {
	if v <= 0 {
		return DefaultHeartbeatIntervalSec
	}
	if v < MinHeartbeatIntervalSec {
		return MinHeartbeatIntervalSec
	}
	return v
}

// ---- Budget / quota (G-21) ----

// BudgetState is the run's spend guard: rounds + wall-clock duration.
// Deliberately NOT token billing — P2b has no token source, so no
// token numbers are fabricated (see p2b report). Zero limit = unlimited.
type BudgetState struct {
	MaxRounds       int     `json:"max_rounds"`
	MaxDurationSec  int     `json:"max_duration_sec"`
	UsedRounds      int     `json:"used_rounds"`
	UsedDurationSec float64 `json:"used_duration_sec"`
	Exhausted       bool    `json:"exhausted"`
	ExhaustedReason string  `json:"exhausted_reason,omitempty"`
}

// budgetExhausted reports whether the budget is spent, with the core
// reason code to record (agent_driver_budget_exhausted family).
func budgetExhausted(b *BudgetState) (bool, string) {
	if b == nil {
		return false, ""
	}
	if b.MaxRounds > 0 && b.UsedRounds >= b.MaxRounds {
		return true, core.ReasonDriverBudgetExhaust
	}
	if b.MaxDurationSec > 0 && b.UsedDurationSec >= float64(b.MaxDurationSec) {
		return true, core.ReasonDriverBudgetExhaust
	}
	return false, ""
}

// ---- Steering (G-21) ----

// Steering kinds: an objective change supersedes the run's utterance
// (goal_steering objective_updated semantics); a note is advisory.
const (
	SteeringObjective = "objective"
	SteeringNote      = "note"
)

// SteeringRecord is one mid-run direction change by the human. Records
// stay on the run until acknowledged; tick / resume / status surface
// the newest unacknowledged one (never silently swallowed).
type SteeringRecord struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Text         string `json:"text"`
	At           string `json:"at"`
	Acknowledged bool   `json:"acknowledged"`
}

// ---- Todos (G-21) ----

// Todo statuses (goal_todos.py subset landed in P2b: open / claimed /
// done / superseded bookkeeping; claim+supersede are engine-level).
const (
	TodoOpen       = "open"
	TodoClaimed    = "claimed"
	TodoDone       = "done"
	TodoSuperseded = "superseded"
)

// TodoItem is one entry of the run's own todo list.
type TodoItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Owner     string `json:"owner,omitempty"`
	Successor string `json:"successor,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TodoSummary is the status-surface aggregate of the todo list; it is
// derived from the same list `goal todo list` prints, so the two can
// never disagree.
type TodoSummary struct {
	Total   int `json:"total"`
	Open    int `json:"open"`
	Claimed int `json:"claimed"`
	Done    int `json:"done"`
}

// newWatchID mints a short random id for driver / supervisor /
// steering records (the store's newID helper is package-private).
func newWatchID(prefix string) string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339Nano) }
