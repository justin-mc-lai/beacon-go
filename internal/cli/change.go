package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// newChangeCmd is the single entry for every change to frozen truth.
// `request`/`list` operate on the canonical intent tree; `legacy` keeps
// the pre-tree feature-package path reachable under the one ability,
// so "change" has exactly one command group.
func newChangeCmd() *cobra.Command {
	c := &cobra.Command{Use: "change", Short: "Change frozen truth: invalidate a tree subtree or a legacy package"}
	c.AddCommand(newChangeRequestCmd(), newChangeListCmd(), newTruthChangeCmd(), newChangeDeprecateCmd(), newChangeGrowCmd(), newChangeContinueCmd(), newChangeReconcileCmd(), newChangeMoveCmd())
	return c
}

func newChangeRequestCmd() *cobra.Command {
	var project, version, nodeID, summary, by, base string
	var asJSON bool
	c := &cobra.Command{
		Use:   "request",
		Short: "Invalidate one frozen subtree; ancestors become change_pending",
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
			proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
			if err != nil {
				return err
			}
			ver, err := resolveProjectVersion(st, proj.ID, idx.Version)
			if err != nil {
				return err
			}
			if nodeID == "" {
				return fail("tree_node_required", "change request", "--node is required")
			}
			rec, err := intenttree.RequestChange(st, dir, ver.ID, nodeID, summary, by, base)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rec)
			}
			_, err = cmd.OutOrStdout().Write([]byte("change " + rec.ID + ": " + rec.NodeID + " (" + rec.Summary + ")\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "subtree root node id (required)")
	c.Flags().StringVar(&summary, "summary", "", "what is changing (required)")
	c.Flags().StringVar(&by, "by", "", "who requests the change")
	c.Flags().StringVar(&base, "base-fingerprint", "", "required optimistic-concurrency base fingerprint")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func newChangeListCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List change records, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", "change list", "global store not found")
			}
			defer st.Close()
			dir, _, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			recs, err := intenttree.ListChanges(dir)
			if err != nil {
				return err
			}
			return printJSON(cmd, map[string]any{"changes": recs})
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON (default)")
	return c
}
