// Topology S3: live topology detection — the generation path for
// projects that have no GitScopeGraph yet (AC-171's companion).
//
// Detection is a measured probe, not a guess: it inspects the
// project root and produces a DRAFT graph (recorded with
// source=generated_draft) that admission refuses to consume until
// a human confirms it (goal topology confirm). The判据:
//
//   - one or more git repositories directly under the root ->
//     multi_git_workspace (root itself, when a repo, is the
//     primary scope; each sub-repo a satellite). When the root is
//     NOT a repo, the primary scope still names the root — its
//     gate verdict will be multi_git_primary_missing until a human
//     fixes the draft, exactly v1's fail-closed placeholder.
//   - root is a repo and contains independent package roots
//     (DERIVED criterion: a directory holding its own package
//     manifest — go.mod, package.json, Cargo.toml, pyproject.toml,
//     setup.py, pom.xml, build.gradle[.kts]) -> monorepo: the root
//     is a non-writable primary scope and each package root is a
//     writable service scope keyed by its relative path (v1 keyed
//     monorepo services by service_key as the in-repo pathspec).
//   - root is a repo and nothing else is found -> single_repo.
//
// Sub-repository detection uses IsGitRepoRoot so a subdirectory
// of the root repo is never mistaken for its own repository.
package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
)

// packageManifests are the files that mark an independent package
// root for the monorepo detection criterion (DERIVED, see header).
var packageManifests = []string{
	"go.mod", "package.json", "Cargo.toml", "pyproject.toml",
	"setup.py", "pom.xml", "build.gradle", "build.gradle.kts",
}

// detectSkipDirs are never probed for sub-repos or package roots.
var detectSkipDirs = map[string]bool{
	".git": true, ".beacon": true, ".beacon-worktrees": true,
	"node_modules": true, "vendor": true,
}

// DetectGitScopeGraph probes a project root and returns the draft
// graph plus human-readable notes on what was found. It errors
// (fail-closed) when no git topology exists at all.
func DetectGitScopeGraph(root string) (GitScopeGraph, []string, error) {
	gctx := context.Background()
	abs, err := filepath.Abs(root)
	if err != nil {
		return GitScopeGraph{}, nil, err
	}
	rootGit := harness.IsGitRepo(gctx, abs)
	entries, err := os.ReadDir(abs)
	if err != nil {
		return GitScopeGraph{}, nil, err
	}
	var subRepos []string
	for _, en := range entries {
		if !en.IsDir() || detectSkipDirs[en.Name()] || strings.HasPrefix(en.Name(), ".") {
			continue
		}
		if harness.IsGitRepoRoot(gctx, filepath.Join(abs, en.Name())) {
			subRepos = append(subRepos, en.Name())
		}
	}
	sort.Strings(subRepos)
	writable := func(b bool) *bool { return &b }
	mk := func(id, gitRoot, role, path string, w *bool) GitScope {
		return GitScope{ScopeID: id, GitRoot: gitRoot, Role: role, Path: path,
			Writable: w, WorktreePath: gitRoot, DirtyPolicy: "fail_closed",
			BranchResolutionMode: "per-feature"}
	}
	var notes []string
	g := GitScopeGraph{SchemaVersion: GitScopeGraphSchema, ProducedBy: "beacon-goal-topology-detect"}
	switch {
	case len(subRepos) > 0:
		g.ProjectTopology = TopologyMultiGit
		g.Scopes = append(g.Scopes, mk("primary", abs, "primary", "", writable(rootGit)))
		for _, sub := range subRepos {
			g.Scopes = append(g.Scopes, mk(sub, filepath.Join(abs, sub), "satellite", "", writable(true)))
		}
		notes = append(notes, "found "+strconv.Itoa(len(subRepos))+" git repositor(ies) under the root: "+strings.Join(subRepos, ", "))
		if !rootGit {
			notes = append(notes, "the root itself is not a git repository: the primary scope will judge multi_git_primary_missing until the draft is corrected")
		}
	case rootGit:
		pkgRoots := detectPackageRoots(abs)
		if len(pkgRoots) == 0 {
			g.ProjectTopology = TopologySingleRepo
			g.Scopes = append(g.Scopes, mk("primary", abs, "primary", "", writable(true)))
			notes = append(notes, "single git repository, no sub-repositories and no independent package roots")
		} else {
			g.ProjectTopology = TopologyMonorepo
			g.Scopes = append(g.Scopes, mk("primary", abs, "primary", "", writable(false)))
			for _, rel := range pkgRoots {
				g.Scopes = append(g.Scopes, mk(rel, abs, "service", rel, writable(true)))
			}
			notes = append(notes, "single git repository with independent package roots (manifest criterion, DERIVED): "+strings.Join(pkgRoots, ", "))
		}
	default:
		return GitScopeGraph{}, nil, core.NewError(core.ReasonWorktreeNotGitRepo, "topology.DetectGitScopeGraph",
			"no git topology under "+abs+": the root is not a repository and no sub-repositories were found")
	}
	return g, notes, nil
}

// detectPackageRoots finds directories (depth 1-2, root excluded)
// holding a package manifest.
func detectPackageRoots(root string) []string {
	var found []string
	var walk func(dir, rel string, depth int)
	walk = func(dir, rel string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, en := range entries {
			if !en.IsDir() || detectSkipDirs[en.Name()] || strings.HasPrefix(en.Name(), ".") {
				continue
			}
			childRel := en.Name()
			if rel != "" {
				childRel = rel + "/" + en.Name()
			}
			child := filepath.Join(dir, en.Name())
			for _, m := range packageManifests {
				if _, err := os.Stat(filepath.Join(child, m)); err == nil {
					found = append(found, childRel)
					break
				}
			}
			if depth < 2 {
				walk(child, childRel, depth+1)
			}
		}
	}
	walk(root, "", 1)
	sort.Strings(found)
	return found
}
