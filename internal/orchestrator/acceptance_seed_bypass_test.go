// acceptance-seed-red-test (implement round 2): the three-kind rule is
// judged on the artifacts a return actually brings. A rejected return
// must not leave pointers behind that let a later empty return pass.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

// A test-only return is refused, and the empty retry that follows must
// still be refused: the refused test artifact stays unregistered and
// cannot pay for the empty return.
func TestHarvestRejectsEmptyReturnAfterTestOnlyRejection(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	testFile := seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")

	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status:    "done",
		Summary:   "wrote the acceptance test",
		Artifacts: []ArtifactInput{{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile}},
	})
	if err == nil || v.Accepted {
		t.Fatalf("test-only return accepted (%+v); want a refusal", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultImplementArtifactMissing {
		t.Errorf("first refusal reason = %s, want %s", rc, core.ReasonResultImplementArtifactMissing)
	}

	v, err = env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "done, nothing else to report",
	})
	if err == nil || v.Accepted {
		t.Fatalf("empty retry harvested stale artifacts (%+v); want a refusal", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactsMissing {
		t.Errorf("empty retry reason = %s, want %s", rc, core.ReasonResultArtifactsMissing)
	}

	// The refusal did not corrupt the run: a whole return still lands.
	impl := seedFile(t, dir, "impl.go", "package orchestrator\n")
	v, err = env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "whole return", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile},
			{Kind: ArtifactKindImplement, AbsPath: impl},
		},
	})
	if err != nil || !v.Accepted {
		t.Fatalf("whole return refused: %+v %v", v, err)
	}

}

// Round 4: the CLI lane entry (HarvestLaneResult, lanes.go) is the
// third door onto the same rule. It must judge this return's own
// artifacts, leave artifact_refs untouched when it refuses, and still
// take a whole return afterwards.
func TestHarvestLaneEntryRejectsEmptyReturnAfterTestOnlyRejection(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	testFile := seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")

	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status:    "done",
		Summary:   "wrote the acceptance test",
		Artifacts: []ArtifactInput{{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile}},
	})
	if err == nil || v.Accepted {
		t.Fatalf("CLI lane test-only return accepted (%+v); want a refusal", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultImplementArtifactMissing {
		t.Errorf("first refusal reason = %s, want %s", rc, core.ReasonResultImplementArtifactMissing)
	}
	stored, listErr := env.eng.store.Artifacts().ListByRun(runID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(stored) != 0 {
		t.Fatalf("refused CLI lane return left artifact_refs behind: %+v", stored)
	}

	v, err = env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "done, nothing else to report",
	})
	if err == nil || v.Accepted {
		t.Fatalf("CLI lane empty retry accepted (%+v); want a refusal", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactsMissing {
		t.Errorf("empty retry reason = %s, want %s", rc, core.ReasonResultArtifactsMissing)
	}

	impl := seedFile(t, dir, "impl.go", "package orchestrator\n")
	v, err = env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "whole return", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile},
			{Kind: ArtifactKindImplement, AbsPath: impl},
		},
	})
	if err != nil || !v.Accepted {
		t.Fatalf("CLI lane whole return refused: %+v %v", v, err)
	}
}

// The same bypass existed on the delegation return path; it must stay
// closed there too.
func TestReceiveRejectsEmptyReturnAfterTestOnlyRejection(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	dv, err := env.eng.DelegateRalph(context.Background(), run.RunID, true)
	if err != nil {
		t.Fatalf("DelegateRalph: %v", err)
	}
	dir := t.TempDir()
	testFile := seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")

	if _, err := env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status:    "done",
		Summary:   "wrote the acceptance test",
		Artifacts: []ArtifactInput{{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile}},
	}); err == nil {
		t.Fatal("test-only return accepted; want a refusal")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultImplementArtifactMissing {
		t.Errorf("first refusal reason = %s, want %s", rc, core.ReasonResultImplementArtifactMissing)
	}

	if _, err := env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Summary: "done, nothing else to report",
	}); err == nil {
		t.Fatal("empty retry harvested stale artifacts; want a refusal")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactsMissing {
		t.Errorf("empty retry reason = %s, want %s", rc, core.ReasonResultArtifactsMissing)
	}

	// The delegation is still out, so a whole return can still land.
	impl := filepath.Join(dir, "impl.go")
	if _, err := env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Summary: "whole return", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: filepath.Join(dir, "red-ac.txt")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: testFile},
			{Kind: ArtifactKindImplement, AbsPath: impl},
		},
	}); err != nil {
		t.Fatalf("whole return refused: %v", err)
	}
}

