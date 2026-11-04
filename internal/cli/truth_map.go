// beacon truth map (R2b, M16): the truth-map as a standalone
// face — Python commands/truth_map.py's resolve/verify. The
// judgement is the same function the admission bundle's
// truth_map_verify item reads (zero copies).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newTruthMapCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "map",
		Short: "Truth map: resolve a feature package's map, or verify layout + freshness (truth_map.py)",
	}
	c.AddCommand(newTruthMapResolveCmd(), newTruthMapVerifyCmd(), newTruthMapBuildCmd())
	return c
}

func newTruthMapBuildCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "build",
		Short: "Build feature-map.md: every AC -> entry point + runnable verification command",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.BuildFeatureMap(project, feature)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "feature map: %s (%d/%d mapped, %s)\n", v.Path, v.Mapped, len(v.Rows), v.Status)
			for _, r := range v.Rows {
				fmt.Fprintf(out, "  %-8s %-32s %-44s %s\n", r.AC, r.EntryPoint, r.Verification, r.Status)
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

func newTruthMapResolveCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve the feature package's truth map (derived from registration + the markdown package)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.TruthMapResolve(project, feature)
			if err != nil {
				return err
			}
			return printTruthMap(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthMapVerifyCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify the truth map: layout problems + frozen-fingerprint freshness, with diagnostics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.TruthMapVerify(project, feature)
			if err != nil {
				return err
			}
			return printTruthMap(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root (required)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	_ = c.MarkFlagRequired("feature")
	return c
}

func printTruthMap(cmd *cobra.Command, v orchestrator.TruthMapView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "truth map (%s): %s — %s\n", v.Mode, v.FeatureSlug, v.Status)
	fmt.Fprintf(out, "  package dir: %s\n", v.TruthPackageDir)
	fmt.Fprintf(out, "  authority:   %s (json_authority=%v)\n", v.AuthoritySource, v.JSONAuthority)
	if v.Mode == "verify" {
		fmt.Fprintf(out, "  layout ok:   %v; fingerprint: %s\n", v.LayoutOK, v.FingerprintStatus)
		if v.FeatureMapStatus != "" {
			fmt.Fprintf(out, "  feature map: %s\n", v.FeatureMapStatus)
		}
		for _, d := range v.Diagnostics {
			fmt.Fprintf(out, "  [%s] %s: %s\n", d.Severity, d.Code, d.Message)
		}
	}
	return nil
}
