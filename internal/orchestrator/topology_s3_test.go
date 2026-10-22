// Topology S3 tests (AC-171/172/173): graph parity + projection,
// the three topologies' gate behavior against REAL git
// repositories, admission/bundle consumption, draft confirm, facts.
package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// seedSingleRepoTopology records a confirmed single-repo
// projection for (featureID, root) — the S3 seed used by the S1/S2
// git fixtures whose scenarios predate the AC-173 gate.
func seedSingleRepoTopology(t *testing.T, eng *Engine, featureID, root string) BranchContext {
	t.Helper()
	g := GitScopeGraph{
		SchemaVersion: GitScopeGraphSchema, ProjectTopology: TopologySingleRepo,
		Scopes: []GitScope{{ScopeID: "primary", GitRoot: root, Role: "primary",
			WorktreePath: root, DirtyPolicy: "fail_closed"}},
	}
	bc, err := eng.ProjectGitScopeGraph(featureID, g, TopologySourceFile, "tester")
	if err != nil {
		t.Fatalf("seed topology: %v", err)
	}
	return bc
}

// topoEnv registers a project rooted at root (which the caller has
// already shaped on disk) with one frozen feature.
type topoEnv struct {
	st        *store.Store
	eng       *Engine
	projectID string
	featureID string
}

func newTopoEnv(t *testing.T, root, truthPath string) topoEnv {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "beacon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p, err := st.Projects().Register(store.Project{RootPath: root, Name: "topo"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Projects().AddVersion(store.ProjectVersion{ProjectID: p.ID, Version: "v2.0.0", DocsRoot: "docs", IsCurrent: true})
	if err != nil {
		t.Fatal(err)
	}
	f, err := st.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: v.ID, Slug: "feat-topo", TruthPath: truthPath, TestsPath: "tests.md", Status: "frozen",
	})
	if err != nil {
		t.Fatal(err)
	}
	return topoEnv{st: st, eng: NewEngine(st), projectID: p.ID, featureID: f.ID}
}

