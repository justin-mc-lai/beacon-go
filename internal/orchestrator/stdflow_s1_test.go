// Standard flow S1 tests (AC-136/137/139/140): branch
// discipline, committed-baseline consumption, the refreeze
// transaction checks and the worktree stale-truth quarantine —
// all against REAL git repositories in temp dirs (behavior-level,
// no git mocks).
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// s1GitEnv: a truthEnv-shaped project (docs/beacon/v2.0.0) whose
// root is a real git repository on the given initial branch.
type s1GitEnv struct {
	st   *store.Store
	eng  *Engine
	root string
}

func newS1GitEnv(t *testing.T, branch string) s1GitEnv {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "beacon.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(t.TempDir(), "proj")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "init", "-b", branch)
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "init")
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Projects().AddVersion(store.ProjectVersion{
		ProjectID: p.ID, Version: "v2.0.0", DocsRoot: filepath.Join("docs", "beacon", "v2.0.0"), IsCurrent: true,
	}); err != nil {
		t.Fatal(err)
	}
	return s1GitEnv{st: st, eng: NewEngine(st), root: root}
}

func (env s1GitEnv) ctx(t *testing.T, slug string) store.FeatureContext {
	t.Helper()
	ctx, err := env.st.ResolveFeature(env.root, slug, "")
	if err != nil {
		t.Fatalf("ResolveFeature(%s): %v", slug, err)
	}
	return ctx
}

func (env s1GitEnv) create(t *testing.T, slug string) TruthPackageView {
	t.Helper()
	v, err := env.eng.CreateTruthPackage(env.root, "", slug)
	if err != nil {
		t.Fatalf("CreateTruthPackage(%s): %v", slug, err)
	}
	completeIntentForTest(t, env.eng, env.ctx(t, slug))
	return v
}

func (env s1GitEnv) commitAll(t *testing.T, msg string) {
	t.Helper()
	gitCmd(t, env.root, "add", "-A")
	gitCmd(t, env.root, "commit", "-m", msg)
}

func (env s1GitEnv) pkgTruthPath(slug string) string {
	return filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", slug, "truth.md")
}

// s1Reason extracts the reason code of an expected error.
func s1Reason(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return reasonOf(err)
}

// ---- AC-136: canonical branch discipline ----

func TestS1BranchGuardRejectsNonCanonical(t *testing.T) {
	env := newS1GitEnv(t, "main")
	env.create(t, "feat-x")
	gitCmd(t, env.root, "checkout", "-b", "dev")
	// Freeze on a non-canonical branch: refused, frozen code.
	_, err := env.eng.FreezeTruthPackage(env.ctx(t, "feat-x"), "tester")
	if rc := s1Reason(t, err); rc != core.ReasonTruthCanonicalBranchMismatch {
		t.Fatalf("freeze on dev reason = %s (%v)", rc, err)
	}
	// Create on a non-canonical branch: refused too.
	_, err = env.eng.CreateTruthPackage(env.root, "", "feat-y")
	if rc := s1Reason(t, err); rc != core.ReasonTruthCanonicalBranchMismatch {
		t.Fatalf("create on dev reason = %s (%v)", rc, err)
	}
	// Back on canonical: freeze passes.
	gitCmd(t, env.root, "checkout", "main")
	if _, err := env.eng.FreezeTruthPackage(env.ctx(t, "feat-x"), "tester"); err != nil {
		t.Fatalf("freeze on main: %v", err)
	}
	// Change registration is a write too: refused off-canonical.
	gitCmd(t, env.root, "checkout", "dev")
	_, err = env.eng.RequestTruthChange(env.ctx(t, "feat-x"), "tweak", "tester")
	if rc := s1Reason(t, err); rc != core.ReasonTruthCanonicalBranchMismatch {
		t.Fatalf("change on dev reason = %s (%v)", rc, err)
	}
	// Detached HEAD counts as mismatch (fail-closed).
	gitCmd(t, env.root, "checkout", "main")
	gitCmd(t, env.root, "checkout", "--detach", "HEAD")
	_, err = env.eng.FreezeTruthPackage(env.ctx(t, "feat-x"), "tester")
	if rc := s1Reason(t, err); rc != core.ReasonTruthCanonicalBranchMismatch {
		t.Fatalf("freeze detached reason = %s (%v)", rc, err)
	}
}

