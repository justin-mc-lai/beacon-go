package cli

import (
	"path/filepath"
	"testing"
)

// P2c CLI chain: admit + implement plan -> parallel-plan (conflict)
// -> delegate-ralph --simulate -> receive-result (reject then accept)
// on one run; design-stage bind flow on a UX run of the same feature.
func TestGoalP2CChain(t *testing.T) {
	root, _ := seedGoalProject(t)

	out, err := runCLI(t, "goal", "run", "build the thing",
		"--project", root, "--feature", "feat-a", "--executor-id", "host-1", "--json")
	if err != nil {
		t.Fatalf("goal run: %v", err)
	}
	runID := strField(t, parseDoc(t, out), "run_id")

	for i := 0; i < 2; i++ {
		out, err = runCLI(t, "goal", "tick", "--run-id", runID, "--executor", "recording", "--json")
		if err != nil {
			t.Fatalf("tick %d: %v (out %s)", i, err, out)
		}
	}
	if got := strField(t, parseDoc(t, out), "current_stage"); got != "implement" {
		t.Fatalf("current_stage = %q, want implement", got)
	}

	// admit: all items pass on the seeded (frozen) feature. S2
	// (AC-141) extended the checker from five items to seven
	// (baseline_merged + dirty_policy; both not_applicable on this
	// non-git seeded feature) — the count assertion follows.
	out, err = runCLI(t, "goal", "admit", "--run-id", runID, "--json")
	if err != nil {
		t.Fatalf("admit: %v (out %s)", err, out)
	}
	adm := parseDoc(t, out)
	if adm["allow_implement"] != true || adm["reason_code"] != "goal_implement_admission_ok" {
		t.Errorf("admission = %v", adm)
	}
	// S3 (declared change): admission grew the AC-173 topology_gate
	// item — 7 -> 8 items (not_applicable on this non-git fixture).
	if items, _ := adm["items"].([]any); len(items) != 8 {
		t.Errorf("admission items = %d, want 8", len(items))
	}

	// implement plan with an explicit ralph ask.
	out, err = runCLI(t, "goal", "implement", "plan", "--run-id", runID, "--lane", "ralph", "--json")
	if err != nil {
		t.Fatalf("implement plan: %v", err)
	}
	plan := parseDoc(t, out)
	dec, _ := plan["decision"].(map[string]any)
	if dec["lane"] != "ralph" || dec["source"] != "explicit" {
		t.Errorf("decision = %v", dec)
	}

	// parallel-plan: two lakes touching the same surface -> conflict,
	// planned-only, and the run binds to its satellite worktree.
	out, err = runCLI(t, "goal", "parallel-plan", "--run-id", runID,
		"--lake", "feat-a", "--lake", "other",
		"--touch", "feat-a=api.go", "--touch", "other=api.go", "--json")
	if err != nil {
		t.Fatalf("parallel-plan: %v (out %s)", err, out)
	}
	pp := parseDoc(t, out)
	if pp["mode"] != "main_star_satellite_pool" || pp["planned_only"] != true {
		t.Errorf("parallel plan = %v", pp)
	}
	if conflicts, _ := pp["conflicts"].([]any); len(conflicts) != 1 {
		t.Errorf("conflicts = %v", pp["conflicts"])
	}

	// delegate-ralph --simulate: labelled simulated, pack is whole.
	out, err = runCLI(t, "goal", "delegate-ralph", "--run-id", runID, "--simulate", "--json")
	if err != nil {
		t.Fatalf("delegate-ralph: %v (out %s)", err, out)
	}
	dv := parseDoc(t, out)
	d, _ := dv["delegation"].(map[string]any)
	if d["status"] != "delegated" || d["simulated"] != true {
		t.Fatalf("delegation = %v", d)
	}
	delegationID, _ := d["id"].(string)
	pkg, _ := d["package"].(map[string]any)
	if pkg["truth_path"] == "" || pkg["lane"] != "ralph" {
		t.Errorf("package = %v", pkg)
	}

	// receive without artifacts -> rejected with the harvest code.
	out, err = runCLI(t, "goal", "receive-result", "--run-id", runID,
		"--delegation-id", delegationID, "--json")
	if err == nil {
		t.Fatalf("receive without artifacts must fail (out %s)", out)
	}
	// receive with an artifact -> returned.
	art := filepath.Join(t.TempDir(), "impl.go")
	out, err = runCLI(t, "goal", "receive-result", "--run-id", runID,
		"--delegation-id", delegationID, "--artifact", art, "--summary", "done", "--json")
	if err != nil {
		t.Fatalf("receive-result: %v (out %s)", err, out)
	}
	d, _ = parseDoc(t, out)["delegation"].(map[string]any)
	if d["status"] != "returned" || d["simulated"] != true {
		t.Errorf("delegation after receive = %v", d)
	}

	// Design flow on a UX run of the same feature: unbound refuses
	// admission; binding opens it.
	out, err = runCLI(t, "goal", "run", "make it pretty",
		"--project", root, "--feature", "feat-a", "--ux", "--json")
	if err != nil {
		t.Fatalf("goal run --ux: %v", err)
	}
	uxRun := strField(t, parseDoc(t, out), "run_id")

	out, err = runCLI(t, "goal", "design-stage", "status", "--run-id", uxRun, "--json")
	if err != nil {
		t.Fatalf("design-stage status: %v", err)
	}
	ds := parseDoc(t, out)
	if ds["required"] != true || ds["bound"] != false || ds["reason_code"] != "design_primitive_not_bound" {
		t.Errorf("design status = %v", ds)
	}
	if _, err = runCLI(t, "goal", "admit", "--run-id", uxRun, "--json"); err == nil {
		t.Fatal("admit must refuse an unbound UX run")
	}
	out, err = runCLI(t, "goal", "design-stage", "bind", "--run-id", uxRun,
		"--primitive-set", "restrained-product", "--version", "v1", "--source-ref", "DESIGN.md", "--json")
	if err != nil {
		t.Fatalf("design-stage bind: %v (out %s)", err, out)
	}
	if parseDoc(t, out)["bound"] != true {
		t.Errorf("after bind = %s", out)
	}
	out, err = runCLI(t, "goal", "admit", "--run-id", uxRun, "--json")
	if err != nil {
		t.Fatalf("admit after bind: %v (out %s)", err, out)
	}
	if parseDoc(t, out)["allow_implement"] != true {
		t.Errorf("admission after bind = %s", out)
	}
}
