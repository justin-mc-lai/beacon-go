package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
)

func newPlanDriftCmd() *cobra.Command {
	var project, version, nodeID, snapshotID string
	var asJSON bool
	var agentsMD bool
	var agentsMap string
	c := &cobra.Command{
		Use:   "drift",
		Short: "Report requirement-level drift against one immutable snapshot (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentsMD && cmd.Flags().Changed("snapshot") {
				return fmt.Errorf("--snapshot is not used by --agents-md; canonical comparison does not use snapshots")
			}
			if !agentsMD && cmd.Flags().Changed("agents-map") {
				return fmt.Errorf("--agents-map requires --agents-md")
			}
			if cmd.Flags().Changed("agents-map") && strings.TrimSpace(agentsMap) == "" {
				return fmt.Errorf("agents_md_drift_invalid: explicit --agents-map must not be blank")
			}
			dir, _, idx, err := resolveTreeRead(project, version, "plan drift")
			if err != nil {
				return err
			}
			driftRootID := nodeID
			if driftRootID == "" {
				driftRootID = idx.RootID
			}
			if agentsMD {
				mapPath := agentsMap
				if mapPath == "" {
					mapPath = "docs/agents-md-drift-map.json"
				}
				if mapPath != "" && !filepath.IsAbs(mapPath) {
					mapPath = filepath.Join(project, mapPath)
				}
				rep, err := intenttree.AgentsMDDrift(dir, driftRootID, filepath.Join(project, "AGENTS.md"), mapPath)
				if err != nil {
					return err
				}
				if asJSON {
					if err := printJSON(cmd, rep); err != nil {
						return err
					}
					if rep.Blocked > 0 {
						return fmt.Errorf("agents_md_drift_blocked: %d product items lack canonical contracts", rep.Blocked)
					}
					return nil
				}
				fmt.Fprintf(cmd.OutOrStdout(), "node: %s\nevaluated %d · changed %d · blocked %d\n", rep.NodeID, rep.Evaluated, len(rep.Changed), rep.Blocked)
				for _, e := range rep.Changed {
					fmt.Fprintf(cmd.OutOrStdout(), "changed %s %s\n", e.ID, e.Path)
				}
				for _, item := range rep.Items {
					if item.Status == "blocked" {
						fmt.Fprintf(cmd.OutOrStdout(), "blocked item %d: %s\n", item.Item, item.Reason)
					}
				}
				if rep.Blocked > 0 {
					return fmt.Errorf("agents_md_drift_blocked: %d product items lack canonical contracts", rep.Blocked)
				}
				return nil
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
	c.Flags().BoolVar(&agentsMD, "agents-md", false, "compare numbered AGENTS.md product requirements (read-only)")
	c.Flags().StringVar(&agentsMap, "agents-map", "", "item-to-canonical-field JSON mapping (default docs/agents-md-drift-map.json)")
	return c
}
