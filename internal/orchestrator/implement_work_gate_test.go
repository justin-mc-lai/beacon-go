// Implement work-gate behaviour tests (implement-work-gate AC-001..006):
// real git worktrees, real gate evaluation, real trail events. The pure
// judgement lives in internal/gate (implement_work_test.go).
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

// newWorkGateEnv is a git-backed gate fixture whose truth.md is
// COMMITTED on the canonical branch (so the S1 baseline probe yields a
// consumable baseline commit) and whose confirmed single-repo topology
// projection clears the S3 admission leg — otherwise admission would
// block for reasons unrelated to this lake's check.
func newWorkGateEnv(t *testing.T) gateTestEnv {
	t.Helper()
	env := newGateEnv(t, frozenTruthDoc)
	gitCmd(t, env.root, "init", "-b", "main")
	gitCmd(t, env.root, "config", "user.email", "test@example.com")
	gitCmd(t, env.root, "config", "user.name", "Test")
	gitCmd(t, env.root, "add", ".")
	gitCmd(t, env.root, "commit", "-m", "truth baseline")
	seedSingleRepoTopology(t, env.eng, env.ctx.Feature.ID, env.root)
	return env
}

// workItem pulls the implement_work_recorded item out of an evaluation.
func workItem(t *testing.T, ev gate.Evaluation) gate.CheckItem {
	t.Helper()
	for _, c := range ev.Checks {
		if c.Check == gate.CheckImplementWorkRecorded {
			return c
		}
	}
	t.Fatalf("evaluation carries no %s item: %+v", gate.CheckImplementWorkRecorded, ev.Checks)
	return gate.CheckItem{}
}

// worktreeSnapshot is the read-only state AC-006 compares before/after.
func worktreeSnapshot(t *testing.T, root string) (head, status string) {
	t.Helper()
	return strings.TrimSpace(gitCmd(t, root, "rev-parse", "HEAD")), gitCmd(t, root, "status", "--porcelain")
}

// ---- AC-001: work exists -> pass ----

func TestImplementGatePassesWhenWorkExists(t *testing.T) {
	env := newWorkGateEnv(t)
	run := env.createRun(t)

	// (a) uncommitted change in the bound worktree.
	writeFile(t, filepath.Join(env.root, "work.go"), "package main\n")
	ev, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("dirty work tree must pass the implement gate, got %+v", ev)
	}
	if item := workItem(t, ev); !item.Applicable || !item.OK {
		t.Fatalf("implement_work_recorded = %+v, want applicable ok", item)
	}

	// (b) a new commit past the recorded baseline, clean worktree.
	gitCmd(t, env.root, "add", "work.go")
	gitCmd(t, env.root, "commit", "-m", "implement the lake")
	ev, err = env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("moved HEAD must pass the implement gate, got %+v", ev)
	}
	item := workItem(t, ev)
	if !item.Applicable || !item.OK || !strings.Contains(item.Detail, "moved past baseline") {
		t.Fatalf("moved-HEAD item = %+v, want an applicable ok naming the movement", item)
	}
}

// ---- AC-002: clean + unmoved -> blocked + implement_no_work ----

func TestImplementGateBlocksEmptyRun(t *testing.T) {
	env := newWorkGateEnv(t)
	run := env.createRun(t)

	head, _ := worktreeSnapshot(t, env.root)
	ev, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictBlocked {
		t.Fatalf("an untouched worktree must block the implement gate, got %+v", ev)
	}
	if !contains(ev.ReasonCodes, core.ReasonImplementNoWork) {
		t.Fatalf("want %s among the reason codes, got %v", core.ReasonImplementNoWork, ev.ReasonCodes)
	}
	item := workItem(t, ev)
	if item.OK || !item.Applicable || item.ReasonCode != core.ReasonImplementNoWork {
		t.Fatalf("empty-run item = %+v, want an applicable failure with %s", item, core.ReasonImplementNoWork)
	}
	if !strings.Contains(item.Detail, head[:12]) {
		t.Fatalf("the item must name the unmoved HEAD %s, got %q", head[:12], item.Detail)
	}
	// A blocked implement verdict is what release consumes: the gate's
	// own ledger row carries the code, not just the printed view.
	latest, err := env.st.Gates().Latest(env.ctx.Feature.ID, "implement")
	if err != nil {
		t.Fatal(err)
	}
	if latest.Verdict != string(gate.VerdictBlocked) || !contains(latest.ReasonCodes, core.ReasonImplementNoWork) {
		t.Fatalf("persisted verdict = %+v, want blocked + %s", latest, core.ReasonImplementNoWork)
	}
}

// ---- AC-003: --allow-empty declaration -> not_applicable ----

