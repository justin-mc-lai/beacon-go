// program drive (issue #7): route rollup + binding survive; a
// blocked lake never advances its dependents; a bound lake run is
// reused instead of duplicated.
package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func writePackage(t *testing.T, root, truthPath, slug, status string) {
	t.Helper()
	dir := filepath.Join(root, filepath.Dir(truthPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", "---\nslug: "+slug+"\nstatus: "+status+"\n---\n# Truth\n\n- **AC-001**：x\n")
	write("tests.md", "---\nslug: "+slug+"\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | unit | `true` | ok |\n")
	write("tasks.md", "---\nslug: "+slug+"\n---\n- [ ] TASK-001 · ac=AC-001\n")
	write("evidence.md", "# Evidence\n\n## Evidence Index\n\n| Surface | Authority | Canonical Artifact | Status | Route |\n|---|---|---|---|---|\n| truth | requirement_truth | truth.md | frozen | truth |\n| tests | test_truth | tests.md | frozen | qa |\n")
}

func addFeature(t *testing.T, env testEnv, slug, status string) store.FeaturePackage {
	t.Helper()
	f, err := env.st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: env.feature.ProjectVersionID,
		Slug:             slug,
		TruthPath:        "docs/beacon/v2.0.0/features/" + slug + "/truth.md",
		TestsPath:        "docs/beacon/v2.0.0/features/" + slug + "/tests.md",
		Status:           status,
	})
	if err != nil {
		t.Fatal(err)
	}
	writePackage(t, env.root, "docs/beacon/v2.0.0/features/"+slug+"/truth.md", slug, status)
	return f
}

func TestProgramRouteRollupAndBinding(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	addFeature(t, env, "feat-b", "frozen")
	lakeRun := env.createRun(t)
	r, m, _, err := env.eng.loadRun(lakeRun.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.LoopStage = string(core.LoopReleaseGate)
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true,
		Lakes: []ProgramLakeInput{
			{Slug: "feat-a"},
			{Slug: "feat-b", DependsOn: []string{"feat-a"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	r, m, _, err = env.eng.loadRun(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Program.lake("feat-a").RunID = lakeRun.RunID
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}

	route, err := env.eng.ProgramRoute(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var a *LakeRoute
	for i := range route.Lakes {
		if route.Lakes[i].Lake == "feat-a" {
			a = &route.Lakes[i]
		}
	}
	if a == nil {
		t.Fatalf("feat-a missing from route: %+v", route.Lakes)
	}
	if a.GoalRunID != lakeRun.RunID || a.RunStage != string(core.LoopReleaseGate) || a.ProgramStatus != "awaiting_release" {
		t.Fatalf("feat-a route = %+v", a)
	}
	if route.Rollup.AwaitingRelease != 1 || route.ProgramStatus != "awaiting_release" {
		t.Fatalf("rollup = %+v status = %s", route.Rollup, route.ProgramStatus)
	}
}

func TestProgramDriveBlockedKeepsDependentsPending(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	addFeature(t, env, "feat-b", "draft") // dependency is blocked by truth
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true,
		Lakes: []ProgramLakeInput{
			{Slug: "feat-a"},
			{Slug: "feat-b", DependsOn: []string{"feat-a"}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{Executor: "recording"})
	if err != nil {
		t.Fatal(err)
	}
	if v.AdvancedLake != "feat-a" || v.LakeStatus != "blocked" || v.ReasonCode == "" {
		t.Fatalf("drive = %+v", v)
	}
	route, err := env.eng.ProgramRoute(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range route.Lakes {
		if row.Lake == "feat-b" && row.ProgramStatus == "completed" {
			t.Fatalf("blocked dependency must not advance feat-b: %+v", row)
		}
	}
}

func TestProgramDriveReusesBoundLakeRun(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	lakeRun := env.createRun(t)
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true, Lakes: []ProgramLakeInput{{Slug: "feat-a"}},
	}); err != nil {
		t.Fatal(err)
	}
	r, m, _, err := env.eng.loadRun(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Program.lake("feat-a").RunID = lakeRun.RunID
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	before, err := env.st.Runs().ListByFeature(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{Executor: "recording"}); err != nil {
		t.Fatal(err)
	}
	after, err := env.st.Runs().ListByFeature(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("drive created a duplicate Lake run: before=%d after=%d", len(before), len(after))
	}
}

func TestProgramDriveReturnsAwaitingRelease(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	lakeRun := env.createRun(t)
	r, m, _, err := env.eng.loadRun(lakeRun.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.LoopStage = string(core.LoopReleaseGate)
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true, Lakes: []ProgramLakeInput{{Slug: "feat-a"}},
	}); err != nil {
		t.Fatal(err)
	}
	r, m, _, err = env.eng.loadRun(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Program.lake("feat-a").RunID = lakeRun.RunID
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{Executor: "recording"})
	if err != nil {
		t.Fatal(err)
	}
	if v.LakeStatus != "awaiting_release" || v.ProgramStatus != "awaiting_release" {
		t.Fatalf("drive = %+v", v)
	}
}

func TestProgramDriveDeliveredWhenAllComplete(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	fb := addFeature(t, env, "feat-b", "frozen")
	host := env.createRun(t)
	// CreateRun resets the host feature's lifecycle; mark both
	// features completed afterwards.
	for _, id := range []string{env.feature.ID, fb.ID} {
		if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
			FeatureID: id, Stage: core.StageRelease, Status: core.StatusCompleted,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "p", Freeze: true,
		Lakes: []ProgramLakeInput{{Slug: "feat-a"}, {Slug: "feat-b"}},
	}); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ProgramDrive(host.RunID, ProgramDriveOptions{Executor: "recording"})
	if err != nil {
		t.Fatal(err)
	}
	if v.ProgramStatus != "delivered" {
		t.Fatalf("drive = %+v", v)
	}
}
