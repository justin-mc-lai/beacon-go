package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

// newImplementTasksCmd is registered under the implement ability by its owner.
func newImplementTasksCmd() *cobra.Command {
	c := &cobra.Command{Use: "tasks", Short: "Read or execute the admitted feature's local task DAG"}
	var statusRunID string
	status := &cobra.Command{
		Use: "status", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			report, runErr := eng.TaskStatus(statusRunID)
			if report.RunID != "" {
				if err := printJSON(cmd, report); err != nil {
					return err
				}
			}
			return runErr
		},
	}
	status.Flags().StringVar(&statusRunID, "run-id", "", "run whose feature task status is read (required)")
	_ = status.MarkFlagRequired("run-id")
	var runID string
	var opts orchestrator.TaskRunOptions
	var timeoutSeconds int
	run := &cobra.Command{
		Use: "run", Args: cobra.NoArgs,
		Short: "Execute Command then Verify without a shell; stop on failure and retain immutable evidence",
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeoutSeconds < 0 || opts.MaxRepairs < 0 || opts.MaxRepairs > orchestrator.MaxTaskRepairs {
				return fmt.Errorf("task_repair_invalid: timeout must be nonnegative; max-repairs must be 0..%d", orchestrator.MaxTaskRepairs)
			}
			if int64(timeoutSeconds) > int64((1<<63-1)/time.Second) {
				return fmt.Errorf("task_repair_invalid: timeout is too large")
			}
			opts.Timeout = time.Duration(timeoutSeconds) * time.Second
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			report, runErr := eng.RunTasks(cmd.Context(), runID, opts)
			if report.RunID != "" {
				if err := printJSON(cmd, report); err != nil {
					return err
				}
			}
			return runErr
		},
	}
	run.Flags().StringVar(&runID, "run-id", "", "run whose feature task DAG executes (required)")
	run.Flags().StringVar(&opts.By, "by", "kernel_task_runner", "identity on registered attempt evidence")
	run.Flags().StringVar(&opts.RepairCommand, "repair-cmd", "", "one explicit external repair executable (no shell)")
	run.Flags().StringArrayVar(&opts.RepairArgs, "repair-arg", nil, "one repair argv element; repeat for multiple arguments")
	run.Flags().IntVar(&opts.MaxRepairs, "max-repairs", 0, "total repair invocation bound (hard cap 5; default disabled)")
	run.Flags().IntVar(&timeoutSeconds, "timeout-sec", 0, "per-command and per-repair timeout; zero uses harness default")
	_ = run.MarkFlagRequired("run-id")
	c.AddCommand(status, run)
	return c
}
