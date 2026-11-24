package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/intenttree"
	"beacon-go/internal/store"
)

func newTruthSnapshotCmd() *cobra.Command {
	c := &cobra.Command{Use: "snapshot", Short: "Immutable version-line tree snapshots"}
	c.AddCommand(newTruthSnapshotCreateCmd(), newTruthSnapshotListCmd())
	return c
}

func newTruthSnapshotCreateCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "create",
		Short: "Write an immutable snapshot of the current frozen tree",
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
			snap, id, err := intenttree.CreateSnapshot(dir, idx.Version)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"snapshot_id": id, "snapshot": snap})
			}
			_, err = cmd.OutOrStdout().Write([]byte("snapshot: " + id + "\n"))
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}

func newTruthSnapshotListCmd() *cobra.Command {
	var project, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List immutable snapshots, newest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail("store_not_found", "truth snapshot list", "global store not found")
			}
			defer st.Close()
			dir, _, _, err := resolveTree(st, project, version)
			if err != nil {
				return err
			}
			ids, err := intenttree.ListSnapshots(dir)
			if err != nil {
				return err
			}
			_ = store.EnvStorePath
			return printJSON(cmd, map[string]any{"snapshots": ids})
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&version, "version", "", "docs version (default current)")
	c.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return c
}