func TestImplementGateAllowsEmpty(t *testing.T) {
	env := newWorkGateEnv(t)
	run := env.createRun(t)

	res, err := env.eng.AllowEmptyImplement(env.ctx, run.RunID, "human", "no code for this lake")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Appended || res.EventID == "" {
		t.Fatalf("first declaration must append, got %+v", res)
	}
	// Idempotent: a second declaration appends nothing.
	again, err := env.eng.AllowEmptyImplement(env.ctx, run.RunID, "human", "no code for this lake")
	if err != nil {
		t.Fatal(err)
	}
	if again.Appended || again.EventID != "" {
		t.Fatalf("second declaration must be a no-op, got %+v", again)
	}

	ev, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass || len(ev.ReasonCodes) != 0 {
		t.Fatalf("an allow-empty run must pass with no codes, got %+v", ev)
	}
	item := workItem(t, ev)
	if item.Applicable || !item.OK || item.ReasonCode != core.ReasonImplementEmptyAllowed {
		t.Fatalf("allow-empty item = %+v, want not_applicable + %s", item, core.ReasonImplementEmptyAllowed)
	}
	// The other admission items are untouched: the item count is the
	// admission item count plus exactly this one.
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.Checks) != len(adm.Items)+1 {
		t.Fatalf("checks (%d) must be admission items (%d) + the work item", len(ev.Checks), len(adm.Items))
	}

	// The config switch is the other not_applicable leg (implement_work_gate=off).
	env2 := newWorkGateEnv(t)
	run2 := env2.createRun(t)
	offEnv := NewEngine(env2.st)
	if err := offEnv.SetImplementWorkGate("off"); err != nil {
		t.Fatal(err)
	}
	ev, err = offEnv.EvaluateGate(env2.ctx, "implement", run2.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass {
		t.Fatalf("gate off must not block, got %+v", ev)
	}
	if item := workItem(t, ev); item.Applicable || item.ReasonCode != core.ReasonImplementWorkGateOff {
		t.Fatalf("gate-off item = %+v, want not_applicable + %s", item, core.ReasonImplementWorkGateOff)
	}
	if err := offEnv.SetImplementWorkGate("maybe"); err == nil {
		t.Fatal("a bad implement_work_gate value must be refused, never guessed")
	} else if rc, ok := core.ReasonCodeOf(err); !ok || rc != core.ReasonImplementWorkGateInvalid {
		t.Fatalf("want %s, got %v", core.ReasonImplementWorkGateInvalid, err)
	}
}

// ---- AC-004: unreadable worktrees are not red ----

func TestImplementGateUnverifiedIsNotRed(t *testing.T) {
	// (a) a non-git project root.
	plain := newGateEnv(t, frozenTruthDoc)
	run := plain.createRun(t)
	ev, err := plain.eng.EvaluateGate(plain.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict != gate.VerdictPass || contains(ev.ReasonCodes, core.ReasonImplementNoWork) {
		t.Fatalf("non-git project must not be judged red, got %+v", ev)
	}
	item := workItem(t, ev)
	if item.Applicable || !item.OK || item.ReasonCode != core.ReasonImplementWorkUnverified {
		t.Fatalf("non-git item = %+v, want not_applicable + %s", item, core.ReasonImplementWorkUnverified)
	}

	// (b) a git project whose bound worktree disappears.
	env := newWorkGateEnv(t)
	run = env.createRun(t)
	if err := os.RemoveAll(env.root); err != nil {
		t.Fatal(err)
	}
	ev, err = env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Verdict == gate.VerdictPass {
		// Admission itself may block; the work item must never pass.
		t.Logf("verdict = %s (admission-level effect of the missing root)", ev.Verdict)
	}
	item = workItem(t, ev)
	if item.Applicable || !item.OK || item.ReasonCode != core.ReasonImplementWorkUnverified {
		t.Fatalf("missing-root item = %+v, want not_applicable + %s", item, core.ReasonImplementWorkUnverified)
	}
	if contains(ev.ReasonCodes, core.ReasonImplementNoWork) {
		t.Fatalf("a missing root must never be reported as implement_no_work: %v", ev.ReasonCodes)
	}
}

// ---- AC-006: the check is read-only and deterministic ----

func TestImplementGateIsReadOnly(t *testing.T) {
	env := newWorkGateEnv(t)
	run := env.createRun(t)
	// Dirty worktree so the item has real git state to read.
	writeFile(t, filepath.Join(env.root, "work.go"), "package main\n")

	headBefore, statusBefore := worktreeSnapshot(t, env.root)
	first, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	headAfter, statusAfter := worktreeSnapshot(t, env.root)
	if headBefore != headAfter {
		t.Fatalf("HEAD moved during evaluation: %s -> %s", headBefore, headAfter)
	}
	if statusBefore != statusAfter {
		t.Fatalf("worktree status changed during evaluation:\nbefore=%q\nafter=%q", statusBefore, statusAfter)
	}
	// The dirty file is still exactly one untracked file: nothing was
	// staged, committed or stashed on the way through.
	if trimmed := strings.TrimSpace(statusAfter); trimmed != "?? work.go" {
		t.Fatalf("unexpected worktree status %q", trimmed)
	}

	second, err := env.eng.EvaluateGate(env.ctx, "implement", run.RunID, "qa-bot")
	if err != nil {
		t.Fatal(err)
	}
	if first.Verdict != second.Verdict || strings.Join(first.ReasonCodes, ",") != strings.Join(second.ReasonCodes, ",") {
		t.Fatalf("re-evaluation must be deterministic: %+v vs %+v", first, second)
	}
	if strings.Join(checkList(first), ",") != strings.Join(checkList(second), ",") {
		t.Fatalf("re-evaluation must produce the same check items: %v vs %v", checkList(first), checkList(second))
	}
}

// ---- helpers ----

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func checkList(ev gate.Evaluation) []string {
	out := make([]string, 0, len(ev.Checks))
	for _, c := range ev.Checks {
		out = append(out, c.Check+"="+c.Detail)
	}
	return out
}
