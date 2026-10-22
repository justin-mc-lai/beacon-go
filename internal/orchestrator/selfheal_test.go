package orchestrator

import (
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

// Pre phase: plan shows both auto steps as planned and executes
// nothing; Execute runs both real checks and records the execution.
func TestSelfHealPrePlanAndExecute(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	plan, err := env.eng.PlanSelfHeal(run.RunID, "pre")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Executed || len(plan.Steps) != 2 {
		t.Fatalf("pre plan = %+v", plan)
	}
	for _, s := range plan.Steps {
		if !s.Auto || s.Status != HealPlanned {
			t.Errorf("pre step = %+v", s)
		}
	}
	if hasEvent(traceEventTypes(t, env, run.RunID), "self_heal_executed") {
		t.Error("planning must not execute")
	}
	plan, err = env.eng.ExecuteSelfHeal(run.RunID, "pre")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range plan.Steps {
		if s.Status != HealExecuted || s.Detail == "" {
			t.Errorf("executed pre step = %+v", s)
		}
	}
	if !hasEvent(traceEventTypes(t, env, run.RunID), "self_heal_executed") {
		t.Error("self_heal_executed event missing")
	}
}

// Mid phase: the evidence-manifest repair really rebuilds the
// manifest from artifact refs and appends the repair event; the
// manual workspace-admit step is reported, never executed.
func TestSelfHealMidRepairsEvidenceManifest(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	if _, err := env.st.Artifacts().Add(store.ArtifactRef{
		RunID: run.RunID, Kind: "evidence",
		AbsPath: filepath.Join(env.root, "evidence.md"), ContentHash: "abc123", SizeBytes: 10,
	}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.eng.ExecuteSelfHeal(run.RunID, "mid")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]SelfHealStep{}
	for _, s := range plan.Steps {
		byID[s.ID] = s
	}
	if got := byID["repair-evidence-manifest"]; got.Status != HealExecuted || !strings.Contains(got.Detail, "1 artifact refs") {
		t.Errorf("repair step = %+v", got)
	}
	if got := byID["workspace-admit"]; got.Status != HealManualOnly || got.Auto {
		t.Errorf("workspace-admit step = %+v", got)
	}
	bundle, err := env.eng.Trace(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range bundle.Events {
		if ev.EventType == "evidence_manifest_repaired" && strings.Contains(string(ev.Payload), `"artifact_count":1`) {
			found = true
		}
	}
	if !found {
		t.Error("evidence_manifest_repaired event with artifact_count=1 missing")
	}
}

// Post phase: the install step requires human confirmation and is
// NEVER auto-executed — not by plan, not by execute.
func TestSelfHealPostNeverAutoExecutes(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	for _, exec := range []bool{false, true} {
		var plan SelfHealPlan
		var err error
		if exec {
			plan, err = env.eng.ExecuteSelfHeal(run.RunID, "post")
		} else {
			plan, err = env.eng.PlanSelfHeal(run.RunID, "post")
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Steps) != 1 {
			t.Fatalf("post plan = %+v", plan)
		}
		s := plan.Steps[0]
		if s.Status == HealExecuted {
			t.Fatalf("post step auto-executed (exec=%v): %+v", exec, s)
		}
		if s.Status != HealRequiresConfirmation || !s.RequiresHumanConfirmation || s.Auto {
			t.Errorf("post step = %+v", s)
		}
	}
	// Phase derivation: a fresh run plans pre; a blocked run plans mid.
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	plan, err := env.eng.PlanSelfHeal(run.RunID, "")
	if err != nil || plan.Phase != "mid" {
		t.Errorf("derived phase for blocked run = %+v %v", plan, err)
	}
	if _, err := env.eng.PlanSelfHeal(run.RunID, "bogus"); err == nil {
		t.Error("bogus phase must fail")
	}
}
