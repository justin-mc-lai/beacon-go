// goal artifacts (P6 E2E fix): the manual registration face for run
// artifacts, folded into the goal group (no new top-level command).
// `register` lands a real file in artifact_refs with its sha256 —
// the write path whose absence stopped the P6 end-to-end chain at
// the qa gate (stage agents produce a QA report on disk, but only
// registered pointers count as gate evidence). `list` is the
// read-only twin of `spine artifacts` over the same presentation.
//
// Registration is a declaration, not a judgement: it records who
// registered what, with the hash at registration time. Gates keep
// their own defence — every evaluation re-hashes the file, so a
// tampered artifact turns stale and blocks.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGoalArtifactsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "artifacts",
		Short: "Register and list a goal run's artifact pointers (artifact_refs)",
		Long: "Artifacts are the pointers the formal gates trust as evidence.\n" +
			"register declares that a real file belongs to a run and records\n" +
			"its sha256 + size at registration time; it judges nothing — gate\n" +
			"evaluations re-hash the file on every read, so tampering turns\n" +
			"the pointer stale and blocks. Kinds are a controlled vocabulary:\n" +
			"qa, qa_*, scorecard, test_result(s) (the qa gate's QA family),\n" +
			"plus evidence, implement_artifact, report, log, artifact.",
	}
	c.AddCommand(newGoalArtifactsRegisterCmd(), newGoalArtifactsListCmd())
	return c
}

func newGoalArtifactsRegisterCmd() *cobra.Command {
	var runID, kind, path, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "register",
		Short: "Register a real file as a run artifact (computes sha256, writes artifact_refs)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			res, err := eng.RegisterArtifact(runID, kind, path, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			a := res.Artifact
			fmt.Fprintf(cmd.OutOrStdout(), "artifact: %s\nkind:     %s\npath:     %s\nhash:     %s\nsize:     %d\nrun:      %s\n",
				a.ID, a.Kind, a.AbsPath, a.ContentHash, a.SizeBytes, a.RunID)
			if res.AlreadyRegistered {
				fmt.Fprintln(cmd.OutOrStdout(), "note:     already registered with the same hash (idempotent, nothing written)")
			}
			if res.Supersedes != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note:     supersedes %s (file content changed since that registration)\n", res.Supersedes)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&kind, "kind", "", "artifact kind from the controlled vocabulary, e.g. qa_report (required)")
	c.Flags().StringVar(&path, "path", "", "path of the real file to register (required)")
	c.Flags().StringVar(&by, "by", "", "registrant identity (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("kind")
	_ = c.MarkFlagRequired("path")
	_ = c.MarkFlagRequired("by")
	return c
}

func newGoalArtifactsListCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List a run's registered artifacts with stale flags (same view as `spine artifacts`)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			views, err := eng.ListRunArtifacts(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"run_id": runID, "artifact_refs": views})
			}
			if len(views) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no artifacts for run %s\n", runID)
				return nil
			}
			for _, v := range views {
				mark := ""
				if v.Stale {
					mark = " stale(" + v.StaleReason + ")"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s hash=%s size=%d%s\n", v.Kind, v.AbsPath, v.ContentHash, v.SizeBytes, mark)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