// Round 3: artifact_refs is keyed on (run_id, abs_path) and keeps the
// first kind recorded for a path. One file therefore cannot pose as two
// of the three kinds: the rule must judge the folded set that will be
// stored, not the declared list.
func TestHarvestRejectsKindReusedOnOnePath(t *testing.T) {
	cases := []struct {
		name      string
		artifacts func(t *testing.T, dir string) []ArtifactInput
		wantCode  string
	}{
		{
			name: "red and implement share a path",
			artifacts: func(t *testing.T, dir string) []ArtifactInput {
				shared := seedFile(t, dir, "shared.go", "package orchestrator\n")
				return []ArtifactInput{
					{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: shared},
					{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
					{Kind: ArtifactKindImplement, AbsPath: shared},
				}
			},
			wantCode: core.ReasonResultImplementArtifactMissing,
		},
		{
			name: "implement and acceptance test share a path",
			artifacts: func(t *testing.T, dir string) []ArtifactInput {
				shared := seedFile(t, dir, "shared_test.go", "package orchestrator\n")
				return []ArtifactInput{
					{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
					{Kind: ArtifactKindImplement, AbsPath: shared},
					{Kind: ArtifactKindAcceptanceTest, AbsPath: shared},
				}
			},
			wantCode: core.ReasonResultAcceptanceTestMissing,
		},
		{
			name: "all three kinds share a path",
			artifacts: func(t *testing.T, dir string) []ArtifactInput {
				shared := seedFile(t, dir, "shared.go", "package orchestrator\n")
				return []ArtifactInput{
					{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: shared},
					{Kind: ArtifactKindAcceptanceTest, AbsPath: shared},
					{Kind: ArtifactKindImplement, AbsPath: shared},
				}
			},
			wantCode: core.ReasonResultImplementArtifactMissing,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, runID := seedImplementRun(t)
			v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
				Status: "done", Summary: tc.name, Artifacts: tc.artifacts(t, t.TempDir()),
			})
			if err == nil || v.Accepted {
				t.Fatalf("%s was accepted (%+v); want a refusal", tc.name, v)
			}
			if rc, _ := core.ReasonCodeOf(err); rc != tc.wantCode {
				t.Errorf("reason = %s, want %s", rc, tc.wantCode)
			}
			arts, listErr := env.eng.store.Artifacts().ListByRun(runID)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if len(arts) != 0 {
				t.Fatalf("rejected return left artifacts behind: %+v", arts)
			}
		})
	}
}

// The delegation return shares the rule, so the same folded set applies
// there: one path declared as red and implement lands only red and the
// return is refused.
func TestReceiveRejectsKindReusedOnOnePath(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	run := advanceToImplement(t, env)
	dv, err := env.eng.DelegateRalph(context.Background(), run.RunID, true)
	if err != nil {
		t.Fatalf("DelegateRalph: %v", err)
	}
	dir := t.TempDir()
	shared := seedFile(t, dir, "shared.go", "package orchestrator\n")
	_, err = env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Summary: "one path, two kinds", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: shared},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
			{Kind: ArtifactKindImplement, AbsPath: shared},
		},
	})
	if err == nil {
		t.Fatal("one path posing as red and implement was accepted; want a refusal")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultImplementArtifactMissing {
		t.Errorf("reason = %s, want %s", rc, core.ReasonResultImplementArtifactMissing)
	}
	arts, listErr := env.eng.store.Artifacts().ListByRun(run.RunID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(arts) != 0 {
		t.Fatalf("rejected return left artifacts behind: %+v", arts)
	}
	if dv2, err := env.eng.ReceiveRalphResult(run.RunID, dv.Delegation.ID, LaneResultInput{
		Status: "done", Summary: "whole return", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
			{Kind: ArtifactKindImplement, AbsPath: seedFile(t, dir, "impl.go", "package orchestrator\n")},
		},
	}); err != nil || dv2.Delegation.Status != DelegationReturned {
		t.Fatalf("whole return refused after the same-path refusal: %+v %v", dv2, err)
	}
}

