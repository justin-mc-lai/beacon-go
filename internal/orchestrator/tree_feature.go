package orchestrator

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"beacon-go/internal/core"
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
	dir, err := intenttree.ResolveTreeLocation(proj.RootPath, ver.DocsRoot)
	if err != nil {
		return store.FeatureContext{}, false, err
	}
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store.FeatureContext{}, false, nil
		}
		return store.FeatureContext{}, false, err
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
	ctx, err := e.ensureDerivedFeature(proj, ver, slug, chosen.ID, body)
	if err != nil {
		return store.FeatureContext{}, false, err
	}
	return ctx, true, nil
}

// ensureDerivedFeature is the single materialization entry for a
// tree-derived feature package. Callers decide which tree node owns the
// slug; this helper owns the file and store writes, so a re-run never
// overwrites an existing derived truth/tests body.
func (e *Engine) ensureDerivedFeature(proj store.Project, ver store.ProjectVersion, slug, nodeID string, body intenttree.Node) (store.FeatureContext, error) {
	// A package row may already exist; re-resolve it first.
	if ctx, err := e.store.ResolveFeature(proj.RootPath, slug, ver.Version); err == nil {
		if err := e.reconcileDerivedFeature(proj, ctx, nodeID, body); err != nil {
			return store.FeatureContext{}, err
		}
		return ctx, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.FeatureContext{}, err
	}
	pkgDir := filepath.Join(ver.DocsRoot, "features", slug)
	truthRel := filepath.ToSlash(filepath.Join(pkgDir, "truth.md"))
	testsRel := filepath.ToSlash(filepath.Join(pkgDir, "tests.md"))
	truthAbs := filepath.Join(proj.RootPath, filepath.FromSlash(truthRel))
	testsAbs := filepath.Join(proj.RootPath, filepath.FromSlash(testsRel))
	if _, statErr := os.Stat(truthAbs); errors.Is(statErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(truthAbs), 0o755); err != nil {
			return store.FeatureContext{}, err
		}
		front := derivedFrontmatter(slug, ver.Version, nodeID, body)
		if err := os.WriteFile(truthAbs, []byte(front+intenttree.GenerateDoc(body)), 0o644); err != nil {
			return store.FeatureContext{}, err
		}
	}
	if _, statErr := os.Stat(testsAbs); errors.Is(statErr, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(testsAbs), 0o755); err != nil {
			return store.FeatureContext{}, err
		}
		if err := os.WriteFile(testsAbs, []byte("# Tests (tree-derived)\n\n| AC | TC | Kind | Selector | Assertion |\n|---|---|---|---|---|\n"), 0o644); err != nil {
			return store.FeatureContext{}, err
		}
	}
	pkg, err := e.store.Features().CreatePackage(store.FeaturePackage{
		ProjectVersionID: ver.ID, Slug: slug, TruthPath: truthRel, TestsPath: testsRel, Status: "derived",
	})
	if err != nil {
		// A concurrent creator may have won; resolve it.
		if ctx, rerr := e.store.ResolveFeature(proj.RootPath, slug, ver.Version); rerr == nil {
			return ctx, nil
		}
		return store.FeatureContext{}, err
	}
	return store.FeatureContext{Project: proj, Version: ver, Feature: pkg}, nil
}

// packageStatusForNode maps the tree lifecycle onto the package
// frontmatter. A scaffold is always draft: the generated placeholder is
// not a frozen requirement, whatever state the tree node claims.
func packageStatusForNode(n intenttree.Node) (status, maturity string) {
	if !intenttree.DocReady(n) {
		return "draft", "scaffold"
	}
	if moduleTreeStatusFrozen(n.Status) {
		return "frozen", "complete"
	}
	return "draft", "complete"
}

func derivedFrontmatter(slug, version, nodeID string, body intenttree.Node) string {
	status, maturity := packageStatusForNode(body)
	return "---\nslug: " + slug + "\nversion: " + version +
		"\nstatus: " + status + "\npackage_maturity: " + maturity +
		"\nsource: tree\ntree_node_id: " + nodeID +
		"\nparser_contract: beacon-feature-package-v2\ntruth_source_model: tree_derived\n---\n"
}

