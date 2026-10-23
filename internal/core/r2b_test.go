package core

import (
	"strings"
	"testing"
)

// ---- H5: RouteReviewDecision, Python's seven branches in order ----

func pkt(route string, findings ...ReviewFinding) ReviewPacket {
	n := len(findings)
	return ReviewPacket{
		ReviewID: "r1", ArtifactRef: "a1", RecommendedRoute: route,
		Findings: findings, DeclaredFindingCount: &n, SynthesisMarker: true,
	}
}

func TestRouteReviewDecisionBranches(t *testing.T) {
	// Incomplete packet -> error_recovery, fail-closed.
	res := RouteReviewDecision(ReviewRouteInput{Packet: ReviewPacket{}, AutoDecide: true})
	if res.NextStage != LoopErrorRecovery || res.ReasonCode != ReasonPlannerReviewPacketIncompl {
		t.Errorf("incomplete: %+v", res)
	}
	// Drift route wins over the P0 check (the source's order).
	res = RouteReviewDecision(ReviewRouteInput{
		Packet: pkt("beacon-gen-change", ReviewFinding{Severity: "P0"}), AutoDecide: true,
	})
	if res.NextStage != LoopDriftRouting || res.ReasonCode != "planner_review_route_beacon_gen_change" {
		t.Errorf("drift: %+v", res)
	}
	// User decision required + cache miss -> wait for the human.
	p := pkt("", ReviewFinding{Severity: "P2"})
	p.UserDecisionRequired = true
	res = RouteReviewDecision(ReviewRouteInput{Packet: p, AutoDecide: true})
	if res.NextStage != LoopErrorRecovery || res.ReasonCode != ReasonPlannerReviewUserDecisionReq {
		t.Errorf("user decision: %+v", res)
	}
	// Cache hit -> falls through to the severity checks (P2 advisory).
	res = RouteReviewDecision(ReviewRouteInput{Packet: p, AutoDecide: true, DecisionCacheHit: true})
	if res.NextStage != LoopStandardExecutionPrecheck || !res.AdvisoryOnly {
		t.Errorf("user decision cached: %+v", res)
	}
	// P0 blocks.
	res = RouteReviewDecision(ReviewRouteInput{
		Packet: pkt("", ReviewFinding{Severity: "P0"}), AutoDecide: true,
	})
	if res.NextStage != LoopErrorRecovery || res.ReasonCode != ReasonPlannerReviewBlockingFinding {
		t.Errorf("p0: %+v", res)
	}
	// P1 with the human gate forced (auto_decide off) -> human gate.
	res = RouteReviewDecision(ReviewRouteInput{
		Packet: pkt("", ReviewFinding{Severity: "P1"}), AutoDecide: false,
	})
	if res.NextStage != LoopErrorRecovery || res.ReasonCode != ReasonPlannerReviewHumanGateP1 {
		t.Errorf("p1 gate: %+v", res)
	}
	// P1 auto-decided on an implement-ready route -> advisory continue.
	res = RouteReviewDecision(ReviewRouteInput{
		Packet: pkt("continue_delivery", ReviewFinding{Severity: "P1"}), AutoDecide: true,
	})
	if res.NextStage != LoopStandardExecutionPrecheck || !res.P1Continue || !res.ContinuedWithAdvisory {
		t.Errorf("p1 advisory: %+v", res)
	}
	// Unhandled route -> error_recovery.
	res = RouteReviewDecision(ReviewRouteInput{
		Packet: pkt("somewhere_else", ReviewFinding{Severity: "P3"}), AutoDecide: true,
	})
	if res.NextStage != LoopErrorRecovery || res.ReasonCode != ReasonPlannerReviewRouteUnhandled {
		t.Errorf("unhandled: %+v", res)
	}
}

// ---- H6: quota normalisation + five segments ----

