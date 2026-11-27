package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/gate"
	"beacon-go/internal/intenttree"
)

func TestCaptureSDDLedgerNotStartedRefused(t *testing.T) {
	env, runID := newTaskRunnerEnv(t, taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	taskExecutable(t, env.root, "command.sh", "echo implementation")
	taskExecutable(t, env.root, "verify.sh", "echo verified > .beacon/proof.txt")
	truth := filepath.Join(env.root, "truth.md")
	raw, err := os.ReadFile(truth)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, truth, strings.Replace(string(raw), "---\n", "---\npackage_contract: truth-tasks\n", 1))
	// Admission belongs to a separate contract; this narrow fixture exercises
	// the same persisted task report and stale-candidate guard as release.
	row, _, _, err := env.eng.loadRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	dir := intenttree.TreeDir(env.root, env.ctx.Version.DocsRoot)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteIndex(dir, intenttree.Index{Schema: intenttree.Schema, Version: env.ctx.Version.Version, RootID: "root"}); err != nil {
		t.Fatal(err)
	}
	if err := env.eng.validateDeliveryTasks(row); err == nil {
		t.Fatal("not started SDD ledger accepted")
	}
}

func TestCaptureSDDLedgerExecutedAndStale(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	taskExecutable(t, env.root, "command.sh", "echo implemented")
	taskExecutable(t, env.root, "verify.sh", "echo verified > .beacon/proof.txt")
	truth := strings.Replace(sddTruthFixture(1, 0, false, ""), "slug: demo", "slug: feat-a", 1)
	truth = strings.Replace(truth, "---\n", "---\nstatus: frozen\npackage_author: writer\n", 1)
	truth = strings.Replace(truth, "package_author: writer\n", "package_author: writer\nintent_confirmed: false\nintent_confirmation_by: pending\nintent_confirmation_fingerprint: pending\n", 1)
	truth += "\n## Grounding\n| Source | Reference | Supports |\n|---|---|---|\n| user | request | behavior |\n\n## Assumptions\nNone.\n"
	writeFile(t, filepath.Join(env.root, "truth.md"), truth)
	writeFile(t, filepath.Join(env.root, "tasks.md"), taskRunnerHeader+taskRunnerRow("TASK-A", "-", "./command.sh", "./verify.sh", ".beacon/proof.txt"))
	fp := env.eng.SDDReviewFingerprint(env.ctx)
	receipt := SDDReviewReceipt{Fingerprint: fp, Reviewer: "checker", Verdict: "pass", Summary: "Fixture behavior, task, and grounded intent align."}
	b, _ := json.Marshal(receipt)
	p := filepath.Join(env.root, "review.json")
	writeFile(t, p, string(b))
	if _, err := env.eng.RecordSDDReview(env.ctx, p); err != nil {
		t.Fatal(err)
	}
	pkg := env.eng.packageFiles(env.ctx)
	signed, _ := setFrontmatterScalar(pkg.Truth.Text, "intent_confirmation_fingerprint", gate.SDDReviewFingerprint(pkg))
	signed, _ = setFrontmatterScalar(signed, "intent_confirmation_by", "owner")
	writeFile(t, filepath.Join(env.root, "truth.md"), signed)
	run := env.createRun(t)
	dir := intenttree.TreeDir(env.root, env.ctx.Version.DocsRoot)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := intenttree.WriteIndex(dir, intenttree.Index{Schema: intenttree.Schema, Version: env.ctx.Version.Version, RootID: "root"}); err != nil {
		t.Fatal(err)
	}
	report, err := env.eng.RunTasks(context.Background(), run.RunID, TaskRunOptions{})
	if err != nil {
		t.Fatalf("SDD real execution: %+v %v", report, err)
	}
	row, _, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.validateDeliveryTasks(row); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(env.root, "changed.go"), "package changed\n")
	if err := env.eng.validateDeliveryTasks(row); err == nil || !strings.Contains(err.Error(), "delivery_ledger_open") {
		t.Fatalf("stale SDD accepted: %v", err)
	}
}
