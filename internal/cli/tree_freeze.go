package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newTruthNodeCmd() *cobra.Command {
	c := &cobra.Command{Use: "node", Short: "Node-level operations"}
	c.AddCommand(newTruthNodeFreezeCmd(), newTruthNodeUpdateCmd())
	return c
}

func newTruthNodeFreezeCmd() *cobra.Command {
	var project, version, nodeID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "freeze",
		Short: "Freeze one node after its exit conditions hold",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, projRoot, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			_ = projRoot
			proj, err := st.Projects().GetByRootPath(store.ResolveRootPath(project))
			if err != nil {
				return err
			}
			ver, err := resolveProjectVersion(st, proj.ID, idx.Version)
			if err != nil {
				return err
			}
			if nodeID == "" {
				return fail("tree_node_required", "truth node freeze", "--node is required")
			}
			rep, err := intenttree.FreezeNode(st, dir, ver.ID, nodeID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			verb := "frozen"
			if rep.Already {
				verb = "already frozen"
			}
			_, err = cmd.OutOrStdout().Write([]byte(verb + ": " + rep.NodeID + " (" + rep.Status + ")\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "node id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