func TestQuotaNormalizeAndSegments(t *testing.T) {
	cfg := NormalizeQuotaConfig(nil)
	if cfg.Compute != 1.0 || cfg.WindowHours != 24 || cfg.SlotMinutes != 1 || cfg.AllowedSlots != 1440 {
		t.Errorf("defaults: %+v", cfg)
	}
	// Segment order: health beats operator gate beats evidence
	// beats focus beats slots.
	st := QuotaState{HealthBlocked: true, OperatorGate: true}
	if d := QuotaShouldRun(st, nil); d.Decision != "skip" || d.ReasonCode != ReasonHealthBlocked {
		t.Errorf("health: %+v", d)
	}
	st = QuotaState{OperatorGate: true, EvidenceWait: true}
	if d := QuotaShouldRun(st, nil); d.ReasonCode != ReasonOperatorGateOpen {
		t.Errorf("operator: %+v", d)
	}
	st = QuotaState{EvidenceWait: true, FocusWait: true}
	if d := QuotaShouldRun(st, nil); d.ReasonCode != ReasonEvidenceWait {
		t.Errorf("evidence: %+v", d)
	}
	st = QuotaState{FocusWait: true}
	if d := QuotaShouldRun(st, nil); d.ReasonCode != ReasonFocusWait {
		t.Errorf("focus: %+v", d)
	}
	// Slots: allowed 2, one spent -> run; two spent -> exhausted.
	cfg2 := &QuotaConfig{AllowedSlots: 2}
	st = QuotaState{Ledger: []QuotaLedgerEntry{{Slots: 1}}}
	if d := QuotaShouldRun(st, cfg2); !d.ShouldRun || d.SpentSlots != 1 {
		t.Errorf("slots run: %+v", d)
	}
	st = QuotaState{Ledger: []QuotaLedgerEntry{{Slots: 2}}}
	if d := QuotaShouldRun(st, cfg2); d.Decision != "quota_exhausted" || d.ReasonCode != ReasonQuotaExhausted {
		t.Errorf("slots exhausted: %+v", d)
	}
	// Spend without validated writeback is refused.
	if _, ok, code := QuotaRecordSpend(nil, 1, false, "t0"); ok || code != ReasonSpendWithoutValidWB {
		t.Errorf("quota spend guard: %v %s", ok, code)
	}
}

// ---- M9: handoff budget + spend guards ----

func TestHandoffBudgetTruncation(t *testing.T) {
	long := strings.Repeat("line\n", HandoffMaxLines+5)
	res := EnforceHandoffBudget(long)
	if !res.Truncated {
		t.Fatal("over-line handoff must be marked truncated")
	}
	if got := len(pySplitLines(res.Text)); got != HandoffMaxLines {
		t.Errorf("lines after truncation = %d, want %d", got, HandoffMaxLines)
	}
	short := EnforceHandoffBudget("one\ntwo")
	if short.Truncated || short.Text != "one\ntwo" {
		t.Errorf("short handoff must pass untouched: %+v", short)
	}
}

func TestSpendGuardsAndIdempotency(t *testing.T) {
	l := NewSpendLedger("goal-1", 2)
	// Quiet sources never spend (checked first).
	if ok, code := SpendEligible(l, "poll", true, "d1"); ok || code != ReasonQuietSkipNoSpend {
		t.Errorf("quiet: %v %s", ok, code)
	}
	// No validated writeback.
	if ok, code := SpendEligible(l, "delivery_completion", false, "d1"); ok || code != ReasonSpendWithoutWriteback {
		t.Errorf("writeback: %v %s", ok, code)
	}
	// Source outside the whitelist.
	if ok, code := SpendEligible(l, "random_source", true, "d1"); ok || code != ReasonSpendSourceNotAllowed {
		t.Errorf("source: %v %s", ok, code)
	}
	// Happy path + commit.
	l2, replayed, code := CommitSpend(l, "delivery_completion", "d1", "wb-1")
	if replayed || code != "" || l2.Spent != 1 {
		t.Fatalf("commit: %+v %v %s", l2, replayed, code)
	}
	// Same digest again: eligibility conflicts, commit replays.
	if ok, code := SpendEligible(l2, "delivery_completion", true, "d1"); ok || code != ReasonSpendIdempotencyConf {
		t.Errorf("dup eligible: %v %s", ok, code)
	}
	l3, replayed, code := CommitSpend(l2, "delivery_completion", "d1", "wb-1")
	if !replayed || code != ReasonSpendIdempotentReplay || l3.Spent != 1 {
		t.Errorf("dup commit: %+v %v %s", l3, replayed, code)
	}
	// Quota: limit 2 — second spend ok, third refused.
	l4, _, _ := CommitSpend(l2, "repair", "d2", "wb-2")
	if ok, code := SpendEligible(l4, "delivery_completion", true, "d3"); ok || code != ReasonProgramBudgetExhausted {
		t.Errorf("exhausted: %v %s", ok, code)
	}
	halt := HaltOnSpendExhaust(l4)
	if halt.State != "halt_budget" || halt.ResumeAction != "none" {
		t.Errorf("halt packet: %+v", halt)
	}
}
