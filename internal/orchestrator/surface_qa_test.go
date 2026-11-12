// qa-surface-e2e: freeze-time surface validation, artifact extension
// whitelist, and the two real gate verdicts (TC-SUR-018).
package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

func seedSurfaceFrontmatter(t *testing.T, path, block string) {
	t.Helper()
	text := readFile(t, path)
	updated := strings.Replace(text, "status: draft\n", "status: draft\n"+block, 1)
	if updated == text {
		t.Fatalf("fixture frontmatter has no status: draft line:\n%s", text)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTruthFreezeSurfaceStrict(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "surf-ok")
	ctx := env.ctx(t, "surf-ok")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	seedSurfaceFrontmatter(t, truthPath, "surfaces: [api, desktop]\n")

	v, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("freeze with valid surfaces: %v", err)
	}
	if v.Status != "frozen" || !v.SurfacesDeclared || !v.SurfaceStrict {
		t.Fatalf("frozen view = %+v", v)
	}
	if len(v.Surfaces) != 2 || v.Surfaces[0] != "api" || v.Surfaces[1] != "desktop" {
		t.Fatalf("surfaces = %v", v.Surfaces)
	}
	frozen := readFile(t, truthPath)
	scalars, _, ok := gate.ParseFrontmatter(frozen)
	if !ok || scalars["surface_strict"] != "true" {
		t.Fatalf("freeze must write surface_strict: true, frontmatter = %v", scalars)
	}
}

