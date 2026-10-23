// Bounded turn budget + spend ledger (R2b, parity audit M9): the
// pure half of beacon/utils/goal_budget.py. One turn is a five-step
// contract (choose -> deliver -> validate -> writeback -> spend);
// handoff text is capped so long-running state stays compact; and
// spending is a ledger with three guards: no spend without a
// validated writeback, quiet-skip sources never spend, and one
// index_digest spends exactly once (idempotency), from a source
// whitelist only.
package core

import "strings"

// BudgetSchema is goal_budget.BUDGET_SCHEMA; SpendSchema is
// goal_budget.SPEND_SCHEMA.
const (
	BudgetSchema = "goal-turn-budget.v1"
	SpendSchema  = "goal-spend-ledger.v1"
)

// Handoff caps (goal_budget.HANDOFF_MAX_LINES / HANDOFF_MAX_CHARS,
// verbatim).
const (
	HandoffMaxLines = 16
	HandoffMaxChars = 1800
)

// TurnSteps is the bounded-delivery turn contract (TURN_STEPS).
var TurnSteps = []string{"choose", "deliver", "validate", "writeback", "spend"}

// QuietSkipSources never spend (goal_budget.QUIET_SKIP_SOURCES):
// preflight / dry-run / quiet-skip / polling work is free.
var QuietSkipSources = map[string]bool{
	"preflight": true, "dry_run": true, "quiet_skip": true, "poll": true,
}

// SpendSources is the spend whitelist (goal_budget.SPEND_SOURCES).
var SpendSources = map[string]bool{
	"delivery_completion": true, "repair": true, "self_repair": true, "capability_repair": true,
}

