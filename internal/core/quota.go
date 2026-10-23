// Goal compute quota (R2b, parity audit H6): the pure half of
// beacon/utils/goal_quota.py. The quota kernel decides whether an
// AUTOMATIC goal turn may run, in a fixed five-segment order:
//
//	health -> operator gate -> evidence wait -> focus wait -> quota
//
// A skip never looks like a run, and a recovery action rides on its
// own boolean (the source's two-boolean contract: "may ordinary
// delivery continue" vs "is there a bounded recovery action").
// The quota body is a time-window slot model: a window of
// window_hours holds one slot per slot_minutes, scaled by the
// compute factor; spent slots are counted from the run's ledger.
package core

// QuotaSchema is goal_quota.QUOTA_SCHEMA.
const QuotaSchema = "goal-quota.v1"

// Quota wait-state names (quota_should_run returns these as both
// `state` and `reason_code` when a wait segment hits).
const (
	QuotaStateHealthBlocked = "health_blocked"
	QuotaStateOperatorGate  = "operator_gate_open"
	QuotaStateEvidenceWait  = "evidence_wait"
	QuotaStateFocusWait     = "focus_wait"
	QuotaStateEligible      = "quota_eligible"
	QuotaStateExhausted     = "quota_exhausted"
	QuotaDecisionRun        = "run"
	QuotaDecisionSkip       = "skip"
	QuotaDecisionExhausted  = "quota_exhausted"
)

// QuotaConfig is the normalised quota configuration
// (normalize_quota_config's output shape).
type QuotaConfig struct {
	Compute      float64 `json:"compute"`
	WindowHours  int     `json:"window_hours"`
	SlotMinutes  int     `json:"slot_minutes"`
	AllowedSlots int     `json:"allowed_slots"`
}

// NormalizeQuotaConfig mirrors normalize_quota_config: defaults
// compute=1.0, window_hours=24, slot_minutes=1; a non-positive
// allowed_slots is derived as window_hours*60/slot_minutes*compute
// (integer-truncated, as the source's int() cast).
func NormalizeQuotaConfig(cfg *QuotaConfig) QuotaConfig {
	out := QuotaConfig{Compute: 1.0, WindowHours: 24, SlotMinutes: 1}
	if cfg != nil {
		if cfg.Compute != 0 {
			out.Compute = cfg.Compute
		}
		if cfg.WindowHours != 0 {
			out.WindowHours = cfg.WindowHours
		}
		if cfg.SlotMinutes != 0 {
			out.SlotMinutes = cfg.SlotMinutes
		}
		out.AllowedSlots = cfg.AllowedSlots
	}
	if out.AllowedSlots <= 0 {
		slot := out.SlotMinutes
		if slot < 1 {
			slot = 1
		}
		out.AllowedSlots = int(float64(out.WindowHours*60) / float64(slot) * out.Compute)
	}
	return out
}

// QuotaLedgerEntry is one recorded slot spend (record_spend's
// ledger element).
type QuotaLedgerEntry struct {
	Slots int    `json:"slots"`
	At    string `json:"at"`
}

// SpentSlots mirrors spent_slots: the ledger's slot total.
func SpentSlots(ledger []QuotaLedgerEntry) int {
	total := 0
	for _, e := range ledger {
		total += e.Slots
	}
	return total
}

// QuotaState carries the run-side facts quota_should_run reads:
// the four wait flags plus the recovery/scheduler passthroughs and
// the spend ledger.
type QuotaState struct {
	HealthBlocked  bool               `json:"health_blocked"`
	OperatorGate   bool               `json:"operator_gate_open"`
	EvidenceWait   bool               `json:"evidence_wait"`
	FocusWait      bool               `json:"focus_wait"`
	RecoveryAction bool               `json:"recovery_action"`
	RecoveryKind   string             `json:"recovery_kind,omitempty"`
	SchedulerHint  string             `json:"scheduler_hint,omitempty"`
	Ledger         []QuotaLedgerEntry `json:"quota_ledger,omitempty"`
}

// QuotaDecision is quota_should_run's verdict packet.
type QuotaDecision struct {
	Schema         string `json:"schema"`
	ShouldRun      bool   `json:"should_run"`
	Decision       string `json:"decision"` // run | skip | quota_exhausted
	State          string `json:"state"`
	ReasonCode     string `json:"reason_code,omitempty"`
	RecoveryAction bool   `json:"recovery_action"`
	RecoveryKind   string `json:"recovery_kind,omitempty"`
	SchedulerHint  string `json:"scheduler_hint,omitempty"`
	SpentSlots     int    `json:"spent_slots"`
	AllowedSlots   int    `json:"allowed_slots"`
}

// QuotaShouldRun mirrors quota_should_run: the four wait segments
// in source order, then the slot quota. Any wait hit returns a skip
// (never a run); exhausting the slots returns quota_exhausted.
func QuotaShouldRun(st QuotaState, cfg *QuotaConfig) QuotaDecision {
	ncfg := NormalizeQuotaConfig(cfg)
	gates := []struct {
		on   bool
		code string
	}{
		{st.HealthBlocked, QuotaStateHealthBlocked},
		{st.OperatorGate, QuotaStateOperatorGate},
		{st.EvidenceWait, QuotaStateEvidenceWait},
		{st.FocusWait, QuotaStateFocusWait},
	}
	for _, g := range gates {
		if g.on {
			return QuotaDecision{
				Schema: QuotaSchema, ShouldRun: false, Decision: QuotaDecisionSkip,
				State: g.code, ReasonCode: g.code,
				RecoveryAction: st.RecoveryAction, RecoveryKind: st.RecoveryKind,
				SchedulerHint: st.SchedulerHint,
				SpentSlots:    SpentSlots(st.Ledger), AllowedSlots: ncfg.AllowedSlots,
			}
		}
	}
	used := SpentSlots(st.Ledger)
	eligible := used < ncfg.AllowedSlots
	d := QuotaDecision{
		Schema: QuotaSchema, ShouldRun: eligible, SchedulerHint: st.SchedulerHint,
		SpentSlots: used, AllowedSlots: ncfg.AllowedSlots,
	}
	if eligible {
		d.Decision, d.State = QuotaDecisionRun, QuotaStateEligible
	} else {
		d.Decision, d.State, d.ReasonCode = QuotaDecisionExhausted, QuotaStateExhausted, ReasonQuotaExhausted
	}
	return d
}

// QuotaRecordSpend mirrors record_spend: a spend lands exactly once
// per call and ONLY after a validated writeback; otherwise it is
// refused with spend_without_validated_writeback and the ledger is
// returned unchanged.
func QuotaRecordSpend(ledger []QuotaLedgerEntry, slots int, writebackValidated bool, at string) ([]QuotaLedgerEntry, bool, string) {
	if !writebackValidated {
		return ledger, false, ReasonSpendWithoutValidWB
	}
	if slots < 1 {
		slots = 1
	}
	out := append(append([]QuotaLedgerEntry{}, ledger...), QuotaLedgerEntry{Slots: slots, At: at})
	return out, true, ""
}
