package cli

import (
	"beacon-go/internal/orchestrator"
	"github.com/spf13/cobra"
)

// Enrich stable command receipts from the same final read-only status. Legacy
// receipts keep their established shape and do not inherit a new gate.
func goalWorkflowView(runID string) (*orchestrator.WorkflowView, error) {
	eng, closeFn, err := goalReadEngine()
	if err != nil {
		return nil, err
	}
	defer closeFn()
	v, err := eng.Status(runID)
	if err != nil {
		return nil, err
	}
	if v.Workflow == nil || v.Workflow.Mode == "legacy" {
		return nil, nil
	}
	return v.Workflow, nil
}

func newGoalWorkflowCmd() *cobra.Command {
	root := &cobra.Command{Use: "workflow", Short: "Bind, prepare and inspect stable stage deliverables"}
	for _, action := range []string{"bind", "prepare", "confirm", "check"} {
		action := action
		var runID, file, by, preparedHash string
		c := &cobra.Command{Use: action, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			if action == "check" {
				eng, closeFn, err := goalReadEngine()
				if err != nil {
					return err
				}
				defer closeFn()
				v, err := eng.WorkflowCheck(runID)
				if err != nil {
					return err
				}
				if err := printJSON(cmd, v); err != nil {
					return err
				}
				if !v.Ready {
					return fail(v.Missing[0], "workflow check", v.Next)
				}
				return nil
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			if action == "confirm" {
				v, err := eng.ConfirmWorkflow(runID, preparedHash, by)
				if err != nil {
					return err
				}
				return printJSON(cmd, v)
			}
			if action == "bind" {
				v, err := eng.BindWorkflow(runID, file, by)
				if err != nil {
					return err
				}
				return printJSON(cmd, v)
			}
			v, err := eng.PrepareWorkflow(runID)
			if err != nil {
				return err
			}
			return printJSON(cmd, v)
		}}
		c.Flags().StringVar(&runID, "run-id", "", "explicit run ID")
		_ = c.MarkFlagRequired("run-id")
		if action == "bind" {
			c.Flags().StringVar(&file, "file", "", "workflow JSON")
			c.Flags().StringVar(&by, "by", "", "confirming owner")
			_ = c.MarkFlagRequired("file")
			_ = c.MarkFlagRequired("by")
		}
		if action == "confirm" {
			c.Flags().StringVar(&by, "by", "", "explicit confirming identity")
			c.Flags().StringVar(&preparedHash, "prepared-hash", "", "expected hash from prepare/check")
			_ = c.MarkFlagRequired("by")
			_ = c.MarkFlagRequired("prepared-hash")
		}
		root.AddCommand(c)
	}
	return root
}