// HandoffBudgetResult is enforce_handoff_budget's report.
type HandoffBudgetResult struct {
	Schema    string `json:"schema"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
	Lines     int    `json:"lines"`
	Chars     int    `json:"chars"`
	MaxLines  int    `json:"max_lines"`
	MaxChars  int    `json:"max_chars"`
}

// pySplitLines is Python's str.splitlines() for \n-terminated text:
// a trailing newline TERMINATES the last line instead of starting
// an empty one (strings.Split would count one extra), and a \r\n
// pair is one break.
func pySplitLines(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, "\n")
	if strings.HasSuffix(s, "\n") {
		parts = parts[:len(parts)-1]
	}
	for i := range parts {
		parts[i] = strings.TrimSuffix(parts[i], "\r")
	}
	return parts
}

// EnforceHandoffBudget mirrors enforce_handoff_budget: truncate to
// 16 lines, then to 1800 chars — truncation is applied and REPORTED
// (truncated=true), never a silent cut and never a refusal. Char
// counting follows the source's len() over the string (rune count).
func EnforceHandoffBudget(text string) HandoffBudgetResult {
	lines := pySplitLines(text)
	truncated := false
	if len(lines) > HandoffMaxLines {
		lines = lines[:HandoffMaxLines]
		truncated = true
	}
	body := strings.Join(lines, "\n")
	if r := []rune(body); len(r) > HandoffMaxChars {
		body = string(r[:HandoffMaxChars])
		truncated = true
	}
	return HandoffBudgetResult{
		Schema: BudgetSchema, Text: body, Truncated: truncated,
		Lines: len(pySplitLines(body)), Chars: len([]rune(body)),
		MaxLines: HandoffMaxLines, MaxChars: HandoffMaxChars,
	}
}

// TurnContract mirrors turn_contract: the bounded-delivery shape a
// driver turn advertises.
type TurnContract struct {
	Schema              string   `json:"schema"`
	Mode                string   `json:"mode"`
	Steps               []string `json:"steps"`
	SpendAfterWriteback bool     `json:"spend_after_writeback"`
}

// BoundedTurnContract returns turn_contract()'s packet.
func BoundedTurnContract() TurnContract {
	return TurnContract{
		Schema: BudgetSchema, Mode: "bounded_delivery",
		Steps: append([]string{}, TurnSteps...), SpendAfterWriteback: true,
	}
}

// SpendGuard mirrors spend_guard: a turn without a validated
// writeback may not spend (spend_without_writeback).
func SpendGuard(writebackValidated bool) (ok bool, reasonCode string) {
	if writebackValidated {
		return true, ""
	}
	return false, ReasonSpendWithoutWriteback
}

// SpendEntry is one committed spend (goal_budget ledger entry).
type SpendEntry struct {
	Index        int    `json:"index"`
	Source       string `json:"source"`
	IndexDigest  string `json:"index_digest"`
	WritebackRef string `json:"writeback_ref"`
}

// SpendLedger is the persistent per-goal spend ledger
// (goal_budget.new_ledger's shape).
type SpendLedger struct {
	Schema     string       `json:"schema"`
	GoalID     string       `json:"goal_id"`
	QuotaLimit int          `json:"quota_limit"`
	Spent      int          `json:"spent"`
	Entries    []SpendEntry `json:"entries,omitempty"`
}

// NewSpendLedger mirrors new_ledger (quota_limit clamped at 0).
func NewSpendLedger(goalID string, quotaLimit int) SpendLedger {
	if quotaLimit < 0 {
		quotaLimit = 0
	}
	return SpendLedger{Schema: SpendSchema, GoalID: goalID, QuotaLimit: quotaLimit}
}

// SpendQuotaExhausted mirrors quota_exhausted: spent >= quota_limit.
// Note the source's edge: a zero limit is exhausted from the start.
func SpendQuotaExhausted(l SpendLedger) bool {
	return l.Spent >= l.QuotaLimit
}

// SpendEligible mirrors spend_eligible, in the source's check order:
// quiet-skip source -> writeback guard -> idempotency conflict ->
// source whitelist -> quota.
func SpendEligible(l SpendLedger, source string, writebackValidated bool, indexDigest string) (ok bool, reasonCode string) {
	if QuietSkipSources[source] {
		return false, ReasonQuietSkipNoSpend
	}
	if ok, code := SpendGuard(writebackValidated); !ok {
		return false, code
	}
	for _, e := range l.Entries {
		if e.IndexDigest == indexDigest {
			return false, ReasonSpendIdempotencyConf
		}
	}
	if !SpendSources[source] {
		return false, ReasonSpendSourceNotAllowed
	}
	if SpendQuotaExhausted(l) {
		return false, ReasonProgramBudgetExhausted
	}
	return true, ""
}

// CommitSpend mirrors commit_spend: appends one entry; committing
// the same index_digest twice is an idempotent replay (ok, already
// spent) rather than a second charge.
func CommitSpend(l SpendLedger, source, indexDigest, writebackRef string) (SpendLedger, bool, string) {
	for _, e := range l.Entries {
		if e.IndexDigest == indexDigest {
			return l, true, ReasonSpendIdempotentReplay
		}
	}
	l.Entries = append(append([]SpendEntry{}, l.Entries...), SpendEntry{
		Index: len(l.Entries) + 1, Source: source,
		IndexDigest: indexDigest, WritebackRef: writebackRef,
	})
	l.Spent = len(l.Entries)
	return l, false, ""
}

// SpendHaltPacket mirrors halt_on_exhaust: the halt_budget stop
// packet when the ledger's quota is consumed.
type SpendHaltPacket struct {
	State        string `json:"state"`
	ReasonCode   string `json:"reason_code,omitempty"`
	Spent        int    `json:"spent,omitempty"`
	QuotaLimit   int    `json:"quota_limit,omitempty"`
	ResumeAction string `json:"resume_action"`
}

// HaltOnSpendExhaust mirrors halt_on_exhaust.
func HaltOnSpendExhaust(l SpendLedger) SpendHaltPacket {
	if SpendQuotaExhausted(l) {
		return SpendHaltPacket{
			State: "halt_budget", ReasonCode: ReasonProgramBudgetExhausted,
			Spent: l.Spent, QuotaLimit: l.QuotaLimit, ResumeAction: "none",
		}
	}
	return SpendHaltPacket{State: "running", ResumeAction: "continue"}
}
