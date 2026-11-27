package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
	buildversion "beacon-go/internal/version"
	"beacon-go/internal/versionmanifest"
)

func writeVersionFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"truth.md":       "---\nslug: feat-a\nstatus: draft\n---\n# truth\n",
		"tests.md":       "---\nslug: feat-a\n---\n# tests\n",
		"tasks.md":       "---\nslug: feat-a\n---\n# tasks\n",
		"restatement.md": "---\nslug: feat-a\n---\n## 1 交付形态\nx\n## 2 界面与交互\nx\n## 3 业务流转\nx\n## 4 边界与不做\nx\n## 5 歧义与默认值\nx\n",
		"evidence.md":    "# evidence\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVersionCreationRefusesUnregisteredScope(t *testing.T) {
	for _, method := range []string{"open", "bump"} {
		for _, probe := range []string{"unregistered-project", "unregistered-version", "closed-store"} {
			t.Run(method+"/"+probe, func(t *testing.T) {
				env := newTestEnv(t, "draft")
				root, from := env.root, "v2.0.0"
				if probe == "unregistered-project" {
					root = t.TempDir()
				}
				if probe == "unregistered-version" {
					from = "v9.0.0"
				}
				parent := versionmanifest.Manifest{Schema: versionmanifest.Schema, Version: from, Status: versionmanifest.StatusDraft}
				path := versionmanifest.Path(root, from)
				if err := versionmanifest.Save(path, parent); err != nil {
					t.Fatal(err)
				}
				before, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if probe == "closed-store" {
					if err := env.st.Close(); err != nil {
						t.Fatal(err)
					}
				}
				create := env.eng.OpenVersion
				if method == "bump" {
					create = env.eng.BumpProjectVersion
				}
				if _, err := create(root, from, "patch"); err == nil {
					t.Fatal("accepted an unavailable project/version scope")
				}
				next, err := BumpDocsVersion(from, "patch")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := os.Lstat(versionmanifest.Path(root, next)); !os.IsNotExist(err) {
					t.Fatalf("refusal left a next manifest: %v", err)
				}
				after, err := os.ReadFile(path)
				if err != nil || string(before) != string(after) {
					t.Fatalf("refusal changed parent: %v", err)
				}
			})
		}
	}
}

func TestBaselineAndOpenVersion(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)

	m, err := env.eng.BaselineVersion(env.root, "v2.0.0")
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if m.Status != versionmanifest.StatusBaseline || len(m.Lakes) != 1 {
		t.Fatalf("manifest = %+v", m)
	}
	want := versionmanifest.HashContract(
		[]byte("---\nslug: feat-a\nstatus: draft\n---\n# truth\n"),
		[]byte("---\nslug: feat-a\n---\n# tests\n"),
		[]byte("---\nslug: feat-a\n---\n# tasks\n"),
		[]byte("---\nslug: feat-a\n---\n## 1 交付形态\nx\n## 2 界面与交互\nx\n## 3 业务流转\nx\n## 4 边界与不做\nx\n## 5 歧义与默认值\nx\n"),
	)
	if m.Lakes[0].Package != want {
		t.Fatalf("package hash = %s want %s", m.Lakes[0].Package, want)
	}

	child, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if child.Version != "v2.0.1" || child.Status != versionmanifest.StatusDraft || child.Lakes[0].Status != versionmanifest.LakeInherited {
		t.Fatalf("child = %+v", child)
	}
}

func TestMarkAndRecordVersionLake(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The feature lives in v2.0.0; v2.1.0 is the current draft. The
	// change must land in v2.1.0, never rewrite the baseline.
	if err := env.eng.MarkVersionChange(fc, "chg_test"); err != nil {
		t.Fatal(err)
	}
	m, err := versionmanifest.LoadVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Lakes[0].Status != versionmanifest.LakeDirty || len(m.Changes) != 1 {
		t.Fatalf("dirty manifest = %+v", m)
	}

	if err := env.eng.RecordVersionPackage(fc, "newhash", []string{"chg_test"}); err != nil {
		t.Fatal(err)
	}
	m, err = versionmanifest.LoadVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Lakes[0].Package != "newhash" || m.Lakes[0].Status != versionmanifest.LakeChanged {
		t.Fatalf("recorded manifest = %+v", m.Lakes[0])
	}
	base, err := versionmanifest.LoadVersion(env.root, "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if base.Lakes[0].Package == "newhash" || base.Status != versionmanifest.StatusBaseline {
		t.Fatalf("baseline was rewritten: %+v", base.Lakes[0])
	}
}

