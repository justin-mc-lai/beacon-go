package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// AC-001 / TC-001: a manual executor run is created knowingly, status
// never reports stage_executor_not_registered, and each stage is
// completed by an explicit goal tick --by identity.
func TestManualExecutorRunAdvancesViaTick(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", ExecutorID: "room-host",
		ExecutorMode: ExecutorModeManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Blocked || st.ReasonCode == ReasonExecutorNotReg {
		t.Fatalf("manual run must not be reported as an unregistered executor: %+v", st)
	}
	if st.ExecutorMode != ExecutorModeManual || st.TickHint == "" {
		t.Fatalf("status must name the executor and its tick obligation: %+v", st)
	}
	ctx := context.Background()
	// Without an identity there is nothing to record: park with the
	// reason that names the fix — never stage_executor_not_registered.
	v, err := env.eng.Tick(ctx, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonManualTickRequired {
		t.Fatalf("manual tick without --by = %+v, want %s", v, core.ReasonManualTickRequired)
	}
	// The identity is new evidence: the same tick with --by advances.
	v, err = env.eng.TickBy(ctx, run.RunID, "renlink-dev-room")
	if err != nil {
		t.Fatal(err)
	}
	if v.Blocked || v.CurrentStage != StageTruth {
		t.Fatalf("manual tick --by must advance plan -> truth, got %+v", v)
	}
	if v.ReasonCode == ReasonExecutorNotReg {
		t.Fatalf("manual run still reports stage_executor_not_registered")
	}
}

// AC-002: an unknown executor selector is refused before any run row
// exists.
func TestCreateRunRejectsUnknownExecutor(t *testing.T) {
	env := newTestEnv(t, "active")
	_, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", ExecutorMode: "telepathy",
	})
	if err == nil {
		t.Fatal("unknown executor must be refused at creation")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonExecutorUnsupported {
		t.Fatalf("reason = %s, want %s", rc, core.ReasonExecutorUnsupported)
	}
	runs, err := env.st.Runs().ListByFeature(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("refused creation must not leave a run row, got %d", len(runs))
	}
}

// AC-003 / TC-004: every successful stage exit persists an evaluated
// pass on the stage's gate axis; a stage no gate covers parks with
// stage_gate_not_evaluated instead of advancing on a guess.
func TestTickPersistsStageExitVerdict(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	ctx := context.Background()
	if _, err := env.eng.Tick(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
	g, err := env.st.Gates().Latest(env.feature.ID, "plan")
	if err != nil || g.Verdict != "pass" {
		t.Fatalf("plan exit verdict must persist as a pass, got %+v err=%v", g, err)
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType == "stage_exit_evaluated" {
			found = true
		}
	}
	if !found {
		t.Fatal("stage_exit_evaluated record missing from the trail")
	}

	// Negative path: a stage the kernel has no gate axis for cannot be
	// left — the tick parks and names the missing judgement.
	if v := EvaluateStageExit(ExitInput{Stage: "intake"}); v.Evaluated || v.ReasonCode != core.ReasonStageGateNotEvaluated {
		t.Fatalf("unknown stage verdict = %+v, want unevaluated/%s", v, core.ReasonStageGateNotEvaluated)
	}
	unknown := env.createRun(t)
	rraw, m, _, err := env.eng.loadRun(unknown.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.StagePlan[0].Stage = "intake"
	env.eng.RegisterExecutor("intake", env.rec)
	if err := env.eng.saveRun(rraw, m); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.Tick(ctx, unknown.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != core.ReasonStageGateNotEvaluated {
		t.Fatalf("unevaluated stage tick = %+v, want blocked/%s", v, core.ReasonStageGateNotEvaluated)
	}
}

// AC-005 / TC-008: route emits next actions in topological order and a
// truth freeze suggestion naming the lake whose truth is not frozen.
func TestProgramRouteNextActionsTopological(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	addFeature(t, env, "feat-b", "draft")
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "prog", Freeze: true,
		Lakes: []ProgramLakeInput{
			{Slug: "feat-a"},
			{Slug: "feat-b", DependsOn: []string{"feat-a"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.ProgramRoute(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.NextActions) == 0 {
		t.Fatalf("route must emit next actions, got %+v", view)
	}
	indexOf := func(lake string) int {
		for i, a := range view.NextActions {
			if a.Lake == lake {
				return i
			}
		}
		return -1
	}
	ia, ib := indexOf("feat-a"), indexOf("feat-b")
	if ia < 0 || ib < 0 || ia > ib {
		t.Fatalf("next actions must follow topological order (feat-a before feat-b): %+v", view.NextActions)
	}
	var freeze *ProgramNextAction
	for i := range view.NextActions {
		if view.NextActions[i].Lake == "feat-b" {
			freeze = &view.NextActions[i]
		}
	}
	if freeze == nil || freeze.Action != "truth_freeze" || freeze.Command == "" {
		t.Fatalf("feat-b must carry a truth freeze next action, got %+v", view.NextActions)
	}
	if !strings.Contains(freeze.Command, "feat-b") {
		t.Fatalf("truth freeze suggestion must name the lake slug: %q", freeze.Command)
	}
	for i, a := range view.NextActions {
		if a.Order != i+1 {
			t.Fatalf("next action order must be 1..N, got %+v", view.NextActions)
		}
	}
}

// AC-004 / TC-007: an external result writes back onto the lake run and
// program route reports that lake completed.
func TestProgramRouteShowsLakeCompletedAfterExternalResult(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	lakeRun := env.createRun(t)
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "prog", Freeze: true,
		Lakes: []ProgramLakeInput{{Slug: "feat-a"}},
	}); err != nil {
		t.Fatal(err)
	}
	rraw, m, _, err := env.eng.loadRun(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Program.lake("feat-a").RunID = lakeRun.RunID
	if err := env.eng.saveRun(rraw, m); err != nil {
		t.Fatal(err)
	}
	// No artifacts -> refused; the write-back must bring evidence.
	if _, err := env.eng.ReceiveExternalResult(lakeRun.RunID, ExternalResultInput{
		By: "renlink-dev-room", Summary: "lake finished",
	}); err == nil {
		t.Fatal("external result without artifacts must be refused")
	}
	art := filepath.Join(t.TempDir(), "lake-report.md")
	if err := os.WriteFile(art, []byte("# lake report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ReceiveExternalResult(lakeRun.RunID, ExternalResultInput{
		By: "renlink-dev-room", Summary: "lake finished",
		Artifacts: []ArtifactInput{{Kind: "evidence", AbsPath: art}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != "completed" {
		t.Fatalf("externally completed run status = %s", v.Status)
	}
	arts, err := env.st.Artifacts().ListByRun(lakeRun.RunID)
	if err != nil || len(arts) == 0 {
		t.Fatalf("artifact pointer not registered: %+v err=%v", arts, err)
	}
	route, err := env.eng.ProgramRoute(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range route.Lakes {
		if row.Lake == "feat-a" && row.Status == LakeCompleted {
			return
		}
	}
	t.Fatalf("lake must read completed after the external write-back: %+v", route.Lakes)
}
