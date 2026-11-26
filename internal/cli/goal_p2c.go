// Goal P2c command surfaces: implement lanes (G-22), delegation
// (G-10) and admit (G-04).
//
// Honesty rules: the kernel runs no agent. Delegation and executor
// commands are recorded on the run for the host agent; the host does
// the work and reports it with `goal tick --by`. Lanes are metadata,
// not parallel executors.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
)

func newGoalImplementCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "implement",
		Short: "Implement lanes: single / team / ralph under one admission and one harvest rule",
	}
	c.AddCommand(newGoalImplementPlanCmd())
	return c
}

func newGoalImplementPlanCmd() *cobra.Command {
	var runID, lane, scale string
	var asJSON bool
	c := &cobra.Command{
		Use:   "plan",
		Short: "Show the lane routing decision + shared admission verdict for a run (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.PlanImplement(runID, orchestrator.LaneRequest{Explicit: lane, Scale: scale})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "run:       %s\nlane:      %s (source %s — %s)\nadmission: %s (%s)\n",
				v.RunID, v.Decision.Lane, v.Decision.Source, v.Decision.Reason,
				v.Admission.Status, v.Admission.ReasonCode)
			for _, it := range v.Admission.Items {
				fmt.Fprintf(out, "  %-16s ok=%v %s\n", it.Check, it.OK, it.Detail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&lane, "lane", "", "explicit lane: single|team|ralph")
	c.Flags().StringVar(&scale, "scale", "", "task scale override (large routes to ralph)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalAdmitCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "admit",
		Short: "Run the shared implement admission checker for a run (read-only, per-item verdicts)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.CheckImplementAdmission(runID)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "admission: %s (%s) for run %s\n", v.Status, v.ReasonCode, v.RunID)
				for _, it := range v.Items {
					fmt.Fprintf(out, "  %-16s ok=%v %s\n", it.Check, it.OK, it.Detail)
				}
			}
			if !v.AllowImplement {
				return fail(v.ReasonCode, "goal admit", "implement admission blocked for run "+runID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalReceiveResultCmd() *cobra.Command {
	var runID, delegationID, status, summary, kind, by string
	var artifacts []string
	var asJSON, external bool
	c := &cobra.Command{
		Use:   "receive-result",
		Short: "Collect a delegation's return under the shared harvest rule (artifacts + implement evidence)",
		Long: "Receive the return of a delegation created by goal delegate-ralph. " +
			"(The v1 surface had no receive command — returns landed via the host " +
			"loop; the name is new in P2c and noted in the report.) A done return " +
			"must bring artifact files (--artifact, absolute paths) and the " +
			"implement evidence produced after the delegation went out. " +
			"--external instead writes back a result produced by an outside room " +
			"(no delegation exists): it needs --by, --summary and at least one " +
			"absolute --artifact path, registers the pointers and closes the run " +
			"as externally completed so program route reports the lake completed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !external && strings.TrimSpace(delegationID) == "" {
				return fail(core.ReasonDelegationNotFound, "goal receive-result",
					"--delegation-id is required unless --external is set")
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var inputs []orchestrator.ArtifactInput
			for _, raw := range artifacts {
				// One --artifact may name its own kind as
				// "kind=/abs/path"; the --kind flag stays the
				// default for bare paths. A returned implement
				// result needs three different kinds, which a
				// single shared flag cannot express.
				artKind, path := kind, raw
				if i := strings.Index(raw, "="); i > 0 && !strings.ContainsAny(raw[:i], "/\\") {
					artKind, path = raw[:i], raw[i+1:]
				}
				inputs = append(inputs, orchestrator.ArtifactInput{Kind: artKind, AbsPath: path})
			}
			if external {
				v, err := eng.ReceiveExternalResult(runID, orchestrator.ExternalResultInput{
					By: by, Summary: summary, Artifacts: inputs,
				})
				if err != nil {
					if v.RunID != "" && asJSON {
						_ = printJSON(cmd, v)
					}
					return err
				}
				if asJSON {
					return printJSON(cmd, v)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "external result: run %s status %s artifacts=%d by %s\n",
					v.RunID, v.Status, len(artifacts), by)
				return nil
			}
			v, err := eng.ReceiveRalphResult(runID, delegationID, orchestrator.LaneResultInput{
				Status: status, Summary: summary, Artifacts: inputs,
			})
			if v.RunID != "" {
				if asJSON {
					if perr := printJSON(cmd, v); perr != nil {
						return perr
					}
				} else {
					d := v.Delegation
					fmt.Fprintf(cmd.OutOrStdout(), "delegation: %s status: %s simulated: %v\n",
						d.ID, d.Status, d.Simulated)
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&delegationID, "delegation-id", "", "delegation id from goal delegate-ralph (required)")
	c.Flags().BoolVar(&external, "external", false, "write back a result produced outside the kernel (no delegation-id needed)")
	c.Flags().StringVar(&by, "by", "", "who reports an external result (required with --external)")
	c.Flags().StringVar(&status, "status", "done", "return status: done|failed")
	c.Flags().StringVar(&summary, "summary", "", "return summary")
	c.Flags().StringArrayVar(&artifacts, "artifact", nil, "artifact file absolute path (repeatable)")
	c.Flags().StringVar(&kind, "kind", "implement_artifact", "artifact kind for --artifact pointers")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
