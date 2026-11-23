package cli

import (
	"beacon-go/internal/orchestrator"
	"fmt"
	"github.com/spf13/cobra"
)

func newTruthSemanticReviewCmd() *cobra.Command {
	var project, feature, version, file string
	var asJSON bool
	c := &cobra.Command{Use: "semantic-review", Short: "Inspect the review fingerprint or record an external independent review receipt", RunE: func(cmd *cobra.Command, args []string) error {
		st, err := openWriteStore()
		if err != nil {
			return err
		}
		defer st.Close()
		fc, err := resolveGateFeature(st, project, feature, version)
		if err != nil {
			return err
		}
		eng := orchestrator.NewEngine(st)
		fingerprint := eng.SDDReviewFingerprint(fc)
		if file != "" {
			fingerprint, err = eng.RecordSDDReview(fc, file)
			if err != nil {
				return err
			}
		}
		if asJSON {
			return printJSON(cmd, map[string]string{"fingerprint": fingerprint})
		}
		fmt.Fprintln(cmd.OutOrStdout(), fingerprint)
		return nil
	}}
	c.Flags().StringVar(&project, "project", ".", "project root")
	c.Flags().StringVar(&feature, "feature", "", "feature slug")
	c.Flags().StringVar(&version, "version", "", "docs version")
	c.Flags().StringVar(&file, "file", "", "external JSON review receipt; omit to inspect fingerprint")
	c.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	_ = c.MarkFlagRequired("feature")
	return c
}
