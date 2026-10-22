package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// seedGoalProject registers one project/version/feature via the DAOs
// and returns root + feature id. truth is pre-frozen via a truth_freeze
// gate pass so the chain can walk through the truth stage.
func seedGoalProject(t *testing.T) (string, store.FeaturePackage) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(store.EnvStorePath, dbPath)
	root := filepath.Join(t.TempDir(), "goal-proj")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "goal-proj"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{ProjectVersionID: v.ID, Slug: "feat-a", TruthPath: "truth.md", TestsPath: "tests.md"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: "truth_freeze", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	return root, f
}

// Full CLI chain: run -> tick x4 (recording executor + seeded gates) ->
// release_gate -> complete rejected without verifier -> attach-verifier
// -> complete --human-confirmed -> done; trace aggregates the trail.
// Also: unknown run id exits non-zero with the source reason_code.
func TestGoalCLIFullChain(t *testing.T) {
	root, feature := seedGoalProject(t)

	out, err := runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	doc := parseDoc(t, out)
	assertKeys(t, doc, "run_id", "status", "loop_stage", "current_stage", "stage_plan")
	runID := strField(t, doc, "run_id")
	if got := strField(t, doc, "loop_stage"); got != "governance_precheck" {
		t.Fatalf("loop_stage = %q", got)
	}
	if got := strField(t, doc, "current_stage"); got != "plan" {
		t.Fatalf("current_stage = %q", got)
	}

	// Seed the qa pass so the qa exit gate can open later.
	st, err := store.Open(mustEnvStorePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Gates().Record(store.GateResult{FeatureID: feature.ID, GateType: "qa", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	st.Close()

	tick := func() map[string]any {
		t.Helper()
		out, err := runCLI(t, "goal", "tick", "--run-id", runID, "--executor", "recording", "--json")
		if err != nil {
			t.Fatalf("goal tick: %v (out %s)", err, out)
		}
		return parseDoc(t, out)
	}
	stages := []string{"truth", "implement", "qa", "release"}
	for i, want := range stages {
		doc = tick()
		if got := strField(t, doc, "current_stage"); got != want {
			t.Fatalf("tick %d current_stage = %q want %q (doc %v)", i, got, want, doc)
		}
	}
	if got := strField(t, doc, "loop_stage"); got != "release_gate" {
		t.Fatalf("loop_stage = %q want release_gate", got)
	}

	// Complete without verifier -> non-zero, reason on the error.
	out, err = runCLI(t, "goal", "complete", "--run-id", runID, "--human-confirmed", "--confirmed-by", "lai", "--json")
	if err == nil {
		t.Fatal("complete without verifier must fail")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != "product_goal_complete_requires_verifier" {
		t.Errorf("complete reason = %q (err %v)", rc, err)
	}
	// The rejection still printed exactly one JSON document.
	_ = parseDoc(t, out)

	out, err = runCLI(t, "goal", "attach-verifier", "--run-id", runID,
		"--ref", "evidence/qa-report.md", "--source", "qa-bot", "--json")
	if err != nil {
		t.Fatalf("attach-verifier: %v", err)
	}
	if v := parseDoc(t, out)["verifier"].(map[string]any); v["present"] != true {
		t.Errorf("verifier = %v", v)
	}

	out, err = runCLI(t, "goal", "complete", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("complete: %v (out %s)", err, out)
	}
	doc = parseDoc(t, out)
	if got := strField(t, doc, "status"); got != "completed" {
		t.Errorf("status = %q", got)
	}
	if got := strField(t, doc, "loop_stage"); got != "done" {
		t.Errorf("loop_stage = %q", got)
	}

	// Status + trace read surfaces.
	out, err = runCLI(t, "goal", "status", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal status: %v", err)
	}
	if got := strField(t, parseDoc(t, out), "status"); got != "completed" {
		t.Errorf("status after complete = %q", got)
	}
	out, err = runCLI(t, "goal", "trace", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("goal trace: %v", err)
	}
	traceDoc := parseDoc(t, out)
	events, _ := traceDoc["events"].([]any)
	if len(events) < 8 {
		t.Errorf("trace events = %d want >= 8 (%v)", len(events), traceDoc["event_count"])
	}
	last := events[len(events)-1].(map[string]any)
	if last["event_type"] != "goal_completed" {
		t.Errorf("last event = %v", last["event_type"])
	}

	// Unknown run: non-zero exit + source reason_code, no stdout doc.
	out, err = runCLI(t, "goal", "status", "--run-id", "run_nope", "--json")
	if err == nil {
		t.Fatal("status of unknown run must fail")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != goalReasonRunNotFound {
		t.Errorf("reason = %q (err %v)", rc, err)
	}
	if out != "" {
		t.Errorf("stdout on not-found = %q want empty", out)
	}
}

func mustEnvStorePath(t *testing.T) string {
	t.Helper()
	p, err := store.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The loop placeholder points at goal and carries no behaviour of its own.
func TestLoopPlaceholderPointsToGoal(t *testing.T) {
	out, err := runCLI(t, "loop")
	if err != nil {
		t.Fatalf("loop placeholder: %v", err)
	}
	if !strings.Contains(out, "beacon goal") {
		t.Errorf("loop output = %q", out)
	}
	out, err = runCLI(t, "goal", "--help")
	if err != nil || !strings.Contains(out, "tick") || !strings.Contains(out, "attach-verifier") {
		t.Errorf("goal help = %q, %v", out, err)
	}
}
