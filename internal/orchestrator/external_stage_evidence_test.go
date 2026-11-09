package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

// ---- helpers ----

func createExternalRun(t *testing.T, env testEnv, mode string) RunView {
	t.Helper()
	v, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a",
		ExecutorID: "room-actor", ExecutorMode: mode,
	})
	if err != nil {
		t.Fatalf("CreateRun(%s): %v", mode, err)
	}
	return v
}

func payloadsOf(t *testing.T, st *store.Store, runID, eventType string) []map[string]any {
	t.Helper()
	events, err := st.ListEventsByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, ev := range events {
		if ev.EventType != eventType {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatalf("payload of %s: %v", ev.EventType, err)
		}
		out = append(out, p)
	}
	return out
}

func nullRunID(runID string) sql.NullString {
	return sql.NullString{String: runID, Valid: true}
}

// appendRealStageExecuted seeds a real (non-recording) stage_executed
// event, the shape a subprocess/ralph/manual completion leaves.
func appendRealStageExecuted(t *testing.T, env testEnv, runID, stage string) {
	t.Helper()
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID,
		RunID:     nullRunID(runID),
		EventType: "stage_executed",
		Payload:   []byte(`{"stage":"` + stage + `","executor":"subprocess","source":"local","outcome":"done","placeholder":false,"recording":false}`),
	}); err != nil {
		t.Fatalf("append real stage_executed: %v", err)
	}
}

func qaCheck(ev gate.Evaluation, name string) gate.CheckItem {
	for _, c := range ev.Checks {
		if c.Check == name {
			return c
		}
	}
	return gate.CheckItem{Check: name, Detail: "missing from evaluation"}
}

// ---- AC-001 / TC-001 ----

func TestExternalStageCompletionRecordsRealStageExecuted(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
	const by = "羽球营地 前端"
	v, err := env.eng.TickBy(context.Background(), run.RunID, by)
	if err != nil {
		t.Fatalf("TickBy: %v", err)
	}
	if v.Blocked || v.CurrentStage != StageTruth {
		t.Fatalf("external tick = %+v, want an advance to %s", v, StageTruth)
	}
	events := payloadsOf(t, env.st, run.RunID, "stage_executed")
	if len(events) != 1 {
		t.Fatalf("stage_executed events = %d, want exactly 1", len(events))
	}
	p := events[0]
	if p["stage"] != StagePlan {
		t.Errorf("stage = %v, want %s", p["stage"], StagePlan)
	}
	if p["executor"] != ExecutorModeExternal {
		t.Errorf("executor = %v, want external", p["executor"])
	}
	if p["source"] != ExecutorModeExternal {
		t.Errorf("source = %v, want external", p["source"])
	}
	if p["by"] != by {
		t.Errorf("by = %v, want %q", p["by"], by)
	}
	if p["placeholder"] != false {
		t.Errorf("placeholder = %v, want false", p["placeholder"])
	}
	if p["recording"] != false {
		t.Errorf("recording = %v, want false", p["recording"])
	}
	if p["outcome"] != "done" {
		t.Errorf("outcome = %v, want done", p["outcome"])
	}
}

// ---- AC-001 / TC-002 ----

func TestExternalStageEventCannotBeForgedExternally(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
	// A caller can only supply an identity. JSON-looking text in --by
	// stays a plain identity string; it never becomes payload fields.
	const forged = `{"executor":"recording","placeholder":true,"source":"recording"}`
	if _, err := env.eng.TickBy(context.Background(), run.RunID, forged); err != nil {
		t.Fatalf("TickBy: %v", err)
	}
	events := payloadsOf(t, env.st, run.RunID, "stage_executed")
	if len(events) != 1 {
		t.Fatalf("stage_executed events = %d, want exactly 1", len(events))
	}
	p := events[0]
	if p["executor"] != ExecutorModeExternal || p["source"] != ExecutorModeExternal {
		t.Errorf("kernel fields were forged: %+v", p)
	}
	if p["placeholder"] != false {
		t.Errorf("placeholder was forged: %+v", p)
	}
	if p["by"] != forged {
		t.Errorf("by = %v, want the raw identity string", p["by"])
	}
	wantKeys := map[string]bool{
		"stage": true, "executor": true, "source": true, "by": true,
		"placeholder": true, "recording": true, "outcome": true,
	}
	for k := range p {
		if !wantKeys[k] {
			t.Errorf("caller payload leaked key %q into the kernel event: %+v", k, p)
		}
	}
}

