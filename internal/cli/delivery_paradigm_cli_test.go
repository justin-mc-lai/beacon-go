package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func runIDOf(t *testing.T, out string) string {
	t.Helper()
	doc := parseDoc(t, out)
	id, _ := doc["run_id"].(string)
	if id == "" {
		t.Fatalf("no run_id in %s", out)
	}
	return id
}

// seedDeliveryProject seeds a registered project whose package passes
// the strict materials bundle (required frontmatter, AC coverage,
// bound task rows) so a run can actually enter implement.
func seedDeliveryProject(t *testing.T) string {
	t.Helper()
	root, _ := seedGateProject(t, true)
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
language: zh
parser_contract: beacon-feature-package-v2
canonical_refs:
  truth: truth.md
  tests: tests.md
  tasks: tasks.md
  evidence: evidence.md
---
# Truth

- **AC-001**：the one requirement
`)
	write("tests.md", `---
slug: feat-gate
---
| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | `+"`go test ./...`"+` | passes |
`)
	write("tasks.md", `---
slug: feat-gate
---
- [ ] the work · ac=AC-001 · evidence=truth.md
`)
	write("evidence.md", `# Evidence

| Surface | Authority | Artifact |
|---|---|---|
| truth | requirement_truth | truth.md |
`)
	return root
}

// runIDFromHuman reads the `run:    <id>` line of the human view.
func runIDFromHuman(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "run:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "run:"))
		}
	}
	t.Fatalf("no run line in %s", out)
	return ""
}

// AC-001 / TC-002: creating a manual run says up front that nothing
// auto-drives it and names the completion command.
func TestGoalRunManualExecutorPrintsTickHint(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "feat-gate", "--executor", "manual")
	if err != nil {
		t.Fatalf("manual run: %v\n%s", err, out)
	}
	if !strings.Contains(out, "does NOT auto-drive") || !strings.Contains(out, "goal tick --run-id") ||
		!strings.Contains(out, "--by <identity>") {
		t.Fatalf("creation output must state the tick contract:\n%s", out)
	}
	id := runIDFromHuman(t, out)
	statusOut, err := runCLI(t, "goal", "status", "--run-id", id)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(statusOut, "executor: manual") || !strings.Contains(statusOut, "--by <identity>") {
		t.Fatalf("status must repeat the executor contract:\n%s", statusOut)
	}
}

// AC-002 / TC-003: an unknown executor is refused non-zero with
// stage_executor_unsupported and no run is written.
func TestGoalRunRejectsUnknownExecutor(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "feat-gate", "--executor", "telepathy")
	if err == nil {
		t.Fatalf("unknown executor must fail:\n%s", out)
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonExecutorUnsupported {
		t.Fatalf("reason = %s, want %s", rc, core.ReasonExecutorUnsupported)
	}
	if strings.Contains(out, "run:") {
		t.Fatalf("refused creation must not print a run:\n%s", out)
	}
}

// AC-003 / TC-005: after the plan and implement exits, gate status
// reports those gates as evaluated instead of not_evaluated.
func TestGateStatusShowsPlanAndImplementEvaluated(t *testing.T) {
	root := seedDeliveryProject(t)
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
	out, err = runCLI(t, "gate", "status", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("gate status: %v", err)
	}
	doc := parseDoc(t, out)
	gates, _ := doc["gates"].([]any)
	seen := map[string]map[string]any{}
	for _, g := range gates {
		m, _ := g.(map[string]any)
		seen[m["gate_type"].(string)] = m
	}
	for _, gt := range []string{"plan", "implement"} {
		row := seen[gt]
		if row == nil || row["evaluated"] != true || row["verdict"] != "pass" {
			t.Fatalf("%s gate must be evaluated=true pass, got %v (all %v)", gt, row, doc["gates"])
		}
	}
}

// AC-004 / TC-006: an external result needs no delegation id, lands the
// artifact pointer and closes the run.
func TestGoalReceiveResultExternalWithoutDelegation(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "goal", "run", "--project", root, "--feature", "feat-gate",
		"--executor", "external", "--json")
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	id := runIDOf(t, out)
	art := filepath.Join(t.TempDir(), "lake-result.md")
	if err := os.WriteFile(art, []byte("# result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err = runCLI(t, "goal", "receive-result", "--run-id", id, "--external",
		"--by", "renlink-dev-room", "--summary", "lake done", "--artifact", art)
	if err != nil {
		t.Fatalf("external receive: %v\n%s", err, out)
	}
	if !strings.Contains(out, "external result") {
		t.Fatalf("unexpected output: %s", out)
	}
	statusOut, err := runCLI(t, "goal", "status", "--run-id", id)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(statusOut, "status: completed") {
		t.Fatalf("externally received run must read completed:\n%s", statusOut)
	}
}

// AC-007 / TC-011: conformance exits 0 for a delivered run and non-zero
// (naming the failed items) when a required fact is missing.
func TestGoalConformanceAllPassAndFailure(t *testing.T) {
	root := seedDeliveryProject(t)
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
	out, err = runCLI(t, "goal", "conformance", "--run-id", id, "--json")
	if err != nil {
		t.Fatalf("conformance must pass: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	if doc["pass"] != true {
		t.Fatalf("conformance doc = %v", doc)
	}

	// Negative: a fresh run with no gates/verifier must fail and name
	// the failed items.
	root2, _ := seedGateProject(t, true)
	out2, err := runCLI(t, "goal", "run", "--project", root2, "--feature", "feat-gate",
		"--executor", "recording", "--json")
	if err != nil {
		t.Fatalf("run2: %v\n%s", err, out2)
	}
	id2 := runIDOf(t, out2)
	failOut, failErr := runCLI(t, "goal", "conformance", "--run-id", id2)
	if failErr == nil {
		t.Fatalf("conformance must fail for an unfinished run:\n%s", failOut)
	}
	if rc, _ := core.ReasonCodeOf(failErr); rc != core.ReasonConformanceFailed {
		t.Fatalf("reason = %s, want %s", rc, core.ReasonConformanceFailed)
	}
	if !strings.Contains(failOut, "FAIL") || !strings.Contains(failOut, "verifier_attached") {
		t.Fatalf("failure output must list the failed items:\n%s", failOut)
	}
}

// AC-006 / TC-010: gate integrity warns (audit) on a UX mismatch and
// blocks it under --strict.
func TestGateIntegrityStrictBlocksUxMismatch(t *testing.T) {
	root, _ := seedGateProject(t, true)
	truth := filepath.Join(root, "truth.md")
	raw, err := os.ReadFile(truth)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(raw), "status: frozen", "status: frozen\nux_required: false", 1) +
		"\n## UI 规格说明书\n\n- 首页布局\n"
	if err := os.WriteFile(truth, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := runCLI(t, "gate", "integrity", "--project", root, "--feature", "feat-gate",
		"--evaluator", "qa-bot"); err != nil {
		t.Fatalf("default integrity must stay audit-only: %v\n%s", err, out)
	}
	out, err := runCLI(t, "gate", "integrity", "--project", root, "--feature", "feat-gate",
		"--evaluator", "qa-bot", "--strict")
	if err == nil {
		t.Fatalf("--strict must block the ux mismatch:\n%s", out)
	}
	if !strings.Contains(out, "truth_ux_requirement_mismatch") {
		t.Fatalf("finding must be reported:\n%s", out)
	}
}
