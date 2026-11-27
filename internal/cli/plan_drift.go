package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newPlanDriftCmd() *cobra.Command {
	var project, version, nodeID, snapshotID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "drift",
		Short: "Report requirement-level drift against one immutable snapshot (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _, idx, err := resolveTreeRead(project, version, "plan drift")
			if err != nil {
				return err
			}
			driftRootID := nodeID
			if driftRootID == "" {
				driftRootID = idx.RootID
			}
			rep, err := intenttree.Drift(dir, driftRootID, snapshotID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "snapshot: %s\nnode: %s\n", rep.SnapshotID, rep.NodeID)
			fmt.Fprintf(cmd.OutOrStdout(), "added %d · changed %d · removed %d\n",
				len(rep.Added), len(rep.Changed), len(rep.Removed))
			for _, e := range rep.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "changed %s %s\n", e.ID, e.Path)
			}
			for _, e := range rep.Added {
				fmt.Fprintf(cmd.OutOrStdout(), "added %s %s\n", e.ID, e.Path)
			}
			for _, e := range rep.Removed {
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s %s\n", e.ID, e.Path)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "subtree root node id (default tree root)")
	c.Flags().StringVar(&snapshotID, "snapshot", "latest", "snapshot id or explicit latest key")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
