// Goal program integrated-delivery (program-integrated-delivery
// AC-001..003, truth.md surfaces: [cli]). This is the explicit CLI
// entry for plan B: freeze, enable, handoff, dispatch, merge, total
// QA, one human signature, finalize. Nothing here changes legacy
// Program behavior — a plan stays legacy until `integrated enable`.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalProgramIntegratedCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "integrated",
		Short: "Opt-in integrated delivery: verified handoffs, parallel lakes, one candidate signature",
		Long: "Plan B for a frozen program: a lake starts only from a verified, merged\n" +
			"upstream handoff; disjoint lakes run on separate worktrees; main-star merges\n" +
			"stay serial; and the whole sea is released by ONE human signature bound to\n" +
			"one candidate SHA + evidence digest. The program keeps legacy behavior until\n" +
			"`integrated enable` is called.",
	}
	c.AddCommand(
		newGoalProgramIntegratedEnableCmd(),
		newGoalProgramIntegratedHandoffCmd(),
		newGoalProgramIntegratedDispatchCmd(),
		newGoalProgramIntegratedMergeCmd(),
		newGoalProgramIntegratedRouteCmd(),
		newGoalProgramIntegratedQACmd(),
		newGoalProgramIntegratedApproveCmd(),
		newGoalProgramIntegratedFinalizeCmd(),
	)
	return c
}

func printIntegratedView(cmd *cobra.Command, v orchestrator.ProgramIntegratedView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "program: %s (%s)\nstage:   %s\n", v.ProgramSlug, v.Mode, v.Stage)
	if v.Candidate != nil {
		fmt.Fprintf(out, "candidate: %s (evidence %s)\n", v.Candidate.SHA, v.Candidate.EvidenceDigest)
	}
	if v.Approval != nil {
		fmt.Fprintf(out, "approved:  %s by %s\n", v.Approval.CandidateSHA, v.Approval.SignedBy)
	}
	if len(v.Ready) > 0 {
		fmt.Fprintf(out, "ready:   %v\n", v.Ready)
	}
	for lake, reason := range v.Blocked {
		fmt.Fprintf(out, "blocked: %s (%s)\n", lake, reason)
	}
	if v.ReasonCode != "" {
		fmt.Fprintf(out, "reason:  %s\n", v.ReasonCode)
	}
	return nil
}

func newGoalProgramIntegratedEnableCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "enable",
		Short: "Flip a frozen program into integrated delivery (legacy behavior is untouched until this call)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedEnable(runID)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramIntegratedHandoffCmd() *cobra.Command {
	var runID, lake, source, integration string
	var evidence []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "handoff",
		Short: "Verify and record one lake's handoff (independent QA + merged SHA)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			h, err := eng.ProgramIntegratedHandoff(context.Background(), runID, lake, source, integration, evidence)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, h)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "handoff: %s valid=%v source=%s integrated=%s\n",
				h.Lake, h.Valid, h.SourceSHA, h.IntegrationSHA)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "lake slug (required)")
	c.Flags().StringVar(&source, "source-sha", "", "verified source commit (required)")
	c.Flags().StringVar(&integration, "integration-sha", "", "merged commit on the main star (required)")
	c.Flags().StringArrayVar(&evidence, "qa-evidence", nil, "independent QA evidence path (repeatable)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("lake")
	_ = c.MarkFlagRequired("source-sha")
	_ = c.MarkFlagRequired("integration-sha")
	return c
}

func newGoalProgramIntegratedDispatchCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "dispatch",
		Short: "Start the next non-conflicting lake batch (reuses existing bindings on recovery)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			res, err := eng.ProgramIntegratedDispatch(context.Background(), runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "batch:    %d (parallel=%v)\n", res.Batch, res.Parallel)
			for _, d := range res.Dispatched {
				fmt.Fprintf(out, "dispatch: %s run=%s worktree=%s\n", d.Lake, d.RunID, d.WorktreePath)
			}
			if len(res.Recovered) > 0 {
				fmt.Fprintf(out, "recovered: %v\n", res.Recovered)
			}
			if len(res.Serial) > 0 {
				fmt.Fprintf(out, "serial:    %v\n", res.Serial)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramIntegratedMergeCmd() *cobra.Command {
	var runID, lake string
	var asJSON bool
	c := &cobra.Command{
		Use:   "merge",
		Short: "Merge one lake's branch into the main star (serialized; conflicts stay blocked)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedMerge(context.Background(), runID, lake)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "lake slug (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("lake")
	return c
}

func newGoalProgramIntegratedRouteCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "route",
		Short: "Show the integrated route: ready and blocked lakes with the shared start judgement (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedRoute(runID)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramIntegratedQACmd() *cobra.Command {
	var runID, verdict, by string
	var evidence []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "qa",
		Short: "Record total QA for the one candidate (only pass enters awaiting_approval)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedQA(context.Background(), runID, verdict, evidence, by)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&verdict, "verdict", "pass", "total QA verdict (only pass binds a candidate)")
	c.Flags().StringArrayVar(&evidence, "evidence", nil, "QA evidence ref (repeatable)")
	c.Flags().StringVar(&by, "by", "", "independent QA identity (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("by")
	return c
}

func newGoalProgramIntegratedApproveCmd() *cobra.Command {
	var runID, candidateSHA, evidenceDigest, confirmedBy string
	var asJSON bool
	c := &cobra.Command{
		Use:   "approve",
		Short: "The ONE human signature bound to the candidate SHA + evidence digest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedApprove(context.Background(), runID, candidateSHA, evidenceDigest, confirmedBy)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&candidateSHA, "candidate-sha", "", "bound candidate SHA (required)")
	c.Flags().StringVar(&evidenceDigest, "evidence-digest", "", "bound evidence digest (required)")
	c.Flags().StringVar(&confirmedBy, "confirmed-by", "", "human identity signing the sea (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("candidate-sha")
	_ = c.MarkFlagRequired("evidence-digest")
	_ = c.MarkFlagRequired("confirmed-by")
	return c
}

func newGoalProgramIntegratedFinalizeCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "finalize",
		Short: "Complete every lake and close the host (idempotent for the same candidate)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramIntegratedFinalize(context.Background(), runID)
			if err != nil {
				return err
			}
			return printIntegratedView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
