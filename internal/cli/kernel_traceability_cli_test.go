package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func emitCLITraceEvidence(t *testing.T, name, command string, outcome any) {
	t.Helper()
	dir := os.Getenv("BEACON_TRACEABILITY_EVIDENCE_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	b, err := json.MarshalIndent(map[string]any{"command": command, "result": "pass", "observed": outcome}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// deliverConformanceRun walks one recording run from plan to a released,
// verifier-attached, exported delivery through the real CLI. The caller
// then rewrites the package to isolate the traceability judgement.
func deliverConformanceRun(t *testing.T, root string) string {
	t.Helper()
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "feat-gate",
		"--executor", "recording", "--executor-id", "maker", "--json")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	id := runIDOf(t, out)
	for i := 0; i < 3; i++ {
		if out, err = runCLI(t, "goal", "tick", "--run-id", id, "--executor", "recording"); err != nil {
			t.Fatalf("tick %d: %v\n%s", i, err, out)
		}
	}
	qaArt := filepath.Join(t.TempDir(), "qa-report.md")
	if err := os.WriteFile(qaArt, []byte("# qa\npass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err = runCLI(t, "goal", "artifacts", "register", "--run-id", id,
		"--kind", "qa_evidence", "--path", qaArt, "--by", "qa-bot"); err != nil {
		t.Fatalf("artifact register: %v\n%s", err, out)
	}
	if out, err = runCLI(t, "goal", "tick", "--run-id", id, "--executor", "recording"); err != nil {
		t.Fatalf("tick to release: %v\n%s", err, out)
	}
	verRef := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verRef, []byte("# independent verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err = runCLI(t, "goal", "attach-verifier", "--run-id", id,
		"--ref", verRef, "--source", "independent-verifier", "--executor-id", "maker"); err != nil {
		t.Fatalf("attach verifier: %v\n%s", err, out)
	}
	if out, err = runCLI(t, "goal", "complete", "--run-id", id,
		"--human-confirmed", "--confirmed-by", "owner"); err != nil {
		t.Fatalf("complete: %v\n%s", err, out)
	}
	if out, err = runCLI(t, "gate", "evaluate", "release", "--project", root,
		"--feature", "feat-gate", "--run", id, "--evaluator", "release-audit"); err != nil {
		t.Fatalf("release gate: %v\n%s", err, out)
	}
	if out, err = runCLI(t, "projects", "export-state", "--project", root); err != nil {
		t.Fatalf("export-state: %v\n%s", err, out)
	}
	return id
}

// TC-004 / AC-002: the CLI reports the traceability failure non-zero and
// both the text output and the JSON items/failed carry the reason code
// and the missing AC id.
func TestGoalConformanceTraceabilityExitCode(t *testing.T) {
	root := seedDeliveryProject(t)
	id := deliverConformanceRun(t, root)
	if _, err := os.Stat(filepath.Join(root, "evidence")); os.IsNotExist(err) {
		if merr := os.MkdirAll(filepath.Join(root, "evidence"), 0o755); merr != nil {
			t.Fatal(merr)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "evidence", "covered-001.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", `---
slug: feat-gate
version: v2.0.0
status: frozen
language: en
traceability_required: true
canonical_refs:
  truth: truth.md
  tests: tests.md
  tasks: tasks.md
  evidence: evidence.md
---
# Truth

- **AC-001**：the first requirement
- **AC-002**：the second requirement
`)
	write("tests.md", "---\nslug: feat-gate\n---\n"+
		"| AC ID | TC ID | Layer | Command | Assertion | Evidence |\n"+
		"|-------|-------|-------|---------|-----------|----------|\n"+
		"| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | holds | evidence/covered-001.txt |\n")

	out, err := runCLI(t, "goal", "conformance", "--run-id", id)
	if err == nil {
		t.Fatalf("missing TC must exit non-zero:\n%s", out)
	}
	for _, want := range []string{"FAIL", "traceability_missing_tc", "AC-002"} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q:\n%s", want, out)
		}
	}
	textOut := out

	out, err = runCLI(t, "goal", "conformance", "--run-id", id, "--json")
	if err == nil {
		t.Fatalf("missing TC must exit non-zero with --json:\n%s", out)
	}
	doc := parseDoc(t, out)
	failed, _ := doc["failed"].([]any)
	var failStrings []string
	joined := ""
	for _, f := range failed {
		failStrings = append(failStrings, f.(string))
	}
	if len(failStrings) > 0 {
		joined = strings.Join(failStrings, "|")
	}
	for _, want := range []string{"ac_tc_evidence_coverage", "traceability_missing_tc", "AC-002"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failed array missing %q: %v", want, failed)
		}
	}
	items, _ := doc["items"].([]any)
	found := false
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item["check"] != "ac_tc_evidence_coverage" {
			continue
		}
		found = true
		codes, _ := item["reason_codes"].([]any)
		if len(codes) != 1 || codes[0] != "traceability_missing_tc" {
			t.Fatalf("item reason_codes = %v", item["reason_codes"])
		}
		missing, _ := item["missing"].([]any)
		if len(missing) != 1 || missing[0] != "AC-002" {
			t.Fatalf("item missing = %v", item["missing"])
		}
	}
	if !found {
		t.Fatalf("JSON items missing ac_tc_evidence_coverage: %v", items)
	}
	if dir := os.Getenv("BEACON_TRACEABILITY_EVIDENCE_DIR"); dir != "" {
		body := "command: go test ./internal/cli -run TestGoalConformanceTraceabilityExitCode -count=1\n" +
			"result: PASS (the fixture run exits non-zero and names the missing AC)\n\n" +
			"text output:\n" + textOut + "\njson failed array:\n" + strings.Join(failStrings, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(dir, "traceability-cli-exit.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// seedRefrozenCLIPackage creates a real package, signs its intent, freezes
// it, opens a change that edits AC-001 and adds AC-002, then refreezes.
// It returns the project root and the closed change document path.
func seedRefrozenCLIPackage(t *testing.T) (string, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := store.ResolveRootPath(filepath.Join(t.TempDir(), "trace-proj"))
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	proj, err := st.Projects().Register(store.Project{RootPath: root, Name: "trace-proj"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: proj.ID, Version: "v2.0.0",
		DocsRoot: filepath.Join("docs", "beacon", "v2.0.0"), IsCurrent: true,
	}); err != nil {
		t.Fatal(err)
	}
	eng := orchestrator.NewEngine(st)
	if _, err := eng.CreateTruthPackage(root, "v2.0.0", "trace-cli"); err != nil {
		t.Fatalf("create: %v", err)
	}
	ctx, err := st.ResolveFeature(root, "trace-cli", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.UpdateIntent(ctx, gate.IntentSnapshot{
		Utterance: "test utterance for traceability", Goal: "record an AC diff at refreeze",
		NonGoals: "no UI", Constraints: "fail closed", ScopeMode: "focused",
	}, "tester"); err != nil {
		t.Fatalf("intent: %v", err)
	}
	truthPath := filepath.Join(root, ctx.Feature.TruthPath)
	raw, err := os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(raw),
		"（一句话说清这个需求解决什么问题、给谁用、做到什么算完。冻结前由作者补齐。）",
		"给验收方一条可判定的完成标准：本次变更覆盖的 AC 能直接对照。", 1)
	// A first freeze now requires an explicit surface declaration.
	body = strings.Replace(body, "status: draft\n", "status: draft\nsurfaces: [api]\n", 1)
	if err := os.WriteFile(truthPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := eng.IntentState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range state.OpenQuestions {
		if _, err := eng.AnswerQuestion(ctx, q.ID, "test answer, confirmed.", "answered", "tester"); err != nil {
			t.Fatalf("answer %s: %v", q.ID, err)
		}
	}
	if _, err := eng.ConfirmIntent(ctx, "tester"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	cv, err := eng.RequestTruthChange(ctx, "edit AC-001 and add AC-002", "alice")
	if err != nil {
		t.Fatalf("change: %v", err)
	}
	raw, err = os.ReadFile(truthPath)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(raw),
		"| AC-001 | 作者写下第一条可验证的完成标准（可被 tests.md 的 TC-001 直接验证） |",
		"| AC-001 | the first criterion, now sharper |\n| AC-002 | the second criterion |", 1)
	text = strings.Replace(text,
		"| AC-001 | user utterance | truth.md#User-Intent-Snapshot |",
		"| AC-001 | user utterance | truth.md#User-Intent-Snapshot |\n| AC-002 | user utterance | truth.md#User-Intent-Snapshot |", 1)
	if err := os.WriteFile(truthPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	testsPath := filepath.Join(root, ctx.Feature.TestsPath)
	tests, err := os.ReadFile(testsPath)
	if err != nil {
		t.Fatal(err)
	}
	tests = []byte(strings.Replace(string(tests),
		"| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |\n",
		"| AC-001 | TC-001 | behavior | `go test ./... -run TestAC001` | ac_001_holds |\n| AC-002 | TC-002 | behavior | `go test ./... -run TestAC002` | ac_002_holds |\n", 1))
	if err := os.WriteFile(testsPath, tests, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.RefreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("refreeze: %v", err)
	}
	return root, filepath.Join(root, filepath.FromSlash(cv.ChangeDoc))
}

// TC-008 / AC-004: truth show --json exposes the same change_ac_diff the
// change document records.
func TestTruthShowExposesChangeACDiff(t *testing.T) {
	root, changeDoc := seedRefrozenCLIPackage(t)
	out, err := runCLI(t, "truth", "show", "--project", root, "--feature", "trace-cli", "--json")
	if err != nil {
		t.Fatalf("truth show: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	diff, ok := doc["change_ac_diff"].(map[string]any)
	if !ok {
		t.Fatalf("JSON has no change_ac_diff: %v", doc)
	}
	added, _ := diff["added"].([]any)
	if len(added) != 1 || added[0] != "AC-002" {
		t.Fatalf("added = %v, want AC-002", diff["added"])
	}
	changed, _ := diff["changed"].([]any)
	if len(changed) != 1 {
		t.Fatalf("changed = %v, want one AC", diff["changed"])
	}
	ch, _ := changed[0].(map[string]any)
	if ch["ac"] != "AC-001" || ch["before"] == ch["after"] {
		t.Fatalf("changed row = %v, want AC-001 with distinct prefixes", ch)
	}
	removed, _ := diff["removed"].([]any)
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want empty", removed)
	}

	raw, err := os.ReadFile(changeDoc)
	if err != nil {
		t.Fatal(err)
	}
	section := string(raw)
	section = section[strings.Index(section, "## AC Diff"):]
	for _, want := range []string{"AC-002", "added:", "changed:", "removed:", "(none)",
		ch["before"].(string), ch["after"].(string)} {
		if !strings.Contains(section, want) {
			t.Fatalf("change document AC Diff missing %q:\n%s", want, section)
		}
	}
	emitCLITraceEvidence(t, "truth-show-change-ac-diff.json",
		"go test ./internal/cli -run TestTruthShowExposesChangeACDiff -count=1",
		map[string]any{"truth_show_json": diff, "change_doc_section": section})
}
