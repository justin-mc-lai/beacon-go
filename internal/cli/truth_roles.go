package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/sddroles"
)

func newTruthRolesCmd() *cobra.Command {
	var utterance string
	var asJSON bool
	c := &cobra.Command{
		Use:   "roles",
		Short: "Plan the on-demand subagents an utterance needs for SDD",
		Long: "Match an utterance against SDD signals and return the subagent plan:\n" +
			"which roles run in which stage, why each one woke, and which specialists\n" +
			"stay deferred. Selection is deterministic; no model call. The roles are\n" +
			"the SDD subset of the awesome-claude-code-subagents taxonomy.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(utterance) == "" {
				return fmt.Errorf("--utterance is required")
			}
			plan := sddroles.Select(utterance)
			if asJSON {
				return printJSON(cmd, plan)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "signals:  %s\n", joinSignals(plan.Signals))
			for _, stage := range plan.Stages {
				fmt.Fprintf(out, "%s:\n", stage.Name)
				for _, r := range stage.Roles {
					fmt.Fprintf(out, "  %-24s %s\n", r.ID, r.Why)
				}
			}
			if len(plan.Deferred) > 0 {
				fmt.Fprintln(out, "deferred:")
				for _, d := range plan.Deferred {
					fmt.Fprintf(out, "  %-24s %s\n", d.ID, d.Trigger)
				}
			}
			cov := plan.Coverage
			fmt.Fprintf(out, "coverage: intent=%t fsm=%t tests=%t tasks=%t ready=%t\n",
				cov.Intent, cov.FSM, cov.Tests, cov.Tasks, cov.Ready)
			return nil
		},
	}
	c.Flags().StringVar(&utterance, "utterance", "", "the one-sentence intent (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func joinSignals(signals []sddroles.Signal) string {
	if len(signals) == 0 {
		return "(none)"
	}
	parts := make([]string, len(signals))
	for i, s := range signals {
		parts[i] = string(s)
	}
	return strings.Join(parts, ", ")
}
