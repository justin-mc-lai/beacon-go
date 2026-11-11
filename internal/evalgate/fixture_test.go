package evalgate

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

// The base package the fixture delivers: frozen, one AC, a TC row and
// no traceability contract. Conformance cases rewrite it after the run
// is delivered, exactly like the #1 tests do.
const baseTruthDoc = `---
slug: feat-a
version: v1.6.19
status: frozen
language: en
parser_contract: beacon-feature-package-v2
canonical_refs:
  truth: truth.md
  tests: tests.md
  tasks: tasks.md
  evidence: evidence.md
---
# Truth

- **AC-001**：the one requirement
`

const baseTestsDoc = `---
slug: feat-a
---
| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | ` + "`go test ./... -run TestAC001`" + ` | passes |
`

// traceabilityTruthDoc declares the contract and defines two ACs.
const traceabilityTruthDoc = `---
slug: feat-a
version: v1.6.19
status: frozen
language: en
traceability_required: true
parser_contract: beacon-feature-package-v2
canonical_refs:
  truth: truth.md
  tests: tests.md
  tasks: tasks.md
  evidence: evidence.md
---
# Truth

- **AC-001**：the first requirement
- **AC-002**：the second requirement
`

// traceabilityTestsDoc is the fully covered matrix (one executable TC
// with in-package evidence per AC).
const traceabilityTestsDoc = `---
slug: feat-a
---
| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | ` + "`go test ./... -run TestAC001`" + ` | ac_001_holds | evidence/covered-001.txt |
| AC-002 | TC-002 | behavior | ` + "`go test ./... -run TestAC002`" + ` | ac_002_holds | evidence/covered-002.txt |
`

// fixture is one isolated project + isolated global store. The store path
// comes from store.EnvStorePath, so the in-process CLI reads exactly the
// same database the engine wrote.
type fixture struct {
	t     *testing.T
	root  string
	st    *store.Store
	eng   *orchestrator.Engine
	ctx   store.FeatureContext
	runID string
}

func newFixture(t *testing.T, truthDoc string) *fixture {
	return newFixtureWithStatus(t, truthDoc, "active")
}

// newFixtureWithStatus seeds the feature row with one lifecycle status;
// the truth exit gate reads "draft" here (the package is not filled),
// which is what produces stage_exit_truth_not_frozen.
func newFixtureWithStatus(t *testing.T, truthDoc, featureStatus string) *fixture {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	root := store.ResolveRootPath(filepath.Join(dir, "proj"))
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatalf("register project: %v", err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatalf("add version: %v", err)
	}
	if _, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md", Status: featureStatus,
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	eng := orchestrator.NewEngine(st)
	eng.RegisterExecutorForAllStages(orchestrator.NewRecordingExecutor(st, "rec-1"))
	f := &fixture{t: t, root: root, st: st, eng: eng}
	f.writeBase(truthDoc)
	ctx, err := st.ResolveFeature(root, "feat-a", "")
	if err != nil {
		t.Fatalf("resolve feature: %v", err)
	}
	f.ctx = ctx
	return f
}

