package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newDoctorRebuildTreeCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "rebuild-tree",
		Short: "Rebuild the SQLite tree cache from canonical repo JSON (never the reverse)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, _, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if findings := intenttree.ValidateIndex(idx); len(findings) > 0 {
				return fail(intenttree.ReasonInvalidIndex, "doctor rebuild-tree", strings.Join(findings, "; "))
			}
			cache := make([]store.TreeNode, 0, len(idx.Nodes))
			for _, e := range idx.Nodes {
				n, nerr := intenttree.ReadNode(dir, e.ID)
				if nerr != nil {
					return nerr
				}
				cache = append(cache, store.TreeNode{
					ID: n.ID, ParentID: n.ParentID, Kind: string(n.Kind), Path: n.Path,
					Status: string(n.Status), Title: n.Title, Fingerprint: n.Fingerprint,
					FingerprintAlg: n.FingerprintAlg, DocPath: intenttree.NodePath(dir, n.ID),
					CreatedAt: store.NowUTC(), UpdatedAt: store.NowUTC(),
				})
			}
			// Resolve the version id from the registered project/version.
			root := store.ResolveRootPath(project)
			proj, perr := st.Projects().GetByRootPath(root)
			if perr != nil {
				return perr
			}
			v, verr := resolveProjectVersion(st, proj.ID, idx.Version)
			if verr != nil {
				return verr
			}
			for i := range cache {
				cache[i].ProjectVersionID = v.ID
			}
			if err := st.ReplaceTreeNodes(v.ID, cache); err != nil {
				return err
			}
			return printJSON(cmd, map[string]any{"rebuilt": len(cache), "version": idx.Version})
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output (default)")
	return c
}
