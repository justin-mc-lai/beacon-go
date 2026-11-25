package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newTruthNodeUpdateCmd() *cobra.Command {
	var project, version, nodeID, file string
	var asJSON bool
	c := &cobra.Command{
		Use:   "update",
		Short: "Update a draft/change_pending node body (title, intent, ACs, verification, feature FSM)",
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
			if nodeID == "" || file == "" {
				return fail("tree_update_required", "truth node update", "--node and --file are required")
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			var patch intenttree.NodePatch
			if err := json.Unmarshal(raw, &patch); err != nil {
				return err
			}
			n, err := intenttree.UpdateNode(st, dir, nodeID, patch)
			if err != nil {
				return err
			}
			_ = ver
			if asJSON {
				return printJSON(cmd, n)
			}
			_, err = cmd.OutOrStdout().Write([]byte("updated: " + n.ID + " " + string(n.Status) + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().StringVar(&nodeID, "node", "", "node id (required)")
	c.Flags().StringVar(&file, "file", "", "JSON patch file (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