// writeBase writes the package the delivered run is created against.
func (f *fixture) writeBase(truthDoc string) {
	f.t.Helper()
	f.write("truth.md", truthDoc)
	f.write("tests.md", baseTestsDoc)
	f.write("tasks.md", "---\nslug: feat-a\n---\n- [ ] the work · ac=AC-001\n")
	f.write("evidence.md", "# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| truth | requirement_truth | truth.md |\n")
	// intent-contract-bridge AC-002: the recording executor is kernel
	// driven, so the package needs a confirmed intent to be admitted.
	f.write("intent.json", `{"schema":"beacon-intent-v1","feature_slug":"feat-a","version":"v2.0.0","truth_path":"truth.md","recorded_at":"2026-11-11T00:00:00Z","recorded_by":"fixture","confirmed":true,"briefing":{"original_prompt":"eval fixture","restated_intent":"deliver the eval fixture","identified_risks":[],"boundaries":[],"proposed_slugs":["feat-a"],"confirmed":true,"degraded":false}}`)
	if err := os.MkdirAll(filepath.Join(f.root, "evidence"), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) write(name, body string) {
	f.t.Helper()
	if err := os.WriteFile(filepath.Join(f.root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// rewritePackage replaces truth/tests and writes the evidence files
// (project-relative paths such as evidence/covered-001.txt).
func (f *fixture) rewritePackage(truthDoc, testsDoc string, files map[string]string, removeTests bool) {
	f.t.Helper()
	f.write("truth.md", truthDoc)
	if removeTests {
		if err := os.Remove(filepath.Join(f.root, "tests.md")); err != nil && !os.IsNotExist(err) {
			f.t.Fatal(err)
		}
	} else {
		f.write("tests.md", testsDoc)
	}
	for name, body := range files {
		abs := filepath.Join(f.root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			f.t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			f.t.Fatal(err)
		}
	}
}

// parkAtTruth creates one run and ticks it until the current stage is
// truth (the plan stage has been executed and its exit gate passed).
func (f *fixture) parkAtTruth(uxRequired bool) orchestrator.RunView {
	f.t.Helper()
	run, err := f.eng.CreateRun(orchestrator.CreateRunOptions{
		ProjectRoot: f.root, FeatureSlug: "feat-a", Utterance: "eval fixture",
		UXRequired: uxRequired, ExecutorID: "maker", ExecutorMode: orchestrator.ExecutorModeRecording,
	})
	if err != nil {
		f.t.Fatalf("create run: %v", err)
	}
	f.runID = run.RunID
	v, err := f.eng.Tick(context.Background(), run.RunID)
	if err != nil {
		f.t.Fatalf("tick to truth: %v", err)
	}
	if v.Blocked || v.CurrentStage != orchestrator.StageTruth {
		f.t.Fatalf("tick to truth = %+v", v)
	}
	return v
}

// deliver walks one run from plan to a released, verifier-attached,
// state-exported delivery, so the four base conformance judgements pass.
func (f *fixture) deliver() {
	f.t.Helper()
	run, err := f.eng.CreateRun(orchestrator.CreateRunOptions{
		ProjectRoot: f.root, FeatureSlug: "feat-a", Utterance: "eval fixture",
		ExecutorID: "maker", ExecutorMode: orchestrator.ExecutorModeRecording,
	})
	if err != nil {
		f.t.Fatalf("create run: %v", err)
	}
	f.runID = run.RunID
	step := func(want string) {
		f.t.Helper()
		v, err := f.eng.Tick(context.Background(), run.RunID)
		if err != nil {
			f.t.Fatalf("tick to %s: %v", want, err)
		}
		if v.Blocked || v.CurrentStage != want {
			f.t.Fatalf("tick = %+v, want stage %s", v, want)
		}
	}
	step(orchestrator.StageTruth)
	step(orchestrator.StageImplement)
	step(orchestrator.StageQA)
	qaArtifact := filepath.Join(f.t.TempDir(), "qa-report.md")
	if err := os.WriteFile(qaArtifact, []byte("# qa report\npass\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.eng.RegisterArtifact(run.RunID, "qa_evidence", qaArtifact, "qa-bot"); err != nil {
		f.t.Fatalf("register qa artifact: %v", err)
	}
	step(orchestrator.StageRelease)
	verifier := filepath.Join(f.t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# independent verification\n"), 0o644); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.eng.AttachVerifier(run.RunID, []string{verifier}, "independent-verifier", ""); err != nil {
		f.t.Fatalf("attach verifier: %v", err)
	}
	if _, err := f.eng.Complete(run.RunID, orchestrator.CompleteOptions{HumanConfirmed: true, ConfirmedBy: "owner"}); err != nil {
		f.t.Fatalf("complete: %v", err)
	}
	ev, err := f.eng.EvaluateGate(f.ctx, "release", run.RunID, "release-audit")
	if err != nil {
		f.t.Fatalf("release gate: %v", err)
	}
	if ev.Verdict != "pass" {
		f.t.Fatalf("release gate = %+v, want pass", ev)
	}
	if _, _, err := f.eng.ExportProjectState(f.root); err != nil {
		f.t.Fatalf("export state: %v", err)
	}
}

// conformance reads the run's conformance view (no pass by default).
func (f *fixture) conformance() orchestrator.ConformanceView {
	f.t.Helper()
	conf, err := f.eng.Conformance(f.runID)
	if err != nil {
		f.t.Fatalf("conformance: %v", err)
	}
	return conf
}

// coverageItem pulls the ac_tc_evidence_coverage item out of a view.
func coverageItem(v orchestrator.ConformanceView) orchestrator.ConformanceItem {
	for _, it := range v.Items {
		if it.Check == "ac_tc_evidence_coverage" {
			return it
		}
	}
	return orchestrator.ConformanceItem{}
}

// observeConformance turns a conformance view into one observation:
// exit 1 when the delivery is not recognised, status/reason/names from
// the coverage item.
func observeConformance(v orchestrator.ConformanceView) Observation {
	item := coverageItem(v)
	obs := Observation{
		ExitCode: 0, Status: item.Status, Detail: item.Detail, Names: append([]string{}, item.Missing...),
	}
	if !v.Pass {
		obs.ExitCode = 1
	}
	if len(item.ReasonCodes) > 0 {
		obs.ReasonCode = item.ReasonCodes[0]
	}
	return obs
}
