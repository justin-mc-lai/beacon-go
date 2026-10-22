package orchestrator

import (
	"database/sql"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// addLakeFeature registers one more feature package in the test
// env's project version, optionally with frozen truth.
func addLakeFeature(t *testing.T, env testEnv, slug string, frozen bool) store.FeaturePackage {
	t.Helper()
	f, err := env.st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: env.feature.ProjectVersionID, Slug: slug,
		TruthPath: "truth.md", TestsPath: "tests.md", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if frozen {
		if _, err := env.st.Gates().Record(store.GateResult{
			FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass",
		}); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func reasonOf(err error) string {
	rc, _ := core.ReasonCodeOf(err)
	return rc
}

// --- Graph validation: cycle / self / missing / duplicate (pure) ---

func TestValidateProgramGraphProblems(t *testing.T) {
	if got := ValidateProgramGraph(nil); len(got) != 1 || got[0].ReasonCode != core.ReasonProgramLakesMissing {
		t.Errorf("empty: %v", got)
	}
	cyc := ValidateProgramGraph([]ProgramLakeInput{
		{Slug: "a", DependsOn: []string{"b"}},
		{Slug: "b", DependsOn: []string{"a"}},
	})
	if len(cyc) != 1 || cyc[0].ReasonCode != core.ReasonProgramDependencyCycle {
		t.Errorf("cycle: %v", cyc)
	}
	self := ValidateProgramGraph([]ProgramLakeInput{{Slug: "a", DependsOn: []string{"a"}}})
	if len(self) != 1 || self[0].ReasonCode != core.ReasonProgramDependencySelf {
		t.Errorf("self: %v", self)
	}
	miss := ValidateProgramGraph([]ProgramLakeInput{{Slug: "a", DependsOn: []string{"ghost"}}})
	if len(miss) != 1 || miss[0].ReasonCode != core.ReasonProgramDependencyMissing {
		t.Errorf("missing dep: %v", miss)
	}
	dup := ValidateProgramGraph([]ProgramLakeInput{{Slug: "a"}, {Slug: "a"}})
	if len(dup) != 1 || dup[0].ReasonCode != core.ReasonProgramLakeDuplicate {
		t.Errorf("duplicate: %v", dup)
	}
	ok := ValidateProgramGraph([]ProgramLakeInput{
		{Slug: "a"}, {Slug: "b", DependsOn: []string{"a"}},
	})
	if len(ok) != 0 {
		t.Errorf("valid graph: %v", ok)
	}
}

func TestTopoOrder(t *testing.T) {
	order, ok := TopoOrder([]ProgramLake{
		{Slug: "c", DependsOn: []string{"b"}},
		{Slug: "a"},
		{Slug: "b", DependsOn: []string{"a"}},
	})
	if !ok || len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Errorf("order = %v ok=%v", order, ok)
	}
	if _, ok := TopoOrder([]ProgramLake{
		{Slug: "a", DependsOn: []string{"b"}}, {Slug: "b", DependsOn: []string{"a"}},
	}); ok {
		t.Error("cycle must not topo-sort")
	}
}

// --- Plan / freeze / change ---

func TestProgramPlanFreezeChange(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	lakes := []ProgramLakeInput{
		{Slug: "lake-a"}, {Slug: "lake-b", DependsOn: []string{"lake-a"}},
	}
	// A cyclic plan is refused at the door.
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "p", Lakes: []ProgramLakeInput{
		{Slug: "x", DependsOn: []string{"x"}},
	}}); reasonOf(err) != core.ReasonProgramDependencySelf {
		t.Fatalf("self plan err = %v", err)
	}
	st, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Goal: "big", Lakes: lakes})
	if err != nil {
		t.Fatal(err)
	}
	if st.Frozen || len(st.Lakes) != 2 {
		t.Fatalf("state = %+v", st)
	}
	if _, err := env.eng.ProgramFreeze(run.RunID); err != nil {
		t.Fatal(err)
	}
	// Frozen: direct re-plan refused.
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Lakes: lakes}); reasonOf(err) != core.ReasonProgramPlanFrozen {
		t.Fatalf("frozen re-plan err = %v", err)
	}
	// Change without a note refused.
	if _, err := env.eng.ProgramChange(run.RunID, " ", nil, false); reasonOf(err) != core.ReasonProgramChangeNoteReq {
		t.Fatalf("noteless change err = %v", err)
	}
	// Change with a replacement list works and leaves the record.
	st, err = env.eng.ProgramChange(run.RunID, "swap lake-b for lake-c", []ProgramLakeInput{
		{Slug: "lake-a"}, {Slug: "lake-c", DependsOn: []string{"lake-a"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Changes) != 1 || st.Changes[0].Note == "" || len(st.Lakes) != 2 || st.Lakes[1].Slug != "lake-c" {
		t.Errorf("after change: %+v", st)
	}
	if !st.Frozen {
		t.Error("plan must stay frozen after a change")
	}
	evts, _ := env.st.ListEventsByRun(run.RunID)
	found := false
	for _, ev := range evts {
		if ev.EventType == "program_plan_changed" {
			found = true
		}
	}
	if !found {
		t.Error("program_plan_changed event missing")
	}
}

// --- Admission: per-lake verdicts ---

func TestProgramAdmissionPerLake(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	addLakeFeature(t, env, "lake-a", true)  // frozen, no deps -> admitted
	addLakeFeature(t, env, "lake-b", false) // not frozen -> truth verdict
	addLakeFeature(t, env, "lake-d", true)  // frozen, dep on unclosed lake-b
	addLakeFeature(t, env, "lake-e", true)  // frozen, dep on lake-a (admitted in-pass)
	// lake-c intentionally has NO feature package.
	lakes := []ProgramLakeInput{
		{Slug: "lake-a"},
		{Slug: "lake-b"},
		{Slug: "lake-c"},
		{Slug: "lake-d", DependsOn: []string{"lake-b"}},
		{Slug: "lake-e", DependsOn: []string{"lake-a"}},
	}
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Lakes: lakes}); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.ProgramAdmission(run.RunID, "")
	if err != nil {
		t.Fatal(err)
	}
	byLake := map[string]LakeAdmission{}
	for _, adm := range view.Lakes {
		byLake[adm.Lake] = adm
	}
	if !byLake["lake-a"].Admitted {
		t.Errorf("lake-a should be admitted: %+v", byLake["lake-a"])
	}
	if byLake["lake-b"].Admitted || !hasCode(byLake["lake-b"].ReasonCodes, core.ReasonProgramTruthNotFrozen) {
		t.Errorf("lake-b verdict: %+v", byLake["lake-b"])
	}
	if byLake["lake-c"].Admitted || !hasCode(byLake["lake-c"].ReasonCodes, core.ReasonProgramLakeFeatureMiss) {
		t.Errorf("lake-c verdict: %+v", byLake["lake-c"])
	}
	if byLake["lake-d"].Admitted || !hasCode(byLake["lake-d"].ReasonCodes, core.ReasonProgramDependencyUnclos) {
		t.Errorf("lake-d verdict: %+v", byLake["lake-d"])
	}
	if !byLake["lake-e"].Admitted {
		t.Errorf("lake-e should ride lake-a's in-pass admission: %+v", byLake["lake-e"])
	}
	if view.AllAdmitted {
		t.Error("AllAdmitted must be false")
	}
	// Unknown lake filter is an error, not an empty pass.
	if _, err := env.eng.ProgramAdmission(run.RunID, "nope"); reasonOf(err) != core.ReasonProgramLakeNotInPlan {
		t.Errorf("unknown lake err = %v", err)
	}
}

