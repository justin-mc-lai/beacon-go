// beacon review (R2b, M15): the implementation review face —
// Python commands/review.py's spec/quality/design reviews over a
// feature package, with the verdict written back through the H5
// review-decision channel when the feature has a goal run.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newReviewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "review",
		Short: "Implementation reviews over a feature package: spec | quality | design (review.py)",
	}
	c.AddCommand(
		newReviewKindCmd(orchestrator.ReviewSpec, "Spec review: required documents, implementation marker, drift, stale materials"),
		newReviewKindCmd(orchestrator.ReviewQuality, "Quality review: verification + evidence closure, package closure, coverage, plan state"),
		newReviewKindCmd(orchestrator.ReviewDesign, "Design review: spec input, prototype presence, the design gate's verdict"),
	)
	return c
}

func newReviewKindCmd(kind, short string) *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   kind,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.Review(project, feature, kind)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s review of %s: %s (gate_ready=%v)\n", v.Kind, v.FeatureSlug, v.Status, v.GateReady)
			for _, is := range v.Issues {
				fmt.Fprintf(out, "  [%s] %s: %s\n", is.Severity, is.Code, is.Detail)
			}
			if v.ReviewRecorded {
				fmt.Fprintf(out, "written back: routed to %s [%s]\n", v.RouteStage, v.RouteReasonCode)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	_ = c.MarkFlagRequired("feature")
	return c
}
