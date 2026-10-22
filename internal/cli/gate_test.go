// CLI tests for the P3a gate surface: evaluate writes, status and
// spine read back, integrity fracture flow, evaluator obligation.
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// seedGateProject registers one project/feature and writes a real
// package (frozen or draft) under its root. It returns the temp
// project root and the feature.
func seedGateProject(t *testing.T, frozen bool) (string, store.FeaturePackage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	projRoot := filepath.Join(t.TempDir(), "gate-proj")
	if err := os.MkdirAll(projRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	projRoot = store.ResolveRootPath(projRoot)
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: projRoot, Name: "gate-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	status := "draft"
	if frozen {
		status = "frozen"
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-gate", TruthPath: "truth.md", TestsPath: "tests.md", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(projRoot, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("truth.md", "---\nslug: feat-gate\nstatus: "+status+"\n---\n# Truth\n\n- **AC-001**：the one requirement\n")
	write("tests.md", "---\nslug: feat-gate\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-001 | TC-001 | unit | `go test ./...` | passes |\n")
	write("tasks.md", "---\nslug: feat-gate\n---\n- [ ] the work\n")
	write("evidence.md", "# Evidence\n\n(no artifacts referenced yet)\n")
	return projRoot, f
}

func TestGateEvaluateWritesAndStatusReadsBack(t *testing.T) {
	root, _ := seedGateProject(t, true)
	out, err := runCLI(t, "gate", "evaluate", "truth_freeze", "--project", root, "--feature", "feat-gate", "--evaluator", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("evaluate: %v\n%s", err, out)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "gate_type", "verdict", "evaluator", "gate_result_id")
	if strField(t, doc, "verdict") != "pass" {
		t.Fatalf("frozen package must pass, got %v", doc)
	}
	// status reads it back; other gates are not_evaluated.
	out, err = runCLI(t, "gate", "status", "--project", root, "--feature", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	doc = parseDoc(t, out)
	gates, _ := doc["gates"].([]any)
	if len(gates) != 6 {
		t.Fatalf("status must list all six gate types, got %v", doc["gates"])
	}
	seen := map[string]map[string]any{}
	for _, g := range gates {
		m, _ := g.(map[string]any)
		seen[m["gate_type"].(string)] = m
	}
	if seen["truth_freeze"]["verdict"] != "pass" || seen["truth_freeze"]["evaluated"] != true {
		t.Fatalf("status must show the persisted truth_freeze pass, got %v", seen["truth_freeze"])
	}
	if seen["qa"]["evaluated"] != false {
		t.Fatalf("qa must be unevaluated, got %v", seen["qa"])
	}
	// spine gate (read-only) sees the same rows.
	out, err = runCLI(t, "spine", "gate", "feat-gate", "--json")
	if err != nil {
		t.Fatalf("spine gate: %v", err)
	}
	doc = parseDoc(t, out)
	rows, _ := doc["gate_results"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["gate_type"] != "truth_freeze" {
		t.Fatalf("spine gate must read the formal evaluation, got %v", doc["gate_results"])
	}
}

func TestGateEvaluateRequiresEvaluator(t *testing.T) {
	root, _ := seedGateProject(t, true)
	_, err := runCLI(t, "gate", "evaluate", "truth_freeze", "--project", root, "--feature", "feat-gate")
	if err == nil {
		t.Fatal("missing --evaluator must be refused")
	}
	if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonGateEvaluatorRequired {
		t.Fatalf("want gate_evaluator_required, got %v", err)
	}
}

func TestGateEvaluateBlockedExitsNonZeroWithJSON(t *testing.T) {
	root, _ := seedGateProject(t, false) // draft truth
	out, err := runCLI(t, "gate", "evaluate", "truth_freeze", "--project", root, "--feature", "feat-gate", "--evaluator", "qa-bot", "--json")
	if err == nil {
		t.Fatal("blocked evaluation must exit non-zero")
	}
	doc := parseDoc(t, out) // JSON is printed before the failure surfaces
	if strField(t, doc, "verdict") != "blocked" {
		t.Fatalf("draft truth must block, got %v", doc)
	}
	if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonFreezeAckRequired {
		t.Fatalf("headline code must be freeze_ack_required, got %v", err)
	}
}

func TestGateIntegrityFractureFlow(t *testing.T) {
	root, f := seedGateProject(t, false)
	// Break it: tests references an unknown AC, evidence references
	// an unregistered artifact.
	if err := os.WriteFile(filepath.Join(root, "tests.md"),
		[]byte("---\nslug: feat-gate\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|-------|-------|-------|---------|-----------|\n| AC-999 | TC-999 | unit | `go test` | passes |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "evidence.md"),
		[]byte("# Evidence\n\n| qa | artifacts/qa-report.md |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "gate", "integrity", "--project", root, "--feature", "feat-gate", "--evaluator", "qa-bot", "--json")
	if err == nil {
		t.Fatal("fractured package must exit non-zero in block mode")
	}
	doc := parseDoc(t, out)
	if strField(t, doc, "verdict") != "blocked" || strField(t, doc, "mode") != "block" {
		t.Fatalf("want blocked/block, got %v", doc)
	}
	findings, _ := doc["findings"].([]any)
	kinds := map[string]bool{}
	for _, x := range findings {
		kinds[x.(map[string]any)["kind"].(string)] = true
	}
	if !kinds["tests_unknown_ac"] || !kinds["evidence_artifact_unregistered"] {
		t.Fatalf("findings must itemise both fractures, got %v", kinds)
	}
	// And the verdict is persisted where status/spine read.
	st, err := store.Open(os.Getenv(store.EnvStorePath))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	latest, err := st.Gates().Latest(f.ID, "truth_integrity")
	if err != nil || latest.Verdict != "blocked" {
		t.Fatalf("integrity verdict must persist, got %+v err=%v", latest, err)
	}
}
