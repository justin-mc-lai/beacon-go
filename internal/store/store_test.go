package store

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "beacon.db")
	t.Setenv(EnvStorePath, path) // AC-061 isolation surface
	resolved, err := DefaultPath()
	if err != nil || resolved != path {
		t.Fatalf("DefaultPath with env = %q, %v want %q", resolved, err, path)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// seedFeature creates project -> current version -> feature package.
func seedFeature(t *testing.T, s *Store) (Project, ProjectVersion, FeaturePackage) {
	t.Helper()
	p, err := s.Projects().Register(Project{RootPath: filepath.Join(t.TempDir(), "proj"), Name: "demo"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	v, err := s.Projects().AddVersion(ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs/beacon/v2.0.0", IsCurrent: true})
	if err != nil {
		t.Fatalf("AddVersion: %v", err)
	}
	f, err := s.Features().CreatePackage(FeaturePackage{
		ProjectVersionID: v.ID, Slug: "demo-feature",
		TruthPath: "docs/beacon/v2.0.0/features/demo-feature/truth.md",
		TestsPath: "docs/beacon/v2.0.0/features/demo-feature/tests.md",
	})
	if err != nil {
		t.Fatalf("CreatePackage: %v", err)
	}
	return p, v, f
}

func TestOpenIdempotentAndWAL(t *testing.T) {
	s, path := openTestStore(t)
	if v, err := s.UserVersion(); err != nil || v != SchemaVersion {
		t.Fatalf("user_version = %d, %v want %d", v, err, SchemaVersion)
	}
	if mode, err := s.JournalMode(); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v want wal", mode, err)
	}
	if err := s.VerifySchemaContract(); err != nil {
		t.Fatalf("fresh schema must satisfy contract: %v", err)
	}
	s.Close()
	// Re-open: idempotent, version unchanged, no error.
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	defer s2.Close()
	if v, _ := s2.UserVersion(); v != SchemaVersion {
		t.Fatalf("user_version after re-open = %d", v)
	}
}

// AC-080: a DB stamped with a newer user_version is refused (no downgrade).
func TestDowngradeOpenForbidden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beacon.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t(x)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 99`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err = Open(path)
	if err == nil {
		t.Fatal("opening a newer-version DB must fail")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonDowngradeForbidden {
		t.Fatalf("reason = %q want %q (err=%v)", rc, ReasonDowngradeForbidden, err)
	}
}

// AC-031 cross-package consistency: the runs.status CHECK set stored in
// the DB equals core.AllRunStatuses exactly.
func TestRunsCheckMatchesCore(t *testing.T) {
	s, _ := openTestStore(t)
	var ddl string
	if err := s.DB().QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='runs'`).Scan(&ddl); err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`'([A-Za-z]+)'`).FindAllStringSubmatch(ddl, -1)
	var got []string
	for _, g := range m {
		got = append(got, g[1])
	}
	var want []string
	for _, st := range core.AllRunStatuses() {
		want = append(want, st.String())
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("runs CHECK set %v != core set %v", got, want)
	}
	// Behavioural: an invalid status is rejected by DAO validation and,
	// bypassing the DAO, by the DB CHECK itself.
	_, _, f := seedFeature(t, s)
	if _, err := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunStatus("Sleeping")}); err == nil {
		t.Error("DAO must reject invalid run status")
	}
	if _, err := s.DB().Exec(`INSERT INTO runs(id, feature_id, run_kind, status, iteration, started_at) VALUES('r-x', ?, 'goal', 'Sleeping', 0, 'now')`, f.ID); err == nil {
		t.Error("DB CHECK must reject invalid run status")
	}
}

// AC-005: no table may hold requirement truth body columns.
func TestNoTruthBodyColumns(t *testing.T) {
	s, _ := openTestStore(t)
	for table, cols := range SchemaContract {
		live, err := s.tableColumns(table)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range live {
			lc := strings.ToLower(c)
			if strings.Contains(lc, "truth_body") || lc == "body" || lc == "content" || strings.Contains(lc, "truth_content") {
				t.Errorf("table %s column %s looks like a truth body column (AC-005)", table, c)
			}
		}
		_ = cols
	}
}

