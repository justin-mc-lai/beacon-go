package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

// newPlanExpandCmd validates an LLM/host expansion proposal against the
// schema DAG. It is read-only: the caller writes node files after a
// successful check, never the kernel.
func newPlanExpandCmd() *cobra.Command {
	var project, version, nodeID, childrenPath string
	var asJSON bool
	c := &cobra.Command{
		Use:   "expand",
		Short: "Validate a child-node proposal against the tree schema DAG (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", "plan expand", "global store not found")
			}
			defer st.Close()
			dir, _, idx, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			if nodeID == "" {
				return fail("tree_node_required", "plan expand", "--node is required")
			}
			var parent intenttree.IndexEntry
			found := false
			for _, n := range idx.Nodes {
				if n.ID == nodeID {
					parent, found = n, true
					break
				}
			}
			if !found {
				return fail(intenttree.ReasonNodeNotFound, "plan expand", "node not found: "+nodeID)
			}
			if parent.Status != intenttree.StatusDraft {
				return fail("tree_node_not_draft", "plan expand", "only draft nodes can be expanded: "+nodeID)
			}
			if childrenPath == "" {
				return fail("tree_children_required", "plan expand", "--children <file.json> is required")
			}
			raw, err := os.ReadFile(childrenPath)
			if err != nil {
				return err
			}
			var proposal intenttree.Proposal
			if err := json.Unmarshal(raw, &proposal); err != nil {
				return core.NewError("tree_proposal_invalid", "plan expand", err.Error())
			}
			existing := 0
			for _, n := range idx.Nodes {
				if n.ParentID == nodeID {
					existing++
				}
			}
			if existing+len(proposal.Children) > intenttree.MaxChildren {
				return core.NewError(intenttree.ReasonChildLimit, "plan expand",
					fmt.Sprintf("existing %d + proposed %d exceeds the per-expansion cap %d",
						existing, len(proposal.Children), intenttree.MaxChildren))
			}
			if err := intenttree.ValidateProposal(parent.Kind, proposal); err != nil {
				_ = printJSON(cmd, map[string]any{
					"allowed": false, "parent_id": nodeID, "detail": err.Error(),
				})
				return err
			}
			_ = dir
			_ = asJSON
			return printJSON(cmd, map[string]any{
				"allowed": true, "parent_id": nodeID, "proposed": proposal.Children,
				"reason_code": "",
			})
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "parent node id (required)")
	c.Flags().StringVar(&childrenPath, "children", "", "JSON file with a children[] proposal")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output (default)")
	return c
}

func newPlanApplyCmd() *cobra.Command {
	var project, version, nodeID, proposalPath string
	var asJSON bool
	c := &cobra.Command{
		Use:   "apply",
		Short: "Apply a validated child-node proposal to the canonical tree",
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
				return fail("tree_node_required", "plan apply", "--node is required")
			}
			if proposalPath == "" {
				return fail("tree_proposal_required", "plan apply", "--proposal <file.json> is required")
			}
			raw, err := os.ReadFile(proposalPath)
			if err != nil {
				return err
			}
			var p intenttree.Proposal
			if err := json.Unmarshal(raw, &p); err != nil {
				return core.NewError("tree_proposal_invalid", "plan apply", err.Error())
			}
			nodes, err := intenttree.ApplyProposal(st, dir, ver.ID, nodeID, p)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"applied": len(nodes), "parent_id": nodeID, "nodes": nodes})
			}
			for _, n := range nodes {
				fmt.Fprintf(cmd.OutOrStdout(), "draft: %s %s (%s)\n", n.ID, n.Title, n.Kind)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "parent node id (required)")
	c.Flags().StringVar(&proposalPath, "proposal", "", "JSON proposal file (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
