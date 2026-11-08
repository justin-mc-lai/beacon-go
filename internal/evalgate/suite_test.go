package evalgate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/cli"
	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
)

// executor runs one case's fixture and returns what really happened.
type executor func(*testing.T) (Observation, error)

// executorFor is the one place a case ID maps to its fixture runner.
func executorFor(id string) (executor, bool) {
	switch id {
	case "stage_gate/implement_blocked_unfrozen_truth":
		return execStageBlocked, true
	case "stage_gate/implement_pass_complete":
		return execStagePass, true
	case "conformance/all_covered":
		return execConformanceAllCovered, true
	case "conformance/missing_tc":
		return execConformanceMissingTC, true
	case "conformance/missing_evidence_file":
		return execConformanceMissingEvidence, true
	case "conformance/evidence_escape":
		return execConformanceEvidenceEscape, true
	case "conformance/evidence_column_missing":
		return execConformanceEvidenceColumnMissing, true
	case "conformance/failclosed_tests_missing":
		return execFailClosedTestsMissing, true
	case "conformance/failclosed_matrix_unparseable":
		return execFailClosedMatrixUnparseable, true
	case "conformance/failclosed_no_ac":
		return execFailClosedNoAC, true
	case "conformance/not_required_compat":
		return execNotRequiredCompat, true
	case "rejection/unknown_gate_type":
		return execUnknownGateType, true
	case "rejection/missing_evaluator":
		return execMissingEvaluator, true
	case "rejection/unknown_feature":
		return execUnknownFeature, true
	case "rejection/unknown_run_conformance":
		return execUnknownRunConformance, true
	case "no_degradation/expectation_mismatch_detected":
		return execMismatchDetected, true
	case "no_degradation/deterministic_twice":
		return execDeterministicTwice, true
	case "no_degradation/offline_loopback_blocked":
		return execOfflineLoopbackBlocked, true
	}
	return nil, false
}

// ---- fixture helpers ----

func conformanceFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t, baseTruthDoc)
	f.deliver()
	return f
}

func fullEvidence() map[string]string {
	return map[string]string{
		"evidence/covered-001.txt": "TC-001 evidence\n",
		"evidence/covered-002.txt": "TC-002 evidence\n",
	}
}

