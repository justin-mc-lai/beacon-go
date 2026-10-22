// Goal flow surfaces (S2, AC-138/142): the standard execution
// flow's thin CLI face, folded into the existing `goal` group —
// no new top-level command. status is read-only; advance walks
// exactly one step under the flow's fail-closed order + fact
// criteria; bundle evaluates AND persists the AC-142 admission
// evidence bundle for a run (the implement-entry record).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalFlowCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "flow",
		Short: "Standard execution flow (AC-138): status, one-step advance, admission bundle (AC-142)",
		Long: "The standard execution flow is the fixed eight-step procedure that carries a " +
			"requirement from canonical truth to implement: support_advisory -> " +
			"canonical_truth_authoring -> canonical_refreeze -> canonical_commit -> " +
			"feature_worktree_reconcile -> governance_verify -> workspace_admission -> " +
			"implement_resume. Steps advance only in order and only when their facts hold; " +
			"skipping fails closed (standard_flow_order_violation).",
	}
	c.AddCommand(newGoalFlowStatusCmd(), newGoalFlowAdvanceCmd(), newGoalFlowBundleCmd())
	return c
}

func printFlowView(cmd *cobra.Command, v orchestrator.FlowView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "run:  %s (%s)\nflow: %d/%d steps complete", v.RunID, v.FeatureSlug, v.CompletedCount, len(v.Steps))
	if v.NextStep != "" {
		fmt.Fprintf(out, ", next: %s", v.NextStep)
	}
	fmt.Fprintln(out)
	for _, s := range v.Steps {
		fmt.Fprintf(out, "  %-28s %-8s %s\n", s.Step, s.Status, s.Detail)
	}
	if v.Attempted != "" {
		fmt.Fprintf(out, "advance %s: advanced=%v already_done=%v blocked=%v %s %s\n",
			v.Attempted, v.Advanced, v.AlreadyDone, v.Blocked, v.ReasonCode, v.Detail)
	}
	return nil
}

func newGoalFlowStatusCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the run's standard-flow progress (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.FlowStatus(runID)
			if err != nil {
				return err
			}
			return printFlowView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalFlowAdvanceCmd() *cobra.Command {
	var runID, step string
	var asJSON bool
	c := &cobra.Command{
		Use:   "advance",
		Short: "Advance the standard flow by one named step (order + fact criteria, fail-closed)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.FlowAdvance(runID, step)
			if v.RunID != "" {
				if perr := printFlowView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&step, "step", "", "step name from the frozen eight-step table (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("step")
	return c
}

func newGoalFlowBundleCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "bundle",
		Short: "Evaluate and record the AC-142 admission evidence bundle for a run (five verification classes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.EvaluateAdmissionBundle(runID)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "bundle: %s (%s) for run %s — allow_implement=%v\n",
					v.Status, v.ReasonCode, v.RunID, v.AllowImplement)
				for _, it := range v.Items {
					fmt.Fprintf(out, "  %-24s ok=%v applicable=%v [%s] %s\n",
						it.Check, it.OK, it.Applicable, it.Source, it.Detail)
				}
			}
			if !v.AllowImplement {
				return fail(v.ReasonCode, "goal flow bundle", "admission bundle blocked for run "+runID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
