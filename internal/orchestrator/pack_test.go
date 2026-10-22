package orchestrator

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func TestEntryContractFields(t *testing.T) {
	env := newTestEnv(t, "")
	run := env.createRun(t)
	v, err := env.eng.EntryContract(run.RunID)
	if err != nil {
		t.Fatalf("EntryContract: %v", err)
	}
	if v.SchemaVersion == "" || v.PublicPrimary != "goal" || v.PublicCLI != "beacon goal" ||
		!v.SingleEntry || v.DualPublicOrchestration {
		t.Fatalf("contract surface = %+v", v)
	}
	if v.RunID != run.RunID || v.FeatureSlug != "feat-a" || v.CurrentStage != StagePlan || v.LoopStage == "" {
		t.Fatalf("contract run binding = %+v", v)
	}
	if len(v.Stages) != 5 {
		t.Fatalf("stages = %d, want 5 (plan/truth/implement/qa/release)", len(v.Stages))
	}
	for _, s := range v.Stages {
		if len(s.Skills) == 0 || s.LoopStage == "" {
			t.Fatalf("stage %+v lacks skills or loop stage", s)
		}
	}
	if v.NextSkill != "beacon-plan" {
		t.Fatalf("next skill = %q, want beacon-plan", v.NextSkill)
	}
	if !v.VerifierRequired || !v.ReleaseRequiresHuman {
		t.Fatalf("contract invariants = %+v, want verifier + human release required", v)
	}
	if _, err := env.eng.EntryContract("run_nope"); err == nil ||
		!strings.Contains(err.Error(), ReasonRunNotFound) {
		t.Fatalf("missing run err = %v", err)
	}
}

func TestResumePackAggregates(t *testing.T) {
	env := newTestEnv(t, "")
	run := env.createRun(t)
	if _, _, err := env.eng.Checkpoint(run.RunID, "write the tests", "tests"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatalf("Block: %v", err)
	}
	if _, err := env.eng.Steer(run.RunID, "", "switch to SSO first"); err != nil {
		t.Fatalf("Steer: %v", err)
	}
	pack, err := env.eng.ResumePack(run.RunID)
	if err != nil {
		t.Fatalf("ResumePack: %v", err)
	}
	if pack.Run.RunID != run.RunID || !pack.Run.Blocked {
		t.Fatalf("pack run = %+v", pack.Run)
	}
	if pack.Resume.ResumeAction != core.ResumeLoadNextSkill || pack.Resume.NextSkill != "beacon-plan" {
		t.Fatalf("pack resume = %+v", pack.Resume)
	}
	if pack.LatestCheckpoint == nil || pack.LatestCheckpoint.State["next_action"] != "write the tests" {
		t.Fatalf("pack checkpoint = %+v", pack.LatestCheckpoint)
	}
	found := false
	for _, b := range pack.OpenBlockers {
		if b == "manual_block" {
			found = true
		}
	}
	if !found {
		t.Fatalf("open blockers = %v, want manual_block among them", pack.OpenBlockers)
	}
	if pack.PendingSteering == nil || pack.PendingSteering.Text != "switch to SSO first" {
		t.Fatalf("pack steering = %+v", pack.PendingSteering)
	}
	// The pack is read-only: the run is still blocked afterwards
	// (Resume would have cleared the soft block; the pack must not).
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Blocked {
		t.Fatal("ResumePack mutated the run: soft block was cleared")
	}
}

func TestResumePackDoesNotConsumeInterrupt(t *testing.T) {
	env := newTestEnv(t, "")
	run := env.createRun(t)
	if _, err := env.eng.RecordInterrupt(run.RunID, "stage_step"); err != nil {
		t.Fatalf("RecordInterrupt: %v", err)
	}
	pack, err := env.eng.ResumePack(run.RunID)
	if err != nil {
		t.Fatalf("ResumePack: %v", err)
	}
	if pack.Resume.ResumeAction != core.ResumeReplaySubstep {
		t.Fatalf("pack resume action = %q, want replay_substep", pack.Resume.ResumeAction)
	}
	// Because the pack consumed nothing, the acting Resume still
	// sees (and consumes) the interrupt exactly once.
	rv, err := env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if rv.ResumeAction != core.ResumeReplaySubstep {
		t.Fatalf("resume after pack = %q, want replay_substep (pack must not consume)", rv.ResumeAction)
	}
	rv2, err := env.eng.Resume(run.RunID, "")
	if err != nil {
		t.Fatalf("Resume 2: %v", err)
	}
	if rv2.ResumeAction == core.ResumeReplaySubstep {
		t.Fatal("interrupt was replayed twice")
	}
}
