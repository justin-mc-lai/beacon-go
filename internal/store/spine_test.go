package store

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
)

// AC-017/081 + v1 parity: project_key = "<basename>--<sha1(resolved)[:10]>".
// The expected value is recomputed here directly from the v1 Python
// algorithm (hashlib.sha1(str(root).encode()).hexdigest()[:10]).
func TestProjectKeyParity(t *testing.T) {
	root := ResolveRootPath(filepath.Join(t.TempDir(), "my-proj"))
	sum := sha1.Sum([]byte(root))
	want := filepath.Base(root) + "--" + hex.EncodeToString(sum[:])[:10]
	if got := ProjectKey(root); got != want {
		t.Fatalf("ProjectKey = %q want %q", got, want)
	}
	// Trailing slash / relative spellings of the same dir must agree.
	if got := ProjectKey(root + string(os.PathSeparator)); got != want {
		t.Fatalf("ProjectKey with trailing slash = %q want %q", got, want)
	}
}

// AC-017/081: full snapshot joins project/version, lifecycle, latest run,
// latest gate per type and artifact views with frozen field names.
func TestSpineSnapshotFullFields(t *testing.T) {
	s, _ := openTestStore(t)
	p, v, f := seedFeature(t, s)
	_ = s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusBlocked, ReasonCodes: []string{"evidence_missing"}})
	r, _ := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning, Iteration: 3, Metadata: json.RawMessage(`{"phase":"qa","next_action":"phase:release"}`)})
	blob := filepath.Join(t.TempDir(), "evidence.json")
	if err := os.WriteFile(blob, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(`{"ok":true}`))
	_, _ = s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: blob, ContentHash: hex.EncodeToString(digest[:]), SizeBytes: 10})
	_, _ = s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "log", AbsPath: filepath.Join(t.TempDir(), "gone.log"), ContentHash: "deadbeef"})
	_, _ = s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"evidence_missing"}, EvaluatedAt: "2026-10-21T02:00:00Z"})
	_, _ = s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "pass", EvaluatedAt: "2026-10-21T03:00:00Z"})
	_, _ = s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "release", Verdict: "blocked", ReasonCodes: []string{"qa_not_passed"}, EvaluatedAt: "2026-10-21T01:00:00Z"})

	snap, err := s.Spine().Snapshot(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ProjectKey != ProjectKey(p.RootPath) || snap.ProjectRoot != p.RootPath || snap.Version != v.Version {
		t.Errorf("snapshot project join = key %q root %q version %q", snap.ProjectKey, snap.ProjectRoot, snap.Version)
	}
	if snap.FeatureSlug != "demo-feature" || snap.Slug != "demo-feature" || snap.FeatureID != f.ID {
		t.Errorf("snapshot feature = %+v", snap)
	}
	if snap.Stage != core.StageQa || snap.Status != core.StatusBlocked || len(snap.ReasonCodes) != 1 {
		t.Errorf("snapshot lifecycle = %+v", snap)
	}
	// metadata wins for phase/next_action; blocked would also derive an
	// action, but the metadata one must be kept verbatim.
	if snap.Phase != "qa" || snap.NextAction != "phase:release" {
		t.Errorf("snapshot derived = phase %q next_action %q", snap.Phase, snap.NextAction)
	}
	if snap.RunID != r.ID || snap.ActiveRunID != r.ID || snap.RunStatus != core.RunRunning || snap.Iteration != 3 {
		t.Errorf("snapshot run = id %q active %q status %q iter %d", snap.RunID, snap.ActiveRunID, snap.RunStatus, snap.Iteration)
	}
	if snap.ProgressStatus != "in_progress" {
		t.Errorf("progress = %q want in_progress", snap.ProgressStatus)
	}
	// Latest per gate_type only: qa must be the newer "pass", not blocked.
	if len(snap.LatestGates) != 2 {
		t.Fatalf("gates = %+v want 2 types", snap.LatestGates)
	}
	byType := map[string]GateResult{}
	for _, g := range snap.LatestGates {
		byType[g.GateType] = g
	}
	if byType["qa"].Verdict != "pass" || byType["release"].Verdict != "blocked" {
		t.Errorf("latest gates = %+v", byType)
	}
	// phase_promoted is false: a latest gate (release) is blocked.
	if snap.PhasePromoted {
		t.Error("phase_promoted must be false when a latest gate is blocked")
	}
	// Artifact views: the real blob is fresh, the missing one stale.
	if snap.ArtifactCount != 2 || len(snap.ArtifactRefs) != 2 {
		t.Fatalf("artifacts = %d/%d want 2", snap.ArtifactCount, len(snap.ArtifactRefs))
	}
	staleByKind := map[string]ArtifactView{}
	for _, av := range snap.ArtifactRefs {
		staleByKind[av.Kind] = av
	}
	if staleByKind["evidence"].Stale {
		t.Errorf("existing blob must not be stale: %+v", staleByKind["evidence"])
	}
	if !staleByKind["log"].Stale || staleByKind["log"].StaleReason != "file_missing" {
		t.Errorf("missing blob must be stale/file_missing: %+v", staleByKind["log"])
	}
	// Frozen JSON field names (AC-065): marshal and check keys exist.
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"run_id", "project_key", "project_root", "version", "feature_slug",
		"lifecycle_stage", "lifecycle_status", "phase", "phases_completed", "phase_promoted",
		"next_harness", "next_action", "progress_status", "active_run_id", "run_status",
		"iteration", "gate_results", "artifact_refs", "blocking_reason_codes", "updated_at"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("snapshot JSON missing frozen key %q", key)
		}
	}
}