// ---- AC-002 / TC-003 ----

func TestQAGateRejectsRecordingExecutorStageEvent(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	if err := os.WriteFile(blob, []byte("# qa report\npass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	// The recording executor's event: placeholder=true.
	if _, err := env.rec.ExecuteStage(context.Background(),
		RunView{RunID: run.RunID, FeatureID: env.feature.ID}, StageQA); err != nil {
		t.Fatal(err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	c := qaCheck(ev, "qa_stage_executed")
	if c.OK || c.ReasonCode != core.ReasonQAStageNotExecuted {
		t.Fatalf("recording-only qa event must not count: %+v", c)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("qa gate = %s, want blocked", ev.Verdict)
	}

	// Missing marker is recording state as well: fail-closed.
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID, RunID: nullRunID(run.RunID),
		EventType: "stage_executed",
		Payload:   []byte(`{"stage":"qa","executor":"unknown","outcome":"done"}`),
	}); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if c := qaCheck(ev, "qa_stage_executed"); c.OK {
		t.Fatalf("a marker-less stage_executed must be refused: %+v", c)
	}
}

// ---- AC-002 / TC-004 ----

func TestQAGateAcceptsRealExternalStageEvent(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	blob := filepath.Join(env.root, "qa-report.md")
	if err := os.WriteFile(blob, []byte("# qa report\npass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID, RunID: nullRunID(run.RunID),
		EventType: "stage_executed",
		Payload:   []byte(`{"stage":"qa","executor":"external","source":"external","by":"room","outcome":"done","placeholder":false,"recording":false}`),
	}); err != nil {
		t.Fatal(err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if c := qaCheck(ev, "qa_stage_executed"); !c.OK {
		t.Fatalf("real external qa event must count: %+v", c)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("qa gate = %s (%v), want pass", ev.Verdict, ev.ReasonCodes)
	}
}

// ---- AC-003 / TC-005 ----

func TestExternalReceiveResultIsQAStageEvidence(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
	blob := filepath.Join(env.root, "qa-report.md")
	if err := os.WriteFile(blob, []byte("# qa report\npass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ReceiveExternalResult(run.RunID, ExternalResultInput{
		By: "room-actor", Summary: "lake finished",
		Artifacts: []ArtifactInput{{Kind: "qa_report", AbsPath: blob}},
	}); err != nil {
		t.Fatalf("ReceiveExternalResult: %v", err)
	}
	ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if c := qaCheck(ev, "qa_stage_executed"); !c.OK {
		t.Fatalf("external completion + live QA artifact must count: %+v", c)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("qa gate = %s (%v), want pass", ev.Verdict, ev.ReasonCodes)
	}
}

// ---- AC-003 / TC-006 ----

func TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts(t *testing.T) {
	t.Run("no qa artifact", func(t *testing.T) {
		env := newGateEnv(t, frozenTruthDoc)
		run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
		blob := filepath.Join(env.root, "lake-report.md")
		if err := os.WriteFile(blob, []byte("lake report"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := env.eng.ReceiveExternalResult(run.RunID, ExternalResultInput{
			By: "room-actor", Summary: "lake finished",
			Artifacts: []ArtifactInput{{Kind: "evidence", AbsPath: blob}},
		}); err != nil {
			t.Fatal(err)
		}
		ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
		if err != nil {
			t.Fatal(err)
		}
		if c := qaCheck(ev, "qa_stage_executed"); c.OK {
			t.Fatalf("external completion without a QA artifact must not count: %+v", c)
		}
		if ev.Verdict != gate.VerdictBlocked {
			t.Fatalf("qa gate = %s, want blocked", ev.Verdict)
		}
	})

	t.Run("hash mismatch", func(t *testing.T) {
		env := newGateEnv(t, frozenTruthDoc)
		run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
		blob := filepath.Join(env.root, "qa-report.md")
		if err := os.WriteFile(blob, []byte("# qa report\npass\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := env.eng.ReceiveExternalResult(run.RunID, ExternalResultInput{
			By: "room-actor", Summary: "lake finished",
			Artifacts: []ArtifactInput{{Kind: "qa_report", AbsPath: blob}},
		}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(blob, []byte("tampered"), 0o644); err != nil {
			t.Fatal(err)
		}
		ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
		if err != nil {
			t.Fatal(err)
		}
		if c := qaCheck(ev, "qa_stage_executed"); c.OK {
			t.Fatalf("a stale QA artifact must not be equivalence evidence: %+v", c)
		}
		if c := qaCheck(ev, "qa_artifact_refs_valid"); c.OK {
			t.Fatalf("tampered artifact must be stale: %+v", c)
		}
	})

	t.Run("missing source external", func(t *testing.T) {
		env := newGateEnv(t, frozenTruthDoc)
		run := env.createRun(t)
		blob := filepath.Join(env.root, "qa-report.md")
		if err := os.WriteFile(blob, []byte("# qa report\npass\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := env.eng.RegisterArtifact(run.RunID, "qa_report", blob, "qa-bot"); err != nil {
			t.Fatal(err)
		}
		if _, err := env.st.AppendEvent(store.LifecycleEvent{
			FeatureID: env.feature.ID, RunID: nullRunID(run.RunID),
			EventType: "goal_completed",
			Payload:   []byte(`{"source":"manual","summary":"not external"}`),
		}); err != nil {
			t.Fatal(err)
		}
		ev, err := env.eng.EvaluateGate(env.ctx, "qa", "", "qa-bot")
		if err != nil {
			t.Fatal(err)
		}
		if c := qaCheck(ev, "qa_stage_executed"); c.OK {
			t.Fatalf("a completion without source=external must not count: %+v", c)
		}
	})
}

// ---- AC-004 / TC-007 ----

func TestAttachVerifierDoesNotClearUnresolvedBlock(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := env.createRun(t)
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	verifier := filepath.Join(t.TempDir(), "verifier.md")
	if err := os.WriteFile(verifier, []byte("# verification\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.AttachVerifier(run.RunID, []string{verifier}, "independent-verifier", ""); err != nil {
		t.Fatalf("AttachVerifier: %v", err)
	}
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Blocked {
		t.Fatal("attach-verifier must not clear an unresolved block")
	}
	if m.BlockedStreak != 1 {
		t.Fatalf("blocked_streak = %d, want 1 (unchanged by attach-verifier)", m.BlockedStreak)
	}
	attached := payloadsOf(t, env.st, run.RunID, "verifier_attached")
	if len(attached) != 1 {
		t.Fatalf("verifier_attached events = %d, want 1", len(attached))
	}
	if attached[0]["cleared_block"] != false {
		t.Fatalf("verifier_attached cleared_block = %v, want false", attached[0]["cleared_block"])
	}
	// A second block still accumulates: the streak was not reset.
	if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
		t.Fatal(err)
	}
	_, m, _, err = env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.BlockedStreak != 2 {
		t.Fatalf("blocked_streak after second block = %d, want 2", m.BlockedStreak)
	}
}

// ---- AC-004 / TC-008 ----

func TestHardBlockedContinueResumesAfterCauseCleared(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	run := createExternalRun(t, env.testEnv, ExecutorModeExternal)
	for i := 0; i < 3; i++ {
		if _, err := env.eng.Block(run.RunID, "manual_block"); err != nil {
			t.Fatal(err)
		}
	}
	stageBefore := run.CurrentStage
	v, err := env.eng.ContinueBy(context.Background(), run.RunID, "room-actor")
	if err != nil {
		t.Fatalf("ContinueBy: %v", err)
	}
	if v.ReasonCode != ReasonBlockedStreak {
		t.Fatalf("hard-blocked continue reason = %q, want %q", v.ReasonCode, ReasonBlockedStreak)
	}
	if !v.Blocked || v.CurrentStage != stageBefore {
		t.Fatalf("hard-blocked continue advanced the run: %+v", v)
	}
	if _, err := env.eng.Recover(run.RunID, "owner", "the missing gate evidence landed"); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	_, m, _, err := env.eng.loadRun(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Blocked || m.BlockedStreak != 0 || m.LastBlockReason != "" {
		t.Fatalf("recovery did not reset the blocker: %+v", m)
	}
	v, err = env.eng.TickBy(context.Background(), run.RunID, "room-actor")
	if err != nil {
		t.Fatalf("tick after recovery: %v", err)
	}
	if v.Blocked || v.CurrentStage != StageTruth {
		t.Fatalf("tick after recovery = %+v, want an advance to %s", v, StageTruth)
	}
}
