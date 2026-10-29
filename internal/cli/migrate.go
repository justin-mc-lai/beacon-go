// Migration commands (P3b): `beacon migrate v1` — the one-way v1 -> v2
// import (AC-077..079, AC-016). The importer lives in
// internal/migration; this file is only the CLI surface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/migration"
)

func newMigrateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "migrate",
		Short: "One-way data migration into the global store",
	}
	c.AddCommand(newMigrateV1Cmd())
	return c
}

func newMigrateV1Cmd() *cobra.Command {
	var path string
	var dryRun, force, asJSON bool
	c := &cobra.Command{
		Use:   "v1",
		Short: "Import a v1 project (docs/beacon/<version> packages + .machine records) into the store",
		Long: "Import a v1 project into the global store. The v1 directory is only read, never written.\n" +
			"Idempotent on natural keys; re-running imports nothing twice. If the project already holds\n" +
			"v2 runtime data the import refuses unless --force is given (with --force only missing\n" +
			"items are filled; nothing is overwritten or deleted). --dry-run scans and reports only.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			rep, impErr := migration.ImportV1(st, path, migration.Options{DryRun: dryRun, Force: force})
			if rep != nil {
				if asJSON {
					if jerr := printJSON(cmd, rep); jerr != nil {
						return jerr
					}
				} else {
					printMigrationReport(cmd, rep)
				}
			}
			return impErr
		},
	}
	c.Flags().StringVar(&path, "path", ".", "v1 project root to import")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "scan and report only; write nothing")
	c.Flags().BoolVar(&force, "force", false, "allow filling missing items when the project already holds v2 data")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func printMigrationReport(cmd *cobra.Command, rep *migration.Report) {
	out := cmd.OutOrStdout()
	mode := "import"
	if rep.DryRun {
		mode = "dry-run (nothing written)"
	}
	fmt.Fprintf(out, "migrate v1: %s [%s]\n", rep.ProjectRoot, mode)
	if rep.BackupPath != "" {
		fmt.Fprintf(out, "  backup: %s\n", rep.BackupPath)
	}
	for _, cat := range migration.Categories {
		n := rep.Counts[cat]
		fmt.Fprintf(out, "  %-18s imported=%d already_present=%d skipped=%d\n",
			cat, n.Imported, n.AlreadyPresent, n.Skipped)
	}
	for _, s := range rep.Skipped {
		fmt.Fprintf(out, "  skipped [%s] %s: %s\n", s.Category, s.Item, s.Reason)
	}
	for _, u := range rep.Uncertain {
		fmt.Fprintf(out, "  uncertain: %s\n", u)
	}
	for _, l := range rep.Ledgers {
		fmt.Fprintf(out, "  ledger %s: present=%v (%s)\n", l.Kind, l.Present, l.Path)
	}
}
