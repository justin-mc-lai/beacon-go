package cli

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

// resolveTreeRead locates the canonical tree for plan/truth views only.
// Mutation and cache maintenance still use the registered version resolver.
func resolveTreeRead(project, version, op string) (string, string, intenttree.Index, error) {
	root := store.ResolveRootPath(project)
	docs := filepath.Join(root, "docs", "beacon")
	var selected, dir string
	if version != "" && !strings.EqualFold(version, "current") {
		if strings.EqualFold(version, "latest") || strings.EqualFold(version, "auto") {
			return "", "", intenttree.Index{}, fail(intenttree.ReasonVersionNotFound, op, "pass --version explicitly; latest and auto are not supported")
		}
		var err error
		selected, err = orchestrator.NormalizeDocsVersion(version)
		if err != nil {
			return "", "", intenttree.Index{}, err
		}
		dir = filepath.Join(docs, selected, "tree")
	}

	st, exists, err := openReadStore()
	if err != nil {
		return "", "", intenttree.Index{}, err
	}
	if exists {
		defer st.Close()
		p, err := st.Projects().GetByRootPath(root)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", "", intenttree.Index{}, err
		}
		if err == nil {
			versions, err := st.Projects().ListVersions(p.ID)
			if err != nil {
				return "", "", intenttree.Index{}, err
			}
			for _, v := range versions {
				if (selected == "" && v.IsCurrent) || v.Version == selected {
					return resolveTree(st, root, v.Version)
				}
			}
		}
	}
	if selected != "" {
		return readSelectedTree(root, dir, selected, op)
	}
	if strings.EqualFold(version, "current") {
		return "", "", intenttree.Index{}, fail(intenttree.ReasonVersionNotFound, op, "no current version; pass --version explicitly")
	}

	entries, err := os.ReadDir(docs)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", "", intenttree.Index{}, err
	}
	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		norm, err := orchestrator.NormalizeDocsVersion(name)
		if err != nil || norm != name {
			continue
		}
		_, err = os.Stat(intenttree.IndexPath(filepath.Join(docs, name, "tree")))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", intenttree.Index{}, err
		}
		candidates = append(candidates, name)
	}
	switch len(candidates) {
	case 0:
		return "", "", intenttree.Index{}, fail("tree_version_not_materialized", op, "no intent tree in docs/beacon")
	case 1:
		selected = candidates[0]
		return readSelectedTree(root, filepath.Join(docs, selected, "tree"), selected, op)
	default:
		return "", "", intenttree.Index{}, fail("tree_version_ambiguous", op, "pass --version; available trees: "+strings.Join(candidates, ", "))
	}
}

func readSelectedTree(root, dir, version, op string) (string, string, intenttree.Index, error) {
	idx, err := intenttree.ReadIndex(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", "", intenttree.Index{}, fail("tree_version_not_materialized", op, "no intent tree for version "+version)
	}
	if err != nil {
		return "", "", intenttree.Index{}, err
	}
	if idx.Version != version {
		return "", "", intenttree.Index{}, fail(intenttree.ReasonInvalidIndex, op, "tree version "+idx.Version+" does not match selected version "+version)
	}
	if findings := intenttree.ValidateIndex(idx); len(findings) > 0 {
		return "", "", intenttree.Index{}, fail(intenttree.ReasonInvalidIndex, op, strings.Join(findings, "; "))
	}
	return dir, root, idx, nil
}
