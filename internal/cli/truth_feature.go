package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/orchestrator"
)

func newTruthFeatureCmd() *cobra.Command {
	c := &cobra.Command{Use: "feature", Short: "Feature-package maintenance (tree -> derived package)"}
	c.AddCommand(newTruthFeatureSyncCmd())
	return c
}

func newTruthFeatureSyncCmd() *cobra.Command {
	var project, version, nodeID string
	var all, asJSON bool
	c := &cobra.Command{
		Use:   "sync",
		Short: "Materialize or repair the derived package of one/all feature nodes",
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
			targets := []string{nodeID}
			if all || nodeID == "" {
				targets = targets[:0]
				for _, e := range idx.Nodes {
					if e.Kind == intenttree.KindFeature {
						targets = append(targets, e.ID)
					}
				}
			}
			_ = dir
			eng := orchestrator.NewEngine(st)
			out := make([]orchestrator.DerivedFeatureSync, 0, len(targets))
			for _, id := range targets {
				rep, err := eng.SyncDerivedFeature(project, version, id)
				if err != nil {
					return err
				}
				out = append(out, rep)
			}
			if asJSON {
				return printJSON(cmd, out)
			}
			for _, r := range out {
				legacy := ""
				if r.Legacy {
					legacy = " (legacy)"
				}
				cmd.OutOrStdout().Write([]byte(r.NodeID + " -> " + r.Slug + " [" + r.Status + "/" + r.Maturity + "]" + legacy + "\n"))
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "feature node id (default: all features)")
	c.Flags().BoolVar(&all, "all", false, "sync every feature node")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
