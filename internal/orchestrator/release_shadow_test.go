// release_policy + auto_shadow (issue #8 shadow step). The shadow
// answers "would a policy-signed release pass?" and records it, but
// the human gate stays exactly as strong as before.
package orchestrator

import (
	"context"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func TestCreateRunRejectsUnsupportedReleasePolicy(t *testing.T) {
	env := newTestEnv(t, "active")
	for _, bad := range []string{"auto", "banana"} {
		_, err := env.eng.CreateRun(CreateRunOptions{
			ProjectRoot: env.root, FeatureSlug: "feat-a", ReleasePolicy: bad,
		})
		if err == nil {
			t.Fatalf("release_policy %q must be refused", bad)
		}
		if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleasePolicyUnsupported {
			t.Fatalf("release_policy %q reason = %q", bad, rc)
		}
	}
	for _, ok := range []string{"", "human", "auto_shadow"} {
		v, err := env.eng.CreateRun(CreateRunOptions{
			ProjectRoot: env.root, FeatureSlug: "feat-a", ReleasePolicy: ok,
		})
		if err != nil {
			t.Fatalf("release_policy %q must be accepted: %v", ok, err)
		}
		want := ok
		if want == "" {
			want = "human"
		}
		if v.ReleasePolicy != want {
			t.Fatalf("release_policy %q surfaced as %q", ok, v.ReleasePolicy)
		}
	}
}

func TestReleaseShadowNotConfiguredOnHumanPolicy(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t) // default human
	if _, err := env.eng.ReleaseShadow(run.RunID, "policy:auto_shadow"); err == nil {
		t.Fatal("shadow on a human-policy run must be refused")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleaseShadowNotConfigured {
		t.Fatalf("reason = %q", rc)
	}
}

func TestReleaseShadowRecordsVerdictButKeepsHumanGate(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.seedQAPass(t)
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "implement", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", ReleasePolicy: "auto_shadow",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		v, err := env.eng.Tick(ctx, run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if v.Blocked {
			t.Fatalf("tick %d blocked: %+v", i, v)
		}
	}
	shadow, err := env.eng.ReleaseShadow(run.RunID, "policy:auto_shadow")
	if err != nil {
		t.Fatal(err)
	}
	if shadow.Verdict != "pass" || !shadow.HumanConfirmationBypassed {
		t.Fatalf("shadow must pass with a bypassed human confirmation, got %+v", shadow)
	}
	// The human gate is untouched: the run still has no confirmation.
	st, err := env.eng.Status(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if st.ReleaseHumanConfirmed {
		t.Fatal("shadow must never set ReleaseHumanConfirmed")
	}
	if st.ReleaseShadow == nil || st.ReleaseShadow.Verdict != "pass" {
		t.Fatalf("status must surface the shadow verdict, got %+v", st.ReleaseShadow)
	}
	// Complete still refuses without a human.
	if _, err := env.eng.Complete(run.RunID, CompleteOptions{}); err == nil {
		t.Fatal("complete under auto_shadow must still require human confirmation")
	} else if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonReleaseRequiresHuman {
		t.Fatalf("reason = %q", rc)
	}
}

func TestReleaseShadowRecomputesFreshness(t *testing.T) {
	env := newTestEnv(t, "active")
	env.seedTruthFrozen(t)
	env.seedQAPass(t)
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "implement", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	run, err := env.eng.CreateRun(CreateRunOptions{
		ProjectRoot: env.root, FeatureSlug: "feat-a", ReleasePolicy: "auto_shadow",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if v, err := env.eng.Tick(ctx, run.RunID); err != nil || v.Blocked {
			t.Fatalf("tick %d: %+v %v", i, v, err)
		}
	}
	if _, err := env.eng.ReleaseShadow(run.RunID, "policy:auto_shadow"); err != nil {
		t.Fatal(err)
	}
	// Regress the qa gate; the next shadow must see the new fact,
	// not reuse the earlier pass.
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"regression"}}); err != nil {
		t.Fatal(err)
	}
	shadow, err := env.eng.ReleaseShadow(run.RunID, "policy:auto_shadow")
	if err != nil {
		t.Fatal(err)
	}
	if shadow.Verdict != "blocked" {
		t.Fatalf("shadow must recompute and block, got %+v", shadow)
	}
	found := false
	for _, rc := range shadow.ReasonCodes {
		if rc == "gate_prerequisite_not_passed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("shadow must name the regressed gate, got %v", shadow.ReasonCodes)
	}
}
