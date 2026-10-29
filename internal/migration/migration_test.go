// Behavior tests for the P3b v1 migration importer and doctor rebuild.
// The sample project is built to the probed real v1 format (see scan.go):
// docs/beacon/<version>/features/<slug>/{truth,tests,evidence}.md with
// frontmatter status, .machine/execution/<slug>.revision.json and
// <slug>.truth-review-gate.json, project-level .beacon/gate-report.json.
package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildV1Sample creates the minimal v1 project used across the tests:
//
//	v1.1.0/feat-old   frozen, revision lifecycle_status=accepted,
//	                  truth-review gate pass, evidence refs: 2 on disk,
//	                  1 directory, 1 missing
//	v1.2.0/feat-new   draft (lifecycle derived Freeze/InProgress)
//	v1.2.0/feat-weird status "in-review" (unmapped -> lifecycle skipped)
//	.beacon/gate-report.json (version-scoped -> gate skipped)
func buildV1Sample(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "beacon", "version-truth.json"),
		`{"docs_default_line":"v1.2.0","package_version":"1.2.0"}`)

	old := filepath.Join(root, "docs", "beacon", "v1.1.0", "features", "feat-old")
	writeFile(t, filepath.Join(old, "truth.md"),
		"---\nslug: feat-old\nstatus: frozen\n---\n# Truth: feat-old\n")
	writeFile(t, filepath.Join(old, "tests.md"), "# Tests\n")
	writeFile(t, filepath.Join(old, "evidence.md"), `# Evidence Index: feat-old

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v1.1.0/features/feat-old/truth.md | linked | truth |
| qa | qa_verdict | .beacon/evidence/feat-old-report.json | linked | qa |
| release | release_verdict | docs/beacon/v1.1.0/release/ | linked | release |
| qa-extra | qa_verdict | .beacon/evidence/missing.json | linked | qa |
`)
	if err := os.MkdirAll(filepath.Join(root, "docs", "beacon", "v1.1.0", "release"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, ".beacon", "evidence", "feat-old-report.json"), `{"verdict":"pass"}`)
	machine := filepath.Join(root, "docs", "beacon", "v1.1.0", ".machine", "execution")
	writeFile(t, filepath.Join(machine, "feat-old.revision.json"),
		`{"feature_slug":"feat-old","doc_status":"frozen","delivery_status":"accepted","lifecycle_status":"accepted","created_at":"2026-09-01T00:00:00Z"}`)
	writeFile(t, filepath.Join(machine, "feat-old.truth-review-gate.json"),
		`{"feature":"feat-old","version":"v1.1.0","pass":true,"blocked":false,"reason_code":""}`)

	nw := filepath.Join(root, "docs", "beacon", "v1.2.0", "features", "feat-new")
	writeFile(t, filepath.Join(nw, "truth.md"), "---\nslug: feat-new\nstatus: draft\n---\n# Truth: feat-new\n")
	writeFile(t, filepath.Join(nw, "tests.md"), "# Tests\n")

	weird := filepath.Join(root, "docs", "beacon", "v1.2.0", "features", "feat-weird")
	writeFile(t, filepath.Join(weird, "truth.md"), "---\nslug: feat-weird\nstatus: in-review\n---\n# Truth\n")
	writeFile(t, filepath.Join(weird, "tests.md"), "# Tests\n")

	writeFile(t, filepath.Join(root, ".beacon", "gate-report.json"),
		`{"stage":"release","status":"Pass","version":"v1.2.0","run_id":"run-x","updated_at":"2026-09-02T00:00:00Z"}`)
	return root
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "beacon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func dirDigest(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write([]byte(path))
		h.Write(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestImportCountsAndIdempotency(t *testing.T) {
	root := buildV1Sample(t)
	st := openTestStore(t)
	before := dirDigest(t, root)

	rep, err := ImportV1(st, root, Options{})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if got := dirDigest(t, root); got != before {
		t.Fatal("importer modified the v1 project directory")
	}
	want := map[string][3]int{ // imported, already_present, skipped
		"projects":          {1, 0, 0},
		"project_versions":  {2, 0, 0},
		"feature_packages":  {3, 0, 0},
		"feature_lifecycle": {2, 0, 1}, // feat-weird unmapped
		"gate_results":      {1, 0, 1}, // project gate report skipped
		"artifact_refs":     {2, 0, 2}, // directory + missing file skipped
	}
	for cat, w := range want {
		c := rep.Counts[cat]
		if [3]int{c.Imported, c.AlreadyPresent, c.Skipped} != w {
			t.Errorf("%s: got imported=%d already=%d skipped=%d, want %v", cat, c.Imported, c.AlreadyPresent, c.Skipped, w)
		}
	}

	// Spot-check mapped content.
	ctx, err := st.ResolveFeature(root, "feat-old", "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	lc, err := st.Features().GetLifecycle(ctx.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Stage != core.StageRelease || lc.Status != core.StatusCompleted {
		t.Errorf("feat-old lifecycle = %s/%s, want Release/Completed", lc.Stage, lc.Status)
	}
	gates, err := st.Gates().ListByFeature(ctx.Feature.ID)
	if err != nil || len(gates) != 1 || gates[0].GateType != TruthReviewGateType || gates[0].Verdict != "pass" {
		t.Errorf("feat-old gates = %+v, err %v", gates, err)
	}
	runs, err := st.Runs().ListByFeature(ctx.Feature.ID)
	if err != nil || len(runs) != 1 || runs[0].RunKind != MigrationRunKind {
		t.Fatalf("feat-old runs = %+v, err %v", runs, err)
	}
	arts, err := st.Artifacts().ListByRun(runs[0].ID)
	if err != nil || len(arts) != 2 {
		t.Errorf("feat-old artifacts = %d, err %v", len(arts), err)
	}

	// Second run: everything already present, nothing imported twice.
	rep2, err := ImportV1(st, root, Options{})
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	for cat, c := range rep2.Counts {
		if c.Imported != 0 {
			t.Errorf("second run imported %d in %s, want 0", c.Imported, cat)
		}
	}
	if rep2.Counts["feature_packages"].AlreadyPresent != 3 ||
		rep2.Counts["gate_results"].AlreadyPresent != 1 ||
		rep2.Counts["artifact_refs"].AlreadyPresent != 2 {
		t.Errorf("second run already_present counts wrong: %+v", rep2.Counts)
	}
	projects, _ := st.Projects().List()
	features, _ := st.Projects().ListFeatures(projects[0].ID)
	if len(projects) != 1 || len(features) != 3 {
		t.Errorf("after re-import: projects=%d features=%d, want 1/3", len(projects), len(features))
	}
	gates2, _ := st.Gates().ListByFeature(ctx.Feature.ID)
	arts2, _ := st.Artifacts().ListByRun(runs[0].ID)
	if len(gates2) != 1 || len(arts2) != 2 {
		t.Errorf("rows duplicated: gates=%d artifacts=%d", len(gates2), len(arts2))
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	root := buildV1Sample(t)
	st := openTestStore(t)
	rep, err := ImportV1(st, root, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Counts["feature_packages"].Imported != 3 || rep.Counts["artifact_refs"].Imported != 2 {
		t.Errorf("dry-run counts wrong: %+v", rep.Counts)
	}
	projects, err := st.Projects().List()
	if err != nil || len(projects) != 0 {
		t.Errorf("dry-run wrote projects: %v %v", projects, err)
	}
}

func TestImportRefusesV2DataUnlessForced(t *testing.T) {
	root := buildV1Sample(t)
	st := openTestStore(t)
	resolved := store.ResolveRootPath(root)
	p, err := st.Projects().Register(store.Project{RootPath: resolved, Name: "v1sample"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v9.9.9", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{ProjectVersionID: v.ID, Slug: "live", TruthPath: "t", TestsPath: "s", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusInProgress,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Runs().CreateRun(store.Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning}); err != nil {
		t.Fatal(err)
	}

	rep, err := ImportV1(st, root, Options{})
	if err == nil {
		t.Fatal("expected refusal for project with v2 data")
	}
	var ce *core.Error
	if !errors.As(err, &ce) || ce.ReasonCode != ReasonHasV2Data {
		t.Fatalf("refusal error = %v, want reason %s", err, ReasonHasV2Data)
	}
	if rep == nil {
		t.Fatal("refusal must still return the report")
	}
	features, _ := st.Projects().ListFeatures(p.ID)
	if len(features) != 1 {
		t.Fatalf("refused import wrote features: %d", len(features))
	}

	// With --force: missing items are filled, the live lifecycle is kept.
	rep, err = ImportV1(st, root, Options{Force: true})
	if err != nil {
		t.Fatalf("forced import: %v", err)
	}
	if rep.Counts["feature_packages"].Imported != 3 {
		t.Errorf("forced import packages imported=%d, want 3", rep.Counts["feature_packages"].Imported)
	}
	lc, err := st.Features().GetLifecycle(f.ID)
	if err != nil || lc.Stage != core.StageQa || lc.Status != core.StatusInProgress {
		t.Errorf("live lifecycle overwritten: %+v %v", lc, err)
	}
	features, _ = st.Projects().ListFeatures(p.ID)
	if len(features) != 4 {
		t.Errorf("after forced import features=%d, want 4 (live + 3 imported)", len(features))
	}
}

func TestMapLifecycleTable(t *testing.T) {
	cases := []struct {
		name      string
		rev       *RevisionInfo
		pkg       string
		wantStage core.WorkflowStage
		wantStat  core.StageStatus
		wantOK    bool
	}{
		{"accepted", &RevisionInfo{LifecycleStatus: "accepted", DocStatus: "frozen"}, "frozen", core.StageRelease, core.StatusCompleted, true},
		{"verified", &RevisionInfo{LifecycleStatus: "verified"}, "", core.StageQa, core.StatusCompleted, true},
		{"implemented", &RevisionInfo{LifecycleStatus: "implemented"}, "", core.StageImplement, core.StatusCompleted, true},
		{"idle", &RevisionInfo{LifecycleStatus: "idle"}, "", core.StageThink, core.StatusNotStarted, true},
		{"blocked frozen doc", &RevisionInfo{LifecycleStatus: "blocked", DocStatus: "frozen"}, "", core.StageImplement, core.StatusBlocked, true},
		{"blocked draft doc", &RevisionInfo{LifecycleStatus: "blocked"}, "draft", core.StageFreeze, core.StatusBlocked, true},
		{"doc frozen fallback", nil, "frozen", core.StageFreeze, core.StatusCompleted, true},
		{"doc draft fallback", nil, "draft", core.StageFreeze, core.StatusInProgress, true},
		{"unmapped lifecycle", &RevisionInfo{LifecycleStatus: "doing"}, "", "", "", false},
		{"unmapped doc", nil, "in-review", "", "", false},
		{"no source", nil, "", "", "", false},
	}
	for _, c := range cases {
		stage, status, ok, _ := MapLifecycle(FeatureScan{Revision: c.rev, PkgStatus: c.pkg})
		if ok != c.wantOK || (ok && (stage != c.wantStage || status != c.wantStat)) {
			t.Errorf("%s: got %s/%s ok=%v, want %s/%s ok=%v", c.name, stage, status, ok, c.wantStage, c.wantStat, c.wantOK)
		}
	}
}

// doctorFixture imports the sample, then introduces one drift of each
// kind plus a lifecycle divergence that must stay report-only.
func doctorFixture(t *testing.T) (string, *store.Store) {
	t.Helper()
	root := buildV1Sample(t)
	st := openTestStore(t)
	if _, err := ImportV1(st, root, Options{}); err != nil {
		t.Fatal(err)
	}
	// missing_in_store: a new package appears on disk after the import.
	late := filepath.Join(root, "docs", "beacon", "v1.2.0", "features", "feat-late")
	writeFile(t, filepath.Join(late, "truth.md"), "---\nslug: feat-late\nstatus: frozen\n---\n# Truth\n")
	writeFile(t, filepath.Join(late, "tests.md"), "# Tests\n")
	// status_mismatch: the store's package status diverges from disk.
	ctx, err := st.ResolveFeature(root, "feat-new", "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Features().UpdatePackageStatus(ctx.Feature.ID, "bogus"); err != nil {
		t.Fatal(err)
	}
	// lifecycle divergence: store lifecycle no longer matches the
	// disk-derived value (Release/Completed); doctor must not touch it.
	old, err := st.ResolveFeature(root, "feat-old", "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Features().UpdateLifecycle(store.Lifecycle{
		FeatureID: old.Feature.ID, Stage: core.StageQa, Status: core.StatusCompleted,
		ReasonCodes: []string{DerivedReason},
	}); err != nil {
		t.Fatal(err)
	}
	// missing_on_disk: a store-only package.
	versions, err := st.Projects().ListVersions(mustProjectID(t, st, root))
	if err != nil {
		t.Fatal(err)
	}
	var v120 string
	for _, v := range versions {
		if v.Version == "v1.2.0" {
			v120 = v.ID
		}
	}
	if _, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v120, Slug: "feat-ghost", TruthPath: "t", TestsPath: "s", Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	return root, st
}

func mustProjectID(t *testing.T, st *store.Store, root string) string {
	t.Helper()
	p, err := st.Projects().GetByRootPath(store.ResolveRootPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func driftOf(rep *RebuildReport, kind, slug string) *DriftEntry {
	for i := range rep.Drift {
		if rep.Drift[i].Kind == kind && rep.Drift[i].Slug == slug {
			return &rep.Drift[i]
		}
	}
	return nil
}

func TestDoctorReportOnly(t *testing.T) {
	root, st := doctorFixture(t)
	rep, err := Rebuild(st, root, false)
	if err != nil {
		t.Fatal(err)
	}
	if driftOf(rep, DriftMissingInStore, "feat-late") == nil {
		t.Errorf("missing_in_store for feat-late not detected: %+v", rep.Drift)
	}
	if driftOf(rep, DriftMissingOnDisk, "feat-ghost") == nil {
		t.Errorf("missing_on_disk for feat-ghost not detected: %+v", rep.Drift)
	}
	if driftOf(rep, DriftStatusMismatch, "feat-new") == nil {
		t.Errorf("status_mismatch for feat-new not detected: %+v", rep.Drift)
	}
	d := driftOf(rep, DriftStatusMismatch, "feat-old")
	if d == nil || !strings.Contains(d.Detail, "lifecycle") {
		t.Errorf("lifecycle status_mismatch for feat-old not detected: %+v", rep.Drift)
	}
	if rep.EventsWritten != 0 || len(rep.Corrected) != 0 {
		t.Errorf("report-only rebuild wrote corrections: %+v", rep)
	}
	// Nothing changed in the store.
	if _, err := st.ResolveFeature(root, "feat-late", "v1.2.0"); err == nil {
		t.Error("report-only rebuild registered feat-late")
	}
	ctx, _ := st.ResolveFeature(root, "feat-new", "v1.2.0")
	if ctx.Feature.Status != "bogus" {
		t.Errorf("report-only rebuild changed package status to %q", ctx.Feature.Status)
	}
}

func TestDoctorApply(t *testing.T) {
	root, st := doctorFixture(t)
	// AC-011: snapshot of a pre-existing feature before rebuild.
	old, err := st.ResolveFeature(root, "feat-old", "v1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	snapBefore, err := st.Spine().Snapshot(old.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := Rebuild(st, root, true)
	if err != nil {
		t.Fatal(err)
	}
	if rep.EventsWritten == 0 || len(rep.Corrected) == 0 {
		t.Fatalf("apply corrected nothing: %+v", rep)
	}
	// Missing package registered and readable through spine.
	late, err := st.ResolveFeature(root, "feat-late", "v1.2.0")
	if err != nil {
		t.Fatalf("feat-late not registered after apply: %v", err)
	}
	snap, err := st.Spine().Snapshot(late.Feature.ID)
	if err != nil || snap.FeatureSlug != "feat-late" {
		t.Errorf("spine snapshot for feat-late: %+v err %v", snap, err)
	}
	// Package status corrected back to the disk fact.
	nw, err := st.ResolveFeature(root, "feat-new", "v1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if nw.Feature.Status != "draft" {
		t.Errorf("feat-new status = %q, want draft", nw.Feature.Status)
	}
	// Corrections left an append-only event trail.
	events, err := st.ListEvents(nw.Feature.ID)
	if err != nil || len(events) == 0 {
		t.Fatalf("no events for feat-new: %v %v", events, err)
	}
	found := false
	for _, e := range events {
		if e.EventType == "doctor_rebuild_corrected" {
			found = true
		}
	}
	if !found {
		t.Errorf("doctor_rebuild_corrected event missing: %+v", events)
	}
	// The store-only package is reported but never deleted.
	if _, err := st.ResolveFeature(root, "feat-ghost", "v1.2.0"); err != nil {
		t.Error("doctor apply deleted the store-only package feat-ghost")
	}
	// AC-011: lifecycle/gate fields of the pre-existing feature are
	// untouched by the rebuild (lifecycle divergence stays as-is).
	snapAfter, err := st.Spine().Snapshot(old.Feature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapAfter.Stage != snapBefore.Stage || snapAfter.Status != snapBefore.Status ||
		len(snapAfter.LatestGates) != len(snapBefore.LatestGates) {
		t.Errorf("rebuild changed lifecycle/gate fields: before %s/%s gates=%d, after %s/%s gates=%d",
			snapBefore.Stage, snapBefore.Status, len(snapBefore.LatestGates),
			snapAfter.Stage, snapAfter.Status, len(snapAfter.LatestGates))
	}
	lc, _ := st.Features().GetLifecycle(old.Feature.ID)
	if lc.Stage != core.StageQa {
		t.Errorf("rebuild overwrote lifecycle: %s", lc.Stage)
	}
}

// AC-092: a real import snapshots the live ledger before writing, and the
// snapshot is a usable database with the pre-import rows.
func TestImportBacksUpLedgerFirst(t *testing.T) {
	st := openTestStore(t)
	root := buildV1Sample(t)
	if _, err := st.Projects().Register(store.Project{RootPath: "/tmp/pre-existing", Name: "pre"}); err != nil {
		t.Fatal(err)
	}
	rep, err := ImportV1(st, root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rep.BackupPath == "" {
		t.Fatal("import did not record a backup path")
	}
	if _, err := os.Stat(rep.BackupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	backup, err := store.Open(rep.BackupPath)
	if err != nil {
		t.Fatalf("backup is not a usable database: %v", err)
	}
	defer backup.Close()
	projs, err := backup.Projects().List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range projs {
		if p.RootPath == store.ResolveRootPath("/tmp/pre-existing") {
			found = true
		}
	}
	if !found {
		t.Errorf("backup does not contain the pre-import project rows: %+v", projs)
	}
}

// A dry run writes nothing, so it must not create a backup either.
func TestDryRunWritesNoBackup(t *testing.T) {
	st := openTestStore(t)
	root := buildV1Sample(t)
	rep, err := ImportV1(st, root, Options{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep.BackupPath != "" {
		t.Errorf("dry run recorded a backup: %q", rep.BackupPath)
	}
}
