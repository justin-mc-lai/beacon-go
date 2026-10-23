// Truth intelligence faces (R2d): decompose (M3), brainstorm
// (M4), code2truth (M13) and reopen (M14) on the truth ability
// face. All four write through the engine; none of them touches
// a frozen body except reopen, whose whole job is opening the
// next revision of one.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newTruthDecomposeCmd() *cobra.Command {
	var project, feature, version, utterance, subintent, lakesRaw, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "decompose",
		Short: "Decompose one utterance into a draft package (sub-intents, journey seeds, AC/TC/task materialization)",
		Long: "Ports intent_decomposition.decompose_product_intent +\n" +
			"intent_first materialization: split the utterance into classified\n" +
			"sub-intents, match journey seeds, derive ACs (one per clause, one\n" +
			"per journey acceptance seed) with provenance, render the user\n" +
			"journey section from the seeds, and lay down tests.md / tasks.md.\n" +
			"The package must already exist as a draft (`truth create`);\n" +
			"decomposition never signs — the human still records intent and\n" +
			"confirms before freeze.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			var lakes []string
			if strings.TrimSpace(lakesRaw) != "" {
				lakes = strings.Split(lakesRaw, ",")
			}
			v, err := orchestrator.NewEngine(st).DecomposeTruthPackage(ctx, utterance, lakes, subintent, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			d := v.Decomposition
			fmt.Fprintf(out, "package:  %s (%s)\n", v.Slug, v.Version)
			fmt.Fprintf(out, "scope:    %s  intents=%d journeys=%d\n",
				d.ScopeMode, len(d.Intents), len(d.Journeys))
			fmt.Fprintf(out, "acs:      %s\n", strings.Join(v.Materialized, ", "))
			fmt.Fprintf(out, "ready:    %v (missing: %s)\n",
				d.Readiness.Ready, strings.Join(d.Readiness.Missing, ", "))
			fmt.Fprintf(out, "artifact: %s\n", v.ArtifactPath)
			for _, w := range v.Warnings {
				fmt.Fprintf(out, "warning:  %s\n", w)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required; package must exist as a draft)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&utterance, "utterance", "", "the one-sentence intent to decompose (required)")
	c.Flags().StringVar(&subintent, "subintent", "", "declared subintent (methodology primary)")
	c.Flags().StringVar(&lakesRaw, "lakes", "", "comma-separated lake labels, if already known")
	c.Flags().StringVar(&by, "by", "cli", "who decomposes")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("utterance")
	return c
}

func newTruthBrainstormCmd() *cobra.Command {
	var project, feature, version, mode, prompt, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "brainstorm",
		Short: "Generate kernel questions into the ask ledger (Socratic set + package gaps) and report convergence",
		Long: "Ports brainstorm.run_beacon_brainstorm's question generation and\n" +
			"convergence: the four Socratic questions plus one question per\n" +
			"real package gap land in the SAME ask ledger humans use (marked\n" +
			"kernel:brainstorm-*), deduped on re-run. Convergence is the\n" +
			"ledger state: zero open questions means converged — the confirm\n" +
			"gate keeps judging the ledger, no second gate is created.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			v, err := orchestrator.NewEngine(st).GenerateBrainstorm(ctx, mode, prompt, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "package:   %s\n", v.Slug)
			fmt.Fprintf(out, "mode:      %s\n", v.CanonicalMode)
			fmt.Fprintf(out, "generated: %d (%s)\n", v.GeneratedCount, strings.Join(v.GeneratedIDs, ", "))
			fmt.Fprintf(out, "open:      %d / %d\n", v.OpenQuestions, v.TotalQuestions)
			fmt.Fprintf(out, "converged: %v\n", v.Converged)
			for _, r := range v.ConfirmBlocked {
				fmt.Fprintf(out, "blocked:   %s\n", r)
			}
			fmt.Fprintf(out, "next:      %s\n", v.NextRoute)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&mode, "mode", "requirement-convergence",
		"brainstorm mode: requirement-convergence | project-evolution | implementation-readiness | scope-simplification")
	c.Flags().StringVar(&prompt, "prompt", "", "the prompt under examination (recorded on the trail)")
	c.Flags().StringVar(&by, "by", "cli", "who generates")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthCode2TruthCmd() *cobra.Command {
	var project, feature, version, target, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "code2truth",
		Short: "Reverse-draft a truth package from existing code (candidate claims only; freeze still needs the intent gate)",
		Long: "Ports code2truth intake: scan the target codebase (file\n" +
			"inventory, entry points, exported symbols / routes / models),\n" +
			"persist the intake artifacts under .beacon/code2truth/<slug>/,\n" +
			"create a draft package and stamp the candidate claims into its\n" +
			"truth.md under a 反推草稿待人确认 banner. Candidate truth has no\n" +
			"authority — promotion is only through human confirmation and\n" +
			"the normal sign/freeze route.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			v, err := orchestrator.NewEngine(st).Code2TruthIntake(project, version, feature, target, by)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "package: %s (%s) — draft, candidate only\n", v.Slug, v.Version)
			fmt.Fprintf(out, "target:  %s\n", v.TargetRoot)
			fmt.Fprintf(out, "files:   %d (source %d / test %d / doc %d / skill %d)\n",
				v.Inventory.FileCount, v.Inventory.Counts["source"], v.Inventory.Counts["test"],
				v.Inventory.Counts["doc"], v.Inventory.Counts["skill"])
			fmt.Fprintf(out, "symbols: %d exported, %d routes, %d models\n",
				v.Inventory.Symbols.ExportedSymbols, len(v.Inventory.Symbols.Routes), len(v.Inventory.Symbols.Models))
			fmt.Fprintf(out, "claims:  %d\n", len(v.Claims))
			for _, a := range v.Ambiguity {
				fmt.Fprintf(out, "ambiguity: %s %s\n", a.ID, a.Reason)
			}
			fmt.Fprintf(out, "intake:  %s\n", v.IntakeMarkdown)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "beacon project root path (must be registered)")
	c.Flags().StringVar(&feature, "feature", "code2truth-project-intake", "feature slug for the draft package")
	c.Flags().StringVar(&version, "version", "", "docs version (default: the project's current version)")
	c.Flags().StringVar(&target, "target", "", "existing codebase root to scan (required)")
	c.Flags().StringVar(&by, "by", "cli", "who runs the intake")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("target")
	return c
}

func newTruthReopenCmd() *cobra.Command {
	var project, feature, version, reason, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "reopen",
		Short: "Open a NEW revision of a frozen package (old freeze stays on the trail; new revision starts as draft)",
		Long: "Ports reopen.py + revision_state.reopen_revision_state. Reopen is\n" +
			"not change (which edits a frozen package in place and refreezes\n" +
			"the same revision) and not rebase (which only re-bases a pending\n" +
			"change): reopen increments the revision (R1 -> R2), derives the\n" +
			"new draft from the old body, voids the intent signature, marks\n" +
			"the materials stale, and leaves the old revision's freeze fully\n" +
			"on record. The new revision walks ask -> sign -> freeze again.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			v, err := orchestrator.NewEngine(st).ReopenTruthPackage(ctx, reason, by)
			if v.Slug != "" {
				if perr := printTruthView(cmd, v, asJSON); perr != nil {
					return perr
				}
				if !asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "revision: %s -> %s (reason: %s)\n",
						v.PreviousRevisionID, v.NewRevisionID, v.ReopenReason)
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&reason, "reason", "change-request", "why a new revision is opened")
	c.Flags().StringVar(&by, "by", "cli", "who reopens")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}