// makeGitRepo creates a real git repo at dir with the given files
// committed.
func makeGitRepo(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	for name, body := range files {
		fp := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fp, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "init")
	return dir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---- AC-171: parity + projection ----

// The v1.6.7 fixture's shape, verbatim from the source repo's
// git-scope-graph-v167.fixture.json (multi-repository case),

const v167FixtureGraph = `{
  "schema_version": "git-scope-graph.v1",
  "produced_by": "beacon.workspace.topology-resolve",
  "generated_at": "2026-05-13T00:00:00Z",
  "version": "v1.6.7",
  "feature": "beacon-git-scope-graph-v167",
  "project_topology": "multi_git_workspace",
  "scopes": [
    {"scope_id": "core", "git_root": "/repo/core", "role": "primary",
     "branch_resolution_mode": "per-feature", "target_branch": "main",
     "worktree_path": "/repo/core", "admission_artifact_ref": "",
     "dirty_policy": "fail_closed", "status": "ready", "reason_codes": [], "dirty": false},
    {"scope_id": "web", "git_root": "/repo/web", "role": "satellite",
     "branch_resolution_mode": "preserve-current", "target_branch": "feat/web",
     "worktree_path": "/repo/web", "admission_artifact_ref": "",
     "dirty_policy": "fail_closed", "status": "ready", "reason_codes": [], "dirty": false}
  ],
  "topology_gate_status": "pass",
  "reason_codes": [],
  "topology_gate_ref": ".beacon/state/git-scope-graph/v1.6.7/beacon-git-scope-graph-v167.json"
}`

func TestS3FixtureGraphProjectsWithFrozenFields(t *testing.T) {
	g, err := LoadGitScopeGraph([]byte(v167FixtureGraph))
	if err != nil {
		t.Fatalf("the v1.6.7 fixture shape must load: %v", err)
	}
	bc := ProjectBranchContext(g)
	if bc.TopologyKind != TopologyMultiGit || len(bc.Scopes) != 2 {
		t.Fatalf("projection = %+v", bc)
	}
	if bc.TopologyGateStatus != "pass" {
		t.Fatalf("snapshot status = %q, want the graph's declared pass", bc.TopologyGateStatus)
	}
	if !strings.HasPrefix(bc.TopologyGateRef, "bc_") || len(bc.TopologyGateRef) != 19 {
		t.Fatalf("ref = %q, want bc_<16 hex>", bc.TopologyGateRef)
	}
	// Ref stability: same graph -> same ref; changed graph -> new ref.
	if again := ProjectBranchContext(g); again.TopologyGateRef != bc.TopologyGateRef {
		t.Fatalf("ref not stable: %s vs %s", again.TopologyGateRef, bc.TopologyGateRef)
	}
	g.Scopes[1].TargetBranch = "feat/other"
	// TargetBranch is not part of the projected context; a scope
	// identity change is. Flip the satellite's root instead.
	g.Scopes[1].GitRoot = "/repo/web2"
	if changed := ProjectBranchContext(g); changed.TopologyGateRef == bc.TopologyGateRef {
		t.Fatal("ref must change when the graph changes")
	}
	// Writable derivation: satellite writable, primary of a multi
	// workspace read-only unless declared.
	if !bc.Scopes[1].Writable || bc.Scopes[0].Writable {
		t.Fatalf("writable derivation = %+v", bc.Scopes)
	}
}

func TestS3GraphParityFailures(t *testing.T) {
	cases := map[string]string{
		"not json":            `{`,
		"wrong schema":        `{"schema_version":"git-scope-graph.v2","project_topology":"single_repo","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"}]}`,
		"missing schema":      `{"project_topology":"single_repo","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"}]}`,
		"unknown topology":    `{"schema_version":"git-scope-graph.v1","project_topology":"forest","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"}]}`,
		"no scopes":           `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[]}`,
		"scope missing root":  `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[{"scope_id":"p","role":"primary"}]}`,
		"scope missing id":    `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[{"git_root":"/r","role":"primary"}]}`,
		"scope bad role":      `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[{"scope_id":"p","git_root":"/r","role":"owner"}]}`,
		"duplicate scope id":  `{"schema_version":"git-scope-graph.v1","project_topology":"multi_git_workspace","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"},{"scope_id":"p","git_root":"/r2","role":"satellite"}]}`,
		"single_repo two":     `{"schema_version":"git-scope-graph.v1","project_topology":"single_repo","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"},{"scope_id":"s","git_root":"/r","role":"service"}]}`,
		"multi no satellite":  `{"schema_version":"git-scope-graph.v1","project_topology":"multi_git_workspace","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"}]}`,
		"monorepo no service": `{"schema_version":"git-scope-graph.v1","project_topology":"monorepo","scopes":[{"scope_id":"p","git_root":"/r","role":"primary"}]}`,
	}
	for name, doc := range cases {
		if _, err := LoadGitScopeGraph([]byte(doc)); err == nil {
			t.Errorf("%s: loaded, want parity failure", name)
		} else if rc := s1Reason(t, err); rc != core.ReasonGitScopeParityFailed {
			t.Errorf("%s: reason = %s, want %s (%v)", name, rc, core.ReasonGitScopeParityFailed, err)
		}
	}
}

// ---- AC-172: the pure gate ----

func TestS3EvaluateTopologyGatePure(t *testing.T) {
	mkBC := func(kind string, scopes ...BranchScope) BranchContext {
		return BranchContext{Schema: BranchContextSchema, TopologyKind: kind,
			Scopes: scopes, TopologyGateRef: "bc_test"}
	}
	// Satellite blocked -> aggregate blocked (AC-GSG-004 caliber).
	bc := mkBC(TopologyMultiGit,
		BranchScope{ScopeID: "core", Role: "primary", Writable: true},
		BranchScope{ScopeID: "web", Role: "satellite", Writable: true})
	v := EvaluateTopologyGate(bc, []ScopeWorkState{
		{ScopeID: "core"}, {ScopeID: "web", Violations: []string{"dirty.txt"}},
	})
	if v.Status != TopologyGateBlocked || v.Scopes[1].ReasonCodes[0] != core.ReasonSatelliteWorktreeDirty {
		t.Fatalf("multi verdict = %+v", v)
	}
	// Multi primary not a repo -> multi_git_primary_missing.
	v = EvaluateTopologyGate(bc, []ScopeWorkState{
		{ScopeID: "core", NotGit: true}, {ScopeID: "web"},
	})
	if v.Status != TopologyGateBlocked || v.Scopes[0].ReasonCodes[0] != core.ReasonMultiGitPrimaryMissing {
		t.Fatalf("multi primary-missing verdict = %+v", v)
	}
	// Monorepo service dirty -> monorepo_service_dirty:<key>;
	// the read-only primary's dirt is not judged.
	bc = mkBC(TopologyMonorepo,
		BranchScope{ScopeID: "primary", Role: "primary", Writable: false},
		BranchScope{ScopeID: "svc-a", Role: "service", Writable: true, Path: "svc-a"})
	v = EvaluateTopologyGate(bc, []ScopeWorkState{
		{ScopeID: "primary", Violations: []string{"README.md"}},
		{ScopeID: "svc-a", Violations: []string{"main.go"}},
	})
	if v.Status != TopologyGateBlocked || v.Scopes[0].Status != TopologyGatePass ||
		v.Scopes[1].ReasonCodes[0] != "monorepo_service_dirty:svc-a" {
		t.Fatalf("monorepo verdict = %+v", v)
	}
	// Single repo clean -> pass; a missing state never guesses pass.
	bc = mkBC(TopologySingleRepo, BranchScope{ScopeID: "primary", Role: "primary", Writable: true})
	if v = EvaluateTopologyGate(bc, []ScopeWorkState{{ScopeID: "primary"}}); v.Status != TopologyGatePass {
		t.Fatalf("single verdict = %+v", v)
	}
	if v = EvaluateTopologyGate(bc, nil); v.Status != TopologyGateBlocked {
		t.Fatalf("unobserved writable scope must block, got %+v", v)
	}
}

// ---- AC-173: admission + bundle consumption, draft flow ----

// s3AdmissionEnv mirrors s2Setup (create/freeze/commit/baseline)
// WITHOUT the S3 topology seed.
func s3AdmissionEnv(t *testing.T, slug string) s1GitEnv {
	t.Helper()
	e := newS1GitEnv(t, "main")
	e.create(t, slug)
	ctx := e.ctx(t, slug)
	if _, err := e.eng.FreezeTruthPackage(ctx, "tester"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	e.commitAll(t, "freeze "+slug)
	if _, err := e.eng.BaselineTruth(ctx, "tester"); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	return e
}

func TestS3AdmissionTopologyLegs(t *testing.T) {
	env := s3AdmissionEnv(t, "feat-topo-adm")
	run := env.s2Run(t, "feat-topo-adm")

	// No projection: item 8 blocks with topology_gate_missing and
	// (everything else green) it is the admission headline.
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	it := admissionItem(adm, AdmissionCheckTopologyGate)
	if adm.AllowImplement || it.OK || it.ReasonCode != core.ReasonTopologyGateMissing ||
		adm.ReasonCode != core.ReasonTopologyGateMissing {
		t.Fatalf("admission without projection = %+v item=%+v", adm, it)
	}
	// The bundle's branch-governance item carries the topology
	// sub-result: missing, no ref.
	bun, err := env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	bg := bundleItem(bun, BundleBranchGov)
	if bg.Topology == nil || bg.Topology.Status != TopologyGateMissing || bg.Topology.Ref != "" || bg.OK {
		t.Fatalf("branch-gov topology sub-result = %+v", bg)
	}

	// Seeded + confirmed: both consumers green, sub-result names ref.
	bc := seedSingleRepoTopology(t, env.eng, mustFeatID(t, env, run.RunID), env.root)
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it = admissionItem(adm, AdmissionCheckTopologyGate); !adm.AllowImplement || !it.OK {
		t.Fatalf("admission with projection = %+v item=%+v", adm, it)
	}
	bun, err = env.eng.EvaluateAdmissionBundle(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	bg = bundleItem(bun, BundleBranchGov)
	if bg.Topology == nil || bg.Topology.Status != TopologyGatePass || bg.Topology.Ref != bc.TopologyGateRef {
		t.Fatalf("branch-gov topology sub-result after seed = %+v", bg)
	}

	// A protected-path violation (P4c denylist: a root .env file —
	// porcelain reports files, not collapsed untracked dirs)
	// blocks the gate live: topology_gate_blocked at item level.
	writeFile(t, filepath.Join(env.root, ".env"), "SECRET=1\n")
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it = admissionItem(adm, AdmissionCheckTopologyGate); it.OK || it.ReasonCode != core.ReasonTopologyGateBlocked || adm.AllowImplement {
		t.Fatalf("admission with blocked gate = %+v item=%+v", adm, it)
	}
}

func TestS3DetectDraftConfirmFlow(t *testing.T) {
	env := s3AdmissionEnv(t, "feat-topo-draft")
	run := env.s2Run(t, "feat-topo-draft")
	featID := mustFeatID(t, env, run.RunID)

	g, notes, err := DetectGitScopeGraph(env.root)
	if err != nil {
		t.Fatalf("detect: %v (%v)", err, notes)
	}
	if g.ProjectTopology != TopologySingleRepo {
		t.Fatalf("detected topology = %s, want single_repo (%v)", g.ProjectTopology, notes)
	}
	bc, err := env.eng.ProjectGitScopeGraph(featID, g, TopologySourceDraft, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if bc.Confirmed {
		t.Fatal("a detected draft must be recorded unconfirmed")
	}
	// Unconfirmed draft == missing at admission.
	adm, err := env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := admissionItem(adm, AdmissionCheckTopologyGate); it.OK || it.ReasonCode != core.ReasonTopologyGateMissing {
		t.Fatalf("draft admission item = %+v", it)
	}
	// Confirm (idempotent): admission consumes the projection now.
	ctx := env.ctx(t, "feat-topo-draft")
	if _, err := env.eng.ConfirmBranchContext(ctx.Project.ID, "tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := env.eng.ConfirmBranchContext(ctx.Project.ID, "tester"); err != nil {
		t.Fatalf("confirm must be idempotent: %v", err)
	}
	adm, err = env.eng.CheckImplementAdmission(run.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if it := admissionItem(adm, AdmissionCheckTopologyGate); !it.OK || !adm.AllowImplement {
		t.Fatalf("confirmed admission = %+v item=%+v", adm, it)
	}
	// Detect on a root with no git topology fails closed.
	if _, _, err := DetectGitScopeGraph(t.TempDir()); err == nil {
		t.Fatal("detect on a non-git root with no sub-repos must fail")
	} else if rc := s1Reason(t, err); rc != core.ReasonWorktreeNotGitRepo {
		t.Fatalf("detect reason = %s (%v)", rc, err)
	}
}

func TestS3DetectMonorepoAndMulti(t *testing.T) {
	// Monorepo: one repo, one package root with a manifest.
	mono := makeGitRepo(t, filepath.Join(t.TempDir(), "mono"), map[string]string{
		"README.md": "# m\n", "svc/go.mod": "module svc\n", "svc/main.go": "package main\n",
	})
	g, _, err := DetectGitScopeGraph(mono)
	if err != nil {
		t.Fatal(err)
	}
	if g.ProjectTopology != TopologyMonorepo || len(g.Scopes) != 2 || g.Scopes[1].Path != "svc" {
		t.Fatalf("monorepo draft = %+v", g)
	}
	if err := ValidateGitScopeGraph(g); err != nil {
		t.Fatalf("monorepo draft must validate: %v", err)
	}
	// Multi: root not a repo, two sub-repos.
	base := t.TempDir()
	makeGitRepo(t, filepath.Join(base, "a"), map[string]string{"f.txt": "a\n"})
	makeGitRepo(t, filepath.Join(base, "b"), map[string]string{"f.txt": "b\n"})
	g, notes, err := DetectGitScopeGraph(base)
	if err != nil {
		t.Fatal(err)
	}
	if g.ProjectTopology != TopologyMultiGit || len(g.Scopes) != 3 {
		t.Fatalf("multi draft = %+v (%v)", g, notes)
	}
	if err := ValidateGitScopeGraph(g); err != nil {
		t.Fatalf("multi draft must validate: %v", err)
	}
}

// ---- AC-173: the run fact layer ----

func TestS3RunFactLayer(t *testing.T) {
	env := s3AdmissionEnv(t, "feat-topo-fact")
	run := env.s2Run(t, "feat-topo-fact")
	if run.Topology == nil || run.Topology.Status != TopologyGateMissing {
		t.Fatalf("run fact without projection = %+v, want topology missing stated openly", run.Topology)
	}
	bc := seedSingleRepoTopology(t, env.eng, mustFeatID(t, env, run.RunID), env.root)
	run2 := env.s2Run(t, "feat-topo-fact")
	if run2.Topology == nil || run2.Topology.Status != TopologyGatePass ||
		run2.Topology.Ref != bc.TopologyGateRef || run2.Topology.TopologyKind != TopologySingleRepo {
		t.Fatalf("run fact with projection = %+v", run2.Topology)
	}
	// Non-git project: not_applicable, stated — never missing-red.
	ng := newTestEnv(t, "frozen")
	ngRun := ng.createRun(t)
	if ngRun.Topology == nil || ngRun.Topology.Status != TopologyNotApplicable {
		t.Fatalf("non-git run fact = %+v, want not_applicable", ngRun.Topology)
	}
}
