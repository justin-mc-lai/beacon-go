package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

// ---- Loop kill switch (P4c; v1.6.5 loop-token-budget AC-006) ----

func engageKillSwitch(t *testing.T, root string) string {
	t.Helper()
	p := filepath.Join(root, ".beacon", "loop", "pause-all")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("paused by test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoopKillSwitchBlocksDriverStartAndTurn(t *testing.T) {
	env := newTestEnv(t, "frozen")
	run := env.createRun(t)
	pause := engageKillSwitch(t, env.root)

	_, err := env.eng.DriverStart(run.RunID, DriverStartOptions{})
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLoopKillSwitchActive {
		t.Fatalf("driver start with kill switch: reason = %s (%v), want %s", rc, err, core.ReasonLoopKillSwitchActive)
	}
	// Human removes the file -> loops may start again.
	if err := os.Remove(pause); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.DriverStart(run.RunID, DriverStartOptions{}); err != nil {
		t.Fatalf("driver start after switch removed: %v", err)
	}
	// Re-engage mid-run: the very next turn is refused.
	engageKillSwitch(t, env.root)
	_, err = env.eng.DriverRunOnce(context.Background(), run.RunID)
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonLoopKillSwitchActive {
		t.Fatalf("run-once with kill switch: reason = %s (%v), want %s", rc, err, core.ReasonLoopKillSwitchActive)
	}
}

// ---- Post-release hook coverage (P4c; v1.6.3 coverage repair) ----

func TestSelfHealPostCoverageDerivedFromProfileRegistry(t *testing.T) {
	env := newTestEnv(t, "frozen")
	run := env.createRun(t)
	plan, err := env.eng.PlanSelfHeal(run.RunID, "post")
	if err != nil {
		t.Fatal(err)
	}
	var step *SelfHealStep
	for i := range plan.Steps {
		if plan.Steps[i].ID == "post-release-local-install" {
			step = &plan.Steps[i]
		}
	}
	if step == nil {
		t.Fatalf("post plan steps = %+v, want the install step", plan.Steps)
	}
	if step.Status != HealRequiresConfirmation {
		t.Errorf("install step status = %s, want requires_confirmation (never auto)", step.Status)
	}
	// Coverage must name EVERY registered profile: the list is
	// derived from the registry, so it cannot silently drift the
	// way the v1 hardcoded ("codex","claude") loop did.
	for _, p := range harness.BuiltinProfiles() {
		if !strings.Contains(step.Detail, p.Name) {
			t.Errorf("coverage detail %q does not name registered profile %q", step.Detail, p.Name)
		}
	}
	// Executing self-heal must still never run the install step.
	exec, err := env.eng.ExecuteSelfHeal(run.RunID, "post")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range exec.Steps {
		if s.ID == "post-release-local-install" && s.Status != HealRequiresConfirmation {
			t.Errorf("executed install step status = %s, want requires_confirmation", s.Status)
		}
	}
}

// ---- Failure-mode catalog consumption (P4c; v1.6.5 catalog) ----

func TestLongrunDoctorExplainsFailureModes(t *testing.T) {
	env := newTestEnv(t, "frozen")
	run := env.createRun(t)
	payload, _ := json.Marshal(map[string]any{"reason_code": core.ReasonStageExitTruthNotFrozen})
	if _, err := env.st.AppendEvent(store.LifecycleEvent{
		FeatureID: env.feature.ID,
		RunID:     sql.NullString{String: run.RunID, Valid: true},
		EventType: "stage_exit_blocked",
		Payload:   payload,
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := env.eng.LongrunDoctor(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range rep.FailureModes {
		if m.ID == "truth-drift" {
			found = true
			if m.Severity != core.SeverityS2Harmful || m.Mitigation == "" || m.Route == "" {
				t.Errorf("truth-drift mode lacks governed fields: %+v", m)
			}
		}
	}
	if !found {
		t.Errorf("doctor failure_modes = %+v, want truth-drift explained", rep.FailureModes)
	}
}

// ---- Freeze records the package fingerprint (P4c; v1.6.3) ----

func TestFreezeRecordsPackageFingerprint(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-fp")
	ctx := env.ctx(t, "feat-fp")
	if _, err := env.eng.FreezeTruthPackage(ctx, "qa-bot"); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	events, err := env.st.ListEvents(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	var recorded string
	for _, ev := range events {
		if ev.EventType != EventTruthFrozen {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		recorded, _ = p["package_fingerprint"].(string)
		if alg, _ := p["fingerprint_alg"].(string); alg != core.FingerprintAlg {
			t.Errorf("fingerprint_alg = %q, want %q", alg, core.FingerprintAlg)
		}
	}
	if recorded == "" {
		t.Fatal("truth_frozen event carries no package_fingerprint")
	}
	// The recorded fingerprint describes the frozen BODY: computed
	// over the post-flip files it must agree (status normalized).
	truthRaw := readFile(t, filepath.Join(env.root, ctx.Feature.TruthPath))
	testsRaw := readFile(t, filepath.Join(env.root, ctx.Feature.TestsPath))
	if want := core.PackageFingerprint([]byte(truthRaw), []byte(testsRaw)); recorded != want {
		t.Errorf("recorded fingerprint = %s, recomputed over frozen body = %s", recorded, want)
	}
}

// ---- Protected-path merge guard (P4c; v1.6.5 denylist) ----

func TestWorktreeMergeProtectedPathBlocked(t *testing.T) {
	env := newGitEnv(t)
	ctx := context.Background()
	run := poolRun(t, env)
	if _, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{}); err != nil {
		t.Fatal(err)
	}
	wv, err := env.eng.WorktreeStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	var sat string
	for _, r := range wv.Records {
		if r.Lake == "feat-a" {
			sat = r.Path
		}
	}
	if sat == "" {
		t.Fatal("no feat-a satellite in worktree view")
	}
	// The satellite commits a protected path (.env): merging it
	// home would write secrets into the main star.
	if err := os.WriteFile(filepath.Join(sat, ".env"), []byte("TOKEN=secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, sat, "add", ".")
	gitCmd(t, sat, "commit", "-m", "satellite adds env file")
	_, err = env.eng.WorktreeMerge(ctx, run.RunID, "feat-a", "", "")
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonProtectedPathDenylist {
		t.Fatalf("merge reason = %s (%v), want %s", rc, err, core.ReasonProtectedPathDenylist)
	}
	if _, err := os.Stat(filepath.Join(env.root, ".env")); !os.IsNotExist(err) {
		t.Errorf(".env must NOT have landed on the main star (stat err = %v)", err)
	}
}
