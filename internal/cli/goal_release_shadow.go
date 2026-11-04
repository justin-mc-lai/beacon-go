// goal release-shadow (issue #8, shadow step): preview "would a
// policy-signed release pass?" under release_policy=auto_shadow.
// It records an event and a view but never sets the human
// confirmation, so `goal complete` is unchanged.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newGoalReleaseShadowCmd() *cobra.Command {
	var runID, evaluator string
	var asJSON bool
	c := &cobra.Command{
		Use:   "release-shadow",
		Short: "Preview the release gate under auto_shadow (never releases, never confirms)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ReleaseShadow(runID, evaluator)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "policy:    %s\n", v.Policy)
			fmt.Fprintf(out, "verdict:   %s\n", v.Verdict)
			if len(v.ReasonCodes) > 0 {
				fmt.Fprintf(out, "reasons:   %s\n", strings.Join(v.ReasonCodes, ", "))
			}
			fmt.Fprintf(out, "evaluated: %s\n", v.EvaluatedAt)
			fmt.Fprintln(out, "note:      shadow only — goal complete still requires human confirmation")
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&evaluator, "evaluator", "policy:auto_shadow", "shadow evaluator identity")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
