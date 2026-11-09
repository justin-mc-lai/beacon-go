// Kernel recover guard scope: recover only clears evidence-class
// blockers; human gates are cleared by the human confirmation action,
// both paths leave an audit trail, and status tells the truth even
// after a historical recover erased the snapshot.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

func humanGateReasonsForTest() []string {
	return []string{
		core.ReasonReleaseRequiresHuman,
		core.ReasonReleaseAwaitingComplete,
		core.ReasonDriverAwaitingHuman,
		core.ReasonSelfHealNeedsHuman,
		core.ReasonPlannerReviewHumanGateP1,
	}
}

// walkToReleaseGate advances a recording-executor run to release_gate
// and ticks once so the kernel records the human gate blocker.
func walkToReleaseGate(t *testing.T, env gateTestEnv) RunView {
	t.Helper()
	run := env.createRun(t)
	env.seedQAPass(t)
	for i := 0; i < 4; i++ {
		v, err := env.eng.Tick(context.Background(), run.RunID)
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if v.Blocked {
			t.Fatalf("tick %d blocked: %+v", i, v)
		}
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.LoopStage != string(core.LoopReleaseGate) {
		t.Fatalf("status at release = %+v %v", st, err)
	}
	v, err := env.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatalf("release tick: %v", err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("release tick = %+v, want the human gate block", v)
	}
	return v
}

func mustRecover(t *testing.T, env gateTestEnv, runID, by string) RunView {
	t.Helper()
	v, err := env.eng.Recover(runID, by, "the missing evidence landed")
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	return v
}

func mustPayload(t *testing.T, p map[string]any, key string) any {
	t.Helper()
	v, ok := p[key]
	if !ok {
		t.Fatalf("payload misses %q: %v", key, p)
	}
	return v
}

// ---- AC-001 / TC-001 ----

func TestRecoverClearsRecoverableBlockAndResetsStreak(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	v := mustRecover(t, env, run.RunID, "owner")
	if v.Blocked || v.BlockedStreak != 0 || v.LastBlockReason != "" || v.RunStatus != string(core.RunRunning) {
		t.Fatalf("recoverable block not cleared: %+v", v)
	}
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Blocked || m.BlockedStreak != 0 || m.LastBlockReason != "" || m.LastTickKey != "" {
		t.Fatalf("snapshot not reset: %+v", m)
	}
	if m.CurrentStageIdx < len(m.StagePlan) && m.StagePlan[m.CurrentStageIdx].Status != "active" {
		t.Fatalf("stage plan not re-armed: %+v", m.StagePlan[m.CurrentStageIdx])
	}
	if got := payloadsOf(t, env.st, run.RunID, "goal_recovered"); len(got) != 1 {
		t.Fatalf("goal_recovered events = %d, want 1", len(got))
	}
}

// ---- AC-001 / TC-002 ----

func TestRecoverRefusesHumanGateReasonsWithoutStateChange(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	for _, reason := range humanGateReasonsForTest() {
		t.Run(reason, func(t *testing.T) {
			run := env.createRun(t)
			if _, err := env.eng.Block(run.RunID, reason); err != nil {
				t.Fatal(err)
			}
			before, _, _, err := env.eng.loadRun(run.RunID)
			if err != nil {
				t.Fatal(err)
			}
			v, err := env.eng.Recover(run.RunID, "intruder", "trying to clear the human gate")
			if err == nil {
				t.Fatalf("recover on human gate %s must be refused", reason)
			}
			if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonRecoveryHumanGateForbidden {
				t.Fatalf("recover reason = %v, want %s", err, core.ReasonRecoveryHumanGateForbidden)
			}
			if !v.Blocked || v.LastBlockReason != reason {
				t.Fatalf("rejected view = %+v, want the human gate still reported", v)
			}
			after, m, _, err := env.eng.loadRun(run.RunID)
			if err != nil {
				t.Fatal(err)
			}
			if string(before.Metadata) != string(after.Metadata) {
				t.Fatalf("run snapshot changed on refused recover:\nbefore=%s\nafter=%s", before.Metadata, after.Metadata)
			}
			if !m.Blocked || m.BlockedStreak != 1 || m.LastBlockReason != reason || m.ReleaseHumanConfirmed {
				t.Fatalf("refused recover mutated state: %+v", m)
			}
		})
	}
}

// ---- AC-002 / TC-003 ----

func TestHumanConfirmationClearsHumanGateBlock(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := walkToReleaseGate(t, env)
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# independent verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{verifier}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	// Empty identity is refused and writes nothing.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "  "}); err == nil {
		t.Fatal("confirmation without identity must be refused")
	} else if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonReleaseConfirmIDRequired {
		t.Fatalf("empty identity reason = %v, want %s", err, core.ReasonReleaseConfirmIDRequired)
	}
	if got := payloadsOf(t, env.st, run.RunID, "release_confirmed"); len(got) != 0 {
		t.Fatalf("refused confirmation wrote release_confirmed: %v", got)
	}
	if got := payloadsOf(t, env.st, run.RunID, "human_gate_cleared"); len(got) != 0 {
		t.Fatalf("refused confirmation wrote human_gate_cleared: %v", got)
	}
	v, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner"})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if v.Status != "completed" {
		t.Fatalf("status = %s, want completed", v.Status)
	}
	confirmed := payloadsOf(t, env.st, run.RunID, "release_confirmed")
	if len(confirmed) != 1 || confirmed[0]["by"] != "owner" {
		t.Fatalf("release_confirmed = %v", confirmed)
	}
	cleared := payloadsOf(t, env.st, run.RunID, "human_gate_cleared")
	if len(cleared) != 1 {
		t.Fatalf("human_gate_cleared events = %d, want 1", len(cleared))
	}
	if cleared[0]["by"] != "owner" || cleared[0]["stage"] != StageRelease {
		t.Fatalf("human_gate_cleared payload = %v", cleared[0])
	}
	if cleared[0]["prior_reason_code"] != core.ReasonReleaseRequiresHuman {
		t.Fatalf("prior_reason_code = %v", cleared[0]["prior_reason_code"])
	}
	if cleared[0]["prior_blocked_streak"] != float64(1) {
		t.Fatalf("prior_blocked_streak = %v, want 1", cleared[0]["prior_blocked_streak"])
	}
}