func TestProgramSeaSyncAndRouteSource(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "sea", Freeze: true,
		Lakes: []ProgramLakeInput{{Slug: env.feature.Slug}},
	}); err != nil {
		t.Fatal(err)
	}
	man, err := versionmanifest.LoadVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(man.Seas) != 1 || man.Seas[0].Slug != "sea" || man.Seas[0].HostFeature != env.feature.Slug {
		t.Fatalf("seas = %+v", man.Seas)
	}
	view, err := env.eng.ProgramRoute(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if view.LakesSource != "manifest" {
		t.Fatalf("route source = %q, want manifest", view.LakesSource)
	}
}

func TestVersionFreezeReleaseFormalGates(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.MarkVersionChange(fc, "chg_1"); err != nil {
		t.Fatal(err)
	}
	if err := env.eng.RecordVersionPackage(fc, "newhash", []string{"chg_1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.FreezeVersion(env.root, "v2.0.1"); err == nil {
		t.Fatal("freeze without qa must fail")
	}
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "qa", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	host := env.createRun(t)
	r, m, _, err := env.eng.loadRun(host.RunID)
	if err != nil {
		t.Fatal(err)
	}
	m.Verifier = &VerifierInfo{Present: true}
	if err := env.eng.saveRun(r, m); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.FreezeVersion(env.root, "v2.0.1"); err != nil {
		t.Fatalf("freeze with qa+verifier: %v", err)
	}
	if _, err := env.eng.ReleaseVersion(env.root, "v2.0.1"); err == nil {
		t.Fatal("release without release gate must fail")
	}
	if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "release", Verdict: "pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ReleaseVersion(env.root, "v2.0.1"); err != nil {
		t.Fatalf("release with gate: %v", err)
	}
}

func TestVersionPlanAndCompat(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.MarkVersionChange(fc, "chg_1"); err != nil {
		t.Fatal(err)
	}
	if err := env.eng.RecordVersionPackage(fc, "newhash", []string{"chg_1"}); err != nil {
		t.Fatal(err)
	}
	plan, err := env.eng.PlanVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if plan.SuggestedLevel != "patch" || plan.LakesChanged != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	missing, err := env.eng.CompatVersion(env.root, "v2.0.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 {
		t.Fatalf("compat missing = %v", missing)
	}
}

func TestReconcileClosesStaleManifestChange(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	fc, err := env.eng.featureContext(env.feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.eng.MarkVersionChange(fc, "chg_stale"); err != nil {
		t.Fatal(err)
	}
	m, err := env.eng.ReconcileVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Changes) != 1 || m.Changes[0].Status != "closed" {
		t.Fatalf("reconcile = %+v", m.Changes)
	}
}

func TestManifestIllegalTransitions(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)
	if _, err := env.eng.BaselineVersion(env.root, "v2.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch"); err != nil {
		t.Fatal(err)
	}
	// draft -> released is illegal (must freeze first)
	if _, err := env.eng.ReleaseVersion(env.root, "v2.0.1"); err == nil {
		t.Fatal("draft->released must be rejected")
	}
	m, err := versionmanifest.LoadVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if m.Status != versionmanifest.StatusDraft {
		t.Fatalf("status changed after illegal release: %s", m.Status)
	}
	// freeze then release to reach terminal state
	// v2.0.1 has an inherited lake only; FreezeVersion allows inherited lakes.
	if _, err := env.eng.FreezeVersion(env.root, "v2.0.1"); err != nil {
		t.Fatalf("freeze draft: %v", err)
	}
	if _, err := env.eng.ReleaseVersion(env.root, "v2.0.1"); err != nil {
		t.Fatalf("release frozen: %v", err)
	}
	// released -> draft is illegal (no reopen path; freeze rejects non-draft)
	if _, err := env.eng.FreezeVersion(env.root, "v2.0.1"); err == nil {
		t.Fatal("released->draft/freeze must be rejected")
	}
	m2, err := versionmanifest.LoadVersion(env.root, "v2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if m2.Status != versionmanifest.StatusReleased {
		t.Fatalf("status changed after illegal freeze: %s", m2.Status)
	}
}

