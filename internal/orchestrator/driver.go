// Driver (P2b, G-06): the long-run driver of goal_agent_driver.py,
// re-based on the store. One driver per run, pushing the goal one
// engine turn (Tick/Continue) at a time: it writes a heartbeat and a
// checkpoint after each turn, prepares the next turn's prompt digest,
// and stops at budget exhaustion, at terminal goal states, or at the
// release gate. It NEVER completes a run and NEVER confirms a release
// (auto_release is structurally false).
//
// Lock honesty: the v1 driver used a per-run lock FILE with a live-pid
// check. Here the lease lives in the run snapshot and DriverStart
// takes it via a compare-and-swap store update (dao_p2b), which is a
// real cross-process exclusion for the start operation. What it is
// NOT: a fencing token for turn execution — a driver whose lease
// expires mid-turn is not preempted; run-once re-checks the recorded
// driver state but two processes must not drive the same run
// concurrently (see p2b report).
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// LoopKillSwitchRelPath is the project-relative kill-switch file
// (P4c; v1.6.5 loop-token-budget AC-006, spelling verbatim). Its
// PRESENCE pauses automated loops for the project: DriverStart and
// DriverRunOnce refuse with loop_kill_switch_active. It is an
// external human control signal read from the project tree, never
// written by the kernel and never a state store — removing the
// file is the human's resume action.
const LoopKillSwitchRelPath = ".beacon/loop/pause-all"

// loopKillSwitch reports whether the run's project carries the
// kill-switch file, and where.
func (e *Engine) loopKillSwitch(run store.Run) (bool, string) {
	fctx, ok, err := e.featureContextOf(run.FeatureID)
	if err != nil || !ok {
		return false, ""
	}
	p := filepath.Join(fctx.Project.RootPath, filepath.FromSlash(LoopKillSwitchRelPath))
	if _, err := os.Stat(p); err == nil {
		return true, p
	}
	return false, ""
}

func killSwitchError(op, path string) error {
	return core.NewError(core.ReasonLoopKillSwitchActive, op,
		"loop kill switch is engaged ("+path+"); automated loops are paused until a human removes the file")
}

// DriverView is the driver surface returned by every driver operation.
type DriverView struct {
	RunID       string       `json:"run_id"`
	State       string       `json:"state"`
	ReasonCode  string       `json:"reason_code,omitempty"`
	Driver      *DriverState `json:"driver,omitempty"`
	LeaseActive bool         `json:"lease_active"`
	AutoRelease bool         `json:"auto_release"` // always false
	Run         RunView      `json:"run"`
}

func driverViewOf(run store.Run, m *runMeta, slug string) DriverView {
	dv := DriverView{RunID: run.ID, Run: viewOf(run, m, slug), AutoRelease: false}
	if m.Driver != nil {
		dv.State = m.Driver.State
		dv.Driver = m.Driver
		dv.LeaseActive = m.Driver.leaseActive(time.Now())
	} else {
		dv.State = DriverIdle
	}
	return dv
}

// DriverStartOptions carries the driver arm inputs.
type DriverStartOptions struct {
	MaxTurns              int  // budget rounds; <=0 -> default 32
	TurnTimeoutSec        int  // recorded per-turn timeout; <=0 -> default 3600
	MaxDurationSec        int  // budget wall-clock seconds; 0 = unlimited
	AttachSupervisor      bool // source parity: driver_start attaches the supervisor
	SupervisorIntervalSec int
	LeaseTTL              time.Duration // <=0 -> DefaultDriverLeaseTTL
}

