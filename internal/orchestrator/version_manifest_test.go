package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"beacon-go/internal/store"
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