func TestVersionManifestRuntimeMatchesBuildVersion(t *testing.T) {
	env := newTestEnv(t, "frozen")
	writeVersionFixture(t, env.root)

	m, err := env.eng.BaselineVersion(env.root, "v2.0.0")
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	if m.Runtime != buildversion.Version || m.Version != "v2.0.0" {
		t.Fatalf("baseline manifest = %+v, want runtime=%q docs version=v2.0.0", m, buildversion.Version)
	}

	child, err := env.eng.OpenVersion(env.root, "v2.0.0", "patch")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if child.Runtime != buildversion.Version || child.Version != "v2.0.1" {
		t.Fatalf("open manifest = %+v, want runtime=%q docs version=v2.0.1", child, buildversion.Version)
	}

	// Open realigns Runtime from the binary even when the parent line
	// carries a stale value; the docs version axis stays independent.
	parent, err := versionmanifest.LoadVersion(env.root, "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	parent.Runtime = "9.9.9"
	if err := versionmanifest.Save(versionmanifest.Path(env.root, "v2.0.0"), parent); err != nil {
		t.Fatal(err)
	}
	child2, err := env.eng.OpenVersion(env.root, "v2.0.0", "minor")
	if err != nil {
		t.Fatalf("open with stale parent: %v", err)
	}
	if child2.Runtime != buildversion.Version || child2.Version != "v2.1.0" {
		t.Fatalf("open manifest = %+v, want runtime=%q docs version=v2.1.0", child2, buildversion.Version)
	}
}

func TestReleaseVersionAlignsRuntimeOnlyAfterExistingChecks(t *testing.T) {
	for _, probe := range []struct {
		name, status, runtime string
		changed, gatePass     bool
		wantError             bool
	}{
		{name: "stale-runtime", status: versionmanifest.StatusFrozen, runtime: "9.9.9"},
		{name: "missing-runtime", status: versionmanifest.StatusFrozen},
		{name: "blank-runtime", status: versionmanifest.StatusFrozen, runtime: " "},
		{name: "already-aligned", status: versionmanifest.StatusFrozen, runtime: buildversion.Version},
		{name: "changed-with-release-gate", status: versionmanifest.StatusFrozen, changed: true, gatePass: true},
		{name: "draft", status: versionmanifest.StatusDraft, runtime: "9.9.9", wantError: true},
		{name: "baseline", status: versionmanifest.StatusBaseline, runtime: "9.9.9", wantError: true},
		{name: "released", status: versionmanifest.StatusReleased, runtime: "9.9.9", wantError: true},
		{name: "changed-without-release-gate", status: versionmanifest.StatusFrozen, changed: true, wantError: true},
		{name: "changed-with-failed-release-gate", status: versionmanifest.StatusFrozen, changed: true, wantError: true},
	} {
		t.Run(probe.name, func(t *testing.T) {
			env := newTestEnv(t, "frozen")
			m := versionmanifest.Manifest{
				Schema: versionmanifest.Schema, Version: "v2.0.0", Parent: "v1.0.0",
				Runtime: probe.runtime, Status: probe.status,
				FrozenAt: "2026-01-01T00:00:00Z",
			}
			if probe.changed {
				m.Lakes = []versionmanifest.Lake{{Slug: env.feature.Slug, Package: "contract-hash", Status: versionmanifest.LakeChanged}}
			}
			if probe.gatePass || probe.name == "changed-with-failed-release-gate" {
				verdict := "fail"
				if probe.gatePass {
					verdict = "pass"
				}
				if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: "release", Verdict: verdict}); err != nil {
					t.Fatal(err)
				}
			}
			path := versionmanifest.Path(env.root, m.Version)
			if err := versionmanifest.Save(path, m); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := env.eng.ReleaseVersion(env.root, m.Version)
			if (err != nil) != probe.wantError {
				t.Fatalf("ReleaseVersion error = %v, want error=%v", err, probe.wantError)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if probe.wantError {
				if string(before) != string(after) {
					t.Fatal("refused release changed manifest bytes")
				}
				return
			}
			saved, err := versionmanifest.LoadVersion(env.root, m.Version)
			if err != nil {
				t.Fatal(err)
			}
			if got.Runtime != buildversion.Version || saved.Runtime != buildversion.Version || saved.Status != versionmanifest.StatusReleased || saved.ReleasedAt == "" {
				t.Fatalf("release did not persist runtime identity: returned=%+v saved=%+v", got, saved)
			}
			if saved.Version != m.Version || saved.Parent != m.Parent || saved.FrozenAt != m.FrozenAt || len(saved.Lakes) != len(m.Lakes) {
				t.Fatalf("runtime alignment changed docs contract: %+v", saved)
			}
		})
	}
}