// DriverStart arms the driver for a run. A second start while another
// driver holds a live lease is refused with
// agent_driver_dual_start_forbidden; an expired lease is taken over
// (driver_lease_takeover event). The lease is taken with a CAS update
// so the check-and-set is atomic across processes.
func (e *Engine) DriverStart(runID string, opts DriverStartOptions) (DriverView, error) {
	maxTurns := opts.MaxTurns
	if maxTurns <= 0 {
		maxTurns = DefaultDriverMaxTurns
	}
	timeoutSec := opts.TurnTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = DefaultDriverTurnTimeoutSec
	}
	leaseTTL := opts.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = DefaultDriverLeaseTTL
	}
	for attempt := 0; attempt < 2; attempt++ {
		run, m, slug, err := e.loadRun(runID)
		if err != nil {
			return DriverView{}, err
		}
		if run.RunKind == RunKindDiscoveryQueue {
			return DriverView{}, core.NewError(core.ReasonNotAGoalRun, "goal.DriverStart",
				"discovery queue items cannot be driven; consume the queue first")
		}
		if active, path := e.loopKillSwitch(run); active {
			return DriverView{}, killSwitchError("goal.DriverStart", path)
		}
		now := time.Now()
		if d := m.Driver; d != nil && d.leaseActive(now) {
			dv := driverViewOf(run, m, slug)
			dv.ReasonCode = core.ReasonDriverDualStart
			return dv, core.NewError(core.ReasonDriverDualStart, "goal.DriverStart",
				"driver "+d.DriverID+" already holds a live lease for run "+runID)
		}
		tookOver := m.Driver != nil && m.Driver.DriverID != "" && !m.Driver.terminal()
		driverID := newWatchID("drv-")
		turnN := 0
		if m.Driver != nil {
			turnN = m.Driver.TurnN // source: turn counter survives restarts
		}
		m.Driver = &DriverState{
			DriverID: driverID, State: DriverArming, Longrun: true,
			TurnN: turnN, MaxTurns: maxTurns, TurnTimeoutSec: timeoutSec,
			StartedAt: nowISO(), UpdatedAt: nowISO(), AutoRelease: false,
			LeaseExpiresAt: now.Add(leaseTTL).UTC().Format(time.RFC3339Nano),
		}
		if m.Budget == nil {
			m.Budget = &BudgetState{MaxRounds: maxTurns, MaxDurationSec: opts.MaxDurationSec}
		} else {
			m.Budget.MaxRounds = maxTurns
			if opts.MaxDurationSec > 0 {
				m.Budget.MaxDurationSec = opts.MaxDurationSec
			}
		}
		raw, err := json.Marshal(m)
		if err != nil {
			return DriverView{}, err
		}
		landed, err := e.store.Runs().UpdateRunStateCAS(run.ID, run.Metadata, runStatusFor(m), raw, nil)
		if err != nil {
			return DriverView{}, err
		}
		if !landed {
			continue // lost a race: reload and re-check the lease
		}
		_ = e.appendEvent(run, "driver_start", map[string]any{
			"driver_id": driverID, "max_turns": maxTurns, "lease_takeover": tookOver,
		})
		if tookOver {
			_ = e.appendEvent(run, "driver_lease_takeover", map[string]any{
				"driver_id": driverID, "reason_code": core.ReasonDriverLeaseTakeover,
			})
		}
		if opts.AttachSupervisor {
			if _, err := e.AttachSupervisor(runID, opts.SupervisorIntervalSec); err != nil {
				return DriverView{}, err
			}
		}
		run, m, slug, err = e.loadRun(runID)
		if err != nil {
			return DriverView{}, err
		}
		if m.LoopStage == string(core.LoopDone) {
			m.Driver.State = DriverHaltTerminal
			m.Driver.HaltReason = "goal_status_completed"
			m.Driver.LeaseExpiresAt = ""
			_ = e.saveRun(run, m)
		}
		return driverViewOf(run, m, slug), nil
	}
	return DriverView{}, core.NewError(core.ReasonDriverDualStart, "goal.DriverStart",
		"could not acquire the driver lease for run "+runID+" (concurrent modification)")
}

// DriverStatus is the read-only driver view.
func (e *Engine) DriverStatus(runID string) (DriverView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DriverView{}, err
	}
	return driverViewOf(run, m, slug), nil
}