func hasCode(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

// --- Route: topological next batch + live lake status ---

func TestProgramRoute(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	fa := addLakeFeature(t, env, "lake-a", true)
	addLakeFeature(t, env, "lake-b", true)
	addLakeFeature(t, env, "lake-c", false) // not frozen -> blocked
	lakes := []ProgramLakeInput{
		{Slug: "lake-a"},
		{Slug: "lake-b", DependsOn: []string{"lake-a"}},
		{Slug: "lake-c", DependsOn: []string{"lake-b"}},
	}
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Lakes: lakes}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramAdmission(run.RunID, ""); err != nil {
		t.Fatal(err)
	}
	route, err := env.eng.ProgramRoute(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	byLake := map[string]LakeRoute{}
	for _, row := range route.Lakes {
		byLake[row.Lake] = row
	}
	if byLake["lake-a"].Status != LakeAdmitted || byLake["lake-a"].DriverHint == "" {
		t.Errorf("lake-a route: %+v", byLake["lake-a"])
	}
	if byLake["lake-b"].Status != LakeWaiting {
		t.Errorf("lake-b should wait on lake-a: %+v", byLake["lake-b"])
	}
	if byLake["lake-c"].Status != LakeBlocked || byLake["lake-c"].ReasonCode != core.ReasonProgramTruthNotFrozen {
		t.Errorf("lake-c route: %+v", byLake["lake-c"])
	}
	if len(route.NextBatch) != 1 || route.NextBatch[0] != "lake-a" {
		t.Errorf("next batch = %v", route.NextBatch)
	}
	if route.AllComplete {
		t.Error("nothing is complete yet")
	}
	// Complete lake-a via its lifecycle: route re-derives live.
	if err := env.st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: fa.ID, Stage: core.StageRelease, Status: core.StatusCompleted,
	}); err != nil {
		t.Fatal(err)
	}
	route, err = env.eng.ProgramRoute(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range route.Lakes {
		byLake[row.Lake] = row
	}
	if byLake["lake-a"].Status != LakeCompleted {
		t.Errorf("lake-a after completion: %+v", byLake["lake-a"])
	}
	if byLake["lake-b"].Status != LakeAdmitted {
		t.Errorf("lake-b should be admitted once lake-a closed: %+v", byLake["lake-b"])
	}
	// Route on a run without a program is a coded error.
	plain := env.createRun(t)
	if _, err := env.eng.ProgramRoute(plain.RunID); reasonOf(err) != core.ReasonProgramPlanMissing {
		t.Errorf("route without plan err = %v", err)
	}
	_ = sql.ErrNoRows
}
