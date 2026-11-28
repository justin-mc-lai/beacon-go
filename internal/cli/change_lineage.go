package cli

import (
	"beacon-go/internal/intenttree"
	"github.com/spf13/cobra"
)

func newChangeContinueCmd() *cobra.Command {
	var project, version, to, snapshot string
	var apply bool
	var base, previous, by, summary string
	c := &cobra.Command{Use: "continue", Aliases: []string{"derive"}, Short: "Preview or persist a delivery baseline reference for the existing logical tree", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		st, exists, err := openReadStore()
		if err != nil {
			return err
		}
		if !exists {
			return fail("store_not_found", "change continue", "store not found")
		}
		defer st.Close()
		dir, _, idx, err := resolveTree(st, project, version)
		if err != nil {
			return err
		}
		if apply {
			result, err := intenttree.ApplyContinuation(st, project, dir, idx.Version, to, snapshot, base, previous, by, summary)
			if err != nil {
				return err
			}
			return printJSON(cmd, result)
		}
		result, err := intenttree.PreviewContinuation(dir, idx.Version, to, snapshot)
		if err != nil {
			return err
		}
		return printJSON(cmd, result)
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "explicit predecessor docs version")
	c.Flags().StringVar(&to, "to-version", "", "explicit registered delivery version for apply")
	c.Flags().StringVar(&snapshot, "snapshot", "", "explicit frozen snapshot ID; latest refused")
	c.Flags().Bool("json", true, "emit JSON")
	c.Flags().BoolVar(&apply, "apply", false, "persist a delivery lineage manifest under the existing tree")
	c.Flags().StringVar(&base, "base-source-hash", "", "required current snapshot source hash for apply")
	c.Flags().StringVar(&previous, "previous-delivery", "", "explicit predecessor delivery version after first apply")
	c.Flags().StringVar(&by, "by", "", "requesting identity for apply")
	c.Flags().StringVar(&summary, "summary", "", "delivery summary for apply")
	for _, flag := range []string{"version", "to-version", "snapshot"} {
		_ = c.MarkFlagRequired(flag)
	}
	return c
}
