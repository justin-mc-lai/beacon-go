// qa autotest (R2c, H8): the execution half of the qa face. Runs
// the feature's tests.md TC commands for real (harness, no shell),
// lands per-TC evidence under .beacon/autotest/<run-id>/ and
// registers it as test_result artifacts the formal qa gate consumes.
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newQAAutoTestCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "autotest",
		Short: "Execute the feature's tests.md test commands and register the evidence (auto_test.py core chain)",
	}
	c.AddCommand(newQAAutoTestRunCmd())
	return c
}

func newQAAutoTestRunCmd() *cobra.Command {
	var runID, by string
	var timeoutSeconds int
	var asJSON bool
	c := &cobra.Command{
		Use:   "run",
		Short: "Run every executable TC of the run's feature package and register per-TC evidence",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var timeout time.Duration
			if timeoutSeconds > 0 {
				timeout = time.Duration(timeoutSeconds) * time.Second
			}
			report, err := eng.RunAutoTest(context.Background(), runID, orchestrator.AutoTestOptions{
				By: by, Timeout: timeout,
			})
			if report.RunID != "" {
				if asJSON {
					if perr := printJSON(cmd, report); perr != nil {
						return perr
					}
				} else {
					out := cmd.OutOrStdout()
					for _, r := range report.Results {
						line := fmt.Sprintf("%-8s %-10s exit=%d", r.TC, r.Status, r.ExitCode)
						if r.ReasonCode != "" {
							line += " " + r.ReasonCode
						}
						fmt.Fprintln(out, line)
					}
					fmt.Fprintf(out, "autotest: %d passed, %d failed, %d blocked of %d (report %s)\n",
						report.Passed, report.Failed, report.Blocked, report.Total, report.ReportPath)
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id whose feature's tests run (required)")
	c.Flags().StringVar(&by, "by", "", "executor identity recorded on the evidence (required)")
	c.Flags().IntVar(&timeoutSeconds, "timeout-seconds", 0, "per-TC timeout in seconds (default: harness default)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("by")
	return c
}
