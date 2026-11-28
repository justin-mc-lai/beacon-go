package intenttree

import (
	"beacon-go/internal/core"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

var lineageVersionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// resolveTreePath supports direct service callers that pass a delivery tree
// path, while keeping arbitrary legacy tree locations intact.
func resolveTreePath(dir string) (string, error) {
	clean := filepath.Clean(dir)
	versionDir := filepath.Dir(clean)
	beacon := filepath.Dir(versionDir)
	if filepath.Base(clean) != "tree" || filepath.Base(beacon) != "beacon" || filepath.Base(filepath.Dir(beacon)) != "docs" {
		return dir, nil
	}
	root := filepath.Dir(filepath.Dir(beacon))
	docs, err := filepath.Rel(root, versionDir)
	if err != nil {
		return "", err
	}
	return ResolveTreeLocation(root, docs)
}

// ResolveTreeLocation resolves a delivery baseline reference without relabeling
// the canonical version or IDs. Multiple authorities and corrupt aliases fail.
func ResolveTreeLocation(root, docsRoot string) (string, error) {
	direct := filepath.Join(root, docsRoot, "tree")
	version := filepath.Base(filepath.Clean(docsRoot))
	if !lineageVersionPattern.MatchString(version) {
		return direct, nil
	}
	candidates, err := filepath.Glob(filepath.Join(root, "docs", "beacon", "*", "tree", "lineage", version+".json"))
	if err != nil {
		return "", err
	}
	found := ""
	for _, path := range candidates {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var rec LineagePreview
		if err = json.Unmarshal(raw, &rec); err != nil {
			return "", err
		}
		dir := filepath.Dir(filepath.Dir(path))
		if rec.Schema != "beacon-tree-delivery-lineage.v1" || rec.Preview || rec.ToVersion != version || rec.ProjectID == "" || rec.TargetVersionID == "" {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "invalid delivery lineage")
		}
		preview, err := PreviewContinuation(dir, rec.FromVersion, rec.ToVersion, rec.BaselineSnapshotID)
		if err != nil {
			return "", err
		}
		if preview.BaselineSourceHash != rec.BaselineSourceHash {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "baseline hash mismatch")
		}
		if len(rec.Mapping) != len(preview.Mapping) {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "incomplete lineage mapping")
		}
		for i, m := range rec.Mapping {
			if m != preview.Mapping[i] {
				return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "lineage mapping mismatch")
			}
		}
		idx, err := ReadIndex(dir)
		if err != nil {
			return "", err
		}
		if idx.Version != rec.FromVersion {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "canonical version mismatch")
		}
		if found != "" && found != dir {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "ambiguous delivery lineage")
		}
		found = dir
	}
	if found != "" {
		if HasIndex(direct) && filepath.Clean(direct) != filepath.Clean(found) {
			return "", core.NewError(ReasonStateConflict, "intenttree.ResolveTreeLocation", "delivery has two canonical trees")
		}
		return found, nil
	}
	return direct, nil
}

func ValidateDeliveryScope(dir, projectID, versionID, version string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "lineage", version+".json"))
	if err != nil {
		return err
	}
	var rec LineagePreview
	if err = json.Unmarshal(raw, &rec); err != nil {
		return err
	}
	if rec.ProjectID != projectID || rec.TargetVersionID != versionID || rec.ToVersion != version {
		return core.NewError(ReasonStateConflict, "intenttree.ValidateDeliveryScope", "delivery registry scope mismatch")
	}
	return nil
}
