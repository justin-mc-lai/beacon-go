package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newTruthNodeReopenCmd() *cobra.Command {
	var project, version, nodeID, target, by, reason string
	var asJSON bool
	c := &cobra.Command{
		Use:   "reopen",
		Short: "Demote one node along verified -> implemented -> frozen (claims only, never the body)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			dir, _, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if nodeID == "" || target == "" || by == "" {
				return fail("tree_reopen_required", "truth node reopen",
					"--node, --to and --by are required")
			}
			rep, err := intenttree.ReopenNode(st, dir, nodeID, target, by, reason)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			verb := "reopened"
			if rep.Already {
				verb = "already"
			}
			_, err = cmd.OutOrStdout().Write([]byte(verb + ": " + rep.NodeID + " " + rep.Status + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "node id (required)")
	c.Flags().StringVar(&target, "to", "", "target status: frozen | implemented (required)")
	c.Flags().StringVar(&by, "by", "", "operator identity (required)")
	c.Flags().StringVar(&reason, "reason", "", "why the claim is withdrawn")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
