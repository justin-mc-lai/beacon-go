package intenttree

import (
	"path/filepath"
	"strconv"
	"strings"
)

// NodeSlug returns the package slug derived from one node's identity.
// Truth links win; path and title are the frozen-tree fallbacks.
func NodeSlug(truthPath, path, title string) string {
	if slug := TruthPathSlug(truthPath); slug != "" {
		return slug
	}
	if s := strings.TrimSpace(filepath.Base(path)); s != "" && s != "." {
		return s
	}
	return strings.TrimSpace(title)
}

// ModuleWorktreeSlugs returns one worktree slug per module. A collision
// suffix keeps two modules with the same node slug distinct.
func ModuleWorktreeSlugs(idx Index) map[string]string {
	base := make(map[string]string, len(idx.Nodes))
	count := make(map[string]int, len(idx.Nodes))
	for _, n := range idx.Nodes {
		if n.Kind != KindModule {
			continue
		}
		slug := NodeSlug(n.TruthPath, n.Path, n.Title)
		base[n.ID] = slug
		count[slug]++
	}
	out := make(map[string]string, len(base))
	used := make(map[string]bool, len(base))
	for _, n := range idx.Nodes {
		if n.Kind != KindModule {
			continue
		}
		slug := base[n.ID]
		if count[slug] > 1 {
			candidate := slug + "-" + strings.TrimSpace(n.ID)
			for suffix := 2; used[candidate]; suffix++ {
				candidate = slug + "-" + strings.TrimSpace(n.ID) + "-" + strconv.Itoa(suffix)
			}
			slug = candidate
		}
		used[slug] = true
		out[n.ID] = slug
	}
	return out
}

// FeaturePackageSlug returns the canonical package slug of one feature.
// A module run prefixes the node slug with its module's worktree slug;
// direct tree runs use the node slug. The body's effective truth link wins
// over the index copy when present, including legacy truth paths.
func FeaturePackageSlug(idx Index, nodeID, truthPath string) string {
	n, ok := findEntry(idx, nodeID)
	if !ok || n.Kind != KindFeature {
		return ""
	}
	if strings.TrimSpace(truthPath) == "" {
		truthPath = n.TruthPath
	}
	base := NodeSlug(truthPath, n.Path, n.Title)
	moduleSlugs := ModuleWorktreeSlugs(idx)
	if parent, ok := findEntry(idx, n.ParentID); ok && parent.Kind == KindModule {
		if moduleSlug := moduleSlugs[parent.ID]; moduleSlug != "" && moduleSlug != base {
			return moduleSlug + "-" + base
		}
	}
	return base
}

// FeaturePackageSlugs lists every package slug that may resolve to one
// feature node. The first slug is canonical and is used for persistence;
// aliases cover legacy truth links, path basenames and titles. Gate lookup
// and rendering share this list so a derived package slug still resolves.
func FeaturePackageSlugs(idx Index, nodeID, truthPath string) []string {
	n, ok := findEntry(idx, nodeID)
	if !ok || n.Kind != KindFeature {
		return nil
	}
	canonical := FeaturePackageSlug(idx, nodeID, truthPath)
	slugs := []string{canonical}
	seen := map[string]bool{}
	if canonical != "" {
		seen[canonical] = true
	}
	add := func(slug string) {
		slug = strings.TrimSpace(slug)
		if slug == "" || seen[slug] {
			return
		}
		seen[slug] = true
		slugs = append(slugs, slug)
	}
	if strings.TrimSpace(truthPath) == "" {
		truthPath = n.TruthPath
	}
	add(NodeSlug(truthPath, n.Path, n.Title))
	add(TruthPathSlug(n.TruthPath))
	add(TruthPathSlug(truthPath))
	add(filepath.Base(n.Path))
	add(strings.TrimSpace(n.Title))
	return slugs
}
