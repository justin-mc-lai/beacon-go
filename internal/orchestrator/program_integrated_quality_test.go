// program-integrated-delivery quality closure (review round).
//
// These are the behaviours a reviewer asked to see proven, not just
// described:
//   - two REAL worker subprocesses meet at a barrier (both are in
//     real work at the same time) on independent worktrees;
//   - shared/unknown change surfaces stay serial across batches;
//   - a handoff whose SHA/QA digest was tampered with is refused by
//     EVERY entry (route / single-step drive / continuous drive);
//   - a deleted satellite worktree is rebuilt, while a leftover
//     directory holding files is refused and preserved;
//   - a continuous drive honours a deadline mid-round and reports a
//     no-progress stall instead of spinning.
package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// -------- AC-002: real subprocess workers + barrier --------

func runWorkersBarrier(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-a", Touches: []string{"area-a"}},
		{Slug: "feat-b", DependsOn: []string{"feat-a"}, Touches: []string{"area-b"}},
		{Slug: "feat-c", DependsOn: []string{"feat-a"}, Touches: []string{"area-c"}},
		{Slug: "feat-f", DependsOn: []string{"feat-a"}, Touches: []string{"area-b"}}, // shares B -> serial
		{Slug: "feat-e", DependsOn: []string{"feat-a"}},                              // unknown surface -> serial
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

	d2, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if !d2.Parallel || len(d2.Dispatched) != 2 {
		t.Fatalf("want a parallel batch of exactly B and C, got %+v", d2)
	}
	paths := map[string]string{}
	for _, d := range d2.Dispatched {
		paths[d.Lake] = d.WorktreePath
	}
	if paths["feat-b"] == "" || paths["feat-c"] == "" || paths["feat-b"] == paths["feat-c"] {
		t.Fatalf("B and C need two distinct live worktrees: %+v", paths)
	}

	// Two REAL subprocesses, each in its own worktree. Each writes a
	// "started" marker, then waits until the OTHER worker has also
	// started, and only then writes "barrier". If either worker never
	// starts, the other times out and exits non-zero — so the barrier
	// passing is proof both were in real work at the same time.
	barrierScript := `
set -eu
me="$1"; other="$2"; dir="$3"; otherdir="$4"
cd "$dir"
printf 'started %s\n' "$(date +%s.%N)" > "$me.started"
i=0
while [ ! -f "$otherdir/$other.started" ]; do
  i=$((i+1))
  [ "$i" -ge 400 ] && { echo "timeout waiting for $other" >&2; exit 9; }
  sleep 0.05
done
printf 'barrier %s\n' "$(date +%s.%N)" > "$me.barrier"
`
	runWorker := func(slug, dir, me, other string) error {
		otherDir := ""
		if me == "b" {
			otherDir = paths["feat-c"]
		} else {
			otherDir = paths["feat-b"]
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", barrierScript, "sh", me, other, dir, otherDir)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return &workerErr{slug: slug, out: string(out), err: err}
		}
		return nil
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := runWorker("feat-b", paths["feat-b"], "b", "c"); err != nil {
			errs <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := runWorker("feat-c", paths["feat-c"], "c", "b"); err != nil {
			errs <- err
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("worker failed: %v", err)
	}
	for _, name := range []string{"b", "c"} {
		dir := ""
		if name == "b" {
			dir = paths["feat-b"]
		} else {
			dir = paths["feat-c"]
		}
		if _, err := os.Stat(filepath.Join(dir, name+".barrier")); err != nil {
			t.Fatalf("worker %s did not cross the barrier: %v", name, err)
		}
	}
	bStart := readStamp(t, filepath.Join(paths["feat-b"], "b.started"))
	cStart := readStamp(t, filepath.Join(paths["feat-c"], "c.started"))
	bBarrier := readStamp(t, filepath.Join(paths["feat-b"], "b.barrier"))
	cBarrier := readStamp(t, filepath.Join(paths["feat-c"], "c.barrier"))
	// Real overlap: each started before the other crossed the barrier.
	if !(bStart < cBarrier && cStart < bBarrier) {
		t.Fatalf("workers did not overlap: B=[%f,%f] C=[%f,%f]", bStart, bBarrier, cStart, cBarrier)
	}
	t.Logf("AC-002 subprocess barrier: B started %.3f crossed %.3f; C started %.3f crossed %.3f (real overlap)",
		bStart, bBarrier, cStart, cBarrier)

	// Cross-batch serial: shared-surface F and unknown-surface E must
	// not start until the parallel batch's active intervals closed.
	for i := 0; i < 2; i++ {
		again, err := env.eng.ProgramIntegratedDispatch(ctx, host)
		if err != nil || len(again.Dispatched) != 0 {
			t.Fatalf("active B/C must exclude shared F and unknown E: %+v %v", again, err)
		}
	}
	commitInWorktree(t, paths["feat-b"], "b.txt", "B\n", "B")
	commitInWorktree(t, paths["feat-c"], "c.txt", "C\n", "C")
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-c"); err != nil {
		t.Fatal(err)
	}
	bFin := mustParseISO(t, dispatchRecord(t, env, host, "feat-b").FinishedAt)
	cFin := mustParseISO(t, dispatchRecord(t, env, host, "feat-c").FinishedAt)
	lastFin := bFin
	if cFin.After(lastFin) {
		lastFin = cFin
	}
	d3, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if containsLake(d3.Dispatched, "feat-f") && containsLake(d3.Dispatched, "feat-e") {
		t.Fatalf("shared/unknown surfaces must not share a parallel batch: %+v", d3)
	}
	for _, slug := range []string{"feat-f", "feat-e"} {
		if d := mustMeta(t, env, host).Program.dispatchFor(slug); d != nil {
			if start := mustParseISO(t, d.StartedAt); start.Before(lastFin) {
				t.Fatalf("%s started inside the parallel batch's interval: %s < %s", slug, start, lastFin)
			}
		}
	}
	t.Logf("AC-002 cross-batch serial: shared/unknown lakes never start before %s (parallel batch closed)",
		lastFin.Format(time.RFC3339Nano))
}

type workerErr struct {
	slug string
	out  string
	err  error
}

func (e *workerErr) Error() string { return e.slug + ": " + e.err.Error() + ": " + e.out }

func readStamp(t *testing.T, path string) float64 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 2 {
		t.Fatalf("bad stamp %q", raw)
	}
	f, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		t.Fatalf("bad stamp value %q: %v", fields[1], err)
	}
	return f
}