// DriverStop halts the driver (agent_driver_user_stop) and releases
// the lease. Like the source, stop succeeds even if no driver was
// started — the halt is recorded either way.
func (e *Engine) DriverStop(runID string) (DriverView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DriverView{}, err
	}
	if m.Driver == nil {
		m.Driver = &DriverState{State: DriverIdle, MaxTurns: DefaultDriverMaxTurns, TurnTimeoutSec: DefaultDriverTurnTimeoutSec}
	}
	m.Driver.State = DriverHaltTerminal
	m.Driver.HaltReason = core.ReasonDriverUserStop
	m.Driver.LeaseExpiresAt = ""
	m.Driver.UpdatedAt = nowISO()
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "driver_stop", map[string]any{
			"driver_id": m.Driver.DriverID, "reason_code": core.ReasonDriverUserStop,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return DriverView{}, err
	}
	return driverViewOf(run, m, slug), nil
}

// nextPromptDigest builds the condensed next-turn prompt summary:
// current stage -> next skill + the resume action, with any pending
// steering surfaced first (InjectBundle order, condensed).
func nextPromptDigest(v RunView) string {
	var b strings.Builder
	if v.PendingSteering != nil {
		fmt.Fprintf(&b, "steering[%s]: %s; ", v.PendingSteering.Kind, v.PendingSteering.Text)
	}
	fmt.Fprintf(&b, "stage=%s loop=%s", v.CurrentStage, v.LoopStage)
	if v.NextSkill != "" {
		fmt.Fprintf(&b, " next_skill=%s action=load_next_skill", v.NextSkill)
	}
	if v.Blocked {
		fmt.Fprintf(&b, " blocked reason=%s action=resolve_blocker", v.ReasonCode)
	}
	b.WriteString("; continue against frozen truth via resume-pack digest; never self-complete, never auto-release")
	return b.String()
}