func TestTruthFreezeSurfaceRejects(t *testing.T) {
	cases := []struct {
		name   string
		block  string
		body   string
		reason string
	}{
		{"invalid value", "surfaces: [foo]\n", "", core.ReasonTruthSurfaceInvalid},
		{"empty list", "surfaces: []\n", "", core.ReasonTruthSurfaceRequired},
		{"AC surface outside list", "surfaces: [api]\n", "\n| AC ID | Requirement | Surface |\n|---|---|---|\n| AC-002 | window | surface=desktop |\n", core.ReasonTruthACSurfaceUndeclared},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTruthEnv(t)
			env.create(t, "surf-bad")
			ctx := env.ctx(t, "surf-bad")
			truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
			seedSurfaceFrontmatter(t, truthPath, tc.block)
			if tc.body != "" {
				text := readFile(t, truthPath)
				if err := os.WriteFile(truthPath, []byte(text+tc.body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			_, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
			if err == nil {
				t.Fatalf("%s: freeze must be refused", tc.name)
			}
			rc, ok := core.ReasonCodeOf(err)
			if !ok || rc != tc.reason {
				t.Fatalf("%s: reason = %v (ok=%v), want %s", tc.name, err, ok, tc.reason)
			}
			// A refused freeze must leave the package a draft.
			view, err := env.eng.TruthPackage(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != "draft" || view.Frozen {
				t.Fatalf("%s: refused freeze mutated state: %+v", tc.name, view)
			}
		})
	}
}

func TestTruthFreezeLegacyPackageWritesNoStrict(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "surf-legacy")
	ctx := env.ctx(t, "surf-legacy")
	truthPath := filepath.Join(env.root, ctx.Feature.TruthPath)
	v, err := env.eng.FreezeTruthPackage(ctx, "qa-bot")
	if err != nil {
		t.Fatalf("legacy freeze: %v", err)
	}
	if v.Status != "frozen" || v.SurfacesDeclared || v.SurfaceStrict {
		t.Fatalf("legacy view = %+v", v)
	}
	if frozen := readFile(t, truthPath); strings.Contains(frozen, "surface_strict:") {
		t.Fatalf("legacy freeze must not backfill surface_strict:\n%s", frozen)
	}
}

func registerSurfaceFixture(t *testing.T, root, name, text string) string {
	t.Helper()
	p := filepath.Join(root, ".beacon", "evidence", name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestArtifactSurfaceKindAccepts(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	cases := []struct{ kind, name string }{
		{"a11y_tree", "window.a11y.json"},
		{"browser_trace", "trace.zip"},
		{"browser_trace", "trace.json"},
		{"e2e_report", "report.json"},
		{"e2e_report", "report.md"},
	}
	for _, tc := range cases {
		path := registerSurfaceFixture(t, env.root, tc.name, "{}")
		if _, err := env.eng.RegisterArtifact(run.RunID, tc.kind, path, "qa-bot"); err != nil {
			t.Fatalf("%s %s: %v", tc.kind, tc.name, err)
		}
	}
}

func TestArtifactSurfaceKindRejects(t *testing.T) {
	env := newTestEnv(t, "active")
	run := env.createRun(t)
	cases := []struct{ kind, name string }{
		{"a11y_tree", "window.txt"},
		{"browser_trace", "trace.md"},
		{"e2e_report", "report.zip"},
	}
	for _, tc := range cases {
		before, err := env.st.Artifacts().ListByRun(run.RunID)
		if err != nil {
			t.Fatal(err)
		}
		eventsBefore, err := env.st.ListEvents(env.feature.ID)
		if err != nil {
			t.Fatal(err)
		}
		path := registerSurfaceFixture(t, env.root, tc.name, "x")
		_, err = env.eng.RegisterArtifact(run.RunID, tc.kind, path, "qa-bot")
		if err == nil {
			t.Fatalf("%s %s: wrong extension must be refused", tc.kind, tc.name)
		}
		if !strings.Contains(err.Error(), ReasonArtifactKindExtensionNotAllowed) {
			t.Fatalf("%s %s: reason = %v", tc.kind, tc.name, err)
		}
		after, _ := env.st.Artifacts().ListByRun(run.RunID)
		if len(after) != len(before) {
			t.Fatalf("%s %s: artifact_refs grew from %d to %d", tc.kind, tc.name, len(before), len(after))
		}
		eventsAfter, _ := env.st.ListEvents(env.feature.ID)
		for _, e := range eventsAfter[len(eventsBefore):] {
			if e.EventType == EventArtifactRegistered {
				t.Fatalf("%s %s: a refused registration appended %s", tc.kind, tc.name, EventArtifactRegistered)
			}
		}
	}
}

func surfaceTruthDoc(surfaces, strict string) string {
	return strings.Replace(frozenTruthDoc, "status: frozen\n",
		"status: frozen\nsurfaces: "+surfaces+"\nsurface_strict: "+strict+"\n", 1)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

func writeVerdictEvidence(t *testing.T, name string, ev gate.Evaluation) {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "docs", "beacon", "v2.0.0", "features", "qa-surface-e2e", "evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSurfaceEvaluateRealPackage(t *testing.T) {
	// PASS: api + desktop declared, both evidence kinds live, autotest
	// directory present.
	pass := newGateEnv(t, surfaceTruthDoc("[api, desktop]", "true"))
	appendRealStageExecuted(t, pass.testEnv, "", "qa")
	run := pass.createRun(t)
	appendRealStageExecuted(t, pass.testEnv, run.RunID, "qa")
	e2e := registerSurfaceFixture(t, pass.root, "e2e-report.json", "{\"verdict\":\"pass\"}")
	a11y := registerSurfaceFixture(t, pass.root, "window-a11y.json", "{\"role\":\"window\"}")
	if _, err := pass.eng.RegisterArtifact(run.RunID, "e2e_report", e2e, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := pass.eng.RegisterArtifact(run.RunID, "a11y_tree", a11y, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	autotestDir := filepath.Join(pass.root, ".beacon", "autotest", run.RunID)
	if err := os.MkdirAll(autotestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(autotestDir, "result.json"), []byte("{\"passed\":true}"), 0o644); err != nil {
		t.Fatal(err)
	}
	passEv, err := pass.eng.EvaluateGate(pass.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if passEv.Verdict != gate.VerdictPass {
		t.Fatalf("complete package must pass, got %+v", passEv)
	}
	writeVerdictEvidence(t, "qa-evaluate-pass.json", passEv)

	// FAIL: the same declaration with only an api report and no
	// autotest record — both doors close.
	fail := newGateEnv(t, surfaceTruthDoc("[api, desktop]", "true"))
	failRun := fail.createRun(t)
	appendRealStageExecuted(t, fail.testEnv, failRun.RunID, "qa")
	onlyReport := registerSurfaceFixture(t, fail.root, "e2e-report.json", "{\"verdict\":\"pass\"}")
	if _, err := fail.eng.RegisterArtifact(failRun.RunID, "e2e_report", onlyReport, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	failEv, err := fail.eng.EvaluateGate(fail.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if failEv.Verdict != gate.VerdictBlocked || !containsReason(failEv, "qa_surface_evidence_miss:desktop") ||
		!containsReason(failEv, core.ReasonQAAutotestRecordedMissing) {
		t.Fatalf("missing desktop evidence and autotest must block, got %+v", failEv)
	}
	writeVerdictEvidence(t, "qa-evaluate-fail.json", failEv)

	// FAIL (autotest only): api evidence is complete, the run left no
	// autotest record.
	noAuto := newGateEnv(t, surfaceTruthDoc("[api]", "true"))
	noAutoRun := noAuto.createRun(t)
	appendRealStageExecuted(t, noAuto.testEnv, noAutoRun.RunID, "qa")
	apiReport := registerSurfaceFixture(t, noAuto.root, "api-e2e.json", "{\"verdict\":\"pass\"}")
	if _, err := noAuto.eng.RegisterArtifact(noAutoRun.RunID, "e2e_report", apiReport, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	noAutoEv, err := noAuto.eng.EvaluateGate(noAuto.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if noAutoEv.Verdict != gate.VerdictBlocked || !containsReason(noAutoEv, core.ReasonQAAutotestRecordedMissing) {
		t.Fatalf("missing autotest must block, got %+v", noAutoEv)
	}
	if containsReason(noAutoEv, "qa_surface_evidence_miss:api") {
		t.Fatalf("api evidence was registered, surface door must stay open: %+v", noAutoEv)
	}
	writeVerdictEvidence(t, "qa-evaluate-fail-autotest.json", noAutoEv)

	// Legacy waiver: surface_strict false turns both doors into warnings.
	legacy := newGateEnv(t, surfaceTruthDoc("[api, desktop]", "false"))
	legacyRun := legacy.createRun(t)
	appendRealStageExecuted(t, legacy.testEnv, legacyRun.RunID, "qa")
	legacyReport := registerSurfaceFixture(t, legacy.root, "legacy-e2e.json", "{}")
	if _, err := legacy.eng.RegisterArtifact(legacyRun.RunID, "e2e_report", legacyReport, "qa-bot"); err != nil {
		t.Fatal(err)
	}
	legacyEv, err := legacy.eng.EvaluateGate(legacy.ctx, "qa", "", "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if legacyEv.Verdict != gate.VerdictPass {
		t.Fatalf("legacy waiver must pass, got %+v", legacyEv)
	}
	writeVerdictEvidence(t, "qa-evaluate-legacy-waiver.json", legacyEv)
}

func containsReason(ev gate.Evaluation, code string) bool {
	for _, rc := range ev.ReasonCodes {
		if rc == code {
			return true
		}
	}
	for _, c := range ev.Checks {
		if c.ReasonCode == code {
			return true
		}
	}
	return false
}
