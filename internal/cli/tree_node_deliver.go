package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newTruthNodeDeliverCmd() *cobra.Command {
	var project, version, nodeID, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "deliver",
		Short: "Mark a verified node delivered (terminal; explicit human/host act)",
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
			if nodeID == "" || by == "" {
				return fail("tree_delivery_missing", "truth node deliver", "--node and --by are required")
			}
			n, err := intenttree.MarkDelivered(st, dir, nodeID, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, n)
			}
			_, err = cmd.OutOrStdout().Write([]byte("delivered: " + n.ID + " (" + string(n.Status) + ")\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "node id (required)")
	c.Flags().StringVar(&by, "by", "", "who confirms delivery (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
