// Guardrails (P2b, G-21): budget/quota, steering and todos — the
// goal_budget.py / goal_quota.py / goal_steering.py / goal_todos.py
// family, landed at the scope the charter asks for:
//
//   - Budget/Quota: ONE spend ledger on the run snapshot (rounds +
//     wall-clock duration) with the source's exhaustion reason codes.
//     The driver spends one round per turn, only after the turn's
//     work is written back (the spend-after-writeback rule of
//     goal_budget.spend_guard / goal_quota.record_spend, structurally:
//     DriverRunOnce charges after Tick returns and the snapshot is
//     re-saved). Token accounting is deliberately absent — P2b has no
//     token source and fabricating one would be impersonation.
//   - Steering: human direction changes are recorded on the run and
//     surfaced by tick/resume/status until acknowledged.
//   - Todos: the run's own list lives in the run snapshot (its
//     checkpoints are append-only resume points, not a list store);
//     the status summary is derived from the same list, so list and
//     summary can never disagree.
package orchestrator

import (
	"fmt"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ConfigureBudget sets the run's budget limits (rounds + duration
// seconds; 0 = unlimited). Used rounds are preserved.
func (e *Engine) ConfigureBudget(runID string, maxRounds, maxDurationSec int) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	if m.Budget == nil {
		m.Budget = &BudgetState{}
	}
	m.Budget.MaxRounds = maxRounds
	m.Budget.MaxDurationSec = maxDurationSec
	if exhausted, reason := budgetExhausted(m.Budget); exhausted {
		m.Budget.Exhausted = true
		m.Budget.ExhaustedReason = reason
	} else {
		m.Budget.Exhausted = false
		m.Budget.ExhaustedReason = ""
	}
	if err := e.appendEvent(run, "budget_configured", map[string]any{
		"max_rounds": maxRounds, "max_duration_sec": maxDurationSec,
		"used_rounds": m.Budget.UsedRounds,
	}); err != nil {
		return RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// ---- Spend ledger + handoff budget (R2b, M9: goal_budget.py) ----

// spendLimitOf resolves the spend ledger's quota limit from the
// run's own ceilings: the budget's round cap first, then the
// driver's turn cap. Zero means no ceiling is configured anywhere
// — the budget family's "0 = unlimited" caliber — and the spend
// gate stays out of the way (the source's ledger takes an explicit
// limit; a Go run without one has nothing to charge against).
func spendLimitOf(m *runMeta) int {
	if m.Budget != nil && m.Budget.MaxRounds > 0 {
		return m.Budget.MaxRounds
	}
	if m.Driver != nil && m.Driver.MaxTurns > 0 {
		return m.Driver.MaxTurns
	}
	return 0
}

// unboundedSpendProbe stands in for the quota limit when guards
// are evaluated on a run with no configured ceiling: the source's
// budget packet always carries an explicit limit (and treats 0 as
// exhausted from the start — core keeps that edge), while a Go run
// without any ceiling has nothing to exhaust. The probe value is
// evaluation-only; the persisted ledger keeps its real limit.
const unboundedSpendProbe = 1 << 30

// recordSpendOn charges one spend to the run's ledger (in-memory;
// the caller persists). It applies spend_eligible's full guard
// order and commit_spend. Returns the refusal reason ("" when the
// spend landed or no spend gate applies). With force=false (the
// driver's internal charge) a run with no configured ceiling has
// no spend gate at all; with force=true (the public RecordSpend
// door) the whitelist / writeback / idempotency guards always
// apply, with the exhaustion leg evaluated against the probe.
func (e *Engine) recordSpendOn(run store.Run, m *runMeta, source, indexDigest string, writebackValidated bool, writebackRef string, force bool) (alreadySpent bool, reason string) {
	limit := spendLimitOf(m)
	if limit <= 0 && !force {
		return false, ""
	}
	if m.Spend == nil {
		l := core.NewSpendLedger(run.ID, limit)
		m.Spend = &l
	}
	probe := *m.Spend
	if probe.QuotaLimit <= 0 {
		probe.QuotaLimit = unboundedSpendProbe
	}
	if ok, code := core.SpendEligible(probe, source, writebackValidated, indexDigest); !ok {
		return false, code
	}
	l, already, _ := core.CommitSpend(*m.Spend, source, indexDigest, writebackRef)
	*m.Spend = l
	return already, ""
}

// RecordSpend is the public spend door: one guarded charge against
// the run's spend ledger, persisted with its trail event. Refusals
// come back as typed errors carrying the source's reason codes
// (quiet_skip_no_spend / spend_without_writeback /
// spend_idempotency_conflict / spend_source_not_allowed /
// program_drive_budget_exhausted).
func (e *Engine) RecordSpend(runID, source, indexDigest string, writebackValidated bool, writebackRef string) (core.SpendLedger, error) {
	if indexDigest == "" {
		return core.SpendLedger{}, core.NewError("spend_index_digest_required", "goal.RecordSpend",
			"an index digest is required: it is the idempotency key of the spend")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return core.SpendLedger{}, err
	}
	already, reason := e.recordSpendOn(run, m, source, indexDigest, writebackValidated, writebackRef, true)
	if reason != "" {
		return core.SpendLedger{}, core.NewError(reason, "goal.RecordSpend",
			"spend refused for run "+runID+" (source="+source+")")
	}
	_ = already // replay-safe by construction (eligibility refuses a second charge)
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "spend_recorded", map[string]any{
			"source": source, "index_digest": indexDigest, "writeback_ref": writebackRef,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return core.SpendLedger{}, err
	}
	if m.Spend == nil {
		return core.NewSpendLedger(run.ID, spendLimitOf(m)), nil
	}
	return *m.Spend, nil
}

// EnforceHandoffBudget caps a handoff text at the turn budget's
// 16 lines / 1800 chars (goal_budget.enforce_handoff_budget):
// truncation is applied and reported, never silent, never a
// refusal. The driver applies it to every next-turn digest.
func EnforceHandoffBudget(text string) core.HandoffBudgetResult {
	return core.EnforceHandoffBudget(text)
}

// ---- Steering ----

// Steer records a human direction change on the run
// (goal_steering objective_updated semantics): kind=objective
// supersedes the run's objective utterance; kind=note is advisory.
// The record stays unacknowledged — and therefore surfaced by
// Tick/Resume/Status — until AckSteering.
func (e *Engine) Steer(runID, kind, text string) (RunView, error) {
	if kind == "" {
		kind = SteeringObjective
	}
	if kind != SteeringObjective && kind != SteeringNote {
		return RunView{}, core.NewError("invalid_steering_kind", "goal.Steer", "kind must be objective or note")
	}
	if text == "" {
		return RunView{}, core.NewError("steering_text_required", "goal.Steer", "steering text must be non-empty")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	rec := SteeringRecord{
		ID: newWatchID("str-"), Kind: kind, Text: text, At: nowISO(),
	}
	m.Steerings = append(m.Steerings, rec)
	if kind == SteeringObjective {
		m.Utterance = text // NEW_OBJECTIVE_SUPERSEDES
	}
	if err := e.appendEvent(run, "goal_steering_recorded", map[string]any{
		"steering_id": rec.ID, "kind": kind, "text": text,
	}); err != nil {
		return RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// AckSteering acknowledges every pending steering record of the run.
func (e *Engine) AckSteering(runID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	acked := 0
	for i := range m.Steerings {
		if !m.Steerings[i].Acknowledged {
			m.Steerings[i].Acknowledged = true
			acked++
		}
	}
	if acked > 0 {
		if err := e.appendEvent(run, "goal_steering_acknowledged", map[string]any{"count": acked}); err != nil {
			return RunView{}, err
		}
		if err := e.saveRun(run, m); err != nil {
			return RunView{}, err
		}
	}
	return viewOf(run, m, slug), nil
}

// ---- Todos ----

func findTodo(m *runMeta, id string) *TodoItem {
	for i := range m.Todos {
		if m.Todos[i].ID == id {
			return &m.Todos[i]
		}
	}
	return nil
}

// TodoAdd appends an open todo to the run's list. IDs are sequential
// per run (todo-1, todo-2, ...) so CLI output is stable.
func (e *Engine) TodoAdd(runID, title string) (TodoItem, RunView, error) {
	if title == "" {
		return TodoItem{}, RunView{}, core.NewError("todo_title_required", "goal.TodoAdd", "todo title must be non-empty")
	}
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return TodoItem{}, RunView{}, err
	}
	// IDs come from a monotonic per-run counter so a removed todo's id
	// is never recycled (the event trail references ids).
	m.TodoSeq++
	item := TodoItem{
		ID: fmt.Sprintf("todo-%d", m.TodoSeq), Title: title,
		Status: TodoOpen, CreatedAt: nowISO(), UpdatedAt: nowISO(),
	}
	m.Todos = append(m.Todos, item)
	if err := e.appendEvent(run, "goal_todo_added", map[string]any{
		"todo_id": item.ID, "title": title,
	}); err != nil {
		return TodoItem{}, RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return TodoItem{}, RunView{}, err
	}
	return item, viewOf(run, m, slug), nil
}

// TodoList returns the run's todos in insertion order.
func (e *Engine) TodoList(runID string) ([]TodoItem, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return nil, err
	}
	out := make([]TodoItem, len(m.Todos))
	copy(out, m.Todos)
	return out, nil
}

// TodoDone marks one todo done (goal_todos update semantics: a done
// todo cannot be re-done or claimed; claim/double-claim rules of the
// source are enforced by TodoClaim).
func (e *Engine) TodoDone(runID, todoID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	t := findTodo(m, todoID)
	if t == nil {
		return RunView{}, core.NewError(core.ReasonTodoMissing, "goal.TodoDone", "no todo "+todoID+" on run "+runID)
	}
	if t.Status == TodoDone {
		return RunView{}, core.NewError(core.ReasonTodoDone, "goal.TodoDone", "todo "+todoID+" is already done")
	}
	t.Status = TodoDone
	t.UpdatedAt = nowISO()
	if err := e.appendEvent(run, "goal_todo_done", map[string]any{"todo_id": todoID}); err != nil {
		return RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// TodoRemove deletes one todo from the run's list (the "删" of the
// add/remove pair; history stays in the event trail).
func (e *Engine) TodoRemove(runID, todoID string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	idx := -1
	for i := range m.Todos {
		if m.Todos[i].ID == todoID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return RunView{}, core.NewError(core.ReasonTodoMissing, "goal.TodoRemove", "no todo "+todoID+" on run "+runID)
	}
	m.Todos = append(m.Todos[:idx], m.Todos[idx+1:]...)
	if err := e.appendEvent(run, "goal_todo_removed", map[string]any{"todo_id": todoID}); err != nil {
		return RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}

// TodoClaim binds one owner to an open todo (goal_todos.claim_todo):
// a second claim is rejected with double_claim, claiming a done todo
// with todo_done.
func (e *Engine) TodoClaim(runID, todoID, owner string) (RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return RunView{}, err
	}
	t := findTodo(m, todoID)
	if t == nil {
		return RunView{}, core.NewError(core.ReasonTodoMissing, "goal.TodoClaim", "no todo "+todoID+" on run "+runID)
	}
	if t.Status == TodoClaimed {
		return RunView{}, core.NewError(core.ReasonTodoDoubleClaim, "goal.TodoClaim",
			"todo "+todoID+" already claimed by "+t.Owner)
	}
	if t.Status == TodoDone {
		return RunView{}, core.NewError(core.ReasonTodoDone, "goal.TodoClaim", "todo "+todoID+" is done")
	}
	t.Status = TodoClaimed
	t.Owner = owner
	t.UpdatedAt = nowISO()
	if err := e.appendEvent(run, "goal_todo_claimed", map[string]any{
		"todo_id": todoID, "owner": owner,
	}); err != nil {
		return RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return RunView{}, err
	}
	return viewOf(run, m, slug), nil
}
