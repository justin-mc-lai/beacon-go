package orchestrator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// advanceToImplement walks a fresh run through plan and truth (truth
// must be pre-frozen by the caller) and returns the run at implement.
func advanceToImplement(t *testing.T, env testEnv) RunView {
	t.Helper()
	run := env.createRun(t)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		run = v
	}
	if run.CurrentStage != StageImplement {
		t.Fatalf("stage = %s, want implement (%+v)", run.CurrentStage, run)
	}
	return run
}

func admissionItem(res AdmissionResult, check string) AdmissionItem {
	for _, it := range res.Items {
		if it.Check == check {
			return it
		}
	}
	return AdmissionItem{}
}

// --- Lane routing: explicit / scale / lakes / default (G-22) ---

func TestRouteLaneThreeWays(t *testing.T) {
	// Explicit wins over everything.
	d, err := RouteLane(LaneRequest{Explicit: "team", Scale: "large"}, 3)
	if err != nil || d.Lane != LaneTeam || d.Source != LaneSourceExplicit {
		t.Errorf("explicit: %+v %v", d, err)
	}
	// Unknown explicit lane is refused, not defaulted.
	if _, err := RouteLane(LaneRequest{Explicit: "swarm"}, 0); err == nil {
		t.Error("unknown lane must error")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLaneUnknown {
		t.Errorf("reason = %s", rc)
	}
	// Scale field: large -> ralph.
	d, err = RouteLane(LaneRequest{Scale: "large"}, 1)
	if err != nil || d.Lane != LaneRalph || d.Source != LaneSourceScale {
		t.Errorf("scale: %+v %v", d, err)
	}
	// Several lakes -> team.
	d, err = RouteLane(LaneRequest{}, 2)
	if err != nil || d.Lane != LaneTeam || d.Source != LaneSourceLakes {
		t.Errorf("lakes: %+v %v", d, err)
	}
	// Nothing said -> single (v1 default runner).
	d, err = RouteLane(LaneRequest{Scale: "small"}, 1)
	if err != nil || d.Lane != LaneSingle || d.Source != LaneSourceDefault {
		t.Errorf("default: %+v %v", d, err)
	}
}

// --- Team handoff validation (pure) ---

func TestValidateTeamHandoffMissing(t *testing.T) {
	if got := ValidateTeamHandoff(nil); len(got) != 3 {
		t.Errorf("nil handoff missing = %v, want from/to/artifact_refs", got)
	}
	partial := &TeamHandoff{From: "impl-team"}
	if got := ValidateTeamHandoff(partial); len(got) != 2 {
		t.Errorf("partial missing = %v, want to/artifact_refs", got)
	}
	whole := &TeamHandoff{From: "impl-team", To: "qa-team", ArtifactRefs: []string{"/tmp/a.go"}}
	if got := ValidateTeamHandoff(whole); len(got) != 0 {
		t.Errorf("whole missing = %v, want none", got)
	}
}

// --- Lane execution + harvest: one rule for all three lanes ---

func TestLaneHarvestSharedRule(t *testing.T) {
	for _, lane := range []string{LaneSingle, LaneTeam, LaneRalph} {
		t.Run(lane, func(t *testing.T) {
			env := newTestEnv(t, "active")
			env.seedTruthFrozen(t)
			env.eng.RegisterLaneExecutorForAllLanes(env.rec)
			run := advanceToImplement(t, env)
			ctx := context.Background()
			out, err := env.eng.ExecuteLaneStage(ctx, run.RunID, lane)
			if err != nil || out.Status != "done" {
				t.Fatalf("ExecuteLaneStage: %+v %v", out, err)
			}
			art := filepath.Join(t.TempDir(), "out.go")
			in := LaneResultInput{Status: "done", Artifacts: []ArtifactInput{{AbsPath: art}}}
			if lane == LaneTeam {
				in.Handoff = &TeamHandoff{From: "impl", To: "qa", ArtifactRefs: []string{art}}
			}
			// No artifacts brought back -> not collected.
			noArt := in
			noArt.Artifacts = nil
			if lane == LaneTeam {
				noArt.Handoff = &TeamHandoff{From: "impl", To: "qa", ArtifactRefs: []string{art}}
			}
			v, err := env.eng.HarvestLaneResult(run.RunID, lane, noArt)
			if err == nil || v.Accepted {
				t.Fatalf("harvest without artifacts accepted: %+v", v)
			}
			if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactsMissing {
				t.Errorf("reason = %s", rc)
			}
			// Whole result -> collected, for every lane alike.
			v, err = env.eng.HarvestLaneResult(run.RunID, lane, in)
			if err != nil || !v.Accepted || !v.EvidenceFound {
				t.Fatalf("harvest whole: %+v %v", v, err)
			}
		})
	}
}

func TestTeamHandoffRejectedInHarvest(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.eng.RegisterLaneExecutorForAllLanes(env.rec)
	run := advanceToImplement(t, env)
	if _, err := env.eng.ExecuteLaneStage(context.Background(), run.RunID, LaneTeam); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(t.TempDir(), "out.go")
	v, err := env.eng.HarvestLaneResult(run.RunID, LaneTeam, LaneResultInput{
		Status: "done", Artifacts: []ArtifactInput{{AbsPath: art}},
		Handoff: &TeamHandoff{From: "impl", ArtifactRefs: []string{art}}, // no "to"
	})
	if err == nil || v.Accepted {
		t.Fatalf("incomplete handoff accepted: %+v", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonTeamHandoffIncomplete {
		t.Errorf("reason = %s", rc)
	}
	if len(v.Missing) != 1 || v.Missing[0] != "to" {
		t.Errorf("missing = %v, want [to]", v.Missing)
	}
}

func TestExecuteLaneStageUnregisteredBlocks(t *testing.T) {
	env := newTestEnv(t, "active") // stage executors only, no lane executors
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	_, err := env.eng.ExecuteLaneStage(context.Background(), run.RunID, LaneSingle)
	if err == nil {
		t.Fatal("unregistered lane executor must fail")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonExecutorNotRegistered {
		t.Errorf("reason = %s", rc)
	}
}

// --- Delegate-ralph (G-10) ---

func TestDelegateRalphSimulatedFullFlow(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.eng.RegisterLaneExecutorForAllLanes(env.rec)
	run := advanceToImplement(t, env)
	ctx := context.Background()

	dv, err := env.eng.DelegateRalph(ctx, run.RunID, true)
	if err != nil {
		t.Fatalf("DelegateRalph: %v", err)
	}
	d := dv.Delegation
	if d.Status != DelegationDelegated || !d.Simulated {
		t.Fatalf("delegation = %+v", d)
	}
	// The pack is whole: truth pointer, stage plan, admission, rules.
	if d.Package.TruthPath == "" || !strings.Contains(d.Package.TruthPath, "truth.md") {
		t.Errorf("package truth path = %q", d.Package.TruthPath)
	}
	if len(d.Package.StagePlan) != 5 || !d.Package.Admission.AllowImplement {
		t.Errorf("package = %+v", d.Package)
	}
	if len(d.Package.Acceptance) == 0 || len(d.Package.Rollback) == 0 {
		t.Errorf("package rules missing: %+v", d.Package)
	}
	// A second delegation while one is out is refused.
	if _, err := env.eng.DelegateRalph(ctx, run.RunID, false); err == nil {
		t.Error("second active delegation must fail")
	}
	// Return without artifacts: evidence exists (simulated) but the
	// harvest is unwhole -> rejected, delegation stays out.
	_, err = env.eng.ReceiveRalphResult(run.RunID, d.ID, LaneResultInput{Status: "done"})
	if err == nil {
		t.Fatal("return without artifacts must be rejected")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactsMissing {
		t.Errorf("reason = %s", rc)
	}
	// Whole return -> returned; receiving never completes the run.
	art := filepath.Join(t.TempDir(), "impl.go")
	dv, err = env.eng.ReceiveRalphResult(run.RunID, d.ID, LaneResultInput{
		Status: "done", Summary: "did it", Artifacts: []ArtifactInput{{AbsPath: art}},
	})
	if err != nil {
		t.Fatalf("ReceiveRalphResult: %v", err)
	}
	if dv.Delegation.Status != DelegationReturned || !dv.Delegation.Simulated {
		t.Errorf("delegation after return = %+v", dv.Delegation)
	}
	if dv.Run.Status == "completed" || dv.Run.CurrentStage != StageImplement {
		t.Errorf("run after return = %+v", dv.Run)
	}
	// Receiving twice is refused.
	if _, err := env.eng.ReceiveRalphResult(run.RunID, d.ID, LaneResultInput{Status: "done"}); err == nil {
		t.Error("second receive must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDelegationNotActive {
		t.Errorf("reason = %s", rc)
	}
}

func TestDelegateReceiveWithoutEvidenceRejected(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	dv, err := env.eng.DelegateRalph(context.Background(), run.RunID, false)
	if err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(t.TempDir(), "impl.go")
	_, err = env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Artifacts: []ArtifactInput{{AbsPath: art}},
	})
	if err == nil {
		t.Fatal("return without post-delegation evidence must be rejected")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultEvidenceMissing {
		t.Errorf("reason = %s", rc)
	}
}

// --- Parallel plan (G-11) ---

func TestBuildParallelPlanShapes(t *testing.T) {
	single := BuildParallelPlan([]LakeInput{{Slug: "a"}}, false)
	if single.Mode != ParallelSingle || len(single.Satellites) != 0 || single.IntentSource != "single_default" {
		t.Errorf("single = %+v", single)
	}
	pool := BuildParallelPlan([]LakeInput{{Slug: "a"}, {Slug: "b"}}, false)
	if pool.Mode != ParallelPool || len(pool.Satellites) != 2 || !pool.OneLakeOneSatellite {
		t.Fatalf("pool = %+v", pool)
	}
	for _, s := range pool.Satellites {
		if s.MergeTarget != "main_star" || s.MayWriteTruthOnMain {
			t.Errorf("satellite = %+v", s)
		}
		joined := strings.Join(s.Constraints, " ")
		if !strings.Contains(joined, "main_star") || !strings.Contains(joined, core.ReasonSatelliteTruthWrite) {
			t.Errorf("satellite constraints must state merge direction + truth ban: %v", s.Constraints)
		}
		if s.Worktree != ".beacon/worktrees/satellite/"+s.Lake {
			t.Errorf("worktree = %q", s.Worktree)
		}
	}
	forced := BuildParallelPlan([]LakeInput{{Slug: "a"}}, true)
	if forced.Mode != ParallelPool || forced.IntentSource != "explicit_flag" {
		t.Errorf("forced = %+v", forced)
	}
	if !pool.PlannedOnly || !single.PlannedOnly {
		t.Error("plans must be planned_only in P2c")
	}
}

func TestBuildParallelPlanTouchConflict(t *testing.T) {
	plan := BuildParallelPlan([]LakeInput{
		{Slug: "a", Touches: []string{"api.go", "a.go"}},
		{Slug: "b", Touches: []string{"api.go"}},
		{Slug: "c", Touches: []string{"c.go"}},
	}, false)
	if len(plan.Conflicts) != 1 || !plan.RequiresDecision {
		t.Fatalf("conflicts = %+v", plan.Conflicts)
	}
	cf := plan.Conflicts[0]
	if cf.Surface != "api.go" || len(cf.Lakes) != 2 {
		t.Errorf("conflict = %+v", cf)
	}
}

func TestApplyParallelPlanBindsWorktree(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := env.createRun(t)
	plan, view, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{{Slug: "feat-a"}, {Slug: "other"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != ParallelPool {
		t.Fatalf("mode = %s", plan.Mode)
	}
	if view.Implement == nil || view.Implement.WorktreeRef != ".beacon/worktrees/satellite/feat-a" {
		t.Fatalf("view implement = %+v", view.Implement)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !adm.AllowImplement {
		t.Errorf("admission after plan binding = %+v", adm)
	}
}

// --- Design stage (G-18) + admission design item ---

func TestDesignBindingAdmission(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", UXRequired: true, ExecutorID: "host-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Status before binding: required, unbound, bridge point stated.
	sv, err := env.eng.DesignStageStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !sv.Required || sv.Bound || sv.ReasonCode != core.ReasonDesignPrimitiveNotBound || sv.BridgeNote == "" {
		t.Errorf("design status = %+v", sv)
	}
	// Admission refuses: design item carries the code.
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if adm.AllowImplement {
		t.Fatalf("admission must refuse unbound UX run: %+v", adm)
	}
	if it := admissionItem(adm, "design_bound"); it.OK || it.ReasonCode != core.ReasonDesignPrimitiveNotBound {
		t.Errorf("design item = %+v", it)
	}
	// Bind -> admission's design item passes and the gate opens.
	sv, err = env.eng.BindDesignPrimitive(run.RunID, DesignBinding{
		PrimitiveSet: "restrained-product", Version: "v1", SourceRef: "DESIGN.md", BoundBy: "lai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sv.Bound || sv.Binding.PrimitiveSet != "restrained-product" {
		t.Errorf("after bind = %+v", sv)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !adm.AllowImplement {
		t.Errorf("admission after bind = %+v", adm)
	}
	// Binding on a run without a design stage is refused.
	plain := env.createRun(t)
	if _, err := env.eng.BindDesignPrimitive(plain.RunID, DesignBinding{PrimitiveSet: "x"}); err == nil {
		t.Error("bind without design stage must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonDesignStageNotRequired {
		t.Errorf("reason = %s", rc)
	}
}

// --- Admission: each of the five items fails with its own code ---

func TestAdmissionItemsEachFail(t *testing.T) {
	t.Run("truth_not_frozen", func(t *testing.T) {
		env := newTestEnv(t, "draft") // no freeze seeded
		run := env.createRun(t)
		adm, err := env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		it := admissionItem(adm, "truth_frozen")
		if it.OK || it.ReasonCode != core.ReasonFreezeAckRequired || adm.ReasonCode != core.ReasonFreezeAckRequired {
			t.Errorf("item = %+v headline = %s", it, adm.ReasonCode)
		}
	})
	t.Run("worktree_missing_and_mismatch", func(t *testing.T) {
		env := newTestEnv(t, "active")
		env.seedTruthFrozen(t)
		run := env.createRun(t)
		if _, _, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{{Slug: "feat-a"}, {Slug: "other"}}, false); err != nil {
			t.Fatal(err)
		}
		_, m, _, err := env.eng.loadRun(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := env.st.Runs().GetByID(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		m.Implement.WorktreeRef = ""
		if err := env.eng.saveRun(stored, m); err != nil {
			t.Fatal(err)
		}
		adm, err := env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if it := admissionItem(adm, "worktree_binding"); it.OK || it.ReasonCode != core.ReasonAdmissionWorktreeMiss {
			t.Errorf("missing item = %+v", it)
		}
		m.Implement.WorktreeRef = ".beacon/worktrees/satellite/other"
		if err := env.eng.saveRun(stored, m); err != nil {
			t.Fatal(err)
		}
		adm, err = env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if it := admissionItem(adm, "worktree_binding"); it.OK || it.ReasonCode != core.ReasonAdmissionWorktreeMism {
			t.Errorf("mismatch item = %+v", it)
		}
	})
	t.Run("blocker_streak", func(t *testing.T) {
		env := newTestEnv(t, "active")
		env.seedTruthFrozen(t)
		run := env.createRun(t)
		for i := 0; i < 3; i++ {
			if _, err := env.eng.Block(run.RunID, "manual_hold"); err != nil {
				t.Fatal(err)
			}
		}
		adm, err := env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if it := admissionItem(adm, "blocker_streak"); it.OK || it.ReasonCode != ReasonBlockedStreak {
			t.Errorf("blocker item = %+v", it)
		}
	})
	t.Run("steering_unacknowledged", func(t *testing.T) {
		env := newTestEnv(t, "active")
		env.seedTruthFrozen(t)
		run := env.createRun(t)
		if _, err := env.eng.Steer(run.RunID, SteeringObjective, "change direction"); err != nil {
			t.Fatal(err)
		}
		adm, err := env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if it := admissionItem(adm, "steering_ack"); it.OK || it.ReasonCode != core.ReasonSteeringUnacknowledged {
			t.Errorf("steering item = %+v", it)
		}
		if _, err := env.eng.AckSteering(run.RunID); err != nil {
			t.Fatal(err)
		}
		adm, err = env.eng.CheckImplementAdmission(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if !adm.AllowImplement {
			t.Errorf("admission after ack = %+v", adm)
		}
	})
}

// Tick, the delegation packer and the admit surface must all consult
// the SAME admission verdict: one run, one state, three surfaces,
// one reason code.
func TestAdmissionSharedAcrossTickDelegateAdmit(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	if _, err := env.eng.Steer(run.RunID, SteeringObjective, "pivot"); err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if adm.ReasonCode != core.ReasonSteeringUnacknowledged {
		t.Fatalf("admit headline = %s", adm.ReasonCode)
	}
	v, err := env.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != adm.ReasonCode {
		t.Errorf("tick verdict = %+v, want blocked %s", v, adm.ReasonCode)
	}
	_, err = env.eng.DelegateRalph(context.Background(), run.RunID, false)
	if err == nil {
		t.Fatal("delegate must be refused by the same admission")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != adm.ReasonCode {
		t.Errorf("delegate reason = %s, want %s", rc, adm.ReasonCode)
	}
}

// A routed lane with no registered lane executor blocks the tick —
// the lane registry is the only execution path once a lane is routed.
func TestTickRoutedLaneWithoutExecutorBlocks(t *testing.T) {
	env := newTestEnv(t, "active") // stage executors only
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	if _, err := env.eng.DelegateRalph(context.Background(), run.RunID, false); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Blocked || v.ReasonCode != ReasonExecutorNotReg {
		t.Errorf("tick with routed unregistered lane = %+v", v)
	}
}
