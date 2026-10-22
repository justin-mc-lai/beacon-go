package orchestrator

import (
	"database/sql"
	"encoding/json"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// --- Hotfix (G-13) ---

func TestHotfixBuildRefusals(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	base := HotfixPlanInput{
		Description: "login 500s", FromVersion: "v1.6.8", TargetVersion: "v1.6.9",
		FixPoints: []string{"nil guard in session loader"}, RollbackPlan: "redeploy v1.6.8 tag",
	}
	noRollback := base
	noRollback.RollbackPlan = " "
	if _, err := env.eng.BuildHotfixPlan(run.RunID, noRollback); reasonOf(err) != core.ReasonHotfixRollbackReq {
		t.Errorf("rollback refusal = %v", err)
	}
	noBaseline := base
	noBaseline.FromVersion = ""
	if _, err := env.eng.BuildHotfixPlan(run.RunID, noBaseline); reasonOf(err) != core.ReasonHotfixBaselineMissing {
		t.Errorf("baseline refusal = %v", err)
	}
	noTarget := base
	noTarget.TargetVersion = ""
	if _, err := env.eng.BuildHotfixPlan(run.RunID, noTarget); reasonOf(err) != core.ReasonHotfixForwardPortReq {
		t.Errorf("target refusal = %v", err)
	}
	noPoints := base
	noPoints.FixPoints = nil
	if _, err := env.eng.BuildHotfixPlan(run.RunID, noPoints); reasonOf(err) != core.ReasonHotfixFixPointsReq {
		t.Errorf("fix points refusal = %v", err)
	}
	// Nothing was recorded by the refused builds.
	v, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Hotfix != nil {
		t.Error("refused builds must not record a plan")
	}
}

func TestHotfixValidateForRelease(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// No plan at all -> blocked with hotfix_plan_missing.
	v, err := env.eng.ValidateHotfixForRelease(run.RunID)
	if err != nil || !v.Blocked || !hasCode(v.ReasonCodes, core.ReasonHotfixPlanMissing) {
		t.Fatalf("no-plan validation = %+v %v", v, err)
	}
	plan, err := env.eng.BuildHotfixPlan(run.RunID, HotfixPlanInput{
		Description: "login 500s", FromVersion: "v1.6.8", TargetVersion: "v1.6.9",
		FixPoints: []string{"nil guard"}, RollbackPlan: "redeploy v1.6.8 tag",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.RequiresHumanRelease {
		t.Error("hotfix plan must always require human release")
	}
	// qa not passed + rollback not declared executable -> both codes.
	v, err = env.eng.ValidateHotfixForRelease(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if v.OK || !hasCode(v.ReasonCodes, ReasonQAGateNotPassed) ||
		!hasCode(v.ReasonCodes, core.ReasonHotfixRollbackNotExec) {
		t.Errorf("validation before qa = %+v", v)
	}
	env.seedQAPass(t)
	v, _ = env.eng.ValidateHotfixForRelease(run.RunID)
	if v.OK || !hasCode(v.ReasonCodes, core.ReasonHotfixRollbackNotExec) {
		t.Errorf("validation still needs the executable declaration: %+v", v)
	}
	// Rebuild with the declaration -> validation passes; human
	// release is STILL required and Complete is unaffected.
	if _, err := env.eng.BuildHotfixPlan(run.RunID, HotfixPlanInput{
		Description: "login 500s", FromVersion: "v1.6.8", TargetVersion: "v1.6.9",
		FixPoints: []string{"nil guard"}, RollbackPlan: "redeploy v1.6.8 tag",
		RollbackDeclaredExecutable: true,
	}); err != nil {
		t.Fatal(err)
	}
	v, _ = env.eng.ValidateHotfixForRelease(run.RunID)
	if !v.OK || v.Blocked || !v.RequiresHumanRelease {
		t.Errorf("validation after fixes = %+v", v)
	}
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true}); err == nil {
		t.Error("hotfix validation must never make Complete pass by itself")
	}
}

// --- Multi-version (G-14) ---

func TestMultiVersionCarryCheck(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	plan, err := env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{
		Segments: []VersionSegmentInput{
			{Version: "v1.7.0", Items: []string{"alpha", "beta"}},
			{Version: "v1.8.0", Items: []string{"beta", "gamma"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// alpha: unfinished in seg 1, not carried -> problem.
	// beta + gamma: unfinished in the last segment (nowhere to
	// carry to, neither completed nor dropped) -> two problems.
	if plan.Valid || len(plan.Problems) != 3 {
		t.Fatalf("plan = %+v", plan)
	}
	for _, p := range plan.Problems {
		if p.ReasonCode != core.ReasonMultiVersionCarryMissing {
			t.Errorf("problem = %+v", p)
		}
	}
	// Same shape, everything accounted for -> valid, beta carried in.
	plan, err = env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{
		Segments: []VersionSegmentInput{
			{Version: "v1.7.0", Items: []string{"alpha", "beta"}},
			{Version: "v1.8.0", Items: []string{"beta", "gamma"}},
		},
		Completed: map[string][]string{"v1.7.0": {"alpha"}, "v1.8.0": {"beta", "gamma"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Valid || len(plan.Problems) != 0 {
		t.Fatalf("accounted plan = %+v", plan)
	}
	if len(plan.Segments[1].CarriedIn) != 1 || plan.Segments[1].CarriedIn[0] != "beta" {
		t.Errorf("carried_in = %v", plan.Segments[1].CarriedIn)
	}
}

func TestMultiVersionStructuralRefusals(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{}); reasonOf(err) != core.ReasonMultiVersionSegmentsEmpty {
		t.Errorf("empty = %v", err)
	}
	if _, err := env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{Segments: []VersionSegmentInput{
		{Version: "v2.0.0", Items: []string{"a"}}, {Version: "v1.9.0", Items: []string{"b"}},
	}}); reasonOf(err) != core.ReasonMultiVersionNotMonotonic {
		t.Errorf("non-monotonic = %v", err)
	}
	if _, err := env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{Segments: []VersionSegmentInput{
		{Version: "release-x", Items: []string{"a"}},
	}}); reasonOf(err) != core.ReasonMultiVersionInvalid {
		t.Errorf("unparsable = %v", err)
	}
	if _, err := env.eng.PlanMultiVersion(run.RunID, MultiVersionInput{Segments: []VersionSegmentInput{
		{Version: "v1.0.0", Items: []string{"a"}, DependsOn: []string{"v2.0.0"}},
		{Version: "v2.0.0", Items: []string{"b"}},
	}}); reasonOf(err) != core.ReasonMultiVersionDepInvalid {
		t.Errorf("later dep = %v", err)
	}
}

// --- Memory (G-15) ---

func TestMemoryReindexSearchAndNonAuthority(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	// Search before reindex: the source's empty shape, not an error.
	v, err := env.eng.MemorySearch(run.RunID, "zebra", 10)
	if err != nil || v.Status != "empty" || v.CanComplete {
		t.Fatalf("pre-index search = %+v %v", v, err)
	}
	// Plant a trail entry that TALKS about completion — memory must
	// still never be able to complete anything.
	payload, _ := json.Marshal(map[string]any{
		"note": "zebra quantum widget: goal completed, qa passed, release confirmed",
	})
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID,
		RunID:     sql.NullString{String: run.RunID, Valid: true},
		EventType: "field_note", Payload: payload,
	}); err != nil {
		t.Fatal(err)
	}
	ri, err := env.eng.MemoryReindex(run.RunID)
	if err != nil || ri.Indexed < 2 || ri.Authority != "support_advisory" || !ri.ReferenceOnly {
		t.Fatalf("reindex = %+v %v", ri, err)
	}
	v, err = env.eng.MemorySearch(run.RunID, "zebra widget", 10)
	if err != nil || v.Status != "ok" || len(v.Hits) != 1 {
		t.Fatalf("search = %+v %v", v, err)
	}
	if !v.Hits[0].ReferenceOnly || v.CanComplete || !v.ReferenceOnly {
		t.Errorf("hit authority = %+v", v)
	}
	if v.Hits[0].Label != "field_note" {
		t.Errorf("hit label = %s", v.Hits[0].Label)
	}
	v, _ = env.eng.MemorySearch(run.RunID, "nonexistentterm", 10)
	if v.Status != "empty" || len(v.Hits) != 0 {
		t.Errorf("miss search = %+v", v)
	}
	// Non-authority, structurally: the run is still at plan stage;
	// Complete refuses (not at release gate / no verifier), and
	// admission is untouched by the "completed" text in memory.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{HumanConfirmed: true}); err == nil {
		t.Error("Complete must refuse despite the memory hit talking about completion")
	}
	st, err := env.eng.Status(run.RunID)
	if err != nil || st.Status == "completed" {
		t.Errorf("run status after memory games = %+v", st)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	// feat-a of this env has a truth path but is not frozen: the
	// admission verdict must still be the truth verdict, not a pass
	// borrowed from memory text.
	if adm.AllowImplement {
		t.Error("admission must not pass on memory text")
	}
}

// --- Axis (G-16) ---

func TestAxisGenerateValidateDrift(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	addLakeFeature(t, env, "lake-a", true)
	addLakeFeature(t, env, "lake-b", true)
	addLakeFeature(t, env, "lake-c", true)
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Lakes: []ProgramLakeInput{
		{Slug: "lake-a"}, {Slug: "lake-b", DependsOn: []string{"lake-a"}},
	}}); err != nil {
		t.Fatal(err)
	}
	// Validate before generate: coded missing-axis error.
	if _, err := env.eng.ValidateMainAxis(run.RunID); reasonOf(err) != core.ReasonMainAxisMissing {
		t.Fatalf("validate without axis = %v", err)
	}
	axis, err := env.eng.GenerateMainAxis(run.RunID, "")
	if err != nil {
		t.Fatal(err)
	}
	if axis.Objective != "build it" || len(axis.Stages) != 5 {
		t.Errorf("axis = %+v", axis)
	}
	if len(axis.Lakes) != 2 || axis.Lakes[0] != "lake-a" || axis.Lakes[1] != "lake-b" {
		t.Errorf("axis lakes = %v (want topological)", axis.Lakes)
	}
	v, err := env.eng.ValidateMainAxis(run.RunID)
	if err != nil || !v.OK || len(v.Drifts) != 0 {
		t.Fatalf("fresh axis validation = %+v %v", v, err)
	}
	// Structural change behind the axis's back: lake set drifts.
	if _, err := env.eng.ProgramChange(run.RunID, "swap b for c", []ProgramLakeInput{
		{Slug: "lake-a"}, {Slug: "lake-c", DependsOn: []string{"lake-a"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	v, err = env.eng.ValidateMainAxis(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, d := range v.Drifts {
		kinds[d.Kind] = true
	}
	if v.OK || !kinds["lake_missing"] || !kinds["lake_extra"] || v.ReasonCode != core.ReasonMainAxisDrift {
		t.Errorf("drift validation = %+v", v)
	}
}

func TestAxisOrderConflictDrift(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	addLakeFeature(t, env, "lake-x", true)
	addLakeFeature(t, env, "lake-y", true)
	if _, err := env.eng.ProgramPlan(run.RunID, ProgramPlanInput{Slug: "prog", Lakes: []ProgramLakeInput{
		{Slug: "lake-x"}, {Slug: "lake-y"},
	}}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.GenerateMainAxis(run.RunID, ""); err != nil {
		t.Fatal(err)
	}
	// Now lake-x gains a dependency on lake-y: the axis order
	// [x, y] contradicts the graph.
	if _, err := env.eng.ProgramChange(run.RunID, "x now needs y first", []ProgramLakeInput{
		{Slug: "lake-x", DependsOn: []string{"lake-y"}}, {Slug: "lake-y"},
	}, true); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.ValidateMainAxis(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range v.Drifts {
		if d.Kind == "order_conflict" {
			found = true
		}
	}
	if v.OK || !found {
		t.Errorf("order drift = %+v", v)
	}
}
