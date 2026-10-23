// release check (R2b, H7): the VERSION-level release check —
// Python release.py `check`'s object. The per-feature release
// GATE stays on `beacon release` (EvaluateRelease); this surface
// answers "can this version ship" over every feature package of
// the version, under a release profile (full|hotfix|docs_only)
// whose soft checks downgrade to warnings.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
)

func newReleaseCheckCmd() *cobra.Command {
	var project, version, profile string
	var asJSON bool
	c := &cobra.Command{
		Use:   "check",
		Short: "Version-level release check over all feature packages of a version (release.py check + profile downgrade)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			res, err := eng.ReleaseVersionCheck(project, version, profile)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			return printReleaseCheck(cmd, res)
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root (required)")
	c.Flags().StringVar(&version, "version", "", "version to check (default: the registered current version)")
	c.Flags().StringVar(&profile, "profile", "full", "release profile: full | hotfix | docs_only")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("project")
	return c
}

func printReleaseCheck(cmd *cobra.Command, res gate.VersionReleaseResult) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "version:  %s (profile %s, %d feature package(s))\n", res.Version, res.Profile, res.FeatureCount)
	for _, it := range res.Checks {
		mark := it.Status
		if it.Downgraded {
			mark = "warn (downgraded by profile " + res.Profile + ")"
		}
		line := fmt.Sprintf("  %-38s %s", it.Name, mark)
		if it.ReasonCode != "" && it.Status != "pass" {
			line += " [" + it.ReasonCode + "]"
		}
		fmt.Fprintln(out, line)
		if len(it.Features) > 0 {
			fmt.Fprintf(out, "      failing: %s\n", joinCommaCLI(it.Features))
		}
	}
	if res.ReleaseReady {
		fmt.Fprintln(out, "release ready: yes")
	} else {
		fmt.Fprintf(out, "release ready: NO (blocked: %s) [%s]\n", joinCommaCLI(res.FailedChecks), res.ReasonCode)
	}
	if len(res.WarningChecks) > 0 {
		fmt.Fprintf(out, "warnings: %s\n", joinCommaCLI(res.WarningChecks))
	}
	return nil
}

func joinCommaCLI(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
