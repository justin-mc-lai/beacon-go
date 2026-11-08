package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// emitTraceEvidence writes the TC's observed facts when the evidence
// directory is supplied. The frozen tests.md commands run without the
// variable; the evidence round sets it, so the artifact is produced by
// the assertions themselves rather than transcribed afterwards.
func emitTraceEvidence(t *testing.T, name, command string, outcome any) {
	t.Helper()
	dir := os.Getenv("BEACON_TRACEABILITY_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if text, ok := outcome.(string); ok {
		body := fmt.Sprintf("# %s\n\ncommand: %s\nresult: PASS\n\n%s\n", name, command, text)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := json.MarshalIndent(map[string]any{
		"command": command, "result": "pass", "observed": outcome,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

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

const traceabilityTestsDoc = `---
slug: feat-a
---
| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | ` + "`go test ./... -run TestAC001`" + ` | ac_001_holds | evidence/covered-001.txt |
| AC-002 | TC-002 | behavior | ` + "`go test ./... -run TestAC002`" + ` | ac_002_holds | evidence/covered-002.txt |
`

// newTraceabilityRunEnv delivers one run whose base conformance facts all
// pass, then lets the caller rewrite the package under test. The package
// lives at the project root (newGateEnv's shape), so its evidence dir is
// <root>/evidence.
func newTraceabilityRunEnv(t *testing.T) (gateTestEnv, string) {
	t.Helper()
	env := newGateEnv(t, frozenTruthDoc)
	runID := deliverParadigmRun(t, env)
	if _, _, err := env.eng.ExportProjectState(env.root); err != nil {
		t.Fatalf("export state: %v", err)
	}
	return env, runID
}

func writeTraceabilityPackage(t *testing.T, env gateTestEnv, truth, tests string, evidence map[string]string) {
	t.Helper()
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(env.root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", truth)
	write("tests.md", tests)
	if err := os.MkdirAll(filepath.Join(env.root, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range evidence {
		if err := os.WriteFile(filepath.Join(env.root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func traceabilityConformance(t *testing.T, env gateTestEnv, runID string) ConformanceView {
	t.Helper()
	conf, err := env.eng.Conformance(runID)
	if err != nil {
		t.Fatalf("conformance: %v", err)
	}
	return conf
}

// TC-001 / AC-001: a fully covered package passes the new check and the
// whole delivery conforms.
func TestConformanceTraceabilityAllCovered(t *testing.T) {
	env, runID := newTraceabilityRunEnv(t)
	writeTraceabilityPackage(t, env, traceabilityTruthDoc, traceabilityTestsDoc, map[string]string{
		"evidence/covered-001.txt": "TC-001 evidence\n",
		"evidence/covered-002.txt": "TC-002 evidence\n",
	})
	conf := traceabilityConformance(t, env, runID)
	if !conf.Pass {
		t.Fatalf("complete traceability package must conform: %+v", conf)
	}
	it := conformanceItem(conf, "ac_tc_evidence_coverage")
	if it.Status != "pass" {
		t.Fatalf("coverage item = %+v, want pass", it)
	}
	if !strings.Contains(it.Detail, "2 AC covered by 2 executable TC") {
		t.Fatalf("detail must count the covered ACs/TCs: %q", it.Detail)
	}
	emitTraceEvidence(t, "traceability-covered.json",
		"go test ./internal/orchestrator -run TestConformanceTraceabilityAllCovered -count=1",
		map[string]any{"exit_code": 0, "conformance": conf})
}

// TC-002 / AC-002: deleting one AC's TC row fails closed and names the
// missing AC in the item, the detail and the failed array.
func TestConformanceTraceabilityMissingTC(t *testing.T) {
	env, runID := newTraceabilityRunEnv(t)
	oneRow := strings.Replace(traceabilityTestsDoc,
		"| AC-002 | TC-002 | behavior | `go test ./... -run TestAC002` | ac_002_holds | evidence/covered-002.txt |\n", "", 1)
	writeTraceabilityPackage(t, env, traceabilityTruthDoc, oneRow, map[string]string{
		"evidence/covered-001.txt": "TC-001 evidence\n",
	})
	conf := traceabilityConformance(t, env, runID)
	if conf.Pass {
		t.Fatal("missing TC must fail conformance")
	}
	it := conformanceItem(conf, "ac_tc_evidence_coverage")
	if it.Status != "fail" {
		t.Fatalf("coverage item = %+v, want fail", it)
	}
	if !contains(it.ReasonCodes, core.ReasonTraceabilityMissingTC) {
		t.Fatalf("reason codes = %v, want %s", it.ReasonCodes, core.ReasonTraceabilityMissingTC)
	}
	if !contains(it.Missing, "AC-002") || !strings.Contains(it.Detail, "AC-002") {
		t.Fatalf("failure must name AC-002: item=%+v detail=%q", it, it.Detail)
	}
	label := strings.Join(conf.Failed, "|")
	if !strings.Contains(label, core.ReasonTraceabilityMissingTC) || !strings.Contains(label, "AC-002") {
		t.Fatalf("failed array must carry the reason code and missing id: %v", conf.Failed)
	}
	emitTraceEvidence(t, "traceability-missing-tc.json",
		"go test ./internal/orchestrator -run TestConformanceTraceabilityMissingTC -count=1",
		map[string]any{"exit_code": 0, "conformance": conf})
}

// TC-003 / AC-002: an Evidence path whose file does not exist fails with
// traceability_missing_evidence naming AC-ID/TC-ID.
func TestConformanceTraceabilityMissingEvidence(t *testing.T) {
	env, runID := newTraceabilityRunEnv(t)
	writeTraceabilityPackage(t, env, traceabilityTruthDoc, traceabilityTestsDoc, map[string]string{
		"evidence/covered-002.txt": "TC-002 evidence\n",
	})
	conf := traceabilityConformance(t, env, runID)
	if conf.Pass {
		t.Fatal("missing evidence file must fail conformance")
	}
	it := conformanceItem(conf, "ac_tc_evidence_coverage")
	if it.Status != "fail" || !contains(it.ReasonCodes, core.ReasonTraceabilityMissingEvidence) {
		t.Fatalf("coverage item = %+v, want fail/%s", it, core.ReasonTraceabilityMissingEvidence)
	}
	if !contains(it.Missing, "AC-001/TC-001") || !strings.Contains(it.Detail, "AC-001/TC-001") {
		t.Fatalf("failure must name AC-001/TC-001: item=%+v detail=%q", it, it.Detail)
	}
	emitTraceEvidence(t, "traceability-missing-evidence.json",
		"go test ./internal/orchestrator -run TestConformanceTraceabilityMissingEvidence -count=1",
		map[string]any{"exit_code": 0, "conformance": conf})
}

// TC-005 / AC-002: missing tests.md, an unparseable matrix and truth with
// no AC definition all fail closed (never pass, never not_required).
func TestConformanceTraceabilityFailClosed(t *testing.T) {
	env, runID := newTraceabilityRunEnv(t)
	results := map[string]any{}
	check := func(name string, conf ConformanceView) {
		t.Helper()
		it := conformanceItem(conf, "ac_tc_evidence_coverage")
		if it.Status == "pass" || it.Status == "not_required" {
			t.Fatalf("%s: status = %q, want a fail-closed failure", name, it.Status)
		}
		if len(it.ReasonCodes) == 0 {
			t.Fatalf("%s: failure carries no reason code: %+v", name, it)
		}
		if conf.Pass {
			t.Fatalf("%s: conformance must not pass", name)
		}
		results[name] = map[string]any{"pass": conf.Pass, "item": it, "failed": conf.Failed}
	}

	// 1. tests.md missing.
	writeTraceabilityPackage(t, env, traceabilityTruthDoc, "", nil)
	if err := os.Remove(filepath.Join(env.root, "tests.md")); err != nil {
		t.Fatal(err)
	}
	check("tests.md missing", traceabilityConformance(t, env, runID))

	// 2. coverage matrix without an AC ID header.
	noMatrix := "---\nslug: feat-a\n---\n| id | case |\n|---|---|\n| AC-001 | nope |\n"
	writeTraceabilityPackage(t, env, traceabilityTruthDoc, noMatrix,
		map[string]string{"evidence/covered-001.txt": "x\n"})
	check("unparseable matrix", traceabilityConformance(t, env, runID))

	// 3. truth declares the contract but defines no AC.
	noAC := strings.Replace(traceabilityTruthDoc,
		"- **AC-001**：the first requirement\n- **AC-002**：the second requirement\n", "nothing yet\n", 1)
	writeTraceabilityPackage(t, env, noAC, traceabilityTestsDoc,
		map[string]string{"evidence/covered-001.txt": "x\n", "evidence/covered-002.txt": "y\n"})
	check("no AC defined", traceabilityConformance(t, env, runID))
	emitTraceEvidence(t, "traceability-failclosed.json",
		"go test ./internal/orchestrator -run TestConformanceTraceabilityFailClosed -count=1",
		results)
}

// TC-006 / AC-003: a package that does not declare the contract reports
// not_required and the four legacy judgements still decide conformance.
func TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	runID := deliverParadigmRun(t, env)
	if _, _, err := env.eng.ExportProjectState(env.root); err != nil {
		t.Fatal(err)
	}
	conf := traceabilityConformance(t, env, runID)
	if !conf.Pass {
		t.Fatalf("legacy package must keep its conformance: %+v", conf)
	}
	it := conformanceItem(conf, "ac_tc_evidence_coverage")
	if it.Status != "not_required" {
		t.Fatalf("coverage item = %+v, want not_required", it)
	}
	for _, name := range []string{"truth_frozen", "gates_evaluated", "verifier_attached", "state_exported"} {
		if got := conformanceItem(conf, name); got.Status != "pass" {
			t.Fatalf("legacy judgement %s = %+v, want pass", name, got)
		}
	}
	if len(conf.Failed) != 0 {
		t.Fatalf("legacy package failed nothing, got %v", conf.Failed)
	}
	emitTraceEvidence(t, "traceability-not-required.json",
		"go test ./internal/orchestrator -run TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict -count=1",
		map[string]any{"conformance": conf})
}

// seedTwoACFrozenPackage creates a two-AC package, freezes it, and
// returns the context plus the truth/tests paths. It exists so the AC
// diff tests start from a package whose AC-002 can move.
func seedTwoACFrozenPackage(t *testing.T, env truthEnv, slug string) store.FeatureContext {
	t.Helper()
	env.create(t, slug)
	ctx := env.ctx(t, slug)
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	text := readFile(t, truthPath)
	text = strings.Replace(text,
		"| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |",
		"| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |\n| AC-002 | 第二条可验证的完成标准（改前文本） |", 1)
	text = strings.Replace(text,
		"| AC-001 | test utterance | conversation:test |",
		"| AC-001 | test utterance | conversation:test |\n| AC-002 | test utterance | conversation:test |", 1)
	if err := os.WriteFile(truthPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	testsPath := filepath.Join(env.root, ctx.Feature.TestsPath)
	tests := readFile(t, testsPath)
	tests = strings.Replace(tests,
		"| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |\n",
		"| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |\n| AC-002 | TC-002 | behavior | `go test ./... -run TestAC002` | ac_002_holds |\n", 1)
	if err := os.WriteFile(testsPath, []byte(tests), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("freeze two-AC package: %v", err)
	}
	return ctx
}

// TC-007 / AC-004: a refreeze records changed AC-002 and added AC-004 in
// the change document, with two different fingerprint prefixes.
func TestRefreezeRecordsACDiff(t *testing.T) {
	env := newTruthEnv(t)
	ctx := seedTwoACFrozenPackage(t, env, "feat-diff")
	cv, err := env.eng.RequestTruthChange(ctx, "widen AC-002, add AC-004", "alice")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	text := readFile(t, truthPath)
	text = strings.Replace(text, "第二条可验证的完成标准（改前文本）", "第二条可验证的完成标准（改后文本）", 1)
	text = strings.Replace(text,
		"| AC-002 | 第二条可验证的完成标准（改后文本） |",
		"| AC-002 | 第二条可验证的完成标准（改后文本） |\n| AC-004 | 新增的第四条完成标准 |", 1)
	text = strings.Replace(text,
		"| AC-002 | test utterance | conversation:test |",
		"| AC-002 | test utterance | conversation:test |\n| AC-004 | test utterance | conversation:test |", 1)
	if err := os.WriteFile(truthPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	testsPath := filepath.Join(env.root, ctx.Feature.TestsPath)
	tests := readFile(t, testsPath)
	tests = strings.Replace(tests,
		"| AC-002 | TC-002 | behavior | `go test ./... -run TestAC002` | ac_002_holds |\n",
		"| AC-002 | TC-002 | behavior | `go test ./... -run TestAC002` | ac_002_holds |\n| AC-004 | TC-004 | behavior | `go test ./... -run TestAC004` | ac_004_holds |\n", 1)
	if err := os.WriteFile(testsPath, []byte(tests), 0o644); err != nil {
		t.Fatal(err)
	}
	rv, err := env.eng.RefreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("refreeze: %v", err)
	}
	if !rv.Frozen || rv.ChangePending {
		t.Fatalf("refrozen view = %+v", rv)
	}
	doc := readFile(t, filepath.Join(env.root, filepath.FromSlash(cv.ChangeDoc)))
	section := doc[strings.Index(doc, "## AC Diff"):]
	changedAt := strings.Index(section, "changed:")
	removedAt := strings.Index(section, "removed:")
	if changedAt < 0 || removedAt < 0 || removedAt < changedAt {
		t.Fatalf("change doc is missing the AC Diff groups:\n%s", section)
	}
	changedBlock := section[changedAt:removedAt]
	if !strings.Contains(changedBlock, "AC-002") || !strings.Contains(changedBlock, "before") || !strings.Contains(changedBlock, "after") {
		t.Fatalf("changed group must name AC-002 with before/after:\n%s", changedBlock)
	}
	if !strings.Contains(section, "added:") || !strings.Contains(section[:changedAt], "AC-004") {
		t.Fatalf("added group must name AC-004:\n%s", section)
	}
	if !strings.Contains(section[removedAt:], "(none)") {
		t.Fatalf("removed group must be empty:\n%s", section[removedAt:])
	}
	if rv.ChangeACDiff == nil {
		t.Fatal("refrozen view must expose change_ac_diff")
	}
	if len(rv.ChangeACDiff.Added) != 1 || rv.ChangeACDiff.Added[0] != "AC-004" {
		t.Fatalf("view added = %v, want AC-004", rv.ChangeACDiff.Added)
	}
	if len(rv.ChangeACDiff.Changed) != 1 || rv.ChangeACDiff.Changed[0].AC != "AC-002" ||
		rv.ChangeACDiff.Changed[0].Before == rv.ChangeACDiff.Changed[0].After {
		t.Fatalf("view changed = %+v, want AC-002 with distinct prefixes", rv.ChangeACDiff.Changed)
	}
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	payloadOK := false
	for _, ev := range events {
		if ev.EventType != EventTruthRefrozen {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if _, ok := p["change_ac_diff"]; ok {
			payloadOK = true
		}
	}
	if !payloadOK {
		t.Fatal("truth_refrozen payload must carry change_ac_diff")
	}
	emitTraceEvidence(t, "change-ac-diff.md",
		"go test ./internal/orchestrator -run TestRefreezeRecordsACDiff -count=1",
		"observed change_ac_diff: "+traceJSON(t, rv.ChangeACDiff)+"\n\nchange document section:\n\n"+section)
}

// TC-009 / AC-004: an unwritable change document fails the refreeze and
// leaves the change pending with no truth_refrozen event.
func TestRefreezeACDiffFailClosed(t *testing.T) {
	env := newTruthEnv(t)
	ctx := seedTwoACFrozenPackage(t, env, "feat-diff-fail")
	cv, err := env.eng.RequestTruthChange(ctx, "edit AC-002", "alice")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	text := strings.Replace(readFile(t, truthPath), "第二条可验证的完成标准（改前文本）", "第二条可验证的完成标准（改后文本）", 1)
	if err := os.WriteFile(truthPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make the change document unreadable/unwritable as a file.
	docPath := filepath.Join(env.root, filepath.FromSlash(cv.ChangeDoc))
	if err := os.Remove(docPath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docPath, 0o755); err != nil {
		t.Fatal(err)
	}
	view, rerr := env.eng.RefreezeTruthPackage(ctx, "qa-bot")
	if rerr == nil {
		t.Fatal("refreeze with an unwritable change document must fail")
	}
	if rc, _ := core.ReasonCodeOf(rerr); rc != ReasonTruthACDiffWriteFailed {
		t.Fatalf("reason = %s, want %s (err=%v)", rc, ReasonTruthACDiffWriteFailed, rerr)
	}
	if !view.ChangePending {
		t.Fatalf("failed refreeze must leave the change pending: %+v", view)
	}
	now, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !now.ChangePending || now.Frozen {
		t.Fatalf("package after failed refreeze = %+v, want change_pending", now)
	}
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType == EventTruthRefrozen {
			t.Fatal("failed refreeze must not write truth_refrozen")
		}
	}
	emitTraceEvidence(t, "refreeze-ac-diff-failclosed.txt",
		"go test ./internal/orchestrator -run TestRefreezeACDiffFailClosed -count=1",
		"refreeze error: "+rerr.Error()+"\npackage state: status="+now.Status+" change_pending=true\nevent scan: no truth_refrozen\n")
}

func traceJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
