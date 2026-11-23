package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

// newGoalLegacyCloseCmd is the explicit backfill door for features delivered
// before the strict qa_candidate binding. It records the old report, its hash,
// the old candidate hash and a whitelisted reason; it never sets a qa verdict.
func newGoalLegacyCloseCmd() *cobra.Command {
	var runID, confirmedBy, by, report, reportSHA, candidateSHA, reason string
	var asJSON bool
	c := &cobra.Command{
		Use:   "legacy-close",
		Short: "Backfill-close a legacy run with a human-signed old report (never a qa pass)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.LegacyClose(runID, orchestrator.LegacyCloseOptions{
				ConfirmedBy: confirmedBy, By: by, ReportPath: report,
				ReportSHA256: reportSHA, CandidateSHA256: candidateSHA, ReasonCode: reason,
			})
			if v.RunID != "" {
				if perr := printGoalView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&confirmedBy, "confirmed-by", "", "human identity approving the legacy closure (required)")
	c.Flags().StringVar(&by, "by", "", "recording identity (defaults to --confirmed-by)")
	c.Flags().StringVar(&report, "report", "", "old independent report path (required)")
	c.Flags().StringVar(&reportSHA, "report-sha256", "", "exact sha256 of the report (required)")
	c.Flags().StringVar(&candidateSHA, "candidate-sha256", "", "old candidate binary sha256 (optional)")
	c.Flags().StringVar(&reason, "reason", "", "legacy_source_drift|legacy_report_without_candidate|legacy_contract_review_misclassified|legacy_evidence_thin")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("confirmed-by")
	_ = c.MarkFlagRequired("report")
	_ = c.MarkFlagRequired("report-sha256")
	_ = c.MarkFlagRequired("reason")
	return c
}
