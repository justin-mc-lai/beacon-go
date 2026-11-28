package cli

import (
	"beacon-go/internal/intenttree"
	"path/filepath"
)

func unlinkedFeatureSlugs(dir string, idx intenttree.Index) ([]string, error) {
	out := []string{}
	for _, e := range idx.Nodes {
		if e.Kind != intenttree.KindFeature {
			continue
		}
		n, err := intenttree.ReadNode(dir, e.ID)
		if err != nil {
			return nil, err
		}
		if n.EffectiveTruthPath() == "" {
			out = append(out, filepath.Base(e.Path))
		}
	}
	return out, nil
}
