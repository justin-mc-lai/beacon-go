// acceptance-seed-red-test (plan stage): executable acceptance seeds.
//
// These tests are the feature's acceptance seed, not its proof of
// delivery: every one of them FAILS on main today and must be turned
// green by the implement round without editing what they assert.
//
//   AC-001  a proposal whose acceptance_seed is prose is refused
//   AC-002  harvest refuses a result that only carries test artifacts
//   AC-003  harvest refuses a result that cannot show the red baseline
//
// They use only behaviour that already exists (ValidateLakeProposal,
// HarvestLaneResult, the lane/harvest fixtures), so they compile
// against main and fail by assertion.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Artifact kinds the acceptance harvest rule is expected to speak.
// Constants are local to this seed file so the red tests do not
// depend on production symbols that do not exist yet.
const (
	seedKindAcceptanceTest = "acceptance_test"
	seedKindRedEvidence    = "acceptance_seed_red"
)

// seedProposal builds a proposal the existing gate accepts today,
// with every lake carrying the same acceptance seed.
func seedProposal(seed string) LakeProposal {
	return LakeProposal{
		Schema:    LakeProposalSchema,
		Utterance: "turn acceptance into executable red tests",
		Lakes: []LakeProposalLake{
			{Slug: "seed-a", Outcome: "a", Value: "v", Metric: "m", Operable: "o", Size: "S", AcceptanceSeed: seed},
			{Slug: "seed-b", Outcome: "b", Value: "v", Metric: "m", Operable: "o", Size: "S", AcceptanceSeed: seed, DependsOn: []string{"seed-a"}},
			{Slug: "seed-c", Outcome: "c", Value: "v", Metric: "m", Operable: "o", Size: "S", AcceptanceSeed: seed, DependsOn: []string{"seed-b"}},
		},
		NotProposed: []NotProposedLake{{Slug: "later", Reason: "not committed yet"}},
	}
}

// AC-001: an acceptance_seed must be a runnable test command. The
// executable control stays accepted; the prose seed must be refused.
func TestValidateLakeProposalRejectsProseAcceptanceSeed(t *testing.T) {
	executable := "go test ./internal/orchestrator -run TestValidateLakeProposal -count=1"
	if err := ValidateLakeProposal(seedProposal(executable)); err != nil {
		t.Fatalf("executable seed refused: %v", err)
	}
	prose := seedProposal(executable)
	prose.Lakes[1].AcceptanceSeed = "a visitor joins the waitlist"
	if err := ValidateLakeProposal(prose); err == nil {
		t.Fatalf("proposal accepted prose acceptance_seed %q; want a refusal that says the seed is not an executable test command", prose.Lakes[1].AcceptanceSeed)
	}
}

// seedImplementRun advances a fresh run to implement and executes the
// stage once, so the harvest's existing evidence requirement is met
// and only the rule under test can cause a refusal.
func seedImplementRun(t *testing.T) (testEnv, string) {
	t.Helper()
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.eng.RegisterLaneExecutorForAllLanes(env.rec)
	run := advanceToImplement(t, env)
	if _, err := env.eng.ExecuteLaneStage(context.Background(), run.RunID, LaneSingle); err != nil {
		t.Fatalf("ExecuteLaneStage: %v", err)
	}
	return env, run.RunID
}

func seedFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// AC-002: the implement return must carry implementation and test
// artifacts separately. A return that only brings the acceptance
// test is the executor grading itself and must not be harvested.
func TestHarvestRejectsTestOnlyArtifacts(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	testFile := seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")

	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status:    "done",
		Summary:   "wrote the acceptance test",
		Artifacts: []ArtifactInput{{Kind: seedKindAcceptanceTest, AbsPath: testFile}},
	})
	if err == nil || v.Accepted {
		t.Fatalf("harvest accepted a test-only result (%+v); want a refusal because no implementation artifact was returned", v)
	}
}

// AC-003: harvest must be able to see the seed fail before the
// implementation lands. Without the red evidence artifact the result
// cannot prove red->green and must be refused.
func TestHarvestRequiresSeedRedEvidence(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	impl := seedFile(t, dir, "impl.go", "package orchestrator\n")
	testFile := seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")

	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status:  "done",
		Summary: "implementation plus acceptance test, no red run",
		Artifacts: []ArtifactInput{
			{Kind: "implement_artifact", AbsPath: impl},
			{Kind: seedKindAcceptanceTest, AbsPath: testFile},
		},
	})
	if err == nil || v.Accepted {
		t.Fatalf("harvest accepted a result without %s evidence (%+v); want a refusal because red->green is unproven", seedKindRedEvidence, v)
	}
}
