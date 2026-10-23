package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

// seedP2DProject seeds a project whose version carries the host
// feature plus three frozen lake features.
func seedP2DProject(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "p2d-proj")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "p2d-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"feat-a", "lake-a", "lake-b", "lake-c"} {
		f, err := st.Features().CreatePackage(store.FeaturePackage{
			ProjectVersionID: v.ID, Slug: slug, TruthPath: "truth.md", TestsPath: "tests.md",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// P2d CLI chain: version -> program plan(freeze)/admit/route ->
// hotfix build/validate -> multi-version -> memory -> axis +
// drift after a program change.
func TestGoalP2DChain(t *testing.T) {
	out, err := runCLI(t, "--version")
	if err != nil || !strings.Contains(out, "v"+Version) {
		t.Fatalf("version = %q err=%v", out, err)
	}
	root := seedP2DProject(t)

	out, err = runCLI(t, "goal", "run", "ship the platform", "--project", root,
		"--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	// Program: three lakes in a chain, frozen at record time.
	out, err = runCLI(t, "goal", "program", "plan", "--run-id", runID, "--program", "platform",
		"--lake", "lake-a", "--lake", "lake-b", "--lake", "lake-c",
		"--depends", "lake-b=lake-a", "--depends", "lake-c=lake-b", "--freeze", "--json")
	if err != nil {
		t.Fatalf("program plan: %v (out %s)", err, out)
	}
	if doc := parseDoc(t, out); doc["frozen"] != true {
		t.Fatalf("plan doc = %v", doc)
	}
	// Frozen plans refuse direct re-plans.
	if _, err = runCLI(t, "goal", "program", "plan", "--run-id", runID, "--program", "platform",
		"--lake", "lake-a", "--json"); err == nil {
		t.Error("frozen re-plan must fail")
	}
	// Admit: the whole chain admits in one topological pass.
	out, err = runCLI(t, "goal", "program", "admit", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("program admit: %v (out %s)", err, out)
	}
	if doc := parseDoc(t, out); doc["all_admitted"] != true {
		t.Errorf("admit doc = %v", doc)
	}
	// Route: only lake-a may start; the rest wait on completions.
	out, err = runCLI(t, "goal", "program", "route", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("program route: %v", err)
	}
	route := parseDoc(t, out)
	if batch, _ := route["next_batch"].([]any); len(batch) != 1 || batch[0] != "lake-a" {
		t.Errorf("next_batch = %v", route["next_batch"])
	}

	// Hotfix: no rollback -> refused; full build -> recorded;
	// validate -> blocked (qa not passed, rollback not declared).
	if _, err = runCLI(t, "goal", "hotfix-plan", "build", "--run-id", runID,
		"--description", "login 500s", "--from-version", "v1.6.8", "--target-version", "v1.6.9",
		"--fix-point", "nil guard"); err == nil {
		t.Error("hotfix without rollback must fail")
	}
	out, err = runCLI(t, "goal", "hotfix-plan", "build", "--run-id", runID,
		"--description", "login 500s", "--from-version", "v1.6.8", "--target-version", "v1.6.9",
		"--fix-point", "nil guard", "--rollback", "redeploy v1.6.8 tag", "--json")
	if err != nil {
		t.Fatalf("hotfix build: %v (out %s)", err, out)
	}
	if doc := parseDoc(t, out); doc["requires_human_release"] != true {
		t.Errorf("hotfix doc = %v", doc)
	}
	if _, err = runCLI(t, "goal", "hotfix-plan", "validate", "--run-id", runID, "--json"); err == nil {
		t.Error("hotfix validate must block before qa pass")
	}

	// Multi-version: everything accounted -> exit 0.
	out, err = runCLI(t, "goal", "multi-version-plan", "--run-id", runID,
		"--segment", "v1.7.0=alpha,beta", "--segment", "v1.8.0=beta",
		"--completed", "v1.7.0:alpha", "--completed", "v1.8.0:beta", "--json")
	if err != nil {
		t.Fatalf("multi-version: %v (out %s)", err, out)
	}
	if doc := parseDoc(t, out); doc["valid"] != true {
		t.Errorf("mv doc = %v", doc)
	}

	// Memory: reindex then search (reference only).
	out, err = runCLI(t, "goal", "memory", "reindex", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("memory reindex: %v", err)
	}
	if doc := parseDoc(t, out); doc["authority"] != "support_advisory" {
		t.Errorf("reindex doc = %v", doc)
	}
	out, err = runCLI(t, "goal", "memory", "search", "--run-id", runID, "-q", "hotfix", "--json")
	if err != nil {
		t.Fatalf("memory search: %v", err)
	}
	if doc := parseDoc(t, out); doc["can_complete"] != false || doc["reference_only"] != true {
		t.Errorf("search doc = %v", doc)
	}

	// Axis: generate + validate clean, then drift via program change.
	out, err = runCLI(t, "goal", "axis", "generate", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("axis generate: %v", err)
	}
	if _, err = runCLI(t, "goal", "axis", "validate", "--run-id", runID, "--json"); err != nil {
		t.Fatalf("axis validate (fresh): %v", err)
	}
	out, err = runCLI(t, "goal", "program", "change", "--run-id", runID,
		"--note", "drop lake-c from the program",
		"--lake", "lake-a", "--lake", "lake-b", "--depends", "lake-b=lake-a", "--json")
	if err != nil {
		t.Fatalf("program change: %v (out %s)", err, out)
	}
	if _, err = runCLI(t, "goal", "axis", "validate", "--run-id", runID, "--json"); err == nil {
		t.Error("axis validate must fail after the lake set drifted")
	}
}
