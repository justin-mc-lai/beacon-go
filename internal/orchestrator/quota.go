// Quota kernel, engine half (R2b, parity audit H6): the run-side
// state and fact gathering for goal_quota.quota_should_run. The
// pure decision (five segments, source order) lives in core
// (core/quota.go); this file maps Go run facts onto the four wait
// flags and persists the slot ledger on the run snapshot.
//
// Fact mapping (the Python state dict is host-supplied; the Go
// kernel reads its own facts — the mapping is the porting call,
// stated here and in the r2b report):
//   - health_blocked:   the run sits in error_recovery — the
//     kernel's own "something is broken" state.
//   - operator_gate_open: a direction change is waiting for its
//     acknowledgement, or a review decision is parked on the
//     user's answer (planner_review_user_decision_required).
//   - evidence_wait:    the run's current stage is qa, the qa stage
//     already executed, and no QA evidence artifact exists yet —
//     the evidence is being produced outside the driver.
//   - focus_wait:       a host interrupt is pending replay — the
//     host's attention (focus) is elsewhere until it returns.
//
// The driver consults the decision before every automatic turn: a
// skip spends neither budget nor slots and is not a stall; an
// exhausted quota halts the driver with quota_exhausted.
package orchestrator

import (
	"encoding/json"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// QuotaRunState is the quota kernel's per-run record: the
// configured quota (nil = source defaults) and the spend ledger.
type QuotaRunState struct {
	Config *core.QuotaConfig       `json:"config,omitempty"`
	Ledger []core.QuotaLedgerEntry `json:"quota_ledger,omitempty"`
}

// QuotaView is the quota surface: normalised config, the gathered
// state and the current decision.
type QuotaView struct {
	RunID    string             `json:"run_id"`
	Config   core.QuotaConfig   `json:"config"`
	State    core.QuotaState    `json:"state"`
	Decision core.QuotaDecision `json:"decision"`
}

func ensureQuota(m *runMeta) *QuotaRunState {
	if m.Quota == nil {
		m.Quota = &QuotaRunState{}
	}
	return m.Quota
}

func quotaConfigOf(m *runMeta) *core.QuotaConfig {
	if m.Quota == nil {
		return nil
	}
	return m.Quota.Config
}

// quotaFacts gathers the four wait flags from the run's own state.
func (e *Engine) quotaFacts(run store.Run, m *runMeta) core.QuotaState {
	st := core.QuotaState{}
	if m.Quota != nil {
		st.Ledger = m.Quota.Ledger
	}
	if m.LoopStage == string(core.LoopErrorRecovery) {
		st.HealthBlocked = true
		st.RecoveryAction = true
		st.RecoveryKind = "error_recovery"
	}
	if m.pendingSteering() != nil {
		st.OperatorGate = true
	}
	if rd := m.ReviewDecision; rd != nil && !rd.Resolved &&
		rd.ReasonCode == core.ReasonPlannerReviewUserDecisionReq {
		st.OperatorGate = true
	}
	if m.InterruptPending {
		st.FocusWait = true
	}
	if m.currentStage() == StageQA {
		if executed, artifacts := e.qaEvidenceFacts(run); executed && artifacts == 0 {
			st.EvidenceWait = true
		}
	}
	return st
}

// qaEvidenceFacts reports whether a qa stage execution is on record
// for the feature and how many QA artifacts exist across its runs
// (the same facts evalQA gathers, narrowed to what the wait flag
// needs).
func (e *Engine) qaEvidenceFacts(run store.Run) (executed bool, artifacts int) {
	runs, err := e.store.Runs().ListByFeature(run.FeatureID)
	if err != nil {
		return false, 0
	}
	for _, r := range runs {
		arts, err := e.store.Artifacts().ListByRun(r.ID)
		if err == nil {
			for _, a := range arts {
				if qaKind(a.Kind) {
					artifacts++
				}
			}
		}
		events, err := e.store.ListEventsByRun(r.ID)
		if err != nil {
			continue
		}
		for _, ev := range events {
			if ev.EventType != "stage_executed" {
				continue
			}
			var p struct {
				Stage   string `json:"stage"`
				Outcome string `json:"outcome"`
			}
			if len(ev.Payload) > 0 {
				_ = json.Unmarshal(ev.Payload, &p)
			}
			if p.Stage == StageQA && p.Outcome != "failed" {
				executed = true
			}
		}
	}
	return executed, artifacts
}

// quotaDecisionFor computes the current quota decision for a loaded
// run (no writes).
func (e *Engine) quotaDecisionFor(run store.Run, m *runMeta) core.QuotaDecision {
	return core.QuotaShouldRun(e.quotaFacts(run, m), quotaConfigOf(m))
}

// QuotaStatus is the read-only quota view of one run.
func (e *Engine) QuotaStatus(runID string) (QuotaView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return QuotaView{}, err
	}
	st := e.quotaFacts(run, m)
	return QuotaView{
		RunID: runID, Config: core.NormalizeQuotaConfig(quotaConfigOf(m)),
		State: st, Decision: core.QuotaShouldRun(st, quotaConfigOf(m)),
	}, nil
}

// ConfigureQuota sets the run's quota configuration (stored raw;
// every decision re-normalises it, as the source does) and returns
// the resulting view.
func (e *Engine) ConfigureQuota(runID string, cfg core.QuotaConfig) (QuotaView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return QuotaView{}, err
	}
	q := ensureQuota(m)
	c := cfg
	q.Config = &c
	if err := e.store.WithTx(func(tx *store.Tx) error {
		if err := e.appendEventOn(tx, run, "quota_configured", map[string]any{
			"compute": cfg.Compute, "window_hours": cfg.WindowHours,
			"slot_minutes": cfg.SlotMinutes, "allowed_slots": cfg.AllowedSlots,
		}); err != nil {
			return err
		}
		return e.saveRunOn(tx, run, m)
	}); err != nil {
		return QuotaView{}, err
	}
	_ = slug
	return e.QuotaStatus(runID)
}

// recordQuotaSpend spends slots on a loaded snapshot (the caller
// persists): refused without a validated writeback, with the
// source's reason code.
func recordQuotaSpend(m *runMeta, slots int, writebackValidated bool) (bool, string) {
	q := ensureQuota(m)
	ledger, ok, code := core.QuotaRecordSpend(q.Ledger, slots, writebackValidated, nowISO())
	if ok {
		q.Ledger = ledger
	}
	return ok, code
}