func TestS1BranchDefaultsAndOverride(t *testing.T) {
	// master-only repository: canonical resolves to master.
	env := newS1GitEnv(t, "master")
	env.create(t, "feat-m")
	v, err := env.eng.FreezeTruthPackage(env.ctx(t, "feat-m"), "tester")
	if err != nil {
		t.Fatalf("freeze on master-only repo: %v", err)
	}
	if v.CanonicalBranch != "master" || v.BranchCheck != BranchCheckOK {
		t.Fatalf("view branch fact = %+v", v)
	}
}

// The override case above needs care: with the override in place,
// even create on main is refused. Split into its own flow.
func TestS1BranchConfigOverride(t *testing.T) {
	env := newS1GitEnv(t, "main")
	gitCmd(t, env.root, "checkout", "-b", "dev")
	if err := os.MkdirAll(filepath.Join(env.root, ".beacon"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.root, ".beacon", "config"),
		[]byte("truth_canonical_branch = dev\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env.create(t, "feat-o")
	if _, err := env.eng.FreezeTruthPackage(env.ctx(t, "feat-o"), "tester"); err != nil {
		t.Fatalf("freeze on override-canonical dev: %v", err)
	}
	gitCmd(t, env.root, "checkout", "main")
	_, err := env.eng.RequestTruthChange(env.ctx(t, "feat-o"), "tweak", "tester")
	if rc := s1Reason(t, err); rc != core.ReasonTruthCanonicalBranchMismatch {
		t.Fatalf("change on main with dev override reason = %s (%v)", rc, err)
	}
}

func TestS1NonGitProjectSurfacesSkip(t *testing.T) {
	env := newTruthEnv(t) // plain temp dir, no git
	env.create(t, "feat-ng")
	v, err := env.eng.FreezeTruthPackage(env.ctx(t, "feat-ng"), "tester")
	if err != nil {
		t.Fatalf("freeze in non-git project: %v", err)
	}
	if v.BranchCheck != BranchCheckNotGit {
		t.Fatalf("branch_check = %q, want not_a_git_repo (never a silent skip)", v.BranchCheck)
	}
	if v.BaselineStatus != BaselineNotGit {
		t.Fatalf("baseline_status = %q, want not_a_git_repo", v.BaselineStatus)
	}
	ff, err := env.eng.FrozenFact(env.ctx(t, "feat-ng"))
	if err != nil {
		t.Fatal(err)
	}
	if !ff.Frozen || ff.BaselineStatus != BaselineNotGit {
		t.Fatalf("fact = %+v", ff)
	}
	// Non-git consumption keeps the P4b caliber: frozen admits.
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-ng", Utterance: "build", ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !adm.AllowImplement {
		t.Fatalf("non-git frozen package must admit (P4b caliber): %+v", adm)
	}
}

// ---- AC-139: committed baseline ----

func TestS1BaselineLifecycle(t *testing.T) {
	env := newS1GitEnv(t, "main")
	env.create(t, "feat-b")
	ctx := env.ctx(t, "feat-b")
	v, err := env.eng.FreezeTruthPackage(ctx, "tester")
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if v.BaselineStatus != BaselineUncommitted {
		t.Fatalf("baseline after freeze = %q, want uncommitted", v.BaselineStatus)
	}
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-b", Utterance: "build", ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if adm.AllowImplement || adm.ReasonCode != core.ReasonCanonicalRefreezeNotCommitted {
		t.Fatalf("admission before commit = %+v, want blocked canonical_refreeze_not_committed", adm)
	}
	// truth baseline before the commit: refused with the same code.
	_, err = env.eng.BaselineTruth(ctx, "tester")
	if rc := s1Reason(t, err); rc != core.ReasonCanonicalRefreezeNotCommitted {
		t.Fatalf("baseline before commit reason = %s (%v)", rc, err)
	}
	// Commit the package, then record the baseline.
	env.commitAll(t, "freeze feat-b")
	bv, err := env.eng.BaselineTruth(ctx, "tester")
	if err != nil {
		t.Fatalf("baseline after commit: %v", err)
	}
	head := strings.TrimSpace(gitCmd(t, env.root, "rev-parse", "HEAD"))
	if !bv.BaselineRecorded || bv.BaselineStatus != BaselineCommitted || bv.BaselineCommit != head {
		t.Fatalf("baseline view = %+v, want recorded committed @%s", bv, head)
	}
	// Idempotent: a second run reports already-recorded, no error.
	bv, err = env.eng.BaselineTruth(ctx, "tester")
	if err != nil || !bv.BaselineAlready {
		t.Fatalf("baseline re-run = %+v err=%v, want already recorded", bv, err)
	}
	// S3 seed (declared change): since AC-173 a git project also
	// needs a recorded, confirmed topology projection before
	// admission can turn green.
	seedSingleRepoTopology(t, env.eng, ctx.Feature.ID, env.root)
	// Admission turns green.
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if !adm.AllowImplement {
		t.Fatalf("admission after baseline = %+v, want allow", adm)
	}
	// An uncommitted body edit makes the baseline stale again.
	raw := readFile(t, env.pkgTruthPath("feat-b"))
	if err := os.WriteFile(env.pkgTruthPath("feat-b"), []byte(raw+"\n<!-- local edit -->\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ff, err := env.eng.FrozenFact(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ff.BaselineStatus != BaselineUncommitted {
		t.Fatalf("fact baseline after edit = %q, want uncommitted", ff.BaselineStatus)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if adm.AllowImplement || adm.ReasonCode != core.ReasonCanonicalRefreezeNotCommitted {
		t.Fatalf("admission after edit = %+v, want blocked again", adm)
	}
}

func TestS1TruthExitGateBaselineLeg(t *testing.T) {
	base := ExitInput{Stage: StageTruth, HasFeature: true, TruthFrozen: true,
		FrozenSource: FrozenSourceFrontmatter, LatestGates: map[string]GateFact{}}
	base.TruthBaselineUncommitted = true
	v := EvaluateStageExit(base)
	if v.Allowed || v.ReasonCode != core.ReasonCanonicalRefreezeNotCommitted {
		t.Fatalf("exit with uncommitted baseline = %+v", v)
	}
	base.TruthBaselineUncommitted = false
	if v := EvaluateStageExit(base); !v.Allowed {
		t.Fatalf("exit with committed baseline = %+v, want pass", v)
	}
}

// ---- AC-137: refreeze transaction ----

func TestS1RefreezeEmptyRefused(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-e")
	ctx := env.ctx(t, "feat-e")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "reason only, no edit", "tester"); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.RefreezeTruthPackage(ctx, "tester")
	if rc := s1Reason(t, err); rc != core.ReasonEmptyRefreezeReasonOnly {
		t.Fatalf("empty refreeze reason = %s (%v)", rc, err)
	}
	// A real edit makes the same refreeze pass (normal flow intact).
	raw := readFile(t, filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-e", "truth.md"))
	if err := os.WriteFile(filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-e", "truth.md"),
		[]byte(strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（已按变更修订）", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := env.eng.RefreezeTruthPackage(ctx, "tester")
	if err != nil {
		t.Fatalf("refreeze after real edit: %v", err)
	}
	if !v.Frozen || v.ChangePending {
		t.Fatalf("refrozen view = %+v", v)
	}
}

func TestS1RefreezeRevisionDrift(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-r")
	ctx := env.ctx(t, "feat-r")
	truthPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-r", "truth.md")
	raw := readFile(t, truthPath)
	raw = strings.Replace(raw, "slug: feat-r\n", "slug: feat-r\nrevision_id: rev-1\n", 1)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.RequestTruthChange(ctx, "rev bump", "tester"); err != nil {
		t.Fatal(err)
	}
	raw = readFile(t, truthPath)
	raw = strings.Replace(raw, "revision_id: rev-1", "revision_id: rev-2", 1)
	raw = strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（修订）", 1)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.RefreezeTruthPackage(ctx, "tester")
	if rc := s1Reason(t, err); rc != core.ReasonRevisionAuthorityDrift {
		t.Fatalf("revision drift refreeze reason = %s (%v)", rc, err)
	}
}

func TestS1RefreezeChainDrift(t *testing.T) {
	env := newTruthEnv(t)
	env.create(t, "feat-c")
	ctx := env.ctx(t, "feat-c")
	truthPath := filepath.Join(env.root, "docs", "beacon", "v2.0.0", "features", "feat-c", "truth.md")
	if _, err := env.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatal(err)
	}
	// Edit the body OUTSIDE the change flow first...
	raw := readFile(t, truthPath)
	raw = strings.Replace(raw, "作者写下第一条可验证的完成标准", "作者写下第一条可验证的完成标准（流程外先行编辑）", 1)
	if err := os.WriteFile(truthPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	// ...then register the change (its open fingerprint no longer
	// matches the frozen body), edit a bit more so the surface
	// check passes, and try to refreeze.
	if _, err := env.eng.RequestTruthChange(ctx, "late registration", "tester"); err != nil {
		t.Fatal(err)
	}
	raw = readFile(t, truthPath)
	if err := os.WriteFile(truthPath, []byte(raw+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := env.eng.RefreezeTruthPackage(ctx, "tester")
	if rc := s1Reason(t, err); rc != core.ReasonRevisionAuthorityDrift {
		t.Fatalf("chain drift refreeze reason = %s (%v)", rc, err)
	}
}

// ---- AC-140: worktree stale quarantine ----

func TestS1WorktreeQuarantineBlocksMerge(t *testing.T) {
	env := newS1GitEnv(t, "main")
	env.create(t, "feat-q")
	env.commitAll(t, "add feat-q package (draft)")
	ctx := context.Background()
	run, err := env.eng.CreateRun(CreateRunOptions{ProjectRoot: env.root, FeatureSlug: "feat-q", Utterance: "build", ExecutorID: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := env.eng.ApplyParallelPlan(run.RunID, []LakeInput{
		{Slug: "feat-q", Touches: []string{"q.go"}},
		{Slug: "lake-b", Touches: []string{"b.go"}},
	}, true); err != nil {
		t.Fatal(err)
	}
	wv, err := env.eng.WorktreeCreate(ctx, run.RunID, WorktreeCreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(wv.Quarantines) != 0 {
		t.Fatalf("fresh worktrees must start clean: %+v", wv.Quarantines)
	}
	var sat string
	for _, r := range wv.Records {
		if r.Lake == "feat-q" {
			sat = r.Path
		}
	}
	if sat == "" {
		t.Fatalf("records = %+v", wv.Records)
	}
	// The satellite locally "refreezes" the package: frontmatter
	// flipped to frozen while canonical is still draft.
	satTruth := filepath.Join(sat, "docs", "beacon", "v2.0.0", "features", "feat-q", "truth.md")
	raw := readFile(t, satTruth)
	flipped, changed := setFrontmatterStatus(raw, "frozen")
	if !changed {
		t.Fatal("satellite truth flip did not apply")
	}
	if err := os.WriteFile(satTruth, []byte(flipped), 0o644); err != nil {
		t.Fatal(err)
	}
	// Status scan records the quarantine.
	wv, err = env.eng.WorktreeStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(wv.Quarantines) != 1 {
		t.Fatalf("quarantines = %+v, want exactly 1", wv.Quarantines)
	}
	q := wv.Quarantines[0]
	if q.FeatureSlug != "feat-q" || q.Reason != QuarantineLocalFrozenCanonicalUnfrozen || q.TruthSHA256 == "" {
		t.Fatalf("quarantine record = %+v", q)
	}
	// A second status does not duplicate the record.
	wv, err = env.eng.WorktreeStatus(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(wv.Quarantines) != 1 {
		t.Fatalf("quarantines after re-scan = %+v, want still 1", wv.Quarantines)
	}
	// Merge is refused with the frozen code.
	_, err = env.eng.WorktreeMerge(ctx, run.RunID, "feat-q", "", "")
	if rc := s1Reason(t, err); rc != core.ReasonStaleFeatureLocalRefreeze {
		t.Fatalf("merge of quarantined satellite reason = %s (%v)", rc, err)
	}
	// Repair the satellite copy back to the canonical body: the
	// fresh scan at merge time no longer finds stale truth, and
	// the (now content-free) merge proceeds.
	canon := readFile(t, env.pkgTruthPath("feat-q"))
	if err := os.WriteFile(satTruth, []byte(canon), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.WorktreeMerge(ctx, run.RunID, "feat-q", "", ""); err != nil {
		t.Fatalf("merge after repair: %v", err)
	}
}
