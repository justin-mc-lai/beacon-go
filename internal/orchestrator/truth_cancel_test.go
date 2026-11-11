package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/store"
)

// cancelFixture freezes a fresh package and opens one change on it.
func cancelFixture(t *testing.T, slug string) (truthEnv, store.FeatureContext, TruthPackageView) {
	t.Helper()
	env := newTruthEnv(t)
	env.create(t, slug)
	ctx := env.ctx(t, slug)
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	cv, err := env.eng.RequestTruthChange(ctx, "cancel probe", "alice")
	if err != nil {
		t.Fatalf("Change: %v", err)
	}
	return env, ctx, cv
}

// lastCancelPayload returns the newest truth_change_cancelled payload.
func lastCancelPayload(t *testing.T, env truthEnv, featureID string) map[string]any {
	t.Helper()
	events, err := env.st.ListEvents(featureID)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].EventType != EventTruthChangeCancelled {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(events[i].Payload, &payload); err != nil {
			t.Fatal(err)
		}
		return payload
	}
	t.Fatal("no truth_change_cancelled event was appended")
	return nil
}

func changeDocPath(env truthEnv, cv TruthPackageView) string {
	return filepath.Join(env.root, filepath.FromSlash(cv.ChangeDoc))
}

func TestTruthCancelIdenticalBodyReturnsFrozen(t *testing.T) {
	env, ctx, cv := cancelFixture(t, "feat-cancel-ok")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	bodySHA := sha256Hex([]byte(readFile(t, truthPath)))
	docBytes := readFile(t, changeDocPath(env, cv))

	v, err := env.eng.CancelTruthChange(ctx, "alice", false)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if v.Status != "frozen" || v.ChangePending || v.ChangeID != "" || len(v.PendingChangeIDs) != 0 {
		t.Fatalf("cancelled view = %+v", v)
	}
	if got := sha256Hex([]byte(readFile(t, truthPath))); got != bodySHA {
		t.Fatalf("cancel touched the truth body: %s -> %s", bodySHA, got)
	}
	if got := readFile(t, changeDocPath(env, cv)); got != docBytes {
		t.Fatal("AC-005: cancel rewrote changes/<id>.md; it must stay byte-identical")
	}

	payload := lastCancelPayload(t, env, ctx.Feature.ID)
	if payload["decision"] != "cancel" {
		t.Fatalf("decision = %v, want cancel", payload["decision"])
	}
	if payload["accepted_current"] != false || payload["baseline_missing"] != false {
		t.Fatalf("payload = %v, want accepted_current=false baseline_missing=false", payload)
	}
	if payload["change_id"] != cv.ChangeID {
		t.Fatalf("change_id = %v, want %s", payload["change_id"], cv.ChangeID)
	}
	if payload["truth_sha256"] != bodySHA || payload["base_truth_sha256"] != bodySHA {
		t.Fatalf("payload hashes = %v, want %s", payload, bodySHA)
	}
}

func TestTruthCancelModifiedBodyRefuses(t *testing.T) {
	env, ctx, cv := cancelFixture(t, "feat-cancel-modified")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	edited := strings.Replace(readFile(t, truthPath), "第一条可验证的完成标准", "第一条可验证的完成标准（已改）", 1)
	if err := os.WriteFile(truthPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	editedSHA := sha256Hex([]byte(readFile(t, truthPath)))
	docBytes := readFile(t, changeDocPath(env, cv))

	if _, err := env.eng.CancelTruthChange(ctx, "alice", false); err == nil ||
		!strings.Contains(err.Error(), "truth refreeze") {
		t.Fatalf("cancel on modified body err = %v, want refreeze hint", err)
	}
	// --accept-current only covers a missing baseline; it never
	// overrides a body that actually moved.
	if _, err := env.eng.CancelTruthChange(ctx, "alice", true); err == nil ||
		!strings.Contains(err.Error(), "truth refreeze") {
		t.Fatalf("cancel --accept-current on modified body err = %v, want refreeze hint", err)
	}
	view, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !view.ChangePending || view.Status != "change_pending" {
		t.Fatalf("refused cancel changed the state: %+v", view)
	}
	if got := sha256Hex([]byte(readFile(t, truthPath))); got != editedSHA {
		t.Fatalf("refused cancel touched the body: %s -> %s", editedSHA, got)
	}
	if got := readFile(t, changeDocPath(env, cv)); got != docBytes {
		t.Fatal("refused cancel rewrote the change document")
	}
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType == EventTruthChangeCancelled || ev.EventType == EventTruthRefrozen {
			t.Fatalf("refused cancel appended %s", ev.EventType)
		}
	}
}

func TestTruthCancelMissingBaselineNeedsAcceptCurrent(t *testing.T) {
	env, ctx, cv := cancelFixture(t, "feat-cancel-legacy")
	docPath := changeDocPath(env, cv)
	doc := readFile(t, docPath)
	blanked := strings.Replace(doc, "base_truth_sha256: ", "base_truth_sha256: \n#", 1)
	if err := os.WriteFile(docPath, []byte(blanked), 0o644); err != nil {
		t.Fatal(err)
	}
	blankedBytes := readFile(t, docPath)
	if _, err := env.eng.CancelTruthChange(ctx, "alice", false); err == nil ||
		!strings.Contains(err.Error(), "--accept-current") {
		t.Fatalf("missing baseline err = %v, want --accept-current hint", err)
	}
	view, err := env.eng.TruthPackage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !view.ChangePending {
		t.Fatalf("refused legacy cancel closed the change: %+v", view)
	}
	v, err := env.eng.CancelTruthChange(ctx, "alice", true)
	if err != nil {
		t.Fatalf("Cancel --accept-current: %v", err)
	}
	if v.Status != "frozen" || v.ChangePending {
		t.Fatalf("accept-current view = %+v", v)
	}
	payload := lastCancelPayload(t, env, ctx.Feature.ID)
	if payload["accepted_current"] != true || payload["baseline_missing"] != true {
		t.Fatalf("payload = %v, want accepted_current=true baseline_missing=true", payload)
	}
	if payload["decision"] != "cancel" {
		t.Fatalf("decision = %v, want cancel", payload["decision"])
	}
	if got := readFile(t, docPath); got != blankedBytes {
		t.Fatal("AC-005: --accept-current cancel rewrote changes/<id>.md")
	}
}

func TestTruthCancelWithoutPendingIsIdempotent(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-cancel-noop")
	ctx := env.ctx(t, "feat-cancel-noop")
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	before, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		v, err := env.eng.CancelTruthChange(ctx, "alice", false)
		if err != nil {
			t.Fatalf("no-pending cancel #%d: %v", i+1, err)
		}
		if v.Status != "frozen" || v.ChangePending {
			t.Fatalf("no-pending cancel #%d changed state: %+v", i+1, v)
		}
	}
	after, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("no-pending cancel appended events: %d -> %d", len(before), len(after))
	}
}
