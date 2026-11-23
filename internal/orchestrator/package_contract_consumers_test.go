package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

const sddConsumerTruth = `---
slug: feat-a
package_contract: truth-tasks
canonical_refs:
  truth: truth.md
  tests: tests.md
  tasks: tasks.md
  evidence: evidence.md
---
# Truth

## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | behavior is verified |

## Coverage Matrix

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|---|---|---|---|---|---|
| AC-001 | TC-001 | behavior | ` + "`go test ./internal/core -run TestBehavior -count=1`" + ` | behavior holds | evidence/covered.txt |
`

func newSDDConsumerEnv(t *testing.T) gateTestEnv {
	t.Helper()
	// Register as legacy first: the persisted TestsPath remains tests.md
	// after the existing package adopts the two-file contract.
	env := newGateEnv(t, frozenTruthDoc)
	if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(sddConsumerTruth), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(env.root, "evidence.md")); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestSDDExistingRegistrationUsesTruthCoverage(t *testing.T) {
	env := newSDDConsumerEnv(t)
	if env.ctx.Feature.TestsPath == env.ctx.Feature.TruthPath || env.ctx.Feature.TestsPath == "" {
		t.Fatal("fixture must retain a separate registered TestsPath")
	}
	pkg := env.eng.packageFiles(env.ctx)
	if pkg.Tests != pkg.Truth || pkg.CoverageFile() != pkg.Truth {
		t.Fatalf("stale registered tests must not override truth matrix: %+v", pkg)
	}
	rows := gate.ParseCoverageRows(pkg.Tests.Text)
	if len(rows) != 1 || rows[0].Evidence != "evidence/covered.txt" {
		t.Fatalf("consumers must receive the truth coverage matrix, got %+v", rows)
	}
	if err := os.Remove(filepath.Join(env.root, "tests.md")); err != nil {
		t.Fatal(err)
	}
	pkg = env.eng.packageFiles(env.ctx)
	if !pkg.Tests.Present || pkg.Tests.Path != "truth.md" {
		t.Fatalf("missing legacy TestsPath must not hide SDD coverage: %+v", pkg.Tests)
	}
}

func TestSDDLayoutAndAuthoringRequireOnlyTruthAndTasks(t *testing.T) {
	env := newSDDConsumerEnv(t)
	if err := os.Remove(filepath.Join(env.root, "tests.md")); err != nil {
		t.Fatal(err)
	}
	view, err := env.eng.TruthMapVerify(env.root, "feat-a")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "pass" || !view.LayoutOK || view.TestsDoc != "truth.md" || view.EvidenceDoc != "" {
		t.Fatalf("optional legacy canonical_refs must not block SDD layout: %+v", view)
	}
	run := store.Run{FeatureID: env.ctx.Feature.ID}
	item := env.eng.bundleTruthMap(run, "feat-a")
	if !item.OK {
		t.Fatalf("admission must share the two-file layout contract: %+v", item)
	}
	ok, code, detail, _, err := env.eng.flowCriterion(run, &runMeta{}, "feat-a", FlowStepAuthoring)
	if err != nil || !ok {
		t.Fatalf("two-file authoring rejected: code=%s detail=%s err=%v", code, detail, err)
	}
	if err := os.Remove(filepath.Join(env.root, "tasks.md")); err != nil {
		t.Fatal(err)
	}
	_, _, problems := env.eng.truthMapLayoutProblems(env.ctx)
	if len(problems) == 0 {
		t.Fatal("SDD layout must still reject a missing required tasks member")
	}
	ok, code, _, _, err = env.eng.flowCriterion(run, &runMeta{}, "feat-a", FlowStepAuthoring)
	if err != nil || ok || code != core.ReasonStageExitTruthMissing {
		t.Fatalf("missing required tasks must block authoring: ok=%v code=%s err=%v", ok, code, err)
	}
}

func TestSDDConformanceTraceabilityDefaultsToTruthMatrix(t *testing.T) {
	for _, setting := range []string{"", "traceability_required: false\n"} {
		t.Run(strings.TrimSpace("default "+setting), func(t *testing.T) {
			env := newSDDConsumerEnv(t)
			truth := strings.Replace(sddConsumerTruth, "package_contract: truth-tasks\n", "package_contract: truth-tasks\n"+setting, 1)
			if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(truth), 0o644); err != nil {
				t.Fatal(err)
			}
			run := store.Run{FeatureID: env.ctx.Feature.ID}
			verdict := env.eng.conformanceTraceability(run)
			if verdict.Status != "fail" || !strings.Contains(strings.Join(verdict.ReasonCodes, ","), core.ReasonTraceabilityMissingEvidence) {
				t.Fatalf("SDD traceability must require evidence by default, got %+v", verdict)
			}
			if err := os.MkdirAll(filepath.Join(env.root, "evidence"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(env.root, "evidence", "covered.txt"), []byte("observed behavior"), 0o644); err != nil {
				t.Fatal(err)
			}
			if verdict = env.eng.conformanceTraceability(run); verdict.Status != "pass" {
				t.Fatalf("truth matrix with actual evidence must pass despite stale TestsPath: %+v", verdict)
			}
		})
	}
}

func TestSDDProgramRequirementIgnoresStaleTestsPath(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-a", Utterance: "verify requirement fingerprint"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(sddConsumerTruth), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := env.eng.programRequirementFP(env.root, run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(env.root, "tests.md")); err != nil {
		t.Fatal(err)
	}
	after, err := env.eng.programRequirementFP(env.root, run.RunID)
	if err != nil || after != before {
		t.Fatalf("legacy tests file must not affect SDD requirement fingerprint: before=%s after=%s err=%v", before, after, err)
	}
	if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(sddConsumerTruth+"\nChanged requirement.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := env.eng.programRequirementFP(env.root, run.RunID)
	if err != nil || changed == before {
		t.Fatalf("truth changes must affect requirement fingerprint: before=%s changed=%s err=%v", before, changed, err)
	}
}

func TestSDDConformanceRejectsMissingTruthCoverageDespiteLegacyTests(t *testing.T) {
	env := newSDDConsumerEnv(t)
	truth := strings.Split(sddConsumerTruth, "## Coverage Matrix")[0]
	if err := os.WriteFile(filepath.Join(env.root, "truth.md"), []byte(truth), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict := env.eng.conformanceTraceability(store.Run{FeatureID: env.ctx.Feature.ID})
	if verdict.Status != "fail" || !strings.Contains(strings.Join(verdict.ReasonCodes, ","), core.ReasonTraceabilityMissingTC) {
		t.Fatalf("legacy tests matrix must not rescue absent SDD truth coverage: %+v", verdict)
	}
}
