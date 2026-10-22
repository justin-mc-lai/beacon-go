// Goal topology surfaces (S3, AC-171/172/173), folded into the
// existing `goal` group — no new top-level command:
//
//	show     the project's recorded BranchContext + live gate fact
//	project  load a v1-shaped GitScopeGraph JSON file, project it
//	         onto BranchContext and record the projection (a human
//	         supplying the graph IS the confirmation)
//	detect   probe the project root, generate a DRAFT graph and
//	         record it as an unconfirmed draft projection
//	confirm  confirm the recorded draft (trail event) so admission
//	         may consume it
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalTopologyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "topology",
		Short: "Git scope topology (AC-171/172/173): show, project, detect, confirm",
		Long: "The topology gate describes a project's git scope graph (single_repo, " +
			"monorepo or multi_git_workspace) as a BranchContext projection. Implement " +
			"admission consumes the projection's topology_gate_ref: no recorded, " +
			"confirmed projection on a git project fails topology_gate_missing; a " +
			"recorded context whose live gate is not pass fails topology_gate_blocked.",
	}
	c.AddCommand(newGoalTopologyShowCmd(), newGoalTopologyProjectCmd(),
		newGoalTopologyDetectCmd(), newGoalTopologyConfirmCmd())
	return c
}

// topologyAnchor resolves --project/--feature to the engine's
// anchor ids.
func topologyAnchor(eng *orchestrator.Engine, project, feature string) (anchorID, projectID string, err error) {
	return eng.ResolveProjectAnchor(project, feature)
}

func printTopologyContext(cmd *cobra.Command, bc orchestrator.BranchContext, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, bc)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "topology: %s ref=%s gate=%s source=%s confirmed=%v\n",
		bc.TopologyKind, bc.TopologyGateRef, bc.TopologyGateStatus, bc.Source, bc.Confirmed)
	for _, s := range bc.Scopes {
		fmt.Fprintf(out, "  %-24s %-9s writable=%v root=%s path=%s\n",
			s.ScopeID, s.Role, s.Writable, s.GitRoot, s.Path)
	}
	return nil
}

func newGoalTopologyShowCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Show the project's recorded BranchContext (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			_, projectID, err := topologyAnchor(eng, project, feature)
			if err != nil {
				return err
			}
			rec, err := eng.LatestBranchContext(projectID)
			if err != nil {
				return err
			}
			if rec == nil {
				if asJSON {
					return printJSON(cmd, map[string]any{"recorded": false,
						"detail": "no BranchContext projection recorded for this project"})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "topology: no projection recorded for this project")
				return nil
			}
			bc := rec.Context
			bc.Confirmed = rec.Confirmed
			return printTopologyContext(cmd, bc, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root path (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug to anchor on (default: the project's first feature)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	return c
}

func newGoalTopologyProjectCmd() *cobra.Command {
	var project, feature, graphPath, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "project",
		Short: "Project a GitScopeGraph JSON file onto BranchContext and record it (AC-171)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			data, err := os.ReadFile(graphPath)
			if err != nil {
				return err
			}
			g, err := orchestrator.LoadGitScopeGraph(data)
			if err != nil {
				return err
			}
			anchorID, _, err := topologyAnchor(eng, project, feature)
			if err != nil {
				return err
			}
			bc, err := eng.ProjectGitScopeGraph(anchorID, g, orchestrator.TopologySourceFile, by)
			if err != nil {
				return err
			}
			return printTopologyContext(cmd, bc, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root path (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug to anchor the projection on")
	c.Flags().StringVar(&graphPath, "graph", "", "path to a GitScopeGraph JSON document (git-scope-graph.v1) (required)")
	c.Flags().StringVar(&by, "by", "", "who projects (recorded on the trail event)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	_ = c.MarkFlagRequired("graph")
	return c
}

func newGoalTopologyDetectCmd() *cobra.Command {
	var project, feature, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "detect",
		Short: "Probe the project root and record a DRAFT topology graph (needs confirm before admission consumes it)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			g, notes, err := orchestrator.DetectGitScopeGraph(project)
			if err != nil {
				return err
			}
			anchorID, _, err := topologyAnchor(eng, project, feature)
			if err != nil {
				return err
			}
			bc, err := eng.ProjectGitScopeGraph(anchorID, g, orchestrator.TopologySourceDraft, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"branch_context": bc, "notes": notes})
			}
			if err := printTopologyContext(cmd, bc, false); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, n := range notes {
				fmt.Fprintf(out, "  note: %s\n", n)
			}
			fmt.Fprintln(out, "  draft: unconfirmed — run `beacon goal topology confirm --project ...` before admission will consume it")
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root path (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug to anchor the draft on")
	c.Flags().StringVar(&by, "by", "", "who detects (recorded on the trail event)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	return c
}

func newGoalTopologyConfirmCmd() *cobra.Command {
	var project, feature, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "confirm",
		Short: "Confirm the project's recorded draft projection so admission may consume it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			_, projectID, err := topologyAnchor(eng, project, feature)
			if err != nil {
				return err
			}
			bc, err := eng.ConfirmBranchContext(projectID, by)
			if err != nil {
				return err
			}
			return printTopologyContext(cmd, bc, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root path (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug to anchor on")
	c.Flags().StringVar(&by, "by", "", "who confirms (recorded on the trail event)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	return c
}