// AC-067/068/069: project registration, single-current-version, slug
// uniqueness within one project version (but reusable across versions).
func TestProjectVersionFeatureConstraints(t *testing.T) {
	s, _ := openTestStore(t)
	p, err := s.Projects().Register(Project{RootPath: "/tmp/proj-a", Name: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Projects().Register(Project{RootPath: "/tmp/proj-a", Name: "dup"}); err == nil {
		t.Error("duplicate root_path must fail (UNIQUE)")
	}
	got, err := s.Projects().GetByRootPath("/tmp/proj-a")
	if err != nil || got.ID != p.ID {
		t.Fatalf("GetByRootPath = %+v, %v", got, err)
	}
	v1, _ := s.Projects().AddVersion(ProjectVersion{ProjectID: p.ID, Version: "v1", DocsRoot: "d1", IsCurrent: true})
	v2, _ := s.Projects().AddVersion(ProjectVersion{ProjectID: p.ID, Version: "v2", DocsRoot: "d2", IsCurrent: true})
	cur, err := s.Projects().CurrentVersion(p.ID)
	if err != nil || cur.ID != v2.ID {
		t.Fatalf("current after adding v2 as current = %+v, %v", cur, err)
	}
	if err := s.Projects().SetCurrent(v1.ID); err != nil {
		t.Fatal(err)
	}
	cur, _ = s.Projects().CurrentVersion(p.ID)
	if cur.ID != v1.ID {
		t.Fatalf("current after SetCurrent = %v want %v", cur.ID, v1.ID)
	}
	versions, _ := s.Projects().ListVersions(p.ID)
	currentCount := 0
	for _, v := range versions {
		if v.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Fatalf("exactly one current version required, got %d", currentCount)
	}
	// Slug uniqueness is scoped to one project version.
	if _, err := s.Features().CreatePackage(FeaturePackage{ProjectVersionID: v1.ID, Slug: "feat", TruthPath: "t", TestsPath: "te"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Features().CreatePackage(FeaturePackage{ProjectVersionID: v1.ID, Slug: "feat", TruthPath: "t", TestsPath: "te"}); err == nil {
		t.Error("duplicate slug in same version must fail")
	}
	if _, err := s.Features().CreatePackage(FeaturePackage{ProjectVersionID: v2.ID, Slug: "feat", TruthPath: "t", TestsPath: "te"}); err != nil {
		t.Errorf("same slug in another version must be allowed: %v", err)
	}
}

// AC-070/028/029 via DAO: lifecycle snapshot upsert validates through core.
func TestLifecycleSnapshotValidation(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	if err := s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageImplement, Status: core.StatusBlocked}); err == nil {
		t.Error("Blocked snapshot without reason codes must fail")
	}
	if err := s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageImplement, Status: core.StatusSkipped}); err == nil {
		t.Error("Skipped snapshot without deferral ref must fail (core rule; snapshot stores status only)")
	}
	err := s.Features().UpdateLifecycle(Lifecycle{
		FeatureID: f.ID, Stage: core.StageImplement, Status: core.StatusBlocked,
		ReasonCodes: []string{"truth_not_frozen"},
	})
	if err != nil {
		t.Fatalf("valid Blocked snapshot: %v", err)
	}
	lc, err := s.Features().GetLifecycle(f.ID)
	if err != nil || lc.Status != core.StatusBlocked || len(lc.ReasonCodes) != 1 {
		t.Fatalf("snapshot = %+v, %v", lc, err)
	}
	// Upsert moves the snapshot forward.
	if err := s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageImplement, Status: core.StatusInProgress}); err != nil {
		t.Fatal(err)
	}
	lc, _ = s.Features().GetLifecycle(f.ID)
	if lc.Status != core.StatusInProgress {
		t.Fatalf("snapshot after upsert = %v", lc.Status)
	}
}

