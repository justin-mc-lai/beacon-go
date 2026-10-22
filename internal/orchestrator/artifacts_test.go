// Artifact registration behavior tests (P6 E2E fix): the manual
// registration face lands real files in artifact_refs with true
// hashes, is idempotent, supersedes on content change, refuses bad
// input fail-closed, and — the point of the fix — feeds the formal
// qa gate exactly like a kernel-written pointer does, while a
// tampered file still blocks as stale and no registration still
// blocks as missing.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

func writeBlob(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRegisterArtifactBasics(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	content := "# qa report\nall green\n"
	writeBlob(t, blob, content)

	res, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "e2e-maker")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	if res.Artifact.ContentHash != hex.EncodeToString(sum[:]) || res.Artifact.SizeBytes != int64(len(content)) {
		t.Fatalf("hash/size wrong: %+v", res.Artifact)
	}
	if res.Artifact.Kind != "qa_report" || res.Artifact.RunID != run.RunID || res.AlreadyRegistered || res.Supersedes != "" {
		t.Fatalf("unexpected registration result: %+v", res)
	}
	rows, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil || len(rows) != 1 || rows[0].ID != res.Artifact.ID {
		t.Fatalf("artifact_refs must hold exactly the new row, got %v err=%v", rows, err)
	}
	events, err := env.st.ListEventsByRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType != EventArtifactRegistered {
			continue
		}
		var p map[string]any
		_ = json.Unmarshal(ev.Payload, &p)
		if p["registered_by"] == "e2e-maker" && p["artifact_id"] == res.Artifact.ID && p["content_hash"] == res.Artifact.ContentHash {
			found = true
		}
	}
	if !found {
		t.Fatal("artifact_registered event with registrant + hash missing")
	}
	views, err := env.eng.ListRunArtifacts(run.RunID)
	if err != nil || len(views) != 1 || views[0].Stale {
		t.Fatalf("list must show the fresh pointer as not stale, got %+v err=%v", views, err)
	}
}

func TestRegisterArtifactRefusals(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	writeBlob(t, blob, "x")

	cases := []struct {
		name            string
		runID, kind, by string
		path            string
		wantCode        string
	}{
		{"missing file", run.RunID, "qa_report", "me", filepath.Join(env.root, "nope.md"), ReasonArtifactFileMissing},
		{"directory", run.RunID, "qa_report", "me", env.root, ReasonArtifactFileMissing},
		{"bad kind", run.RunID, "banana", "me", blob, ReasonArtifactKindNotAllowed},
		{"kernel kind impersonation", run.RunID, "stage_stdout", "me", blob, ReasonArtifactKindNotAllowed},
		{"empty registrant", run.RunID, "qa_report", "  ", blob, ReasonArtifactRegistrarRequired},
		{"unknown run", "run_nope", "qa_report", "me", blob, ReasonRunNotFound},
	}
	for _, tc := range cases {
		if _, err := env.eng.RegisterArtifact(tc.runID, tc.kind, tc.path, tc.by); err == nil {
			t.Fatalf("%s: must be refused", tc.name)
		} else if rc, _ := core.ReasonCodeOf(err); rc != tc.wantCode {
			t.Fatalf("%s: want %s, got %v", tc.name, tc.wantCode, err)
		}
	}
	rows, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("refusals must write nothing, got %d rows", len(rows))
	}
}

func TestRegisterArtifactIdempotentAndSupersede(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	writeBlob(t, blob, "v1")

	first, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "me")
	if err != nil {
		t.Fatal(err)
	}
	again, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "me")
	if err != nil {
		t.Fatal(err)
	}
	if !again.AlreadyRegistered || again.Artifact.ID != first.Artifact.ID {
		t.Fatalf("unchanged re-register must be idempotent, got %+v vs %+v", again, first)
	}
	writeBlob(t, blob, "v2 — different bytes")
	next, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "me")
	if err != nil {
		t.Fatal(err)
	}
	if next.AlreadyRegistered || next.Artifact.ID == first.Artifact.ID || next.Supersedes != first.Artifact.ID {
		t.Fatalf("changed content must append a superseding row, got %+v", next)
	}
	rows, err := env.st.Artifacts().ListByRun(run.RunID)
	if err != nil || len(rows) != 2 {
		t.Fatalf("want 2 rows (original + superseding), got %d err=%v", len(rows), err)
	}
	// The gate's view of history: the superseding row is fresh, the
	// original row now reads stale (its hash no longer matches disk).
	views, err := env.eng.ListRunArtifacts(run.RunID)
	if err != nil || len(views) != 2 {
		t.Fatal(err)
	}
	if !views[0].Stale || views[0].StaleReason != "hash_mismatch" || views[1].Stale {
		t.Fatalf("staleness view wrong after supersede: %+v", views)
	}
}

func TestRegisterArtifactQAGateFlow(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	// The qa stage executed (evidence event), but nothing registered:
	// the gate must still block on missing QA artifacts.
	if _, err := env.rec.ExecuteStage(context.Background(), RunView{RunID: run.RunID, FeatureID: env.feature.ID}, StageQA); err != nil {
		t.Fatal(err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("unregistered evidence must block, got %+v", ev)
	}
	// Register the real report through the new face: gate passes.
	blob := filepath.Join(env.root, "qa-report.md")
	writeBlob(t, blob, "# qa report\nall green\n")
	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "e2e-maker"); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("registered evidence must pass, got %+v", ev)
	}
	// Tamper after registration: staleness is the defence line.
	writeBlob(t, blob, "tampered")
	ev, err = env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked || len(ev.ReasonCodes) == 0 || ev.ReasonCodes[0] != core.ReasonQAArtifactStale {
		t.Fatalf("tampered artifact must block as stale, got %+v", ev)
	}
}
