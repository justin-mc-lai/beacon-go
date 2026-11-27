package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalModuleCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "module",
		Short: "Expand one intent-tree module into ordered feature runs",
	}
	c.AddCommand(newGoalModuleRunCmd())
	return c
}

func newGoalModuleRunCmd() *cobra.Command {
	var project, version, nodeID, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "run",
		Short: "Create one goal run per feature under a module node",
		Long: "Expand a frozen module node in declaration order. Every run carries\n" +
			"the module's pool plan; later runs stay pending at plan until their\n" +
			"predecessor leaves that stage.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			runs, err := eng.ModuleRun(orchestrator.ModuleRunOptions{
				ProjectRoot: project, Version: version, NodeID: nodeID, By: by,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, runs)
			}
			for _, r := range runs {
				fmt.Fprintf(cmd.OutOrStdout(), "run: %s feature: %s node: %s position: %d state: %s\n",
					r.RunID, r.FeatureSlug, r.FeatureNodeID, r.Position, r.State)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "module node id (required)")
	c.Flags().StringVar(&by, "by", "", "executor identity recorded on the runs")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON array")
	_ = c.MarkFlagRequired("node")
	return c
}