// ---- AC-002 / TC-004 ----

func TestRecoverCannotSubstituteForHumanConfirmation(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	if _, err := env.eng.Block(run.RunID, core.ReasonReleaseRequiresHuman); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Recover(run.RunID, "owner", "I say it is done"); err == nil {
		t.Fatal("recover must not substitute for human confirmation")
	}
	if got := payloadsOf(t, env.st, run.RunID, "release_confirmed"); len(got) != 0 {
		t.Fatalf("recover wrote release_confirmed: %v", got)
	}
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Blocked || m.ReleaseHumanConfirmed || m.HumanConfirmedBy != "" {
		t.Fatalf("recover touched confirmation state: %+v", m)
	}
	if m.LastBlockReason != core.ReasonReleaseRequiresHuman || m.BlockedStreak != 1 {
		t.Fatalf("block state changed: %+v", m)
	}
}

// ---- AC-003 / TC-005 ----

func TestRecoverEventCarriesClearReceipt(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	mustRecover(t, env, run.RunID, "owner")
	events := payloadsOf(t, env.st, run.RunID, "goal_recovered")
	if len(events) != 1 {
		t.Fatalf("goal_recovered events = %d, want 1", len(events))
	}
	p := events[0]
	if p["by"] != "owner" {
		t.Errorf("by = %v", p["by"])
	}
	if p["stage"] != StagePlan {
		t.Errorf("stage = %v", p["stage"])
	}
	if p["prior_reason_code"] != "manual_block" {
		t.Errorf("prior_reason_code = %v", p["prior_reason_code"])
	}
	if p["prior_blocked_streak"] != float64(1) {
		t.Errorf("prior_blocked_streak = %v", p["prior_blocked_streak"])
	}
	if p["blocked_after"] != false {
		t.Errorf("blocked_after = %v", p["blocked_after"])
	}
	if p["blocked_streak_after"] != float64(0) {
		t.Errorf("blocked_streak_after = %v", p["blocked_streak_after"])
	}
	if p["decision"] != "allowed" {
		t.Errorf("decision = %v", p["decision"])
	}
	if _, ok := p["note"]; !ok {
		t.Errorf("note missing: %v", p)
	}
}

// ---- AC-003 / TC-006 ----