// A kind that only looks canonical because of surrounding whitespace is
// malformed input: trimming it for the check but storing it verbatim
// would leave a row the three-kind rule cannot see as implement_artifact.
func TestHarvestRejectsUncanonicalArtifactKind(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "padded kind", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
			{Kind: " " + ArtifactKindImplement + " ", AbsPath: seedFile(t, dir, "impl.go", "package orchestrator\n")},
		},
	})
	if err == nil || v.Accepted {
		t.Fatalf("padded artifact kind was accepted (%+v); want a refusal", v)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonResultArtifactKindInvalid {
		t.Errorf("reason = %s, want %s", rc, core.ReasonResultArtifactKindInvalid)
	}
	arts, listErr := env.eng.store.Artifacts().ListByRun(runID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(arts) != 0 {
		t.Fatalf("rejected return left artifacts behind: %+v", arts)
	}
}

// The control: three kinds on three distinct paths still harvest, and
// what lands is exactly the frozen kind vocabulary.
func TestHarvestAcceptsThreeSeparateKinds(t *testing.T) {
	env, runID := seedImplementRun(t)
	dir := t.TempDir()
	v, err := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
		Status: "done", Summary: "three separate kinds", Artifacts: []ArtifactInput{
			{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
			{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
			{Kind: ArtifactKindImplement, AbsPath: seedFile(t, dir, "impl.go", "package orchestrator\n")},
		},
	})
	if err != nil || !v.Accepted {
		t.Fatalf("three separate kinds refused: %+v %v", v, err)
	}
	arts, listErr := env.eng.store.Artifacts().ListByRun(runID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	got := map[string]bool{}
	for _, a := range arts {
		got[a.Kind] = true
	}
	for _, want := range []string{ArtifactKindImplement, ArtifactKindAcceptanceTest, ArtifactKindAcceptanceSeedRed} {
		if !got[want] {
			t.Errorf("stored kinds %v missing %s", got, want)
		}
	}
}

// Registration turns a relative path into an absolute one, so the same
// file addressed both ways is one store key. The fold must use that
// key too; otherwise red=/abs/x plus impl=x.go passes the check and the
// second Add refreshes the red row instead of adding an implement row.
func TestHarvestRejectsRelativeAliasOfSamePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		absKind string
	}{
		{name: "absolute red, relative implement", absKind: ArtifactKindAcceptanceSeedRed},
		{name: "relative red, absolute implement", absKind: ArtifactKindImplement},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env, runID := seedImplementRun(t)
			dir := t.TempDir()
			shared := seedFile(t, dir, "shared.go", "package orchestrator\n")
			rel, relErr := filepath.Rel(cwd, shared)
			if relErr != nil {
				t.Fatal(relErr)
			}
			absSide := shared
			relSide := rel
			if tc.absKind != ArtifactKindAcceptanceSeedRed {
				absSide, relSide = rel, shared
			}
			arts := []ArtifactInput{
				{Kind: ArtifactKindAcceptanceSeedRed, AbsPath: seedFile(t, dir, "red-ac.txt", "FAIL\n")},
				{Kind: ArtifactKindAcceptanceTest, AbsPath: seedFile(t, dir, "acceptance_seed_test.go", "package orchestrator\n")},
				{Kind: ArtifactKindImplement, AbsPath: shared},
			}
			if tc.absKind == ArtifactKindAcceptanceSeedRed {
				arts[0].AbsPath = absSide
				arts[2].AbsPath = relSide
			} else {
				arts[0].AbsPath = relSide
				arts[2].AbsPath = absSide
			}
			v, herr := env.eng.HarvestLaneResult(runID, LaneSingle, LaneResultInput{
				Status: "done", Summary: tc.name, Artifacts: arts,
			})
			if herr == nil || v.Accepted {
				t.Fatalf("%s was accepted (%+v); want a refusal", tc.name, v)
			}
			if rc, _ := core.ReasonCodeOf(herr); rc != core.ReasonResultImplementArtifactMissing {
				t.Errorf("reason = %s, want %s", rc, core.ReasonResultImplementArtifactMissing)
			}
			stored, listErr := env.eng.store.Artifacts().ListByRun(runID)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if len(stored) != 0 {
				t.Fatalf("rejected return left artifacts behind: %+v", stored)
			}
		})
	}
}
