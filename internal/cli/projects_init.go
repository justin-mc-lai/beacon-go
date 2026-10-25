// projects init (R2c, M11): assemble a new project's context —
// register + version in the store, and the docs/state file set
// Python's `beacon init` writes (see orchestrator/init.go for the
// per-file provenance and the two deliberate adaptations).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newProjectsInitCmd() *cobra.Command {
	var project, version, mode, name string
	var dryRun, force, withClaude, withAgents, asJSON bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Assemble a new project's Beacon context (register + docs skeleton + onboarding state)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			report, err := orchestrator.NewEngine(st).InitProject(project, orchestrator.InitOptions{
				Version: version, ProjectMode: mode, RuntimeVersion: Version,
				Name: name, DryRun: dryRun, Force: force,
				WithClaude: withClaude, WithAgents: withAgents,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, report)
			}
			out := cmd.OutOrStdout()
			for _, n := range report.Notes {
				fmt.Fprintf(out, "note: %s\n", n)
			}
			for _, a := range report.Actions {
				fmt.Fprintf(out, "%-10s %-10s %s\n", a.Action, a.Scope, a.Path)
			}
			fmt.Fprintf(out, "init: %s mode=%s version=%s dry_run=%v\n",
				report.Status, report.ProjectMode, report.DocsVersion, report.DryRun)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&version, "version", "", "docs version to assemble (default: the recorded current version, else the newest docs/beacon/<version>, else v0.1)")
	c.Flags().StringVar(&mode, "project-mode", "greenfield", "project mode: greenfield|takeover")
	c.Flags().StringVar(&name, "name", "", "project display name when registering (default: directory base name)")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "plan the file set without writing anything")
	c.Flags().BoolVar(&force, "force", false, "refresh files that already exist")
	c.Flags().BoolVar(&withClaude, "with-claude", true, "write the CLAUDE.md managed stub when absent")
	c.Flags().BoolVar(&withAgents, "with-agents", true, "write the AGENTS.md managed stub when absent")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
