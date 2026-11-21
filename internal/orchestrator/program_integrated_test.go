// program-integrated-delivery (AC-001..003) behavior tests.
// These run against REAL git worktrees: the handoff, parallel
// dispatch and candidate approval only mean something when commits
// actually land on a main star.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/store"
)

func addIntegratedFeature(t *testing.T, env gitEnv, slug string) store.FeaturePackage {
	t.Helper()
	f, err := env.st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: env.feature.ProjectVersionID, Slug: slug,
		TruthPath: "docs/beacon/v2.0.0/features/" + slug + "/truth.md",
		TestsPath: "docs/beacon/v2.0.0/features/" + slug + "/tests.md",
		Status:    "frozen",
	})
	if err != nil {
		t.Fatal(err)
	}
	writePackage(t, env.root, f.TruthPath, slug, "frozen")
	for _, gt := range []string{"truth_freeze", "qa"} {
		if _, err := env.st.Gates().Record(store.GateResult{FeatureID: f.ID, GateType: gt, Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// setupIntegratedProgram builds a git env with one feature per lake,
// a frozen program and integrated mode enabled.
func setupIntegratedProgram(t *testing.T, lakes []ProgramLakeInput) (gitEnv, string) {
	t.Helper()
	env := newGitEnv(t)
	writePackage(t, env.root, "truth.md", "feat-a", "frozen")
	for _, gt := range []string{"truth_freeze", "qa"} {
		if _, err := env.st.Gates().Record(store.GateResult{FeatureID: env.feature.ID, GateType: gt, Verdict: "pass"}); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{"feat-a": true}
	for _, l := range lakes {
		if !seen[l.Slug] {
			seen[l.Slug] = true
			addIntegratedFeature(t, env, l.Slug)
		}
	}
	host := env.createRun(t)
	if _, err := env.eng.ProgramPlan(host.RunID, ProgramPlanInput{
		Slug: "sea", Freeze: true, Lakes: lakes,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramIntegratedEnable(host.RunID); err != nil {
		t.Fatal(err)
	}
	return env, host.RunID
}

func commitInWorktree(t *testing.T, path, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, path, "add", "-A")
	gitCmd(t, path, "commit", "-m", msg)
}

func lakeBranchTip(t *testing.T, env gitEnv, hostID, slug string) string {
	t.Helper()
	_, m, _, err := env.eng.loadRun(hostID)
	if err != nil {
		t.Fatal(err)
	}
	rec := dispatchForWorktree(m, slug)
	if rec == nil {
		t.Fatalf("no worktree record for %s", slug)
	}
	sha, err := harness.HeadCommit(context.Background(), rec.Path)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func mainTip(t *testing.T, env gitEnv) string {
	t.Helper()
	sha, err := harness.HeadCommit(context.Background(), env.root)
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

func attachLakeVerifier(t *testing.T, env gitEnv, hostID, slug string) {
	t.Helper()
	_, m, _, err := env.eng.loadRun(hostID)
	if err != nil {
		t.Fatal(err)
	}
	lake := m.Program.lake(slug)
	ref := filepath.Join(t.TempDir(), slug+"-qa.txt")
	if err := os.WriteFile(ref, []byte("independent qa passed for "+slug), 0o644); err != nil {
		t.Fatal(err)
	}
	prepareProductQA(t, env.testEnv, lake.RunID, "qa-bot")
}

func mustRunFeature(t *testing.T, env gitEnv, id string) string {
	r, err := env.st.Runs().GetByID(id)
	if err != nil {
		t.Fatal(err)
	}
	return r.FeatureID
}

// threeEntryRoutes returns route / single-step drive / continuous
// drive views so a test can prove all three share one judgement.
func threeEntryRoutes(t *testing.T, env gitEnv, hostID string) map[string]ProgramRouteView {
	t.Helper()
	out := map[string]ProgramRouteView{}
	if v, err := env.eng.ProgramIntegratedRoute(hostID); err != nil {
		t.Fatal(err)
	} else {
		out["route"] = v.Route
	}
	if v, err := env.eng.ProgramDrive(hostID, ProgramDriveOptions{Executor: "recording", MaxTurns: 1}); err != nil {
		t.Fatal(err)
	} else {
		out["drive_single"] = v.Route
	}
	if v, err := env.eng.ProgramDrive(hostID, ProgramDriveOptions{Executor: "recording", UntilDone: true, MaxDurationSec: 1, MaxTurns: 1}); err != nil {
		t.Fatal(err)
	} else {
		out["drive_continuous"] = v.Route
	}
	return out
}

func routeReason(v ProgramRouteView, slug string) string {
	for _, row := range v.Lakes {
		if row.Lake == slug {
			return row.ReasonCode
		}
	}
	return "MISSING"
}

// ---- AC-001 / TC-001 ----

func TestProgramIntegratedHandoff(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-a"},
		{Slug: "feat-b", DependsOn: []string{"feat-a"}},
		{Slug: "feat-c", DependsOn: []string{"feat-a"}},
		{Slug: "feat-d", DependsOn: []string{"feat-b", "feat-c"}},
	})

	disp, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if len(disp.Dispatched) != 1 || disp.Dispatched[0].Lake != "feat-a" {
		t.Fatalf("expected only feat-a dispatchable, got %+v", disp.Dispatched)
	}
	commitInWorktree(t, disp.Dispatched[0].WorktreePath, "a.txt", "from-a\n", "feat-a work")
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-a"); err != nil {
		t.Fatal(err)
	}

	// Unintegrated: the dependent must be blocked identically by all
	// three entries.
	for name, v := range threeEntryRoutes(t, env, host) {
		if rc := routeReason(v, "feat-b"); rc != core.ReasonProgramNotIntegrated {
			t.Fatalf("%s: feat-b reason = %q, want %s", name, rc, core.ReasonProgramNotIntegrated)
		}
	}

	src := lakeBranchTip(t, env, host, "feat-a")
	main := mainTip(t, env)
	attachLakeVerifier(t, env, host, "feat-a")
	if _, err := env.eng.ProgramIntegratedHandoff(ctx, host, "feat-a", strings.Repeat("0", len(src)), main, nil); reasonOf(err) != core.ReasonProgramCommitMismatch {
		t.Fatalf("wrong source SHA err = %v", err)
	}
	// A refused handoff records nothing: every entry still blocks.
	for name, v := range threeEntryRoutes(t, env, host) {
		if rc := routeReason(v, "feat-b"); rc != core.ReasonProgramNotIntegrated {
			t.Fatalf("%s after refused handoff: feat-b reason = %q", name, rc)
		}
	}

	h, err := env.eng.ProgramIntegratedHandoff(ctx, host, "feat-a", src, main, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !h.Valid || h.SourceSHA != src || h.IntegrationSHA != main || h.ContractFP == "" {
		t.Fatalf("handoff = %+v", h)
	}
	// The dependent's baseline contains the upstream verified result.
	body := gitCmd(t, env.root, "show", "main:a.txt")
	if !strings.Contains(body, "from-a") {
		t.Fatalf("main star does not contain feat-a's verified work: %q", body)
	}
	// feat-b is now startable through every entry; feat-d still waits.
	for name, v := range threeEntryRoutes(t, env, host) {
		if rc := routeReason(v, "feat-b"); rc == core.ReasonProgramNotIntegrated {
			t.Fatalf("%s: feat-b still unintegrated after a valid handoff", name)
		}
		if rc := routeReason(v, "feat-d"); rc != core.ReasonProgramNotIntegrated {
			t.Fatalf("%s: feat-d reason = %q, want not_integrated", name, rc)
		}
	}

	// TC-001 also proves "交接后证据失效": a recorded handoff whose QA
	// digest or SHA no longer holds is refused identically by route,
	// single-step drive and continuous drive.
	t.Run("handoff_evidence_invalidation", runHandoffTamperRejected)
}

// ---- AC-002 / TC-002 ----

func TestProgramIntegratedParallel(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-a", Touches: []string{"area-a"}},
		{Slug: "feat-b", DependsOn: []string{"feat-a"}, Touches: []string{"area-b"}},
		{Slug: "feat-c", DependsOn: []string{"feat-a"}, Touches: []string{"area-c"}},
		{Slug: "feat-d", DependsOn: []string{"feat-b", "feat-c"}, Touches: []string{"area-d"}},
		{Slug: "feat-e"}, // unknown change surface -> never parallel
		{Slug: "feat-f", DependsOn: []string{"feat-a"}, Touches: []string{"area-b"}}, // shares B's surface -> serial
	})

	d1, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	commitInWorktree(t, d1.Dispatched[0].WorktreePath, "a.txt", "A\n", "feat-a")
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-a"); err != nil {
		t.Fatal(err)
	}
	attachLakeVerifier(t, env, host, "feat-a")
	if _, err := env.eng.ProgramIntegratedHandoff(ctx, host, "feat-a",
		lakeBranchTip(t, env, host, "feat-a"), mainTip(t, env), nil); err != nil {
		t.Fatal(err)
	}

	// B and C are disjoint -> one parallel batch on separate
	// workstations. E's surface is unknown, so it stays serial.
	d2, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if !d2.Parallel || len(d2.Dispatched) != 2 {
		t.Fatalf("expected a parallel batch of 2, got parallel=%v dispatched=%+v", d2.Parallel, d2.Dispatched)
	}
	paths := map[string]string{}
	for _, d := range d2.Dispatched {
		if d.Lake != "feat-b" && d.Lake != "feat-c" {
			t.Fatalf("unexpected lake in parallel batch: %s", d.Lake)
		}
		if d.WorktreePath == "" || d.Batch != d2.Batch || d.Status != "active" {
			t.Fatalf("dispatch row = %+v", d)
		}
		paths[d.Lake] = d.WorktreePath
		if !pathExists(d.WorktreePath) {
			t.Fatalf("worktree missing for %s: %s", d.Lake, d.WorktreePath)
		}
		run, err := env.st.Runs().GetByID(d.RunID)
		if err != nil || run.Status != core.RunRunning {
			t.Fatalf("%s run not running: %+v %v", d.Lake, run, err)
		}
	}
	if paths["feat-b"] == paths["feat-c"] {
		t.Fatal("B and C must start on different workstations")
	}
	if !containsStr(d2.Serial, "feat-e") {
		t.Fatalf("unknown change surface must be serial, serial=%v", d2.Serial)
	}
	if !containsStr(d2.Serial, "feat-f") || containsLake(d2.Dispatched, "feat-f") {
		t.Fatalf("lake sharing an active surface must stay serial, got %+v", d2)
	}
	// AC-002 时间重叠证据 (A): both disjoint lakes are live at the
	// SAME instant right now — concurrency count >= 2, not just "both
	// rows say active".
	if got := activeDispatchCount(t, env, host, time.Now().UTC()); got < 2 {
		t.Fatalf("disjoint lakes are not concurrently active: %d", got)
	}
	bStart, cStart := dispatchStart(t, env, host, "feat-b"), dispatchStart(t, env, host, "feat-c")
	if bStart.IsZero() || cStart.IsZero() {
		t.Fatal("dispatched lakes must record started_at")
	}
	if dispatchRecord(t, env, host, "feat-b").FinishedAt != "" || dispatchRecord(t, env, host, "feat-c").FinishedAt != "" {
		t.Fatal("active lakes must not have finished_at yet")
	}

	// Conflicting commits: B lands, C conflicts, D stays blocked.
	commitInWorktree(t, paths["feat-b"], "shared.txt", "B version\n", "B")
	commitInWorktree(t, paths["feat-c"], "shared.txt", "C version\n", "C")
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-c"); reasonOf(err) != core.ReasonWorktreeMergeConflict {
		t.Fatalf("C conflict err = %v", err)
	}
	// AC-002 时间重叠证据 (A, closed intervals): once both lakes reach
	// their merge attempt, the stored [started_at, finished_at]
	// intervals of the two disjoint lakes genuinely overlap.
	bRec, cRec := dispatchRecord(t, env, host, "feat-b"), dispatchRecord(t, env, host, "feat-c")
	bFin, cFin := mustParseISO(t, bRec.FinishedAt), mustParseISO(t, cRec.FinishedAt)
	if !bStart.Before(cFin) || !cStart.Before(bFin) {
		t.Fatalf("disjoint lake intervals do not overlap: B=[%s,%s] C=[%s,%s]",
			bStart, bFin, cStart, cFin)
	}
	t.Logf("AC-002 overlap proven: B=[%s,%s] C=[%s,%s] (disjoint lakes, same batch, concurrent>=2)",
		bStart.Format(time.RFC3339Nano), bFin.Format(time.RFC3339Nano),
		cStart.Format(time.RFC3339Nano), cFin.Format(time.RFC3339Nano))
	route, err := env.eng.ProgramIntegratedRoute(host)
	if err != nil {
		t.Fatal(err)
	}
	if route.Blocked["feat-d"] != core.ReasonProgramNotIntegrated {
		t.Fatalf("conflict must keep D blocked: %+v", route.Blocked)
	}

	// Interruption recovery: an existing binding is reused, never
	// dispatched twice; an unrelated unknown-surface lake still
	// proceeds (AC-002). A lone serial lake reuses the main
	// workstation, so no satellite is created for it.
	before, err := env.st.Runs().ListByFeature(mustFeatureID(t, env, "feat-b"))
	if err != nil {
		t.Fatal(err)
	}
	d3, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(d3.Recovered, "feat-b") || !containsStr(d3.Recovered, "feat-c") {
		t.Fatalf("recovery must report existing bindings: %+v", d3)
	}
	after, err := env.st.Runs().ListByFeature(mustFeatureID(t, env, "feat-b"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("recovery created a duplicate run: before=%d after=%d", len(before), len(after))
	}
	if got := worktreeRecordCount(t, env, host, "feat-b"); got != 1 {
		t.Fatalf("recovery created a duplicate worktree record: %d", got)
	}
	if !containsLake(d3.Dispatched, "feat-e") {
		t.Fatalf("unrelated lake must still be able to proceed: %+v", d3.Dispatched)
	}
	// 单湖复用主工位 (AC-002): the lone serial lake works on the host
	// workstation, not a fresh satellite.
	eRec := dispatchForWorktree(mustMeta(t, env, host), "feat-e")
	if eRec == nil {
		t.Fatal("no binding for the lone serial lake")
	}
	gotRoot, _ := filepath.EvalSymlinks(eRec.Path)
	wantRoot, _ := filepath.EvalSymlinks(env.root)
	if gotRoot != wantRoot {
		t.Fatalf("lone lake must reuse the main workstation, got %s (root %s)", eRec.Path, env.root)
	}
	if got := worktreeRecordCount(t, env, host, "feat-e"); got != 1 {
		t.Fatalf("lone lake binding duplicated its record: %d", got)
	}
	if got := runCountFor(t, env, "feat-e"); got != 1 {
		t.Fatalf("lone lake duplicated its run: %d", got)
	}
	if got := dispatchCount(t, env, host, "feat-e"); got != 1 {
		t.Fatalf("lone lake re-dispatched: %d", got)
	}
	t.Logf("AC-002 单湖复用主工位: path=%s branch=%s worktrees=1 runs=1 dispatches=1",
		eRec.Path, eRec.Branch)

	// Shared-surface lake stays serial: it only starts after B (its
	// surface peer) already finished, so their intervals cannot
	// overlap.
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-e"); err != nil {
		t.Fatal(err)
	}
	d4, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if !containsLake(d4.Dispatched, "feat-f") {
		t.Fatalf("shared-surface lake must proceed serially: %+v", d4)
	}
	fStart := dispatchStart(t, env, host, "feat-f")
	if fStart.Before(bFin) {
		t.Fatalf("serial lake started inside B's active interval: F=%s B finished=%s", fStart, bFin)
	}
	t.Logf("AC-002 serial proven: shared-surface F started %s after B finished %s (no overlap)",
		fStart.Format(time.RFC3339Nano), bFin.Format(time.RFC3339Nano))

	// TC-002's required behaviours, each on its own real workspace:
	//  - two REAL subprocess workers meet at a barrier (real overlap);
	//  - a deleted satellite is rebuilt, a dirty/leftover directory is
	//    refused and its uncommitted files survive;
	//  - unknown/shared surfaces stay serial across batches (above).
	t.Run("two_real_workers_barrier", runWorkersBarrier)
	t.Run("worktree_recovery_no_data_loss", runWorktreeRecoveryNoDataLoss)
	t.Run("dirty_leftover_preserved", runDirtyLeftoverPreserved)
}

func worktreeListed(t *testing.T, root, path string) bool {
	t.Helper()
	list, err := harness.WorktreeList(context.Background(), root)
	if err != nil {
		t.Fatalf("worktree list: %v", err)
	}
	want, werr := filepath.EvalSymlinks(path)
	if werr != nil {
		want = filepath.Clean(path)
	}
	for p := range list {
		got, gerr := filepath.EvalSymlinks(p)
		if gerr != nil {
			got = filepath.Clean(p)
		}
		if got == want {
			return true
		}
	}
	return false
}

func eventSeen(t *testing.T, env gitEnv, runID, eventType string) bool {
	t.Helper()
	events, err := env.st.ListEventsByRun(runID)
	if err != nil {
		t.Fatal(err)
	}
	for _, ev := range events {
		if ev.EventType == eventType {
			return true
		}
	}
	return false
}

func mustFeatureID(t *testing.T, env gitEnv, slug string) string {
	t.Helper()
	f, err := env.st.Features().GetBySlug(env.feature.ProjectVersionID, slug)
	if err != nil {
		t.Fatal(err)
	}
	return f.ID
}

func worktreeRecordCount(t *testing.T, env gitEnv, hostID, slug string) int {
	t.Helper()
	_, m, _, err := env.eng.loadRun(hostID)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	if m.Implement != nil && m.Implement.Worktrees != nil {
		for _, r := range m.Implement.Worktrees.Records {
			if r.Lake == slug {
				n++
			}
		}
	}
	return n
}

// ---- AC-003 / TC-003 ----

func TestProgramIntegratedApproval(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-a", Touches: []string{"area-a"}},
		{Slug: "feat-b", Touches: []string{"area-b"}},
	})

	d, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Dispatched) != 2 {
		t.Fatalf("both lakes must dispatch: %+v", d.Dispatched)
	}
	for _, row := range d.Dispatched {
		commitInWorktree(t, row.WorktreePath, row.Lake+".txt", row.Lake+"\n", row.Lake)
	}
	for _, slug := range []string{"feat-a", "feat-b"} {
		if _, err := env.eng.ProgramIntegratedMerge(ctx, host, slug); err != nil {
			t.Fatal(err)
		}
	}
	_, m, _, err := env.eng.loadRun(host)
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"feat-a", "feat-b"} {
		setRunStage(t, env.testEnv, m.Program.lake(slug).RunID, string(core.LoopReleaseGate))
		attachLakeVerifier(t, env, host, slug)
		if _, err := env.eng.ProgramIntegratedHandoff(ctx, host, slug,
			lakeBranchTip(t, env, host, slug), mainTip(t, env), nil); err != nil {
			t.Fatal(err)
		}
	}

	// Total QA that fails binds no candidate.
	if _, err := env.eng.ProgramIntegratedQA(ctx, host, "fail", []string{"qa/total.txt"}, "qa-bot"); reasonOf(err) != core.ReasonProgramCandidateNotReady {
		t.Fatalf("failing total QA err = %v", err)
	}
	if _, err := env.eng.ProgramIntegratedQA(ctx, host, "pass", []string{"qa/total.txt"}, "qa-bot"); err == nil {
		t.Fatal("missing total QA file must be refused")
	}
	if err := os.MkdirAll(filepath.Join(env.root, "qa"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"total.txt", "total-2.txt"} {
		if err := os.WriteFile(filepath.Join(env.root, "qa", name), []byte("fixture total QA evidence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	v, err := env.eng.ProgramIntegratedQA(ctx, host, "pass", []string{"qa/total.txt"}, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if v.Stage != ProgramStageAwaitingApproval || v.Candidate == nil || v.Candidate.SHA != mainTip(t, env) {
		t.Fatalf("candidate = %+v stage=%s", v.Candidate, v.Stage)
	}
	candidateSHA, digest := v.Candidate.SHA, v.Candidate.EvidenceDigest

	// The single-lake legacy entry cannot bypass the total signature.
	if _, err := env.eng.ProgramComplete(host, "lai"); reasonOf(err) != core.ReasonProgramApprovalRequired {
		t.Fatalf("legacy complete must be refused in integrated mode: %v", err)
	}
	// A signature that does not match the bound digest is refused.
	if _, err := env.eng.ProgramIntegratedApprove(ctx, host, candidateSHA, "wrong-digest", "lai"); reasonOf(err) != core.ReasonProgramApprovalInvalid {
		t.Fatalf("bad digest err = %v", err)
	}

	// Candidate change: a new main-star commit invalidates the
	// candidate and forces re-verification and a new signature.
	if err := os.WriteFile(filepath.Join(env.root, "late.txt"), []byte("late\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, env.root, "add", "-A")
	gitCmd(t, env.root, "commit", "-m", "late change")
	if _, err := env.eng.ProgramIntegratedApprove(ctx, host, candidateSHA, digest, "lai"); reasonOf(err) != core.ReasonProgramCandidateChanged {
		t.Fatalf("changed candidate err = %v", err)
	}
	v, err = env.eng.ProgramIntegratedQA(ctx, host, "pass", []string{"qa/total-2.txt"}, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	candidateSHA, digest = v.Candidate.SHA, v.Candidate.EvidenceDigest

	// The one signature binds the persisted candidate and enters
	// finalizing.
	if _, err := env.eng.ProgramIntegratedApprove(ctx, host, candidateSHA, digest, "lai"); err != nil {
		t.Fatal(err)
	}
	// This legacy unit fixture has no real implement gate. It must not
	// manufacture a release PASS. The CLI diamond proves positive closure.
	if _, err := env.eng.ProgramIntegratedFinalize(ctx, host); err == nil {
		t.Fatal("finalize must preserve the formal release gate")
	}
}

func mustMeta(t *testing.T, env gitEnv, hostID string) *runMeta {
	t.Helper()
	_, m, _, err := env.eng.loadRun(hostID)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func dispatchRecord(t *testing.T, env gitEnv, hostID, lake string) ProgramDispatch {
	t.Helper()
	d := mustMeta(t, env, hostID).Program.dispatchFor(lake)
	if d == nil {
		t.Fatalf("no dispatch record for %s", lake)
	}
	return *d
}

func dispatchCount(t *testing.T, env gitEnv, hostID, lake string) int {
	t.Helper()
	n := 0
	for _, d := range mustMeta(t, env, hostID).Program.Dispatches {
		if d.Lake == lake {
			n++
		}
	}
	return n
}

func dispatchStart(t *testing.T, env gitEnv, hostID, lake string) time.Time {
	t.Helper()
	return mustParseISO(t, dispatchRecord(t, env, hostID, lake).StartedAt)
}

func mustParseISO(t *testing.T, s string) time.Time {
	t.Helper()
	if s == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("bad timestamp %q: %v", s, err)
	}
	return ts
}

// activeDispatchCount counts lakes whose stored active interval covers at.
func activeDispatchCount(t *testing.T, env gitEnv, hostID string, at time.Time) int {
	t.Helper()
	n := 0
	for _, d := range mustMeta(t, env, hostID).Program.Dispatches {
		start, ok := parseISO(d.StartedAt)
		if !ok || d.Status != "active" {
			continue
		}
		end := at
		if d.FinishedAt != "" {
			if e, ok := parseISO(d.FinishedAt); ok {
				end = e
			}
		}
		if !start.After(at) && !at.After(end) {
			n++
		}
	}
	return n
}

func parseISO(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func runCountFor(t *testing.T, env gitEnv, slug string) int {
	t.Helper()
	runs, err := env.st.Runs().ListByFeature(mustFeatureID(t, env, slug))
	if err != nil {
		t.Fatal(err)
	}
	return len(runs)
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func containsLake(list []ProgramDispatch, want string) bool {
	for _, d := range list {
		if d.Lake == want {
			return true
		}
	}
	return false
}
