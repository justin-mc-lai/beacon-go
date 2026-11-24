package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/intenttree"
)

type proposedChild struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

type expandProposal struct {
	Children []proposedChild `json:"children"`
}

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
			var proposal expandProposal
			if err := json.Unmarshal(raw, &proposal); err != nil {
				return core.NewError("tree_proposal_invalid", "plan expand", err.Error())
			}
			if len(proposal.Children) == 0 {
				return core.NewError("tree_proposal_empty", "plan expand", "children must not be empty")
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
			for _, ch := range proposal.Children {
				if err := intenttree.ValidateChild(parent.Kind, intenttree.Kind(ch.Kind)); err != nil {
					_ = printJSON(cmd, map[string]any{
						"allowed": false, "parent_id": nodeID, "reason_code": intenttree.ReasonInvalidChildKind,
						"detail": ch.Kind + " under " + string(parent.Kind),
					})
					return err
				}
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