// AC-015/TC-015: DB-first — with no blob files at all, lifecycle and gate
// data still come back; artifact pointers degrade to stale flags.
func TestSpineSnapshotDBFirstNoBlobs(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	_ = s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageImplement, Status: core.StatusInProgress})
	r, _ := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning})
	_, _ = s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "log", AbsPath: "/nonexistent-blob-dir/x.log", ContentHash: "abc"})
	_, _ = s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "truth", Verdict: "pass", EvaluatedAt: "2026-10-21T00:00:00Z"})
	snap, err := s.Spine().Snapshot(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Stage != core.StageImplement || snap.Status != core.StatusInProgress {
		t.Errorf("lifecycle lost without blobs: %+v", snap)
	}
	if len(snap.LatestGates) != 1 || snap.LatestGates[0].Verdict != "pass" {
		t.Errorf("gates lost without blobs: %+v", snap.LatestGates)
	}
	if len(snap.ArtifactRefs) != 1 || !snap.ArtifactRefs[0].Stale {
		t.Errorf("missing blob must surface as stale, not error: %+v", snap.ArtifactRefs)
	}
	// Derived fields without any metadata: phase from stage, next from
	// pipeline order (Implement -> Qa).
	if snap.Phase != "generate" || snap.NextAction != "phase:qa" || snap.NextHarness != "qa" {
		t.Errorf("derived = phase %q next %q/%q", snap.Phase, snap.NextHarness, snap.NextAction)
	}
	if !strings.Contains(strings.Join(snap.PhasesCompleted, ","), "truth") {
		t.Errorf("phases_completed = %v want truth included", snap.PhasesCompleted)
	}
}

// AC-082: List filters by project and by lifecycle/run status axes.
func TestSpineListFilters(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f1 := seedFeature(t, s)
	_ = s.Features().UpdateLifecycle(Lifecycle{FeatureID: f1.ID, Stage: core.StageQa, Status: core.StatusInProgress})
	_, _ = s.Runs().CreateRun(Run{FeatureID: f1.ID, RunKind: "goal", Status: core.RunRunning})

	p2, err := s.Projects().Register(Project{RootPath: filepath.Join(t.TempDir(), "other"), Name: "other"})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := s.Projects().AddVersion(ProjectVersion{ProjectID: p2.ID, Version: "v1.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f2, err := s.Features().CreatePackage(FeaturePackage{ProjectVersionID: v2.ID, Slug: "other-feature", TruthPath: "t.md", TestsPath: "x.md"})
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Features().UpdateLifecycle(Lifecycle{FeatureID: f2.ID, Stage: core.StageRelease, Status: core.StatusCompleted})
	_, _ = s.Runs().CreateRun(Run{FeatureID: f2.ID, RunKind: "goal", Status: core.RunSucceeded})

	all, err := s.Spine().List(SpineListFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("List all = %d, %v want 2", len(all), err)
	}
	byProject, err := s.Spine().List(SpineListFilter{ProjectRoot: p2.RootPath})
	if err != nil || len(byProject) != 1 || byProject[0].FeatureSlug != "other-feature" {
		t.Fatalf("List by project = %+v, %v", byProject, err)
	}
	byLifecycle, err := s.Spine().List(SpineListFilter{LifecycleStatus: "Completed"})
	if err != nil || len(byLifecycle) != 1 || byLifecycle[0].FeatureSlug != "other-feature" {
		t.Fatalf("List by lifecycle status = %+v, %v", byLifecycle, err)
	}
	byRun, err := s.Spine().List(SpineListFilter{RunStatus: "Running"})
	if err != nil || len(byRun) != 1 || byRun[0].FeatureSlug != "demo-feature" {
		t.Fatalf("List by run status = %+v, %v", byRun, err)
	}
	byEither, err := s.Spine().List(SpineListFilter{Status: "Succeeded"})
	if err != nil || len(byEither) != 1 || byEither[0].FeatureSlug != "other-feature" {
		t.Fatalf("List by combined status = %+v, %v", byEither, err)
	}
	none, err := s.Spine().List(SpineListFilter{Status: "Blocked"})
	if err != nil || len(none) != 0 {
		t.Fatalf("List no-hit = %d, %v want 0", len(none), err)
	}
}

// AC-010: a blob whose content changed after registration is stale with
// reason hash_mismatch.
func TestArtifactViewHashMismatch(t *testing.T) {
	blob := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(blob, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte("v1"))
	a := ArtifactRef{RunID: "run_x", Kind: "log", AbsPath: blob, ContentHash: hex.EncodeToString(digest[:])}
	if v := ViewOf(a); v.Stale {
		t.Fatalf("fresh blob marked stale: %+v", v)
	}
	if err := os.WriteFile(blob, []byte("v2-changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := ViewOf(a); !v.Stale || v.StaleReason != "hash_mismatch" {
		t.Fatalf("changed blob = %+v want stale/hash_mismatch", v)
	}
}