// AC-012/072: lifecycle events are append-only — API has no update/delete
// and DB triggers reject direct UPDATE/DELETE.
func TestLifecycleEventsAppendOnly(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	e, err := s.AppendEvent(LifecycleEvent{FeatureID: f.ID, EventType: "stage_transition", Payload: json.RawMessage(`{"to":"Implement"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE lifecycle_events SET event_type='forged' WHERE id=?`, e.ID); err == nil {
		t.Error("UPDATE on lifecycle_events must be blocked by trigger")
	}
	if _, err := s.DB().Exec(`DELETE FROM lifecycle_events WHERE id=?`, e.ID); err == nil {
		t.Error("DELETE on lifecycle_events must be blocked by trigger")
	}
	events, err := s.ListEvents(f.ID)
	if err != nil || len(events) != 1 || events[0].EventType != "stage_transition" {
		t.Fatalf("events = %+v, %v", events, err)
	}
}

// Runs + AC-075 checkpoints: status updates validated; checkpoint
// iterations must strictly increase.
func TestRunsAndCheckpoints(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	r, err := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunCreated})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Runs().UpdateStatus(r.ID, core.RunRunning); err != nil {
		t.Fatal(err)
	}
	if err := s.Runs().UpdateStatus(r.ID, core.RunStatus("Nope")); err == nil {
		t.Error("invalid status update must fail")
	}
	got, _ := s.Runs().GetByID(r.ID)
	if got.Status != core.RunRunning {
		t.Fatalf("run status = %v", got.Status)
	}
	if _, err := s.Runs().SaveCheckpoint(Checkpoint{RunID: r.ID, Iteration: 1, State: json.RawMessage(`{"a":1}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Runs().SaveCheckpoint(Checkpoint{RunID: r.ID, Iteration: 1}); err == nil {
		t.Error("checkpoint iteration regression (same) must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonCheckpointRegression {
		t.Fatalf("reason = %q want %q", rc, ReasonCheckpointRegression)
	}
	if _, err := s.Runs().SaveCheckpoint(Checkpoint{RunID: r.ID, Iteration: 3}); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Runs().LatestCheckpoint(r.ID)
	if err != nil || latest.Iteration != 3 {
		t.Fatalf("latest checkpoint = %+v, %v", latest, err)
	}
}

// Gates: blocked verdict needs reason codes; Latest returns newest.
func TestGateDao(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	if _, err := s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "blocked"}); err == nil {
		t.Error("blocked gate without reason codes must fail")
	}
	if _, err := s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "fail", ReasonCodes: []string{"tc_failed"}, EvaluatedAt: "2026-10-21T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "pass", EvaluatedAt: "2026-10-21T01:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Gates().Latest(f.ID, "qa")
	if err != nil || latest.Verdict != "pass" {
		t.Fatalf("latest gate = %+v, %v", latest, err)
	}
}

// AC-010/083: artifact pointers require absolute paths; staleness is a
// hash comparison against the blob's actual hash.
func TestArtifactDaoAndStale(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	r, _ := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunRunning})
	if _, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "log", AbsPath: "relative/x.log", ContentHash: "abc"}); err == nil {
		t.Error("relative abs_path must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonArtifactPathNotAbs {
		t.Fatalf("reason = %q want %q", rc, ReasonArtifactPathNotAbs)
	}
	a, err := s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "log", AbsPath: "/tmp/proj/.beacon/logs/x.log", ContentHash: "abc123", SizeBytes: 42})
	if err != nil {
		t.Fatal(err)
	}
	if IsStale(a, "abc123") {
		t.Error("matching hash must not be stale")
	}
	if !IsStale(a, "different") {
		t.Error("mismatched hash must be stale (AC-010)")
	}
	byHash, err := s.Artifacts().ListByHash("abc123")
	if err != nil || len(byHash) != 1 {
		t.Fatalf("ListByHash = %v, %v", byHash, err)
	}
	byRun, _ := s.Artifacts().ListByRun(r.ID)
	if len(byRun) != 1 {
		t.Fatalf("ListByRun = %d want 1", len(byRun))
	}
}

// AC-081: Spine snapshot aggregates lifecycle + runs + gates + artifacts.
func TestSpineSnapshot(t *testing.T) {
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	_ = s.Features().UpdateLifecycle(Lifecycle{FeatureID: f.ID, Stage: core.StageQa, Status: core.StatusInProgress})
	r, _ := s.Runs().CreateRun(Run{FeatureID: f.ID, RunKind: "goal", Status: core.RunAwaitingQa})
	_, _ = s.Artifacts().Add(ArtifactRef{RunID: r.ID, Kind: "evidence", AbsPath: "/tmp/e.json", ContentHash: "h1"})
	_, _ = s.Gates().Record(GateResult{FeatureID: f.ID, GateType: "qa", Verdict: "blocked", ReasonCodes: []string{"evidence_missing"}, EvaluatedAt: "2026-10-21T02:00:00Z"})
	snap, err := s.Spine().Snapshot(f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Slug != "demo-feature" || snap.Stage != core.StageQa || snap.Status != core.StatusInProgress {
		t.Errorf("snapshot lifecycle = %+v", snap)
	}
	if snap.RunCount != 1 || snap.LatestRun == nil || snap.LatestRun.Status != core.RunAwaitingQa {
		t.Errorf("snapshot runs = %+v", snap)
	}
	if len(snap.LatestGates) != 1 || snap.LatestGates[0].Verdict != "blocked" {
		t.Errorf("snapshot gates = %+v", snap.LatestGates)
	}
	if snap.ArtifactCount != 1 {
		t.Errorf("snapshot artifacts = %d want 1", snap.ArtifactCount)
	}
}

// AC-158: write domain is bound to a track. A different track cannot open
// the DB for writing (fail-closed) but may open it read-only; write
// attempts on a read-only handle fail too.
func TestTrackWriteDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "beacon.db")
	a, err := OpenWithOptions(path, OpenOptions{Track: TrackV2Go})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Projects().Register(Project{RootPath: "/tmp/t1", Name: "t1"}); err != nil {
		t.Fatal(err)
	}
	a.Close()

	if _, err := OpenWithOptions(path, OpenOptions{Track: "track_v1_python"}); err == nil {
		t.Fatal("write open by another track must fail closed")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonDualTrackWriter {
		t.Fatalf("reason = %q want %q (err=%v)", rc, ReasonDualTrackWriter, err)
	}
	ro, err := OpenWithOptions(path, OpenOptions{Track: "track_v1_python", ReadOnly: true})
	if err != nil {
		t.Fatalf("read-only open by another track must be allowed: %v", err)
	}
	defer ro.Close()
	projects, err := ro.Projects().List()
	if err != nil || len(projects) != 1 {
		t.Fatalf("read-only List = %v, %v", projects, err)
	}
	if _, err := ro.Projects().Register(Project{RootPath: "/tmp/t2", Name: "t2"}); err == nil {
		t.Error("write on read-only handle must fail")
	} else if rc, _ := core.ReasonCodeOf(err); rc != ReasonStoreReadOnly {
		t.Fatalf("reason = %q want %q", rc, ReasonStoreReadOnly)
	}
	owner, err := ro.WriterTrack()
	if err != nil || owner != TrackV2Go {
		t.Fatalf("WriterTrack = %q, %v", owner, err)
	}
}

// AC-161: schema drift is detected in both directions — an added column
// and a dropped/renamed table — while the untouched schema passes.
func TestVerifySchemaContractDrift(t *testing.T) {
	s, path := openTestStore(t)
	if err := s.VerifySchemaContract(); err != nil {
		t.Fatalf("clean schema must pass: %v", err)
	}
	if _, err := s.DB().Exec(`ALTER TABLE projects ADD COLUMN surprise TEXT`); err != nil {
		t.Fatal(err)
	}
	err := s.VerifySchemaContract()
	if err == nil {
		t.Fatal("added column must be reported as drift")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != ReasonSchemaContractDrift {
		t.Fatalf("reason = %q want %q", rc, ReasonSchemaContractDrift)
	}
	s.Close()

	// Missing-column direction on a second DB: build schema, drop a table.
	s2, err := Open(filepath.Join(t.TempDir(), "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if _, err := s2.DB().Exec(`DROP TABLE checkpoints`); err != nil {
		t.Fatal(err)
	}
	if err := s2.VerifySchemaContract(); err == nil {
		t.Fatal("dropped table must be reported as drift")
	}
	_ = path
}

// Timestamps are compared as text by every reader (`ORDER BY created_at,
// rowid`), so the stored layout must be fixed width. time.RFC3339Nano
// trims trailing zeros: ".123Z" sorts after ".1234Z" and a whole-second
// "Z" sorts after both, which reorders an event trail.
func TestTimestampsSortChronologically(t *testing.T) {
	base := time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC)
	early := time.Date(2026, 10, 26, 9, 30, 0, 123000000, time.UTC) // .123
	late := time.Date(2026, 10, 26, 9, 30, 0, 123400000, time.UTC)  // .1234

	got := []string{tsFormat(base), tsFormat(early), tsFormat(late)}
	want := []string{
		"2026-10-26T09:30:00.000000000Z",
		"2026-10-26T09:30:00.123000000Z",
		"2026-10-26T09:30:00.123400000Z",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stamp[%d] = %q want %q", i, got[i], want[i])
		}
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("text order must match time order: %q then %q", got[i-1], got[i])
		}
	}

	// Stored events keep the same layout end to end.
	s, _ := openTestStore(t)
	_, _, f := seedFeature(t, s)
	var prev string
	for i := 0; i < 8; i++ {
		e, err := s.AppendEvent(LifecycleEvent{FeatureID: f.ID, EventType: "stage_executed"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := time.Parse(tsLayout, e.CreatedAt); err != nil {
			t.Fatalf("stored created_at %q is not tsLayout: %v", e.CreatedAt, err)
		}
		if prev != "" && e.CreatedAt < prev {
			t.Fatalf("event timestamps went backwards: %q then %q", prev, e.CreatedAt)
		}
		prev = e.CreatedAt
	}
}

// v1 ledger rows carry a fixed six-digit fraction with +00:00; v2 (after
// tsLayout) carries nine digits with Z. Both are fixed width, so text
// order stays time order when the two are mixed: the separator byte only
// decides between two spellings of the same instant, which is a tie and
// therefore allowed. This is the assumption AC-077 import relies on.
func TestMixedTimestampShapesKeepTimeOrder(t *testing.T) {
	texts := []string{
		"2026-03-29T11:48:00.123456+00:00", // v1 row
		"2026-03-29T11:48:00.123456000Z",   // v2 row, same instant
		"2026-03-29T11:48:00.123456789Z",   // v2 row, 789ns later
		"2026-03-29T11:48:00.648178+00:00", // v1 row
		"2026-03-29T11:48:01+00:00",        // v1 row, whole second
	}
	type row struct {
		text string
		at   time.Time
	}
	rows := make([]row, 0, len(texts))
	for _, s := range texts {
		at, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		rows = append(rows, row{s, at})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].text < rows[j].text })
	for i := 1; i < len(rows); i++ {
		if rows[i].at.Before(rows[i-1].at) {
			t.Fatalf("text order broke time order: %q (%s) before %q (%s)",
				rows[i-1].text, rows[i-1].at, rows[i].text, rows[i].at)
		}
	}
}

// AC-008/AC-061: the production default is the Go track's own ledger
// (beacon-go.db), not the v1 Python database (beacon.db). Opening the v1
// file by default would mix two schemas in one authority — the exact
// failure mode the isolated-ledger decision exists to prevent.
func TestDefaultPathIsGoTrackLedger(t *testing.T) {
	t.Setenv(EnvStorePath, "")
	p, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "beacon-go.db" {
		t.Fatalf("DefaultPath = %q want .../beacon-go.db (v1's beacon.db stays untouched)", p)
	}
}