// removeRow drops every coverage row that names one id.
func removeRow(doc, id string) string {
	var kept []string
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| "+id+" |") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// runBeaconCLI executes a fresh root command in-process; the fixture's
// store.EnvStorePath makes it read the same database the engine wrote.
func runBeaconCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := cli.NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

// assertCLITraceabilityFailure pins the CLI contract of AC-002: the
// text output and the --json output both exit non-zero and both carry
// the reason code and the named id.
func assertCLITraceabilityFailure(t *testing.T, runID, reasonCode, named string) {
	t.Helper()
	text, err := runBeaconCLI(t, "goal", "conformance", "--run-id", runID)
	if err == nil {
		t.Fatalf("CLI text conformance exited 0 for a failing package:\n%s", text)
	}
	for _, want := range []string{reasonCode, named} {
		if !strings.Contains(text, want) {
			t.Fatalf("CLI text output missing %q:\n%s", want, text)
		}
	}
	jsonOut, jerr := runBeaconCLI(t, "goal", "conformance", "--run-id", runID, "--json")
	if jerr == nil {
		t.Fatalf("CLI --json conformance exited 0 for a failing package:\n%s", jsonOut)
	}
	var doc struct {
		Failed []string `json:"failed"`
		Items  []struct {
			Check       string   `json:"check"`
			Status      string   `json:"status"`
			ReasonCodes []string `json:"reason_codes"`
			Missing     []string `json:"missing"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &doc); err != nil {
		t.Fatalf("--json output is not one JSON document: %v\n%s", err, jsonOut)
	}
	covered := false
	for _, it := range doc.Items {
		if it.Check != "ac_tc_evidence_coverage" {
			continue
		}
		covered = true
		if it.Status == "pass" || it.Status == "not_required" {
			t.Fatalf("coverage item must fail, got status %q", it.Status)
		}
		if !contains(it.ReasonCodes, reasonCode) {
			t.Fatalf("coverage item reason codes = %v, want %s", it.ReasonCodes, reasonCode)
		}
		if !contains(it.Missing, named) && !strings.Contains(strings.Join(it.Missing, "|"), named) {
			t.Fatalf("coverage item missing = %v, want %s", it.Missing, named)
		}
	}
	if !covered {
		t.Fatalf("--json output carries no ac_tc_evidence_coverage item:\n%s", jsonOut)
	}
	if !strings.Contains(strings.Join(doc.Failed, "|"), reasonCode) {
		t.Fatalf("failed array = %v, want the reason code", doc.Failed)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ---- stage gate ----

// execStageBlocked: truth is a draft, so the implement admission exit
// blocks and carries stage_exit_truth_not_frozen.
func execStageBlocked(t *testing.T) (Observation, error) {
	draft := strings.Replace(baseTruthDoc, "status: frozen", "status: draft", 1)
	f := newFixtureWithStatus(t, draft, "draft")
	f.parkAtTruth(false)
	v, err := f.eng.Tick(context.Background(), f.runID)
	if err != nil {
		return Observation{}, err
	}
	obs := Observation{ExitCode: 0, Status: "pass", ReasonCode: v.ReasonCode,
		Detail: "run " + f.runID + " truth->implement exit passed: next_stage=" + v.CurrentStage}
	if v.Blocked {
		obs.ExitCode, obs.Status = 1, "blocked"
		obs.Detail = "run " + f.runID + " truth->implement exit blocked: next_stage=" + v.CurrentStage +
			" reason=" + v.ReasonCode
		if v.StageGate != nil && v.StageGate.Verdict != "" {
			obs.Detail += " gate_verdict=" + v.StageGate.Verdict
		}
	}
	return obs, nil
}

// execStagePass: frozen truth clears the same admission and reaches
// implement.
func execStagePass(t *testing.T) (Observation, error) {
	f := newFixture(t, baseTruthDoc)
	f.parkAtTruth(false)
	v, err := f.eng.Tick(context.Background(), f.runID)
	if err != nil {
		return Observation{}, err
	}
	obs := Observation{ExitCode: 0, Status: "pass", ReasonCode: v.ReasonCode,
		Detail: "run " + f.runID + " truth->implement exit passed: next_stage=" + v.CurrentStage}
	if v.Blocked || v.CurrentStage != orchestrator.StageImplement {
		obs.ExitCode, obs.Status = 1, "blocked"
		obs.Detail = "run " + f.runID + " truth->implement exit did not reach implement: next_stage=" + v.CurrentStage
	}
	return obs, nil
}

// ---- conformance ----

func execConformanceAllCovered(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	f.rewritePackage(traceabilityTruthDoc, traceabilityTestsDoc, fullEvidence(), false)
	return observeConformance(f.conformance()), nil
}

func execConformanceMissingTC(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	f.rewritePackage(traceabilityTruthDoc, removeRow(traceabilityTestsDoc, "AC-002"), map[string]string{
		"evidence/covered-001.txt": "TC-001 evidence\n",
	}, false)
	obs := observeConformance(f.conformance())
	if obs.ReasonCode != "traceability_missing_tc" || !obs.HasName("AC-002") {
		return Observation{}, fmt.Errorf("missing-TC fixture did not name AC-002: %+v", obs)
	}
	assertCLITraceabilityFailure(t, f.runID, "traceability_missing_tc", "AC-002")
	obs.Detail += "; cli text and --json exited non-zero with the same reason and id"
	return obs, nil
}

func execConformanceMissingEvidence(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	f.rewritePackage(traceabilityTruthDoc, traceabilityTestsDoc, map[string]string{
		"evidence/covered-002.txt": "TC-002 evidence\n",
	}, false)
	return observeConformance(f.conformance()), nil
}

func execConformanceEvidenceEscape(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	escaped := strings.Replace(traceabilityTestsDoc, "evidence/covered-001.txt", "../escape.txt", 1)
	f.rewritePackage(traceabilityTruthDoc, escaped, fullEvidence(), false)
	return observeConformance(f.conformance()), nil
}

func execConformanceEvidenceColumnMissing(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	noColumn := `---
slug: feat-a
---
| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | ` + "`go test ./... -run TestAC001`" + ` | ac_001_holds |
| AC-002 | TC-002 | behavior | ` + "`go test ./... -run TestAC002`" + ` | ac_002_holds |
`
	f.rewritePackage(traceabilityTruthDoc, noColumn, fullEvidence(), false)
	return observeConformance(f.conformance()), nil
}

func execFailClosedTestsMissing(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	f.rewritePackage(traceabilityTruthDoc, "", nil, true)
	obs := observeConformance(f.conformance())
	if obs.Status == "pass" || obs.Status == "not_required" {
		return Observation{}, fmt.Errorf("missing tests.md must fail closed, got %+v", obs)
	}
	assertCLITraceabilityFailure(t, f.runID, "traceability_missing_tc", "AC-001")
	return obs, nil
}

func execFailClosedMatrixUnparseable(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	bad := "---\nslug: feat-a\n---\n| id | case |\n|---|---|\n| AC-001 | nope |\n"
	f.rewritePackage(traceabilityTruthDoc, bad, fullEvidence(), false)
	obs := observeConformance(f.conformance())
	if obs.Status == "pass" || obs.Status == "not_required" {
		return Observation{}, fmt.Errorf("unparseable matrix must fail closed, got %+v", obs)
	}
	return obs, nil
}

func execFailClosedNoAC(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	noAC := strings.Replace(traceabilityTruthDoc,
		"- **AC-001**：the first requirement\n- **AC-002**：the second requirement\n", "nothing yet\n", 1)
	f.rewritePackage(noAC, traceabilityTestsDoc, fullEvidence(), false)
	obs := observeConformance(f.conformance())
	if obs.Status == "pass" || obs.Status == "not_required" {
		return Observation{}, fmt.Errorf("truth without AC must fail closed, got %+v", obs)
	}
	return obs, nil
}

func execNotRequiredCompat(t *testing.T) (Observation, error) {
	f := conformanceFixture(t)
	conf := f.conformance()
	for _, name := range []string{"truth_frozen", "gates_evaluated", "verifier_attached", "state_exported"} {
		found := false
		for _, it := range conf.Items {
			if it.Check == name {
				found = true
				if it.Status != "pass" {
					return Observation{}, fmt.Errorf("legacy judgement %s = %+v, want pass", name, it)
				}
			}
		}
		if !found {
			return Observation{}, fmt.Errorf("legacy judgement %s missing from the conformance view", name)
		}
	}
	return observeConformance(conf), nil
}

// ---- rejection ----

func execUnknownGateType(t *testing.T) (Observation, error) {
	f := newFixture(t, baseTruthDoc)
	_, err := f.eng.EvaluateGate(f.ctx, "nonsense", "", "qa-bot")
	if err == nil {
		return Observation{}, fmt.Errorf("unknown gate type was accepted")
	}
	code := reasonOf(err)
	if err := assertNoGateRows(t, f); err != nil {
		return Observation{}, err
	}
	return Observation{ExitCode: 1, ReasonCode: code, Detail: err.Error()}, nil
}

func execMissingEvaluator(t *testing.T) (Observation, error) {
	f := newFixture(t, baseTruthDoc)
	_, err := f.eng.EvaluateGate(f.ctx, "truth_freeze", "", "   ")
	if err == nil {
		return Observation{}, fmt.Errorf("empty evaluator was accepted")
	}
	code := reasonOf(err)
	if err := assertNoGateRows(t, f); err != nil {
		return Observation{}, err
	}
	return Observation{ExitCode: 1, ReasonCode: code, Detail: err.Error()}, nil
}

func execUnknownFeature(t *testing.T) (Observation, error) {
	f := newFixture(t, baseTruthDoc)
	_, err := f.eng.CreateRun(orchestrator.CreateRunOptions{
		ProjectRoot: f.root, FeatureSlug: "no-such-feature", ExecutorID: "maker",
		ExecutorMode: orchestrator.ExecutorModeRecording,
	})
	if err == nil {
		return Observation{}, fmt.Errorf("run creation for an unknown feature succeeded")
	}
	return Observation{ExitCode: 1, ReasonCode: reasonOf(err), Detail: err.Error()}, nil
}

func execUnknownRunConformance(t *testing.T) (Observation, error) {
	f := newFixture(t, baseTruthDoc)
	_, err := f.eng.Conformance("run_does_not_exist")
	if err == nil {
		return Observation{}, fmt.Errorf("conformance for an unknown run succeeded")
	}
	code := reasonOf(err)
	if code == "" {
		return Observation{}, fmt.Errorf("unknown-run conformance carries no reason code: %v", err)
	}
	return Observation{ExitCode: 1, ReasonCode: code, Detail: err.Error()}, nil
}

func reasonOf(err error) string {
	if err == nil {
		return ""
	}
	if rc, ok := core.ReasonCodeOf(err); ok {
		return rc
	}
	return ""
}

func assertNoGateRows(t *testing.T, f *fixture) error {
	rows, err := f.st.Gates().ListByFeature(f.ctx.Feature.ID)
	if err != nil {
		return err
	}
	if len(rows) != 0 {
		return fmt.Errorf("a rejected gate wrote %d pass/fail rows", len(rows))
	}
	return nil
}

// ---- no degradation ----

// comparisonCases is every non-meta case: the two no_degradation cases
// that themselves run the registry are excluded so the nesting is
// bounded.
func comparisonCases() []Case {
	var out []Case
	for _, c := range Registry() {
		if c.ID == "no_degradation/deterministic_twice" || c.ID == "no_degradation/offline_loopback_blocked" {
			continue
		}
		out = append(out, c)
	}
	return out
}

func findCase(t *testing.T, id string) Case {
	t.Helper()
	for _, c := range Registry() {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("case %s is not in the registry", id)
	return Case{}
}

func mustExecutor(t *testing.T, id string) executor {
	t.Helper()
	ex, ok := executorFor(id)
	if !ok {
		t.Fatalf("case=%s: no executor registered", id)
	}
	return ex
}

// execMismatchDetected injects the four kinds of wrong expectation and
// proves each is reported as a failure naming the case, never a pass or
// a skip.
func execMismatchDetected(t *testing.T) (Observation, error) {
	t.Helper()
	allCovered := findCase(t, "conformance/all_covered")
	missingTC := findCase(t, "conformance/missing_tc")

	// 1: expected failure but the real result succeeds.
	c1 := allCovered
	c1.Expect = &Expect{ExitCode: 1}
	// 2: expected success but the real result fails.
	c2 := missingTC
	c2.Expect = &Expect{ExitCode: 0}
	// 3: wrong reason code.
	c3 := missingTC
	c3.Expect = &Expect{ExitCode: 1, ReasonCode: "traceability_missing_evidence"}

	_, errs := collectReport(t, []Case{c1, c2, c3}, nil)
	if len(errs) != 3 {
		return Observation{}, fmt.Errorf("expected 3 mismatch errors, got %d: %v", len(errs), errs)
	}
	for _, e := range errs {
		msg := e.Error()
		for _, want := range []string{"case=", "expected=", "actual="} {
			if !strings.Contains(msg, want) {
				return Observation{}, fmt.Errorf("mismatch error missing %q: %s", want, msg)
			}
		}
		if strings.Contains(msg, "skip") {
			return Observation{}, fmt.Errorf("mismatch was reported as a skip: %s", msg)
		}
	}

	// 4: unparseable output.
	stub := map[string]executor{
		"conformance/all_covered": func(*testing.T) (Observation, error) {
			return Observation{Unparseable: true}, nil
		},
	}
	_, uerrs := collectReport(t, []Case{allCovered}, stub)
	if len(uerrs) != 1 || !strings.Contains(uerrs[0].Error(), "actual=unparseable-output") {
		return Observation{}, fmt.Errorf("unparseable observation was not rejected: %v", uerrs)
	}
	return Observation{ExitCode: 1, Status: "fail",
		Detail: fmt.Sprintf("4 injected mismatches rejected (%d with case/expected/actual, 1 unparseable)", len(errs))}, nil
}

// execDeterministicTwice runs the comparison set twice and compares the
// normalized bytes.
func execDeterministicTwice(t *testing.T) (Observation, error) {
	t.Helper()
	cases := comparisonCases()
	r1, e1 := collectReport(t, cases, nil)
	if len(e1) > 0 {
		return Observation{}, fmt.Errorf("first run failed: %v", e1)
	}
	r2, e2 := collectReport(t, cases, nil)
	if len(e2) > 0 {
		return Observation{}, fmt.Errorf("second run failed: %v", e2)
	}
	n1, n2 := Normalize(r1), Normalize(r2)
	if !bytes.Equal(n1, n2) {
		return Observation{ExitCode: 1, Status: "fail", Detail: "normalized reports differ: " + firstDiff(n1, n2)}, nil
	}
	volatile := 0
	for _, e := range r1 {
		if volatileRunID.MatchString(e.Actual.Detail) {
			volatile++
			if volatileRunID.MatchString(normalizeText(e.Actual.Detail)) {
				return Observation{}, fmt.Errorf("normalization left a run id behind in %s", e.CaseID)
			}
		}
	}
	if volatile == 0 {
		return Observation{}, fmt.Errorf("no volatile run id was observed; normalization is not exercised")
	}
	return Observation{ExitCode: 0, Status: "pass",
		Detail: fmt.Sprintf("two runs over %d cases normalized to %d identical bytes (volatile run ids: %d)", len(cases), len(n1), volatile)}, nil
}

// firstDiff names the first differing byte of two canonical reports so
// a drift failure is actionable instead of just "differ".
func firstDiff(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := i - 80
	if lo < 0 {
		lo = 0
	}
	high := func(n int) int {
		if i+120 < n {
			return i + 120
		}
		return n
	}
	return fmt.Sprintf("at byte %d: first=%q second=%q", i, string(a[lo:high(len(a))]), string(b[lo:high(len(b))]))
}

// execOfflineLoopbackBlocked blocks every proxy and runs the same set.
func execOfflineLoopbackBlocked(t *testing.T) (Observation, error) {
	t.Helper()
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(key, "http://127.0.0.1:1")
	}
	cases := comparisonCases()
	report, errs := collectReport(t, cases, nil)
	if len(errs) > 0 {
		return Observation{}, fmt.Errorf("offline run failed: %v", errs)
	}
	return Observation{ExitCode: 0, Status: "pass",
		Detail: fmt.Sprintf("%d cases passed with HTTP(S)_PROXY/ALL_PROXY at 127.0.0.1:1 (%d report entries)", len(cases), len(report))}, nil
}

// ---- report collection ----

// collectReport executes every case once and records the paired
// expectation/observation. A mismatch is returned as an error (the
// suite turns it into a non-zero exit); it is never dropped.
func collectReport(t *testing.T, cases []Case, overrides map[string]executor) (Report, []error) {
	t.Helper()
	var report Report
	var errs []error
	for _, c := range cases {
		ex, ok := overrides[c.ID]
		if !ok {
			ex, ok = executorFor(c.ID)
		}
		if !ok {
			errs = append(errs, fmt.Errorf("case=%s: no executor registered", c.ID))
			continue
		}
		obs, err := ex(t)
		entry := ReportEntry{CaseID: c.ID, Class: c.Class, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		if c.Expect != nil {
			entry.Expected = *c.Expect
		}
		if err != nil {
			entry.Actual = Observation{ExitCode: 1, Unparseable: true, Detail: err.Error()}
			errs = append(errs, fmt.Errorf("case=%s: harness error: %v", c.ID, err))
			report = append(report, entry)
			continue
		}
		entry.Actual = obs
		report = append(report, entry)
		if cerr := Check(c, obs); cerr != nil {
			errs = append(errs, cerr)
		}
	}
	return report, errs
}

// ---- evidence ----

// emitEvidence writes the TC's observed facts when the evidence round
// supplies the directory. The frozen tests.md commands run without the
// variable; the artifact is produced by the assertions themselves.
func emitEvidence(t *testing.T, name, command string, observed any) {
	t.Helper()
	dir := os.Getenv("BEACON_EVALGATE_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(map[string]any{
		"command": command, "test_result": "pass", "head": "suite",
		"note":     "test_result is the TC subtest; observed is what the kernel call actually returned",
		"observed": observed,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	cmdSuite       = "go test ./internal/evalgate -count=1"
	cmdRegistry    = "go test ./internal/evalgate -run TestEvalRegistryCoverage -count=1"
	cmdPositive    = "go test ./internal/evalgate -run TestEvalConformancePositive -count=1"
	cmdMissingTC   = "go test ./internal/evalgate -run TestEvalConformanceMissingTC -count=1"
	cmdEvidence    = "go test ./internal/evalgate -run TestEvalConformanceEvidence -count=1"
	cmdFailClosed  = "go test ./internal/evalgate -run TestEvalConformanceFailClosed -count=1"
	cmdNotRequired = "go test ./internal/evalgate -run TestEvalConformanceNotRequiredCompat -count=1"
	cmdStageGate   = "go test ./internal/evalgate -run TestEvalStageGate -count=1"
	cmdRejection   = "go test ./internal/evalgate -run TestEvalRejectionPaths -count=1"
	cmdMismatch    = "go test ./internal/evalgate -run TestEvalMismatchFailsLoudly -count=1"
	cmdDeterminism = "go test ./internal/evalgate -run TestEvalDeterministicOffline -count=1"
)

// ---- TC-001 (suite) and TC-002 ----

// TestEvalSuite runs the whole frozen registry, one subtest per case
// ID, and fails non-zero on the first difference.
func TestEvalSuite(t *testing.T) {
	cases := Registry()
	if err := Validate(cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		c := c
		t.Run(c.ID, func(t *testing.T) {
			ex, ok := executorFor(c.ID)
			if !ok {
				t.Fatalf("case=%s: no executor registered", c.ID)
			}
			obs, err := ex(t)
			if err != nil {
				t.Fatalf("case=%s: harness error: %v", c.ID, err)
			}
			if cerr := Check(c, obs); cerr != nil {
				t.Fatal(cerr)
			}
		})
	}
}

// TestEvalRegistryCoverage: four classes, unique IDs, complete
// expectations; unknown class, duplicate ID and missing expectation are
// refused by name.
func TestEvalRegistryCoverage(t *testing.T) {
	reg := Registry()
	if err := Validate(reg); err != nil {
		t.Fatal(err)
	}
	byClass := map[Class]int{}
	for _, c := range reg {
		byClass[c.Class]++
		if c.Expect == nil {
			t.Fatalf("case=%s carries no expectation", c.ID)
		}
	}
	for _, k := range Classes {
		if byClass[k] == 0 {
			t.Fatalf("class %s has no named case", k)
		}
	}

	cases := []struct {
		name   string
		bad    []Case
		caseID string
	}{
		{"unknown class", append(cloneRegistry(), Case{ID: "conformance/ghost", Class: "nope", Fixture: "x", Expect: &Expect{}}), "conformance/ghost"},
		{"duplicate id", append(cloneRegistry(), cloneRegistry()[0]), reg[0].ID},
		{"missing expectation", withNoExpectation(cloneRegistry(), 0), reg[0].ID},
	}
	for _, tc := range cases {
		err := Validate(tc.bad)
		if err == nil {
			t.Fatalf("%s: bad registry was accepted", tc.name)
		}
		if !strings.Contains(err.Error(), "case="+tc.caseID) {
			t.Fatalf("%s: error must name case=%s: %v", tc.name, tc.caseID, err)
		}
	}
	emitEvidence(t, "evalgate-registry.json", cmdRegistry, map[string]any{
		"cases": len(reg), "classes": byClass, "rejected": []string{"unknown class", "duplicate id", "missing expectation"},
	})
}

func cloneRegistry() []Case {
	reg := Registry()
	out := make([]Case, len(reg))
	for i, c := range reg {
		exp := *c.Expect
		exp.Names = append([]string{}, c.Expect.Names...)
		exp.DetailContains = append([]string{}, c.Expect.DetailContains...)
		c.Expect = &exp
		out[i] = c
	}
	return out
}

func withNoExpectation(reg []Case, i int) []Case {
	reg[i].Expect = nil
	return reg
}

// ---- TC-003 .. TC-007: conformance ----

func TestEvalConformancePositive(t *testing.T) {
	obs, err := execConformanceAllCovered(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "conformance/all_covered"), obs); cerr != nil {
		t.Fatal(cerr)
	}
	emitEvidence(t, "evalgate-conformance-positive.json", cmdPositive, obs)
}

func TestEvalConformanceMissingTC(t *testing.T) {
	obs, err := execConformanceMissingTC(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "conformance/missing_tc"), obs); cerr != nil {
		t.Fatal(cerr)
	}
	emitEvidence(t, "evalgate-conformance-missing-tc.json", cmdMissingTC, obs)
}

func TestEvalConformanceEvidence(t *testing.T) {
	all := map[string]Observation{}
	for _, id := range []string{"conformance/missing_evidence_file", "conformance/evidence_escape", "conformance/evidence_column_missing"} {
		obs, err := mustExecutor(t, id)(t)
		if err != nil {
			t.Fatalf("case=%s: %v", id, err)
		}
		if cerr := Check(findCase(t, id), obs); cerr != nil {
			t.Fatal(cerr)
		}
		all[id] = obs
	}
	emitEvidence(t, "evalgate-conformance-evidence.json", cmdEvidence, all)
}

func TestEvalConformanceFailClosed(t *testing.T) {
	all := map[string]Observation{}
	for _, id := range []string{"conformance/failclosed_tests_missing", "conformance/failclosed_matrix_unparseable", "conformance/failclosed_no_ac"} {
		obs, err := mustExecutor(t, id)(t)
		if err != nil {
			t.Fatalf("case=%s: %v", id, err)
		}
		if cerr := Check(findCase(t, id), obs); cerr != nil {
			t.Fatal(cerr)
		}
		all[id] = obs
	}
	emitEvidence(t, "evalgate-conformance-failclosed.json", cmdFailClosed, all)
}

func TestEvalConformanceNotRequiredCompat(t *testing.T) {
	obs, err := execNotRequiredCompat(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "conformance/not_required_compat"), obs); cerr != nil {
		t.Fatal(cerr)
	}
	emitEvidence(t, "evalgate-conformance-not-required.json", cmdNotRequired, obs)
}

// ---- TC-008 / TC-009: stage gate and rejections ----

func TestEvalStageGate(t *testing.T) {
	blocked, err := execStageBlocked(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "stage_gate/implement_blocked_unfrozen_truth"), blocked); cerr != nil {
		t.Fatal(cerr)
	}
	passed, err := execStagePass(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "stage_gate/implement_pass_complete"), passed); cerr != nil {
		t.Fatal(cerr)
	}
	emitEvidence(t, "evalgate-stage-gate.json", cmdStageGate, map[string]Observation{
		"stage_gate/implement_blocked_unfrozen_truth": blocked,
		"stage_gate/implement_pass_complete":          passed,
	})
}

func TestEvalRejectionPaths(t *testing.T) {
	all := map[string]Observation{}
	for _, id := range []string{
		"rejection/unknown_gate_type", "rejection/missing_evaluator",
		"rejection/unknown_feature", "rejection/unknown_run_conformance",
	} {
		obs, err := mustExecutor(t, id)(t)
		if err != nil {
			t.Fatalf("case=%s: %v", id, err)
		}
		if cerr := Check(findCase(t, id), obs); cerr != nil {
			t.Fatal(cerr)
		}
		all[id] = obs
	}
	emitEvidence(t, "evalgate-rejection-paths.json", cmdRejection, all)
}

// ---- TC-010 / TC-011 ----

func TestEvalMismatchFailsLoudly(t *testing.T) {
	obs, err := execMismatchDetected(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "no_degradation/expectation_mismatch_detected"), obs); cerr != nil {
		t.Fatal(cerr)
	}
	emitEvidence(t, "evalgate-mismatch-fails.json", cmdMismatch, obs)
}

func TestEvalDeterministicOffline(t *testing.T) {
	det, err := execDeterministicTwice(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "no_degradation/deterministic_twice"), det); cerr != nil {
		t.Fatal(cerr)
	}
	off, err := execOfflineLoopbackBlocked(t)
	if err != nil {
		t.Fatal(err)
	}
	if cerr := Check(findCase(t, "no_degradation/offline_loopback_blocked"), off); cerr != nil {
		t.Fatal(cerr)
	}
	t.Logf("determinism: %s", det.Detail)
	t.Logf("offline: %s", off.Detail)
}
