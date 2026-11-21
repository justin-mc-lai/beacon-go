package evalgate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/gate"
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
	// Real runner + candidate + independent product report replace artifact-only QA.
	f.write("qa-case.py", `import os,json,pathlib
assert 2+2==4
pathlib.Path(os.environ['BEACON_TEST_RESULT_PATH']).write_text(json.dumps({'schema':'beacon-test-execution.v1','run_id':os.environ['BEACON_RUN_ID'],'tc':os.environ['BEACON_TC_ID'],'candidate_hash':os.environ['BEACON_QA_CANDIDATE_HASH'],'cases':[{'name':'actual-case','status':'passed','assertions':1}]}))
`)
	f.write("source.py", "print('implementation')\n")
	f.write("tests.md", "| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | python3 qa-case.py | actual assertion |\n")
	hash := func(name string) string {
		b, err := os.ReadFile(filepath.Join(f.root, name))
		if err != nil {
			f.t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(b))
	}
	manifest := map[string]any{"schema": "beacon-qa-candidate.v1", "run_id": run.RunID, "feature_slug": "feat-a", "truth_sha256": hash("truth.md"), "tests_sha256": hash("tests.md"), "source_commit": "", "source_files": []any{map[string]string{"path": "source.py", "sha256": hash("source.py")}}, "maker": "maker", "build_command": "fixture snapshot", "artifacts": []any{map[string]string{"path": filepath.Join(f.root, "source.py"), "sha256": hash("source.py")}}}
	b, _ := json.Marshal(manifest)
	f.write("candidate.json", string(b))
	if _, err := f.eng.RegisterArtifact(run.RunID, "qa_candidate", filepath.Join(f.root, "candidate.json"), "maker"); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.eng.RunAutoTest(context.Background(), run.RunID, orchestrator.AutoTestOptions{By: "maker"}); err != nil {
		f.t.Fatal(err)
	}
	f.write("observed.log", "independent assertion\n")
	if _, err := f.eng.RegisterArtifact(run.RunID, "log", filepath.Join(f.root, "observed.log"), "independent-verifier"); err != nil {
		f.t.Fatal(err)
	}
	var tcs []string
	for _, r := range gate.ParseCoverageRows(string(mustReadFixture(f, "tests.md"))) {
		tcs = append(tcs, r.TC)
	}
	b, _ = json.Marshal(map[string]any{"schema": "beacon-product-qa.v1", "run_id": run.RunID, "candidate_hash": hash("candidate.json"), "scope": "product", "verdict": "pass", "maker": "maker", "checker": "independent-verifier", "covered_tcs": tcs, "evidence_refs": []string{filepath.Join(f.root, "observed.log")}})
	f.write("product-qa.json", string(b))
	productReport := filepath.Join(f.root, "product-qa.json")
	if _, err := f.eng.RegisterArtifact(run.RunID, "qa_report", productReport, "independent-verifier"); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.eng.AttachVerifier(run.RunID, []string{productReport}, "independent-verifier", ""); err != nil {
		f.t.Fatal(err)
	}
	if ev, err := f.eng.EvaluateGate(f.ctx, "qa", run.RunID, "independent-verifier"); err != nil || ev.Verdict != "pass" {
		f.t.Fatalf("QA: %+v %v", ev, err)
	}
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
	if _, err := f.eng.AttachVerifier(run.RunID, []string{productReport}, "independent-verifier", ""); err != nil {
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

func mustReadFixture(f *fixture, name string) []byte {
	b, err := os.ReadFile(filepath.Join(f.root, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return b
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