// DriverRunOnce pushes exactly one driver turn: a budget check, one
// engine Tick (Continue when soft-blocked), budget spend, heartbeat +
// checkpoint, and the next-turn prompt digest. Terminal conditions
// stop the driver instead of ticking: goal done, hard block,
// error_recovery, and — always — the release gate.
func (e *Engine) DriverRunOnce(ctx context.Context, runID string) (DriverView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return DriverView{}, err
	}
	if m.Driver == nil {
		return DriverView{}, core.NewError(core.ReasonDriverNotStarted, "goal.DriverRunOnce",
			"no driver started for run "+runID+"; run goal driver start first")
	}
	if m.Driver.terminal() {
		return driverViewOf(run, m, slug), nil
	}
	// Kill switch gate: a human pausing all loops for the project
	// stops the very next turn, before any budget is spent.
	if active, path := e.loopKillSwitch(run); active {
		_ = e.appendEvent(run, "driver_kill_switch_halt", map[string]any{
			"driver_id": m.Driver.DriverID, "reason_code": core.ReasonLoopKillSwitchActive, "path": path,
		})
		return driverViewOf(run, m, slug), killSwitchError("goal.DriverRunOnce", path)
	}
	// Quota gate (R2b, H6 — goal_quota.quota_should_run): before
	// the budget is even consulted, the five-segment decision runs.
	// A wait state (health / operator gate / evidence / focus) is a
	// SKIP: no turn, no budget, no slot — and not a stall either,
	// so the heartbeat refreshes. An exhausted slot quota halts
	// the driver with quota_exhausted (the source's halt packet).
	if qd := e.quotaDecisionFor(run, m); qd.Decision == core.QuotaDecisionSkip {
		m.LastHeartbeatAt = nowISO()
		m.Driver.State = DriverRequeue
		m.Driver.UpdatedAt = nowISO()
		if err := e.store.WithTx(func(tx *store.Tx) error {
			_ = e.appendEventOn(tx, run, "driver_quota_skip", map[string]any{
				"driver_id": m.Driver.DriverID, "reason_code": qd.ReasonCode,
				"quota_state": qd.State, "spent_slots": qd.SpentSlots, "allowed_slots": qd.AllowedSlots,
			})
			return e.saveRunOn(tx, run, m)
		}); err != nil {
			return DriverView{}, err
		}
		dv := driverViewOf(run, m, slug)
		dv.ReasonCode = qd.ReasonCode
		return dv, nil
	} else if qd.Decision == core.QuotaDecisionExhausted {
		return e.haltDriverQuota(run, m, slug, qd)
	}
	// Budget gate before spending a turn.
	if exhausted, reason := budgetExhausted(m.Budget); exhausted {
		return e.haltDriverBudget(run, m, slug, reason)
	}
	// Release gate: the driver stops and waits for the human. It does
	// not tick release (P2a tick blocks there anyway) and never
	// completes.
	if m.LoopStage == string(core.LoopReleaseGate) || m.currentStage() == StageRelease {
		m.Driver.State = DriverAwaitingHuman
		m.Driver.HaltReason = core.ReasonReleaseRequiresHuman
		m.Driver.LeaseExpiresAt = ""
		m.Driver.UpdatedAt = nowISO()
		if err := e.store.WithTx(func(tx *store.Tx) error {
			_ = e.appendEventOn(tx, run, "driver_awaiting_human", map[string]any{
				"driver_id": m.Driver.DriverID, "reason_code": core.ReasonDriverAwaitingHuman,
			})
			return e.saveRunOn(tx, run, m)
		}); err != nil {
			return DriverView{}, err
		}
		return driverViewOf(run, m, slug), nil
	}
	if m.LoopStage == string(core.LoopDone) || m.LoopStage == string(core.LoopErrorRecovery) {
		m.Driver.State = DriverHaltTerminal
		m.Driver.HaltReason = "goal_status_" + viewOf(run, m, slug).Status
		m.Driver.LeaseExpiresAt = ""
		m.Driver.UpdatedAt = nowISO()
		_ = e.saveRun(run, m)
		return driverViewOf(run, m, slug), nil
	}
	// Mark the turn active (renews the lease), then tick once.
	m.Driver.State = DriverTurnActive
	m.Driver.LeaseExpiresAt = time.Now().Add(DefaultDriverLeaseTTL).UTC().Format(time.RFC3339Nano)
	m.Driver.UpdatedAt = nowISO()
	if err := e.saveRun(run, m); err != nil {
		return DriverView{}, err
	}
	started := time.Now()
	wasBlocked := m.Blocked
	var v RunView
	if m.Blocked {
		v, err = e.Continue(ctx, runID)
	} else {
		v, err = e.Tick(ctx, runID)
	}
	if err != nil {
		return DriverView{}, err
	}
	elapsed := time.Since(started).Seconds()
	// Reload: Tick saved its own snapshot; fold the turn bookkeeping in.
	run, m, slug, err = e.loadRun(runID)
	if err != nil {
		return DriverView{}, err
	}
	if m.Driver == nil { // defensive: Tick preserves metadata; never lose the driver
		return DriverView{}, core.NewError(core.ReasonDriverNotStarted, "goal.DriverRunOnce", "driver state lost during tick")
	}
	m.Driver.TurnN++
	m.Driver.UpdatedAt = nowISO()
	m.Driver.LeaseExpiresAt = time.Now().Add(DefaultDriverLeaseTTL).UTC().Format(time.RFC3339Nano)
	if m.Budget == nil {
		m.Budget = &BudgetState{MaxRounds: m.Driver.MaxTurns}
	}
	m.Budget.UsedRounds++
	m.Budget.UsedDurationSec += elapsed
	m.LastHeartbeatAt = nowISO()
	// The next-turn digest is a handoff text: the turn budget caps
	// it at 16 lines / 1800 chars (R2b, M9 — enforce_handoff_budget;
	// applied and reported, never silently cut).
	handoff := core.EnforceHandoffBudget(nextPromptDigest(v))
	m.Driver.NextPrompt = handoff.Text
	// Quota spend (H6): the turn ran and its writeback (the tick's
	// own snapshot save) has landed, so one slot is spent —
	// record_spend's validated-writeback rule.
	_, _ = recordQuotaSpend(m, 1, true)
	// Spend ledger (M9): the same turn is charged to the goal's
	// spend ledger under its turn digest — delivery turns spend as
	// delivery_completion, continue-after-block turns as repair.
	// A program_drive_budget_exhausted refusal halts the driver
	// after this turn's bookkeeping lands.
	spendSource := "delivery_completion"
	if wasBlocked {
		spendSource = "repair"
	}
	_, spendReason := e.recordSpendOn(run, m, spendSource,
		fmt.Sprintf("%s:turn:%d", run.ID, m.Driver.TurnN), true, "", false)
	// The whole turn-end unit — turn_end event, the optional
	// awaiting-human event, the run save and the closing checkpoint
	// (checkpoint row + goal_checkpoint event + run save) — commits
	// as ONE transaction (P6 write-path fix; this unit alone was 5
	// commits before). The two trail events stay best-effort inside
	// the batch, exactly as they were outside it.
	err = e.store.WithTx(func(tx *store.Tx) error {
		_ = e.appendEventOn(tx, run, "driver_turn_end", map[string]any{
			"driver_id": m.Driver.DriverID, "turn_n": m.Driver.TurnN,
			"stage": v.CurrentStage, "loop_stage": v.LoopStage, "blocked": v.Blocked,
			"handoff_truncated": handoff.Truncated,
			"summary":           fmt.Sprintf("turn=%d stage=%s blocked=%v", m.Driver.TurnN, v.CurrentStage, v.Blocked),
		})
		switch {
		case v.LoopStage == string(core.LoopReleaseGate):
			m.Driver.State = DriverAwaitingHuman
			m.Driver.HaltReason = core.ReasonReleaseRequiresHuman
			m.Driver.LeaseExpiresAt = ""
			_ = e.appendEventOn(tx, run, "driver_awaiting_human", map[string]any{
				"driver_id": m.Driver.DriverID, "reason_code": core.ReasonDriverAwaitingHuman, "turn_n": m.Driver.TurnN,
			})
		case v.LoopStage == string(core.LoopDone) || v.LoopStage == string(core.LoopErrorRecovery):
			m.Driver.State = DriverHaltTerminal
			m.Driver.HaltReason = "goal_status_" + v.Status
			m.Driver.LeaseExpiresAt = ""
		case v.Blocked:
			// Hard block or an exit-gate block the driver cannot clear:
			// stop (source treats goal-blocked as terminal for the driver).
			m.Driver.State = DriverHaltTerminal
			m.Driver.HaltReason = v.ReasonCode
			m.Driver.LastError = v.ReasonCode
			m.Driver.LeaseExpiresAt = ""
		default:
			m.Driver.State = DriverRequeue
		}
		if err := e.saveRunOn(tx, run, m); err != nil {
			return err
		}
		// Source parity: heartbeat + checkpoint close every driver turn.
		_, err := e.checkpointOn(tx, run, m, m.Driver.NextPrompt, fmt.Sprintf("driver-turn-%d", m.Driver.TurnN))
		return err
	})
	if err != nil {
		return DriverView{}, err
	}
	run, m, slug, err = e.loadRun(runID)
	if err != nil {
		return DriverView{}, err
	}
	// Budget spent by this turn may exhaust the budget: halt now so
	// the state is honest even before the next run-once.
	if exhausted, reason := budgetExhausted(m.Budget); exhausted && !m.Driver.terminal() {
		return e.haltDriverBudget(run, m, slug, reason)
	}
	// The spend ledger's own ceiling (M9): a turn the ledger
	// refused for exhaustion halts the driver on the ledger's code.
	if spendReason == core.ReasonProgramBudgetExhausted && !m.Driver.terminal() {
		return e.haltDriverBudget(run, m, slug, spendReason)
	}
	return driverViewOf(run, m, slug), nil
}