func TestRejectedRecoverLeavesRejectionTrail(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	if _, err := env.eng.Block(run.RunID, core.ReasonReleaseRequiresHuman); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Recover(run.RunID, "intruder", "bypass attempt"); err == nil {
		t.Fatal("recover on a human gate must be refused")
	}
	rejected := payloadsOf(t, env.st, run.RunID, "goal_recover_rejected")
	if len(rejected) != 1 {
		t.Fatalf("goal_recover_rejected events = %d, want 1", len(rejected))
	}
	p := rejected[0]
	checks := map[string]any{
		"by":                          "intruder",
		"stage":                       StagePlan,
		"attempted_reason_code":       core.ReasonReleaseRequiresHuman,
		"refusal_reason_code":         core.ReasonRecoveryHumanGateForbidden,
		"blocked_before":              true,
		"blocked_after":               true,
		"blocked_streak_before":       float64(1),
		"blocked_streak_after":        float64(1),
		"human_confirmation_required": true,
		"decision":                    "rejected",
	}
	for key, want := range checks {
		if got := mustPayload(t, p, key); got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
	if got := payloadsOf(t, env.st, run.RunID, "goal_recovered"); len(got) != 0 {
		t.Fatalf("refused recover wrote goal_recovered: %v", got)
	}
}

// ---- AC-004 / TC-007 ----

func TestStatusShowsHumanGateBlockTruthfully(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := walkToReleaseGate(t, env)
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Blocked || st.RunStatus != string(core.RunPaused) || st.BlockedStreak != 1 ||
		st.LastBlockReason != core.ReasonReleaseRequiresHuman {
		t.Fatalf("snapshot-blocked status = %+v", st)
	}
	// Simulate the historical defect: recover cleared the snapshot but
	// the stage_exit_blocked event is still on the trail and no
	// release_confirmed supersedes it.
	row, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Blocked, m.BlockedStreak, m.LastBlockReason = false, 0, ""
	if err := env.eng.saveRun(row, m); err != nil {
		t.Fatal(err)
	}
	st, err = env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Blocked || st.RunStatus != string(core.RunPaused) || st.BlockedStreak != 1 ||
		st.LastBlockReason != core.ReasonReleaseRequiresHuman || st.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("reconstructed status = %+v", st)
	}
}

// ---- AC-004 / TC-008 ----

func TestRecoverAndAttachVerifierDoNotHideHumanGateBlock(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := walkToReleaseGate(t, env)
	before, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.Recover(run.RunID, "intruder", "bypass attempt"); err == nil {
		t.Fatal("recover on a human gate must be refused")
	}
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{verifier}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	after, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Blocked != after.Blocked || before.BlockedStreak != after.BlockedStreak ||
		before.LastBlockReason != after.LastBlockReason || before.RunStatus != after.RunStatus {
		t.Fatalf("block truth changed after refused recover + attach-verifier:\nbefore=%+v\nafter=%+v", before, after)
	}
	if !after.Blocked || after.RunStatus != string(core.RunPaused) || after.LastBlockReason != core.ReasonReleaseRequiresHuman {
		t.Fatalf("human gate hidden: %+v", after)
	}
}

// ---- AC-002 / AC-004 / TC-009 ----

// A second human confirmation after the release gate re-blocked the
// run must leave the confirmation fact the status reconstruction
// compares against (release_confirmed). Writing that fact only on the
// first confirmation let the later human_gate_cleared go unseen and
// rebuilt a completed run as blocked (spec/AC QA finding on 21047a4).
func TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
	ctx := context.Background()
	env.seedQAPass(t)
	for i := 0; i < 4; i++ {
		v, err := env.eng.TickBy(ctx, run.RunID, "room-first")
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		if v.Blocked {
			t.Fatalf("tick %d blocked: %+v", i, v)
		}
	}
	v, err := env.eng.TickBy(ctx, run.RunID, "room-first")
	if err != nil {
		t.Fatalf("release tick: %v", err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("release tick = %+v, want the human gate block", v)
	}
	// First confirmation clears the gate; Complete still fails on the
	// missing verifier, but the confirmation fact is on the trail.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner-1"}); err == nil {
		t.Fatal("complete without a verifier must be refused")
	} else if rc, ok := core.ReasonCodeOf(err); !ok || rc != ReasonVerifierRequired {
		t.Fatalf("first completion reason = %v, want verifier required", err)
	}
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# independent verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{verifier}, "independent-verifier", ""); err != nil {
		t.Fatal(err)
	}
	// A different tick identity is new evidence: the release gate
	// judges again and records a second human-gate block.
	v, err = env.eng.TickBy(ctx, run.RunID, "room-second")
	if err != nil {
		t.Fatalf("reblock tick: %v", err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonReleaseRequiresHuman {
		t.Fatalf("reblock tick = %+v, want the human gate block again", v)
	}
	v, err = env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner-2"})
	if err != nil {
		t.Fatalf("second Complete: %v", err)
	}
	if v.Status != "completed" {
		t.Fatalf("complete status = %s, want completed", v.Status)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "completed" || st.Blocked || st.LastBlockReason != "" || st.RunStatus != string(core.RunSucceeded) {
		t.Fatalf("completed run reported as blocked: %+v", st)
	}
	if got := payloadsOf(t, env.st, run.RunID, "release_confirmed"); len(got) != 2 {
		t.Fatalf("release_confirmed events = %d, want 2 (one per confirmation)", len(got))
	}
	if got := payloadsOf(t, env.st, run.RunID, "human_gate_cleared"); len(got) != 2 {
		t.Fatalf("human_gate_cleared events = %d, want 2", len(got))
	}
}
