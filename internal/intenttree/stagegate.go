package intenttree

import (
	"path/filepath"
	"strings"
)

// StageGate answers one question for a goal stage: may it start, given
// the canonical tree? It is read-only and shares the same structural
// rules the direct tree commands use.
func StageGate(dir, stage, featureSlug string) (bool, string, error) {
	var err error
	dir, err = resolveTreePath(dir)
	if err != nil {
		return false, "", err
	}
	if !HasIndex(dir) {
		return true, "", nil // legacy project: no tree, no extra gate
	}
	idx, err := ReadIndex(dir)
	if err != nil {
		return false, "", err
	}
	if findings := ValidateIndex(idx); len(findings) > 0 {
		return false, ReasonInvalidIndex, nil
	}
	if stage == "plan" {
		if len(idx.Nodes) == 0 {
			return false, "tree_empty", nil
		}
		return true, "", nil
	}
	feature, ok := featureEntry(dir, idx, featureSlug)
	if !ok {
		return false, "tree_feature_not_found", nil
	}
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	var leaves []IndexEntry
	var walk func(string)
	walk = func(id string) {
		for _, ch := range children[id] {
			if ch.Status == StatusDeprecated {
				continue
			}
			if ch.Kind == KindTouchpoint {
				leaves = append(leaves, ch)
			}
			walk(ch.ID)
		}
	}
	walk(feature.ID)
	switch stage {
	case "truth":
		if feature.Status == StatusDraft || feature.Status == StatusChangePending {
			return false, "tree_feature_not_frozen", nil
		}
	case "implement":
		if feature.Status != StatusFrozen && feature.Status != StatusImplemented && feature.Status != StatusVerified && feature.Status != StatusDelivered {
			return false, "tree_feature_not_frozen", nil
		}
		for _, l := range leaves {
			if l.Status == StatusDraft || l.Status == StatusChangePending {
				return false, "tree_leaf_not_frozen", nil
			}
		}
	case "qa":
		for _, l := range leaves {
			if l.Status != StatusVerified && l.Status != StatusDelivered {
				return false, "tree_leaf_not_verified", nil
			}
		}
	case "release":
		root, ok := findEntry(idx, idx.RootID)
		if !ok || (root.Status != StatusVerified && root.Status != StatusDelivered) {
			return false, "tree_root_not_verified", nil
		}
	}
	return true, "", nil
}

// TruthPathSlug returns the package directory slug of a truth.md path.
func TruthPathSlug(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.Base(path) != "truth.md" {
		return ""
	}
	return filepath.Base(filepath.Dir(path))
}

func featureEntry(dir string, idx Index, slug string) (IndexEntry, bool) {
	slug = strings.TrimSpace(slug)
	for _, n := range idx.Nodes {
		if n.Kind != KindFeature {
			continue
		}
		truthPath := n.TruthPath
		if body, err := ReadNode(dir, n.ID); err == nil {
			if effective := body.EffectiveTruthPath(); effective != "" {
				truthPath = effective
			}
		}
		for _, candidate := range FeaturePackageSlugs(idx, n.ID, truthPath) {
			if candidate == slug {
				return n, true
			}
		}
	}
	return IndexEntry{}, false
}
