package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// A package cannot be signed until the author records intent, closes
// every question in the ledger, and confirms.
func TestIntentFreezeBlockedUntilConfirmed(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "ask-first"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "ask-first")

	pre, err := env.eng.FreezeTruthPackage(ctx, "tester")
	if err == nil || !pre.Blocked {
		t.Fatalf("freeze before intent must block, got err=%v view=%+v", err, pre)
	}
	if !containsCode(pre.BlockReasons, core.ReasonIntentQuestionOpen) &&
		!containsCode(pre.BlockReasons, core.ReasonIntentSnapshotIncomple) {
		t.Fatalf("block reasons = %v, want intent findings", pre.BlockReasons)
	}

	// The seeded Q1 is the one question to answer first.
	state, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.OpenQuestions) == 0 {
		t.Fatal("skeleton must seed one open question")
	}
	if _, err := env.eng.AnswerQuestion(ctx, state.OpenQuestions[0].ID, "one clear goal", "answered", "tester"); err != nil {
		t.Fatal(err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	if err := os.WriteFile(truthPath, []byte(replaceSection(raw, "## 人话", "测试需求：目标、用户与完成标准。")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.UpdateIntent(ctx, gate.IntentSnapshot{
		Utterance: "one clear goal", Goal: "deliver it", NonGoals: "nothing else",
		Constraints: "local only", ScopeMode: "focused",
	}, "tester"); err != nil {
		t.Fatal(err)
	}
	// UpdateIntent invalidates the sign-off; confirm must be explicit.
	if _, err := env.eng.ConfirmIntent(ctx, "tester"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if v, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil || !v.Frozen {
		t.Fatalf("freeze after sign-off: %v %+v", err, v)
	}
}

func TestIntentGateNamesMissingPieces(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "gate-pieces"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "gate-pieces")
	state, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(state.BlockReasons, "\n")
	for _, want := range []string{core.ReasonIntentQuestionOpen, core.ReasonIntentSnapshotIncomple, core.ReasonIntentCoverageUntraced} {
		if !strings.Contains(joined, want) {
			t.Fatalf("reasons missing %s:\n%s", want, joined)
		}
	}
}

func TestIntentFSMRequiredWhenDomain(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "fsm-box"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "fsm-box")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	raw = strings.Replace(raw, "domain_required: false", "domain_required: true", 1)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(state.BlockReasons, "\n"), core.ReasonIntentFSMMissing) {
		t.Fatalf("domain package must require an FSM, reasons=%v", state.BlockReasons)
	}
	fsm := "| State | Event | Guard | To | Action |\n" +
		"|-------|-------|-------|----|--------|\n" +
		"| idle | start | ok | running | begin |\n" +
		"| running | error | retry<3 | running | retry |\n" +
		"| running | hang | no heartbeat | hung | mark hang |\n" +
		"| hung | stop | human | idle | illegal inbound blocked |\n" +
		"\n### Illegal Transitions\n\n- illegal: idle -> hung\n"
	raw = readFile(t, truthPath)
	raw = replaceSection(raw, "## FSM 五元组", fsm)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err = env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(state.BlockReasons, "\n"), core.ReasonIntentFSMMissing) {
		t.Fatalf("FSM still reported missing: %v", state.BlockReasons)
	}
}

// Drift names the acceptance criterion that moved, not just "the file
// changed".
func TestTruthDriftNamesChangedRequirement(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "drift-box")
	ctx := env.ctx(t, "drift-box")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	edited := strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（改）", 1)
	if edited == raw {
		t.Fatal("fixture did not change AC-001")
	}
	if err := os.WriteFile(truthPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := env.eng.TruthDrift(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if d.Clean || !d.BodyDrift {
		t.Fatalf("drift = %+v, want unclean body drift", d)
	}
	if len(d.Changed) != 1 || d.Changed[0] != "AC-001" {
		t.Fatalf("changed = %v, want [AC-001]", d.Changed)
	}
}

// A change authored against an older frozen body is refused until the
// author explicitly rebases onto the landed change.
func TestParallelChangeBaseDivergence(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "parallel-box")
	ctx := env.ctx(t, "parallel-box")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	// Change A lands.
	ca, err := env.eng.RequestTruthChange(ctx, "A widens AC-001", "alice")
	if err != nil {
		t.Fatal(err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	if err := os.WriteFile(truthPath, []byte(strings.Replace(raw, "作者写下", "A 作者写下", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RefreezeTruthPackage(ctx, "alice"); err != nil {
		t.Fatalf("refreeze A: %v", err)
	}
	_ = ca

	// Change B is opened on the new base, then we simulate it having
	// been authored before A landed by pointing its base at the old
	// frozen body.
	cb, err := env.eng.RequestTruthChange(ctx, "B edits AC-001", "bob")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an authoring base from before A landed (the normal
	// CLI cannot open two pending changes; this is the defensive
	// case for out-of-band edits / parallel agents).
	oldBase := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	if err := env.eng.appendFeatureEvent(ctx.Feature.ID, EventTruthChangeRequested, map[string]any{
		"change_id": cb.ChangeID, "base_truth_sha256": oldBase,
	}); err != nil {
		t.Fatal(err)
	}
	raw = readFile(t, truthPath)
	if err := os.WriteFile(truthPath, []byte(strings.Replace(raw, "A 作者写下", "B 作者写下", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = env.eng.RefreezeTruthPackage(ctx, "bob")
	if err == nil || !strings.Contains(err.Error(), core.ReasonTruthBaseDiverged) {
		t.Fatalf("refreeze with stale base = %v, want %s", err, core.ReasonTruthBaseDiverged)
	}
	if _, err := env.eng.RebaseChange(ctx, cb.ChangeID, false, "", "bob"); err == nil {
		t.Fatal("rebase without --accept-current must fail")
	}
	if _, err := env.eng.RebaseChange(ctx, cb.ChangeID, true, "merged A", "bob"); err != nil {
		t.Fatalf("rebase: %v", err)
	}
	if _, err := env.eng.RefreezeTruthPackage(ctx, "bob"); err != nil {
		t.Fatalf("refreeze after rebase: %v", err)
	}
}

func containsCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}
