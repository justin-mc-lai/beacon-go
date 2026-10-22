// Doctor commands (P3b): `beacon doctor rebuild` — projection rebuild
// against on-disk facts (AC-011). The logic lives in
// internal/migration; this file is only the CLI surface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/migration"
	"beacon-go/internal/store"
)

func newDoctorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnostics and projection maintenance",
	}
	c.AddCommand(newDoctorRebuildCmd())
	return c
}

func newDoctorRebuildCmd() *cobra.Command {
	var project string
	var apply, asJSON bool
	c := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild a project's store index from its on-disk MD + v1 JSON facts (AC-011)",
		Long: "Compare the store index of a registered project with its on-disk feature packages.\n" +
			"Default: report drift only (missing_in_store / missing_on_disk / status_mismatch).\n" +
			"--apply fills missing packages and derived lifecycles and corrects package status,\n" +
			"appending a lifecycle_events row per correction. Existing lifecycle snapshots and\n" +
			"gate results are never overwritten, and store rows are never deleted.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			var rep *migration.RebuildReport
			if apply {
				st, err := openWriteStore()
				if err != nil {
					return err
				}
				defer st.Close()
				r, err := migration.Rebuild(st, root, true)
				if err != nil {
					return err
				}
				rep = r
			} else {
				st, exists, err := openReadStore()
				if err != nil {
					return err
				}
				if !exists {
					return fail(ReasonProjectNotRegistered, "doctor rebuild", "no store yet; nothing is registered")
				}
				defer st.Close()
				r, err := migration.Rebuild(st, root, false)
				if err != nil {
					return err
				}
				rep = r
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "doctor rebuild: %s (applied=%v)\n", rep.ProjectRoot, rep.Applied)
			if len(rep.Drift) == 0 {
				fmt.Fprintln(out, "  no drift")
			}
			for _, d := range rep.Drift {
				fmt.Fprintf(out, "  drift [%s] %s/%s: %s\n", d.Kind, d.Version, d.Slug, d.Detail)
			}
			for _, c := range rep.Corrected {
				fmt.Fprintf(out, "  corrected: %s\n", c)
			}
			fmt.Fprintf(out, "  events_written=%d\n", rep.EventsWritten)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "registered project root")
	c.Flags().BoolVar(&apply, "apply", false, "apply corrections (default: report drift only)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
