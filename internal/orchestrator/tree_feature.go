package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// ensureTreeFeature resolves a goal feature from the canonical tree when
// the legacy feature_packages table has no row yet. The package row and
// Markdown are derived metadata for the run FK and legacy gates; the tree
// node remains the requirement authority.
func (e *Engine) ensureTreeFeature(projectRoot, version, slug string) (store.FeatureContext, bool, error) {
	proj, err := e.store.Projects().GetByRootPath(store.ResolveRootPath(projectRoot))
	if err != nil {
		return store.FeatureContext{}, false, nil
	}
	vers, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return store.FeatureContext{}, false, err
	}
	ver, ok := pickVersion(vers, version)
	if !ok {
		return store.FeatureContext{}, false, nil
	}
	dir := intenttree.TreeDir(proj.RootPath, ver.DocsRoot)
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		return store.FeatureContext{}, false, nil
	}
	features := make([]intenttree.IndexEntry, 0)
	seen := map[string]bool{}
	for _, en := range idx.Nodes {
		if en.Kind != intenttree.KindFeature || seen[en.ID] {
			continue
		}
		seen[en.ID] = true
		if slug == "" || filepath.Base(en.Path) == slug || en.Title == slug || intenttree.TruthPathSlug(en.TruthPath) == slug {
			features = append(features, en)
		}
	}
	if len(features) == 0 {
		return store.FeatureContext{}, false, nil
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Path < features[j].Path })
	chosen := features[0]
	body, err := intenttree.ReadNode(dir, chosen.ID)
	if err != nil {
		return store.FeatureContext{}, false, err
	}
	if slug == "" {
		slug = filepath.Base(chosen.Path)
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return store.FeatureContext{}, false, nil
	}
	// A package row may already exist; re-resolve it first.
	if ctx, rerr := e.store.ResolveFeature(proj.RootPath, slug, ver.Version); rerr == nil {
		return ctx, true, nil
	}
	pkgDir := filepath.Join(ver.DocsRoot, "features", slug)
	truthRel := filepath.ToSlash(filepath.Join(pkgDir, "truth.md"))
	testsRel := filepath.ToSlash(filepath.Join(pkgDir, "tests.md"))
	truthAbs := filepath.Join(proj.RootPath, filepath.FromSlash(truthRel))
	testsAbs := filepath.Join(proj.RootPath, filepath.FromSlash(testsRel))
	if _, statErr := os.Stat(truthAbs); errors.Is(statErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(truthAbs), 0o755); err != nil {
			return store.FeatureContext{}, false, err
		}
		front := "---\nslug: " + slug + "\nversion: " + ver.Version +
			"\nstatus: frozen\nsource: tree\ntree_node_id: " + chosen.ID +
			"\nparser_contract: beacon-feature-package-v2\ntruth_source_model: tree_derived\n---\n"
		if err := os.WriteFile(truthAbs, []byte(front+intenttree.GenerateDoc(body)), 0o644); err != nil {
			return store.FeatureContext{}, false, err
		}
	}
	if _, statErr := os.Stat(testsAbs); errors.Is(statErr, os.ErrNotExist) {
		_ = os.MkdirAll(filepath.Dir(testsAbs), 0o755)
		_ = os.WriteFile(testsAbs, []byte("# Tests (tree-derived)\n\n| AC | TC | Kind | Selector | Assertion |\n|---|---|---|---|---|\n"), 0o644)
	}
	pkg, err := e.store.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: ver.ID, Slug: slug, TruthPath: truthRel, TestsPath: testsRel, Status: "derived",
	})
	if err != nil {
		// A concurrent creator may have won; resolve it.
		if ctx, rerr := e.store.ResolveFeature(proj.RootPath, slug, ver.Version); rerr == nil {
			return ctx, true, nil
		}
		return store.FeatureContext{}, false, err
	}
	return store.FeatureContext{Project: proj, Version: ver, Feature: pkg}, true, nil
}

func pickVersion(vers []store.ProjectVersion, want string) (store.ProjectVersion, bool) {
	want = strings.TrimSpace(want)
	for _, v := range vers {
		if (want == "" || want == "current") && v.IsCurrent {
			return v, true
		}
		if want != "" && v.Version == want {
			return v, true
		}
	}
	return store.ProjectVersion{}, false
}
