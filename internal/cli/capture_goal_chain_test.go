package cli

import (
	"beacon-go/internal/orchestrator"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

func TestCaptureGoalFullSuccessChain(t *testing.T) {
	testCaptureGoalChain(t, false)
}

func TestCaptureGoalSDDConnectedLedger(t *testing.T) { testCaptureGoalChain(t, true) }

func testCaptureGoalChain(t *testing.T, sdd bool) {
	proj := treeTestEnv(t)
	seedFreezeTree(t, proj, false)
	prepareCaptureTree(t, proj)
	for _, id := range []string{"t1", "t2", "f", "m", "root"} {
		if _, err := freeze(t, proj, id); err != nil {
			t.Fatal(err)
		}
	}
	created := parseDoc(t, mustRun(t, "goal", "run", "--project", proj, "--feature", "f", "--executor", "manual", "--executor-id", "maker", "--json"))
	runID := created["run_id"].(string)
	setStage := func(stage string, index int) {
		st, err := store.Open(storePathFromEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		row, err := st.Runs().GetByID(runID)
		if err != nil {
			t.Fatal(err)
		}
		var meta map[string]any
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		meta["loop_stage"] = stage
		meta["current_stage_idx"] = index
		b, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		if ok, err := st.Runs().UpdateRunStateCAS(runID, row.Metadata, row.Status, b, nil); err != nil || !ok {
			t.Fatalf("fixture stage: %v %v", ok, err)
		}
	}
	refuseLedger := func() {
		for _, group := range []string{"verify", "release"} {
			_, err := runCLI(t, group, "evaluate", "--project", proj, "--feature", "f", "--run", runID, "--evaluator", "checker")
			if err == nil || !strings.Contains(err.Error(), "delivery_ledger_open") {
				t.Fatalf("ledger %s: %v", group, err)
			}
		}
		setStage("qa_routing", 3)
		if _, err := runCLI(t, "goal", "tick", "--run-id", runID, "--by", "checker"); err == nil || !strings.Contains(err.Error(), "delivery_ledger_open") {
			t.Fatalf("ledger tick: %v", err)
		}
		setStage("release_gate", 4)
		if _, err := runCLI(t, "goal", "complete", "--run-id", runID); err == nil || !strings.Contains(err.Error(), "delivery_ledger_open") {
			t.Fatalf("ledger complete: %v", err)
		}
		setStage("qa_routing", 3)
	}
	st, err := store.Open(storePathFromEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	fc, err := st.ResolveFeature(proj, "f", "v0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	q := &kernelQA{t: t, root: proj, run: runID, slug: "f", truth: filepath.Join(proj, fc.Feature.TruthPath), tests: filepath.Join(proj, fc.Feature.TestsPath), manifest: filepath.Join(proj, "candidate.json")}
	q.write(q.tests, "| AC ID | TC ID | Layer | Command | Assertion | Required Tests |\n|---|---|---|---|---|---|\n| AC-001 | TC-001 | behavior | python3 case.py | actual assertion | actual-case |\n")
	if sdd {
		text := "---\nslug: f\nstatus: frozen\npackage_contract: truth-tasks\npackage_author: writer\nintent_confirmed: false\nintent_confirmation_by: pending\nintent_confirmation_fingerprint: pending\nfsm_required: false\nnegative_required: false\n---\n## Grounding\n| Source | Reference | Supports |\n|---|---|---|\n| user | fixture | behavior |\n## Assumptions\nNone.\n## Acceptance Criteria\n| AC ID | Criterion |\n|---|---|\n| AC-001 | behavior |\n## Coverage Matrix\n| AC ID | TC ID | Layer | Command | Assertion | Required Tests |\n|---|---|---|---|---|---|\n| AC-001 | TC-001 | positive | python3 case.py | actual assertion | actual-case |\n"
		q.write(q.truth, text)
		tasks := filepath.Join(filepath.Dir(q.truth), "tasks.md")
		q.write(tasks, "| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |\n|---|---|---|---|---|---|---|---|\n| TASK-A | implement | - | AC-001 | ./task.sh | ./task.sh | .beacon/task-proof.txt | true |\n")
		if err := os.WriteFile(filepath.Join(proj, "task.sh"), []byte("#!/bin/sh\nmkdir -p .beacon\necho verified > .beacon/task-proof.txt\n"), 0700); err != nil {
			t.Fatal(err)
		}
		eng := orchestrator.NewEngine(st)
		receipt := orchestrator.SDDReviewReceipt{Fingerprint: eng.SDDReviewFingerprint(fc), Reviewer: "checker", Verdict: "pass", Summary: "SDD fixture tasks and acceptance bind the grounded behavior."}
		b, _ := json.Marshal(receipt)
		p := filepath.Join(proj, "sdd-review.json")
		q.write(p, string(b))
		if _, err := eng.RecordSDDReview(fc, p); err != nil {
			t.Fatal(err)
		}
		current, err := os.ReadFile(q.truth)
		if err != nil {
			t.Fatal(err)
		}
		doc := string(current)
		doc = strings.Replace(doc, "intent_confirmation_fingerprint: pending", "intent_confirmation_fingerprint: "+receipt.Fingerprint, 1)
		doc = strings.Replace(doc, "intent_confirmation_by: pending", "intent_confirmation_by: owner", 1)
		q.write(q.truth, doc)
		q.tests = q.truth
	}
	q.write(filepath.Join(proj, "case.py"), kernelQARunner)
	q.write(filepath.Join(proj, "source.py"), "print('implementation')\n")
	q.write(filepath.Join(proj, "artifact.txt"), "candidate\n")
	q.candidate()
	q.autotest(true)
	review := q.review("product", "pass", "checker")
	q.call(true, "goal", "attach-verifier", "--run-id", runID, "--source", "checker", "--ref", review)
	for _, id := range []string{"t1", "t2"} {
		mustRun(t, "implement", "report", "--project", proj, "--node", id, "--by", "dev", "--artifact", writeEvidence(t, proj, id))
		mustRun(t, "verify", "leaf", "--project", proj, "--node", id, "--by", "qa", "--artifact", writeCapturedEvidence(t, proj, id))
	}
	mustRun(t, "verify", "rollup", "--project", proj, "--node", "root")
	if sdd {
		refuseLedger()
		mustRun(t, "implement", "tasks", "run", "--run-id", runID)
		// Alter only a registered runtime result: capture's source candidate
		// stays unchanged, proving the task ledger guard itself rejects.
		report, err := orchestrator.NewEngine(st).TaskStatus(runID)
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(report.ReportPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(report.ReportPath, append(original, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		refuseLedger()
		if err := os.WriteFile(report.ReportPath, original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(t, "verify", "evaluate", "--project", proj, "--feature", "f", "--run", runID, "--evaluator", "checker", "--json")
	// Earlier stage behavior has separate tests. This fixture focuses on the QA
	// and release exits consuming real captures plus actual product QA execution.
	row, err := st.Runs().GetByID(runID)
	if err != nil {
		t.Fatal(err)
	}
	var meta map[string]any
	if err := json.Unmarshal(row.Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	meta["loop_stage"] = "qa_routing"
	meta["current_stage_idx"] = 3
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := st.Runs().UpdateRunStateCAS(runID, row.Metadata, row.Status, b, nil); err != nil || !ok {
		t.Fatalf("stage fixture %v %v", ok, err)
	}
	tick := parseDoc(t, mustRun(t, "goal", "tick", "--run-id", runID, "--by", "checker", "--json"))
	if tick["current_stage"] != "release" {
		t.Fatalf("QA did not advance: %v", tick)
	}
	for _, kind := range []string{"truth_freeze", "implement"} {
		if _, err := st.Gates().Record(store.GateResult{FeatureID: fc.Feature.ID, GateType: kind, Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	// No confirmation: actual release evaluate must refuse.
	if _, err := runCLI(t, "release", "evaluate", "--project", proj, "--feature", "f", "--run", runID, "--evaluator", "checker"); err == nil {
		t.Fatal("release without owner accepted")
	}
	completed := parseDoc(t, mustRun(t, "goal", "complete", "--run-id", runID, "--human-confirmed", "--confirmed-by", "owner", "--json"))
	if completed["status"] != "completed" {
		t.Fatalf("complete: %v", completed)
	}
	mustRun(t, "release", "evaluate", "--project", proj, "--feature", "f", "--run", runID, "--evaluator", "checker", "--json")
	if _, err := os.Stat(review); err != nil {
		t.Fatal(err)
	}
}
