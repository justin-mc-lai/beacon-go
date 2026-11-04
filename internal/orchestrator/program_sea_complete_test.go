// program-sea-complete: one command drives every lake to its gate,
// one program-level human signature releases them together.
package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func createRunFor(t *testing.T, env testEnv, slug string) string {
	t.Helper()
	v, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: slug, Utterance: "run " + slug, ExecutorID: "host-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	return v.RunID
}

func setRunStage(t *testing.T, env testEnv, runID, stage string) {
	t.Helper()
	r, m, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	m.LoopStage = stage
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
}

func bindLake(t *testing.T, env testEnv, hostID, slug, runID string) {
	t.Helper()
	r, m, _, err := env.eng.loadRun(hostID)
	if err != nil {
		t.Fatal(err)
	}
	m.Program.lake(slug).RunID = runID
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
}

func setupSea(t *testing.T, statusA, statusB string) (testEnv, string, store.FeaturePackage) {
	t.Helper()
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", statusA)
	fb := addFeature(t, env, "feat-b", statusB)
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "sea", Freeze: true,
		Lakes: []ProgramLakeInput{
			{Slug: "feat-a"},
			{Slug: "feat-b", DependsOn: []string{"feat-a"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return env, host.RunID, fb
}

func TestProgramDriveUntilDoneStartsDependentWhenDependencyIsAtGate(t *testing.T) {
	env, hostID, _ := setupSea(t, "frozen", "frozen")
	runA := createRunFor(t, env, "feat-a")
	setRunStage(t, env, runA, string(core.LoopReleaseGate))
	runB := createRunFor(t, env, "feat-b")
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)

	order, _, err := env.eng.driveOrder(hostID)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != "feat-b" {
		t.Fatalf("release-ready dependency must unlock feat-b, got %v", order)
	}
}

func TestProgramDriveUntilDoneSkipsAwaitingAndContinues(t *testing.T) {
	env, hostID, _ := setupSea(t, "frozen", "frozen")
	runA := createRunFor(t, env, "feat-a")
	runB := createRunFor(t, env, "feat-b")
	setRunStage(t, env, runA, string(core.LoopReleaseGate))
	setRunStage(t, env, runB, string(core.LoopReleaseGate))
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)

	v, err := env.eng.ProgramDrive(hostID, ProgramDriveOptions{UntilDone: true, Executor: "recording"})
	if err != nil {
		t.Fatal(err)
	}
	if v.AdvancedLake != "" || v.ProgramStatus != "awaiting_release" {
		t.Fatalf("all at gate must return awaiting_release without advancing, got %+v", v)
	}
}

func TestProgramDriveUntilDoneStopsOnBlocked(t *testing.T) {
	env, hostID, _ := setupSea(t, "frozen", "draft")
	runA := createRunFor(t, env, "feat-a")
	runB := createRunFor(t, env, "feat-b")
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)

	v, err := env.eng.ProgramDrive(hostID, ProgramDriveOptions{UntilDone: true, Executor: "recording"})
	if err != nil {
		t.Fatal(err)
	}
	if v.AdvancedLake != "feat-a" || v.LakeStatus != "blocked" || v.ReasonCode == "" {
		t.Fatalf("blocked lake must stop the sea, got %+v", v)
	}
}

func TestProgramCompleteSingleSignature(t *testing.T) {
	env, hostID, fb := setupSea(t, "frozen", "frozen")
	runA := createRunFor(t, env, "feat-a")
	runB := createRunFor(t, env, "feat-b")
	setRunStage(t, env, runA, string(core.LoopReleaseGate))
	setRunStage(t, env, runB, string(core.LoopReleaseGate))
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "qa", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: fb.ID, GateType: "qa", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(t.TempDir(), "qa.txt")
	if err := os.WriteFile(ref, []byte("qa ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{runA, runB} {
		if _, err := env.eng.AttachVerifier(runID, []string{ref}, "qa-bot", "host-1"); err != nil {
			t.Fatal(err)
		}
	}

	v, err := env.eng.ProgramComplete(hostID, "lai")
	if err != nil {
		t.Fatal(err)
	}
	if v.ProgramStatus != "delivered" || v.Route.Rollup.Completed != 2 {
		t.Fatalf("complete = %+v", v)
	}
	for _, runID := range []string{runA, runB} {
		_, m, _, err := env.eng.loadRun(runID)
		if err != nil {
			t.Fatal(err)
		}
		if m.LoopStage != string(core.LoopDone) {
			t.Fatalf("lake run %s not done: %s", runID, m.LoopStage)
		}
	}
	events, err := env.st.ListEvents(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType == "program_release_confirmed" {
			found = true
		}
	}
	if !found {
		t.Fatal("program_release_confirmed event missing")
	}
}

func TestProgramCompleteRefusesLakeNotAtGate(t *testing.T) {
	env, hostID, _ := setupSea(t, "frozen", "frozen")
	runA := createRunFor(t, env, "feat-a")
	runB := createRunFor(t, env, "feat-b")
	setRunStage(t, env, runA, string(core.LoopReleaseGate))
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)

	if _, err := env.eng.ProgramComplete(hostID, "lai"); err == nil {
		t.Fatal("lake not at its gate must refuse the whole-sea confirmation")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProgramLakeNotReady {
		t.Fatalf("reason = %q", rc)
	}
}

func TestProgramCompleteRefusesBlockedLake(t *testing.T) {
	env, hostID, _ := setupSea(t, "frozen", "draft")
	runA := createRunFor(t, env, "feat-a")
	runB := createRunFor(t, env, "feat-b")
	setRunStage(t, env, runA, string(core.LoopReleaseGate))
	setRunStage(t, env, runB, string(core.LoopReleaseGate))
	bindLake(t, env, hostID, "feat-a", runA)
	bindLake(t, env, hostID, "feat-b", runB)

	if _, err := env.eng.ProgramComplete(hostID, "lai"); err == nil {
		t.Fatal("blocked lake must refuse the whole-sea confirmation")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProgramBlocked {
		t.Fatalf("reason = %q", rc)
	}
}