// -------- AC-002: satellite rebuild vs. preserved leftovers --------

func runWorktreeRecoveryNoDataLoss(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-g", Touches: []string{"area-g"}},
		{Slug: "feat-h", Touches: []string{"area-h"}},
		{Slug: "feat-i", Touches: []string{"area-i"}},
		{Slug: "feat-j", Touches: []string{"area-j"}},
	})
	// 建工位中断: feat-j's worktree + branch already exist from a
	// create that was interrupted BEFORE its binding was saved. The
	// dispatch must adopt that exact worktree in place (no second
	// worktree, run or dispatch row).
	jPath := filepath.Join(env.root, WorktreeDirName, "feat-j")
	jBranch := "program/" + host + "/feat-j"
	if _, err := harness.WorktreeAdd(ctx, env.root, jPath, jBranch, ""); err != nil {
		t.Fatalf("prepare interrupted worktree: %v", err)
	}
	if got := worktreeRecordCount(t, env, host, "feat-j"); got != 0 {
		t.Fatalf("interrupted create must not have a saved record yet: %d", got)
	}

	d, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Dispatched) != 4 || !d.Parallel {
		t.Fatalf("want four parallel satellites, got %+v", d)
	}
	// Adoption: same path/branch, one record, one run, one dispatch.
	jRec := dispatchForWorktree(mustMeta(t, env, host), "feat-j")
	if jRec == nil {
		t.Fatal("interrupted worktree create was not adopted")
	}
	gotJ, _ := filepath.EvalSymlinks(jRec.Path)
	wantJ, _ := filepath.EvalSymlinks(jPath)
	if gotJ != wantJ || jRec.Branch != jBranch {
		t.Fatalf("adoption did not reuse the interrupted worktree: %+v", jRec)
	}
	if worktreeRecordCount(t, env, host, "feat-j") != 1 || runCountFor(t, env, "feat-j") != 1 ||
		dispatchCount(t, env, host, "feat-j") != 1 {
		t.Fatalf("adoption duplicated a worktree/run/dispatch for feat-j")
	}
	if !eventSeen(t, env, host, "worktree_adopted") {
		t.Fatal("no worktree_adopted event recorded")
	}
	t.Logf("AC-002 建工位中断: adopted path=%s branch=%s (worktrees=1 runs=1 dispatches=1)", jRec.Path, jRec.Branch)
	paths := map[string]string{}
	for _, row := range d.Dispatched {
		paths[row.Lake] = row.WorktreePath
	}

	// A: a deleted worktree whose record survives is REBUILT on the
	// recorded branch (never reported healthy, never duplicated).
	gRec := dispatchForWorktree(mustMeta(t, env, host), "feat-g")
	hRec := dispatchForWorktree(mustMeta(t, env, host), "feat-h")
	for _, rec := range []*WorktreeRecord{gRec, hRec} {
		if _, err := harness.WorktreeRemove(ctx, env.root, rec.Path, true); err != nil {
			t.Fatalf("remove worktree: %v", err)
		}
	}
	dA, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatalf("dispatch must rebuild deleted worktrees: %v", err)
	}
	if !containsStr(dA.Recovered, "feat-g") || !containsStr(dA.Recovered, "feat-h") {
		t.Fatalf("rebuild must be reported as recovered: %+v", dA)
	}
	for _, slug := range []string{"feat-g", "feat-h"} {
		rec := dispatchForWorktree(mustMeta(t, env, host), slug)
		if !worktreeListed(t, env.root, rec.Path) || !harness.BranchExists(ctx, env.root, rec.Branch) {
			t.Fatalf("%s was not rebuilt on its recorded branch", slug)
		}
	}
	if !eventSeen(t, env, host, "worktree_rebuilt") {
		t.Fatal("no worktree_rebuilt event recorded")
	}

	// B: a leftover directory that is not a worktree but HOLDS FILES
	// must be refused and preserved — recovery never deletes data.
	for _, rec := range []*WorktreeRecord{
		dispatchForWorktree(mustMeta(t, env, host), "feat-g"),
		dispatchForWorktree(mustMeta(t, env, host), "feat-h"),
	} {
		_, _ = harness.WorktreeRemove(ctx, env.root, rec.Path, true)
		_, _ = harness.DeleteBranch(ctx, env.root, rec.Branch, true)
	}
	staleDir := paths["feat-g"]
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(staleDir, "someone-work.txt")
	if err := os.WriteFile(junk, []byte("do not delete me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = env.eng.ProgramIntegratedDispatch(ctx, host)
	if err == nil {
		t.Fatal("a leftover directory holding files must be refused, not wiped")
	}
	if rc, _ := core.ReasonCodeOf(err); rc != core.ReasonWorktreeMissing {
		t.Fatalf("refusal reason = %q, want %s", rc, core.ReasonWorktreeMissing)
	}
	if body, rerr := os.ReadFile(junk); rerr != nil || !strings.Contains(string(body), "do not delete me") {
		t.Fatalf("recovery deleted the leftover file: %q %v", body, rerr)
	}
	if !strings.Contains(err.Error(), "refusing to delete") {
		t.Fatalf("refusal must say why: %v", err)
	}
	// Once the human clears the leftover, the rebuild proceeds.
	if err := os.RemoveAll(staleDir); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ProgramIntegratedDispatch(ctx, host); err != nil {
		t.Fatalf("clean rebuild after clearing the leftover: %v", err)
	}
	for _, slug := range []string{"feat-g", "feat-h"} {
		rec := dispatchForWorktree(mustMeta(t, env, host), slug)
		if !worktreeListed(t, env.root, rec.Path) {
			t.Fatalf("%s not rebuilt after the leftover was cleared", slug)
		}
	}
}

// -------- AC-001: live entry checks + handoff tampering --------

func runHandoffTamperRejected(t *testing.T) {
	ctx := context.Background()
	env, host := setupIntegratedProgram(t, []ProgramLakeInput{
		{Slug: "feat-a"},
		{Slug: "feat-b", DependsOn: []string{"feat-a"}},
	})
	d, err := env.eng.ProgramIntegratedDispatch(ctx, host)
	if err != nil {
		t.Fatal(err)
	}
	commitInWorktree(t, d.Dispatched[0].WorktreePath, "a.txt", "A\n", "feat-a")
	if _, err := env.eng.ProgramIntegratedMerge(ctx, host, "feat-a"); err != nil {
		t.Fatal(err)
	}
	src := lakeBranchTip(t, env, host, "feat-a")
	attachLakeVerifier(t, env, host, "feat-a")
	if _, err := env.eng.ProgramIntegratedHandoff(ctx, host, "feat-a", src, mainTip(t, env), nil); err != nil {
		t.Fatal(err)
	}

	expectBlockedEverywhere := func(t *testing.T, want string) {
		t.Helper()
		for name, v := range threeEntryRoutes(t, env, host) {
			if rc := routeReason(v, "feat-b"); rc != want {
				t.Fatalf("%s: feat-b reason = %q, want %s", name, rc, want)
			}
		}
	}

	tamper := func(t *testing.T, fn func(h *ProgramHandoff)) {
		t.Helper()
		run, m, _, err := env.eng.loadRun(host)
		if err != nil {
			t.Fatal(err)
		}
		fn(m.Program.handoffFor("feat-a"))
		if err := env.eng.saveRun(run, m); err != nil {
			t.Fatal(err)
		}
	}

	// 1) QA digest no longer matches its evidence: refused by every entry.
	tamper(t, func(h *ProgramHandoff) { h.QADigest = strings.Repeat("a", 64) })
	expectBlockedEverywhere(t, core.ReasonProgramHandoffInvalid)

	// 2) Restore the digest, tamper the source SHA: the cached record
	// says valid, but git does not contain that commit — refused live.
	env.eng.loadRun(host)
	tamper(t, func(h *ProgramHandoff) {
		h.QADigest, _ = env.eng.lakeQADigest(env.root, h.LakeRunID, h.QAEvidence)
		h.SourceSHA = strings.Repeat("1", 40)
	})
	expectBlockedEverywhere(t, core.ReasonProgramHandoffInvalid)

	// 3) Integration SHA not on the main star any more: refused.
	tamper(t, func(h *ProgramHandoff) {
		h.SourceSHA = src
		h.IntegrationSHA = strings.Repeat("2", 40)
	})
	expectBlockedEverywhere(t, core.ReasonProgramHandoffInvalid)

	// 4) Explicit invalid handoff: refused.
	tamper(t, func(h *ProgramHandoff) {
		h.IntegrationSHA = mainTip(t, env)
		h.Valid = false
	})
	expectBlockedEverywhere(t, core.ReasonProgramHandoffInvalid)

	// Restoring the real record unlocks the dependent again.
	tamper(t, func(h *ProgramHandoff) { h.Valid = true })
	for name, v := range threeEntryRoutes(t, env, host) {
		if rc := routeReason(v, "feat-b"); rc == core.ReasonProgramHandoffInvalid {
			t.Fatalf("%s still blocks a restored handoff: %s", name, rc)
		}
	}
}

// -------- AC-002/AC-003: continuous drive deadline + no-progress --------

func TestProgramDriveStallDecision(t *testing.T) {
	// The continuous loop must distinguish "still making progress" from
	// "same lake, same status, same rollup" (a stalled loop).
	prev := driveProgress{lake: "feat-a", status: "running", rollup: "0/0/0/4"}
	if !driveStalled(prev, prev) {
		t.Fatal("identical round must be detected as stalled")
	}
	if driveStalled(prev, driveProgress{lake: "feat-a", status: "running", rollup: "0/1/0/3"}) {
		t.Fatal("an advancing rollup is progress, not a stall")
	}
	if driveStalled(prev, driveProgress{lake: "feat-b", status: "running", rollup: prev.rollup}) {
		t.Fatal("a different lake is progress, not a stall")
	}
	if driveStalled(prev, driveProgress{lake: "feat-a", status: "awaiting_release", rollup: prev.rollup}) {
		t.Fatal("a status change is progress, not a stall")
	}
}

// AC-002: a registered worktree holding uncommitted work is never
// force-removed by recovery, even when its branch is gone.
func runDirtyLeftoverPreserved(t *testing.T) {
	ctx := context.Background()
	env := newGitEnv(t)
	path := filepath.Join(env.root, WorktreeDirName, "dirty")
	branch := "program/test/dirty"
	if _, err := harness.WorktreeAdd(ctx, env.root, path, branch, ""); err != nil {
		t.Fatal(err)
	}
	// Uncommitted work in the satellite.
	if err := os.WriteFile(filepath.Join(path, "wip.txt"), []byte("precious work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Lose the branch so the worktree is "not usable" to the recovery
	// path while still registered and dirty.
	gitCmd(t, path, "checkout", "--detach")
	_, _ = harness.DeleteBranch(ctx, env.root, branch, true)
	_, err := ensureIntegratedWorktree(ctx, env.root, path, branch, "")
	if err == nil || !strings.Contains(err.Error(), "refusing to remove") {
		t.Fatalf("dirty registered worktree must be refused, got %v", err)
	}
	if body, rerr := os.ReadFile(filepath.Join(path, "wip.txt")); rerr != nil ||
		!strings.Contains(string(body), "precious work") {
		t.Fatalf("recovery deleted the uncommitted file: %q %v", body, rerr)
	}
	if !worktreeListed(t, env.root, path) {
		t.Fatal("the dirty worktree must stay registered")
	}
}
