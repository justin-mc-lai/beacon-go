package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newImplementReportCmd() *cobra.Command {
	var project, version, nodeID, by, outcome, artifact, reason string
	var asJSON bool
	c := &cobra.Command{
		Use:   "report",
		Short: "Report a frozen leaf's execution (done or failed) with an artifact",
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
			if nodeID == "" {
				return fail("tree_node_required", "implement report", "--node is required")
			}
			if outcome == "" {
				outcome = "done"
			}
			rep, err := intenttree.ReportImplementation(st, dir, nodeID, by, outcome, artifact, reason)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			_, err = cmd.OutOrStdout().Write([]byte("implement: " + rep.NodeID + " " + rep.Status + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "touchpoint node id (required)")
	c.Flags().StringVar(&by, "by", "", "executor identity (required)")
	c.Flags().StringVar(&outcome, "outcome", "done", "done or failed")
	c.Flags().StringVar(&artifact, "artifact", "", "artifact path (required for done)")
	c.Flags().StringVar(&reason, "reason", "", "failure reason code/message")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
