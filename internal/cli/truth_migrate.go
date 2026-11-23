package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newTruthMigrateCmd() *cobra.Command {
	var project, feature, version, by, tasksFile string
	var dryRun, asJSON bool
	c := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate legacy truth/tests/tasks to an unsigned SDD truth/tasks scaffold without losing source material",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read a SQLite-consistent snapshot; source lock/index sidecars may appear.
			st, closeStore, err := openMigrationReadStore()
			if err != nil {
				return err
			}
			defer closeStore()
			fc, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			report, err := orchestrator.NewEngine(st).MigrateTruthSDD(fc, orchestrator.SDDMigrationOptions{DryRun: dryRun, By: by, TasksFile: tasksFile})
			if err != nil {
				if report.BackupDir != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "originals preserved at:", report.BackupDir)
				}
				return err
			}
			if asJSON {
				return printJSON(cmd, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "feature: %s\ncontract: %s\ndry-run: %t\nalready SDD: %t\ncoverage rows: %d\n", report.Feature, report.Contract, report.DryRun, report.AlreadySDD, report.CoverageRows)
			if report.BackupDir != "" {
				fmt.Fprintln(cmd.OutOrStdout(), "backup:", report.BackupDir)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "next:", report.Next)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "registered project root")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&by, "by", "", "migration author identity (required; does not sign truth)")
	c.Flags().StringVar(&tasksFile, "tasks-file", "", "explicit executable TASK DAG file for legacy checkbox tasks; commands and dependencies are never guessed")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "validate without changing authored packages or ledger data; SQLite lock/index sidecars may appear")
	c.Flags().BoolVar(&asJSON, "json", false, "emit JSON report")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("by")
	return c
}
