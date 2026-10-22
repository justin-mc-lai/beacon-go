// Topology S3 tests, part 2: the three topologies' gate behavior
// against REAL git repositories (AC-172 live legs).
package orchestrator

import (
	"path/filepath"
	"testing"

	"beacon-go/internal/core"
)

// ---- AC-172: live behavior per topology (real git) ----

func TestS3SingleRepoLiveGate(t *testing.T) {
	root := makeGitRepo(t, filepath.Join(t.TempDir(), "proj"), map[string]string{
		"pkg/truth.md": "truth\n", "code.go": "package x\n",
	})
	env := newTopoEnv(t, root, "pkg/truth.md")
	bc := seedSingleRepoTopology(t, env.eng, env.featureID, root)
	if bc.TopologyGateStatus != TopologyGatePass {
		t.Fatalf("projected status = %q, want pass on a clean repo", bc.TopologyGateStatus)
	}
	fact := env.eng.topologyFactForProject(env.projectID, root, "pkg")
	if fact.Status != TopologyGatePass || fact.Ref != bc.TopologyGateRef {
		t.Fatalf("fact = %+v", fact)
	}
	// Ordinary code dirty: the primary keeps the S2 caliber — pass.
	writeFile(t, filepath.Join(root, "code.go"), "package x // edited\n")
	if fact = env.eng.topologyFactForProject(env.projectID, root, "pkg"); fact.Status != TopologyGatePass {
		t.Fatalf("ordinary dirty must not block the primary scope: %+v", fact)
	}
	// Package dirty: blocked.
	writeFile(t, filepath.Join(root, "pkg", "truth.md"), "truth edited\n")
	if fact = env.eng.topologyFactForProject(env.projectID, root, "pkg"); fact.Status != TopologyGateBlocked {
		t.Fatalf("package dirty must block: %+v", fact)
	}
}

func TestS3MonorepoLiveGate(t *testing.T) {
	root := makeGitRepo(t, filepath.Join(t.TempDir(), "mono"), map[string]string{
		"README.md":          "# mono\n",
		"pkg/truth.md":       "truth\n",
		"svc-a/go.mod":       "module a\n",
		"svc-a/main.go":      "package main\n",
		"svc-b/package.json": "{}\n",
		"svc-b/index.js":     "console.log(1)\n",
	})
	env := newTopoEnv(t, root, "pkg/truth.md")
	no := false
	g := GitScopeGraph{SchemaVersion: GitScopeGraphSchema, ProjectTopology: TopologyMonorepo, Scopes: []GitScope{
		{ScopeID: "primary", GitRoot: root, Role: "primary", Writable: &no, DirtyPolicy: "fail_closed"},
		{ScopeID: "svc-a", GitRoot: root, Role: "service", Path: "svc-a", DirtyPolicy: "fail_closed"},
		{ScopeID: "svc-b", GitRoot: root, Role: "service", Path: "svc-b", DirtyPolicy: "fail_closed"},
	}}
	bc, err := env.eng.ProjectGitScopeGraph(env.featureID, g, TopologySourceFile, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if bc.TopologyGateStatus != TopologyGatePass {
		t.Fatalf("projected status = %q, want pass", bc.TopologyGateStatus)
	}
	// Root-level dirt belongs to the read-only primary: not judged.
	writeFile(t, filepath.Join(root, "README.md"), "# mono edited\n")
	if v := env.eng.evaluateTopologyLive(bc, root, "pkg"); v.Status != TopologyGatePass {
		t.Fatalf("read-only primary dirt must not block: %+v", v)
	}
	// A dirty service blocks with the v1 dynamic code.
	writeFile(t, filepath.Join(root, "svc-b", "index.js"), "console.log(2)\n")
	v := env.eng.evaluateTopologyLive(bc, root, "pkg")
	if v.Status != TopologyGateBlocked {
		t.Fatalf("dirty service must block: %+v", v)
	}
	found := false
	for _, c := range v.ReasonCodes {
		if c == "monorepo_service_dirty:svc-b" {
			found = true
		}
	}
	if !found {
		t.Fatalf("codes = %v, want monorepo_service_dirty:svc-b", v.ReasonCodes)
	}
}

func TestS3MultiGitLiveGate(t *testing.T) {
	base := t.TempDir()
	core_ := makeGitRepo(t, filepath.Join(base, "core"), map[string]string{"main.go": "package main\n"})
	sat := makeGitRepo(t, filepath.Join(base, "sat"), map[string]string{"pkg/truth.md": "truth\n"})
	env := newTopoEnv(t, base, "sat/pkg/truth.md")
	yes := true
	g := GitScopeGraph{SchemaVersion: GitScopeGraphSchema, ProjectTopology: TopologyMultiGit, Scopes: []GitScope{
		{ScopeID: "core", GitRoot: core_, Role: "primary", Writable: &yes, DirtyPolicy: "fail_closed"},
		{ScopeID: "sat", GitRoot: sat, Role: "satellite", DirtyPolicy: "fail_closed"},
	}}
	bc, err := env.eng.ProjectGitScopeGraph(env.featureID, g, TopologySourceFile, "tester")
	if err != nil {
		t.Fatal(err)
	}
	if bc.TopologyGateStatus != TopologyGatePass {
		t.Fatalf("projected status = %q, want pass (detail via fact)", bc.TopologyGateStatus)
	}
	// Satellite dirt aggregates to a blocked gate (AC-172).
	writeFile(t, filepath.Join(sat, "pkg", "truth.md"), "truth edited\n")
	v := env.eng.evaluateTopologyLive(bc, base, "sat/pkg")
	if v.Status != TopologyGateBlocked || v.Scopes[1].ReasonCodes[0] != core.ReasonSatelliteWorktreeDirty {
		t.Fatalf("satellite verdict = %+v", v)
	}
	fact := env.eng.topologyFactForProject(env.projectID, base, "sat/pkg")
	if fact.Status != TopologyGateBlocked || fact.Ref != bc.TopologyGateRef {
		t.Fatalf("fact = %+v", fact)
	}
	// A primary pointing at a non-repo aggregates
	// multi_git_primary_missing.
	g.Scopes[0].GitRoot = base
	bc2, err := env.eng.ProjectGitScopeGraph(env.featureID, g, TopologySourceFile, "tester")
	if err != nil {
		t.Fatal(err)
	}
	v = env.eng.evaluateTopologyLive(bc2, base, "sat/pkg")
	if v.Scopes[0].ReasonCodes[0] != core.ReasonMultiGitPrimaryMissing {
		t.Fatalf("primary-missing verdict = %+v", v)
	}
}
