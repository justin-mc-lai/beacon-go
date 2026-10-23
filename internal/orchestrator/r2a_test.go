package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// M6: scope_mode is validated at the intent write and derived
// when empty.
func TestUpdateIntentScopeModeValidation(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "scope-box"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "scope-box")
	base := gate.IntentSnapshot{
		Utterance: "给设置页加一个开关", Goal: "g", NonGoals: "n", Constraints: "c",
	}
	bad := base
	bad.ScopeMode = "banana"
	if _, err := env.eng.UpdateIntent(ctx, bad, "tester"); err == nil {
		t.Fatal("out-of-vocabulary scope_mode must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonScopeModeInvalid {
		t.Fatalf("want scope_mode_invalid, got %v", err)
	}
	derived := base
	derived.Utterance = "完整复刻旧系统全部能力，一个不少"
	v, err := env.eng.UpdateIntent(ctx, derived, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v.Snapshot.ScopeMode != "full_parity" {
		t.Fatalf("parity utterance must derive full_parity, got %q", v.Snapshot.ScopeMode)
	}
	plain := base
	v, err = env.eng.UpdateIntent(ctx, plain, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v.Snapshot.ScopeMode != "focused" {
		t.Fatalf("plain utterance must derive focused, got %q", v.Snapshot.ScopeMode)
	}
	explicit := base
	explicit.ScopeMode = "MVP"
	v, err = env.eng.UpdateIntent(ctx, explicit, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if v.Snapshot.ScopeMode != "mvp" {
		t.Fatalf("explicit mode must be normalized and kept, got %q", v.Snapshot.ScopeMode)
	}
}

// M7: opening a truth change marks the materials stale; the
// release gate fails on stale materials; a successful refreeze
// clears the marks.
func TestMaterialsStaleLifecycle(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "stale-box")
	ctx := env.ctx(t, "stale-box")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	if got := env.eng.staleMaterialSurfaces(ctx); len(got) != 0 {
		t.Fatalf("fresh package must have no stale surfaces, got %v", got)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "补充边界口径", "tester"); err != nil {
		t.Fatal(err)
	}
	pkgFiles := env.eng.packageFiles(ctx)
	testsText := readFile(t, filepath.Join(env.root, pkgFiles.Tests.Path))
	tasksText := readFile(t, filepath.Join(env.root, pkgFiles.Tasks.Path))
	for name, text := range map[string]string{"tests": testsText, "tasks": tasksText} {
		scalars, _, ok := gate.ParseFrontmatter(text)
		if !ok || scalars["materials_status"] != "stale" {
			t.Fatalf("%s must be marked stale after change opens, frontmatter=%v", name, scalars)
		}
	}
	stale := env.eng.staleMaterialSurfaces(ctx)
	if len(stale) != 2 {
		t.Fatalf("stale surfaces = %v, want tests+tasks", stale)
	}
	ev, err := env.eng.EvaluateGate(ctx, "release", "", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(ev.ReasonCodes, ","), core.ReasonMaterialsStale) {
		t.Fatalf("release verdict must carry materials_status_stale, got %v", ev.ReasonCodes)
	}
	// A real body edit, then refreeze: marks clear.
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	edited := strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（补充边界）", 1)
	if edited == raw {
		t.Fatal("fixture edit failed")
	}
	if err := os.WriteFile(truthPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RefreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	if got := env.eng.staleMaterialSurfaces(ctx); len(got) != 0 {
		t.Fatalf("refreeze must clear stale marks, still stale: %v", got)
	}
	testsText = readFile(t, filepath.Join(env.root, pkgFiles.Tests.Path))
	scalars, _, _ := gate.ParseFrontmatter(testsText)
	if scalars["materials_status"] != "current" {
		t.Fatalf("tests materials_status = %q, want current", scalars["materials_status"])
	}
}

// H2 integration: a broken FSM graph blocks intent confirmation
// with the graph reason code (not the old keyword check).
func TestIntentFSMGraphBlocksConfirm(t *testing.T) {
	env := newTruthEnv(t)
	if _, err := env.eng.CreateTruthPackage(env.root, "", "graph-box"); err != nil {
		t.Fatal(err)
	}
	ctx := env.ctx(t, "graph-box")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	raw := readFile(t, truthPath)
	raw = strings.Replace(raw, "domain_required: false", "domain_required: true", 1)
	fsm := "| State | Event | Guard | To | Action |\n" +
		"|-------|-------|-------|----|--------|\n" +
		"| idle | start | ok | running | begin |\n" +
		"| running | finish | ok |  | close |\n" +
		"\n### Illegal Transitions\n\n- illegal: idle -> done\n"
	raw = replaceSection(raw, "## FSM 五元组", fsm)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := gate.IntentSnapshot{
		Utterance: "test utterance", Goal: "test goal",
		NonGoals: "test non-goals", Constraints: "test constraints",
		ScopeMode: "focused",
	}
	if _, err := env.eng.UpdateIntent(ctx, snap, "tester"); err != nil {
		t.Fatal(err)
	}
	text, err := readTruthText(ctx)
	if err != nil {
		t.Fatal(err)
	}
	text = replaceSection(text, "## 人话", "测试需求：目标、用户与完成标准。")
	if err := writeTruthText(ctx, text); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range view.OpenQuestions {
		if q.Status == "answered" || q.Status == "dropped" {
			continue
		}
		if _, err := env.eng.AnswerQuestion(ctx, q.ID, "test answer", "answered", "tester"); err != nil {
			t.Fatal(err)
		}
	}
	state, err := env.eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(state.BlockReasons, ","), core.ReasonTruthFSMInvalid) {
		t.Fatalf("block reasons must carry truth_fsm_invalid, got %v", state.BlockReasons)
	}
	_, err = env.eng.ConfirmIntent(ctx, "tester")
	if err == nil {
		t.Fatal("a transition without a target must block confirmation")
	}
	if !strings.Contains(err.Error(), "transition_target_missing") {
		t.Fatalf("confirm error must name the graph finding, got %v", err)
	}
}
