// kernel-lake-freeze-closure AC-002/AC-003: feature-scoped artifact
// registration satisfies the evidence check with no run, and a blocked
// freeze/refreeze precheck leaves a feature-dimension lifecycle event.
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/store"
)

func writeLakeFile(t *testing.T, root, rel, text string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureScopedArtifactSatisfiesEvidence(t *testing.T) {
	env := newGateEnv(t, frozenTruthDoc)
	// Absolute artifact paths are compared against the registered project
	// root, which the store normalizes (e.g. /private/var on macOS).
	root := env.ctx.Project.RootPath
	// The evidence body references a project-relative artifact.
	writeLakeFile(t, root, "evidence.md",
		"# Evidence\n\n| Surface | Authority | Artifact |\n|---|---|---|\n| qa | qa_verdict | artifacts/qa-report.md |\n")
	writeLakeFile(t, root, "artifacts/qa-report.md", "# qa report\nall green\n")

	// No run exists for this feature — registration must not need one.
	if runs, err := env.st.Runs().ListByFeature(env.feature.ID); err != nil || len(runs) != 0 {
		t.Fatalf("fixture must start with no run, got %v err=%v", runs, err)
	}
	abs := filepath.Join(root, "artifacts/qa-report.md")
	res, err := env.eng.RegisterFeatureArtifact(env.ctx, "qa_report", abs, "maker-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Artifact.RunID != store.FeatureScopeRunID(env.feature.ID) {
		t.Fatalf("feature-scoped pointer bound to %q", res.Artifact.RunID)
	}
	views, err := env.eng.ListFeatureArtifacts(env.ctx)
	if err != nil || len(views) != 1 || views[0].Stale {
		t.Fatalf("feature scope list must show the fresh pointer, got %+v err=%v", views, err)
	}
	rep, _, _, err := env.eng.integrityReportLive(env.ctx, gate.IntegrityModeBlock)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Findings {
		if f.ReasonCode == core.ReasonIntegrityEvidenceUnreg || f.ReasonCode == core.ReasonIntegrityEvidenceMissing {
			t.Fatalf("feature-scoped artifact did not satisfy the evidence check: %+v", f)
		}
	}
	// The registration is on the feature trail, not a run trail.
	events, err := env.st.ListEvents(env.feature.ID)
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
		if p["scope"] == "feature" && p["artifact_id"] == res.Artifact.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("artifact_registered event with scope=feature missing")
	}
}

func TestTruthPrecheckFailureRecordsFeatureEvent(t *testing.T) {
	env := newTestEnv(t, "draft")
	// A first freeze with no surfaces declaration is blocked by the
	// precheck (truth_surface_required) — the exact class of failure
	// that used to leave no trace in the event stream.
	writeLakeFile(t, env.root, "truth.md", "---\nslug: feat-a\nversion: v2.0.0\nstatus: draft\nlanguage: zh\nparser_contract: beacon-feature-package-v2\ncanonical_refs:\n  truth: truth.md\n  tests: tests.md\n  tasks: tasks.md\n  evidence: evidence.md\n---\n# Truth\n\n- **AC-001**：the one requirement\n")
	writeLakeFile(t, env.root, "tests.md", "---\nslug: feat-a\n---\n| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n| AC-001 | TC-001 | unit | `go test ./...` | passes |\n")
	writeLakeFile(t, env.root, "tasks.md", "---\nslug: feat-a\n---\n- [ ] the work · ac=AC-001\n")
	writeLakeFile(t, env.root, "evidence.md", "# Evidence\n\n(no artifacts yet)\n")
	ctx, err := env.st.ResolveFeature(env.root, "feat-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.FreezeTruthPackage(ctx, "maker-1"); err == nil {
		t.Fatal("freeze without a surfaces declaration must be blocked")
	}
	events, err := env.st.ListEvents(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range events {
		if ev.EventType != EventTruthPrecheckFailed {
			continue
		}
		var p map[string]any
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			t.Fatal(err)
		}
		if p["feature_id"] != env.feature.ID {
			t.Fatalf("event feature_id = %v", p["feature_id"])
		}
		codes, _ := p["reason_codes"].([]any)
		if len(codes) == 0 {
			t.Fatal("precheck failure event carries no reason codes")
		}
		found = true
	}
	if !found {
		t.Fatal("truth_precheck_failed event missing from the feature trail")
	}
	// Feature-dimension trace sees it (AC-003: runs are not required).
	trace, err := env.eng.TraceFeature(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, ev := range trace.Events {
		if ev.EventType == EventTruthPrecheckFailed && !ev.RunID.Valid {
			seen = true
		}
	}
	if !seen {
		t.Fatal("goal trace --feature would not show the run-less precheck failure")
	}
}