// reconcileDerivedFeature keeps an already-materialized derived package
// honest. It never rewrites a package the host agent filled by hand; it
// only (a) refuses a package bound to a different tree node and
// (b) upgrades the untouched generated skeleton from the node body.
func (e *Engine) reconcileDerivedFeature(proj store.Project, ctx store.FeatureContext, nodeID string, body intenttree.Node) error {
	if ctx.Feature.TruthPath == "" {
		return nil
	}
	abs := filepath.Join(proj.RootPath, filepath.FromSlash(ctx.Feature.TruthPath))
	raw, err := os.ReadFile(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	meta := simpleFrontmatter(string(raw))
	if got := meta["tree_node_id"]; got != "" && got != nodeID {
		return core.NewError("tree_feature_package_mismatch", "orchestrator.ensureDerivedFeature",
			"package "+ctx.Feature.Slug+" is bound to tree node "+got+", not "+nodeID)
	}
	if meta["truth_source_model"] != "tree_derived" || !isGeneratedScaffold(string(raw)) {
		return nil
	}
	front := derivedFrontmatter(ctx.Feature.Slug, ctx.Version.Version, nodeID, body)
	return os.WriteFile(abs, []byte(front+intenttree.GenerateDoc(body)), 0o644)
}

// simpleFrontmatter reads the flat `key: value` pairs of a leading
// frontmatter block. Nested keys are ignored; the derived contract only
// uses flat scalars.
func simpleFrontmatter(text string) map[string]string {
	out := map[string]string{}
	if !strings.HasPrefix(text, "---\n") {
		return out
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return out
	}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func isGeneratedScaffold(text string) bool {
	return strings.Contains(text, "[待补充") &&
		strings.Contains(text, "生成骨架：语义正文与真实测试由宿主 agent 补全")
}

// DerivedFeatureSync is the outcome of materializing one feature node's
// derived package from the canonical tree.
type DerivedFeatureSync struct {
	NodeID    string `json:"node_id"`
	Slug      string `json:"slug"`
	TruthPath string `json:"truth_path"`
	Status    string `json:"status"`
	Maturity  string `json:"package_maturity"`
	Legacy    bool   `json:"legacy_package,omitempty"`
}

// SyncDerivedFeature is the converge entry for one feature node: it
// resolves the canonical package slug and lets ensureDerivedFeature
// materialize or repair the package. A legacy package linked by
// truth_path is left untouched (it is not tree-derived).
func (e *Engine) SyncDerivedFeature(projectRoot, version, nodeID string) (DerivedFeatureSync, error) {
	proj, err := e.store.Projects().GetByRootPath(store.ResolveRootPath(projectRoot))
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	vers, err := e.store.Projects().ListVersions(proj.ID)
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	ver, ok := pickVersion(vers, version)
	if !ok {
		return DerivedFeatureSync{}, core.NewError(intenttree.ReasonVersionNotFound, "orchestrator.SyncDerivedFeature",
			"version not found: "+version)
	}
	dir, err := intenttree.ResolveTreeLocation(proj.RootPath, ver.DocsRoot)
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	idx, err := intenttree.ReadIndex(dir)
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	entry, found := intenttree.IndexEntry{}, false
	for _, en := range idx.Nodes {
		if en.ID == nodeID {
			entry, found = en, true
			break
		}
	}
	if !found {
		return DerivedFeatureSync{}, core.NewError(intenttree.ReasonNodeNotFound, "orchestrator.SyncDerivedFeature",
			"node not found: "+nodeID)
	}
	if entry.Kind != intenttree.KindFeature {
		return DerivedFeatureSync{}, core.NewError(intenttree.ReasonInvalidChildKind, "orchestrator.SyncDerivedFeature",
			"only feature nodes own a package: "+nodeID+" is "+string(entry.Kind))
	}
	body, err := intenttree.ReadNode(dir, nodeID)
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	slug := intenttree.FeaturePackageSlug(idx, nodeID, body.EffectiveTruthPath())
	if slug == "" {
		return DerivedFeatureSync{}, core.NewError(intenttree.ReasonChildSlugMissing, "orchestrator.SyncDerivedFeature",
			"feature has no package slug: "+nodeID)
	}
	ctx, err := e.ensureDerivedFeature(proj, ver, slug, nodeID, body)
	if err != nil {
		return DerivedFeatureSync{}, err
	}
	status, maturity := packageStatusForNode(body)
	out := DerivedFeatureSync{NodeID: nodeID, Slug: ctx.Feature.Slug, TruthPath: ctx.Feature.TruthPath,
		Status: status, Maturity: maturity}
	if ctx.Feature.TruthPath != "" {
		abs := filepath.Join(proj.RootPath, filepath.FromSlash(ctx.Feature.TruthPath))
		if raw, rerr := os.ReadFile(abs); rerr == nil {
			meta := simpleFrontmatter(string(raw))
			out.Legacy = meta["truth_source_model"] != "tree_derived"
			if meta["status"] != "" {
				out.Status = meta["status"]
			}
			if meta["package_maturity"] != "" {
				out.Maturity = meta["package_maturity"]
			}
		}
	}
	return out, nil
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