// haltDriverQuota stops the driver on slot-quota exhaustion
// (quota_should_run's quota_exhausted outcome; halt_on_exhaust's
// halt_budget state with resume_action=none — the driver record
// carries the spent/allowed pair on the event).
func (e *Engine) haltDriverQuota(run store.Run, m *runMeta, slug string, qd core.QuotaDecision) (DriverView, error) {
	if m.Driver != nil {
		m.Driver.State = DriverHaltBudget
		m.Driver.HaltReason = core.ReasonQuotaExhausted
		m.Driver.LeaseExpiresAt = ""
		m.Driver.UpdatedAt = nowISO()
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		_ = e.appendEventOn(tx, run, "quota_exhausted", map[string]any{
			"reason_code": core.ReasonQuotaExhausted,
			"spent_slots": qd.SpentSlots, "allowed_slots": qd.AllowedSlots,
			"resume_action": "none",
		})
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return DriverView{}, err
	}
	dv := driverViewOf(run, m, slug)
	dv.ReasonCode = core.ReasonQuotaExhausted
	return dv, nil
}

func (e *Engine) haltDriverBudget(run store.Run, m *runMeta, slug, reason string) (DriverView, error) {
	if m.Budget != nil {
		m.Budget.Exhausted = true
		m.Budget.ExhaustedReason = reason
	}
	if m.Driver != nil {
		m.Driver.State = DriverHaltBudget
		m.Driver.HaltReason = reason
		m.Driver.LeaseExpiresAt = ""
		m.Driver.UpdatedAt = nowISO()
	}
	used, max := 0, 0
	if m.Budget != nil {
		used, max = m.Budget.UsedRounds, m.Budget.MaxRounds
	}
	if err := e.store.WithTx(func(tx *store.Tx) error {
		_ = e.appendEventOn(tx, run, "budget_exhausted", map[string]any{
			"reason_code": reason, "used_rounds": used, "max_rounds": max,
		})
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return DriverView{}, err
	}
	return driverViewOf(run, m, slug), nil
}

// DriverLoopView summarizes a DriverRunLoop execution.
type DriverLoopView struct {
	RunID       string       `json:"run_id"`
	Rounds      int          `json:"rounds"`
	FinalState  string       `json:"final_state"`
	ReasonCode  string       `json:"reason_code,omitempty"`
	Driver      *DriverState `json:"driver,omitempty"`
	AutoRelease bool         `json:"auto_release"` // always false
	Run         RunView      `json:"run"`
}

// DriverRunLoop pushes turns until the driver halts (budget /
// terminal / awaiting human) or maxRounds is reached (<=0 -> the
// driver's max turns). sleep, when non-nil, is called between rounds
// (injected for tests; production passes a real sleeper or nil).
func (e *Engine) DriverRunLoop(ctx context.Context, runID string, maxRounds int, sleep func()) (DriverLoopView, error) {
	limit := maxRounds
	if limit <= 0 {
		st, err := e.DriverStatus(runID)
		if err != nil {
			return DriverLoopView{}, err
		}
		if st.Driver != nil && st.Driver.MaxTurns > 0 {
			limit = st.Driver.MaxTurns
		} else {
			limit = DefaultDriverMaxTurns
		}
	}
	startTurnN := 0
	if st, err := e.DriverStatus(runID); err == nil && st.Driver != nil {
		startTurnN = st.Driver.TurnN
	}
	rounds := 0
	var last DriverView
	for rounds < limit {
		// AC-105: a host interrupt (SIGINT/SIGTERM) cancels the context
		// the CLI passes in. Stop between turns and say why — the run
		// keeps its checkpoint and resumes with `goal resume`.
		if ctx.Err() != nil {
			out := DriverLoopView{RunID: runID, Rounds: rounds, ReasonCode: core.ReasonHostInterrupted, AutoRelease: false}
			if st, err := e.DriverStatus(runID); err == nil {
				out.Run = st.Run
				out.Driver = st.Driver
				out.FinalState = st.State
			}
			return out, nil
		}
		dv, err := e.DriverRunOnce(ctx, runID)
		if err != nil {
			return DriverLoopView{}, err
		}
		last = dv
		if dv.Driver != nil {
			rounds = dv.Driver.TurnN - startTurnN
		}
		if dv.Driver == nil || dv.Driver.terminal() {
			break
		}
		if sleep != nil && rounds < limit {
			sleep()
		}
	}
	out := DriverLoopView{Rounds: rounds, AutoRelease: false, Run: last.Run}
	out.RunID = runID
	out.FinalState = last.State
	out.Driver = last.Driver
	if last.Driver != nil {
		out.ReasonCode = last.Driver.HaltReason
	}
	return out, nil
}
