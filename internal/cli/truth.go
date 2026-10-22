// Truth commands (P4a): the truth ability's real command group,
// replacing the Phase 0 placeholder. The truth ability owns the
// feature-package lifecycle on disk + ledger: create (skeleton),
// freeze, change (gen-change), refreeze (gen-refreeze), and the
// read-only show/list views (which reuse the spine data path —
// ResolveFeature/ListFeatureContexts — plus the engine's package
// view assembly; no second query implementation lives here).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newTruthCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "truth",
		Short: "Truth ability: feature packages — create / freeze / change / refreeze / show / list",
		Long: "Truth is where requirement truth lives: one package per feature under\n" +
			"docs/beacon/<version>/features/<slug>/ (truth.md + tests.md + tasks.md +\n" +
			"evidence.md). create lays down the skeleton; freeze locks a draft after\n" +
			"the completeness + integrity precheck; change registers a change\n" +
			"against frozen truth WITHOUT touching the body; refreeze closes the\n" +
			"change after the author edited. `goal` orchestrates runs over these\n" +
			"packages; truth is the single-package tool entry.",
	}
	c.AddCommand(
		newTruthCreateCmd(), newTruthFreezeCmd(false), newTruthFreezeCmd(true),
		newTruthChangeCmd(), newTruthBaselineCmd(), newTruthShowCmd(), newTruthListCmd(),
	)
	return c
}

func printTruthView(cmd *cobra.Command, v orchestrator.TruthPackageView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "package:  %s (%s)\nstatus:   %s\n", v.Slug, v.Version, v.Status)
	fmt.Fprintf(out, "dir:      %s\n", v.PackageDir)
	if v.LifecycleStage != "" {
		fmt.Fprintf(out, "lifecycle: %s/%s\n", v.LifecycleStage, v.LifecycleStatus)
	}
	if v.BranchCheck != "" {
		line := "branch:   check=" + v.BranchCheck
		if v.BranchCheck != orchestrator.BranchCheckNotGit {
			line += fmt.Sprintf(" current=%s canonical=%s", v.CurrentBranch, v.CanonicalBranch)
		}
		fmt.Fprintln(out, line)
	}
	if v.Frozen && v.BaselineStatus != "" {
		line := "baseline: " + v.BaselineStatus
		if v.BaselineCommit != "" {
			line += " commit=" + v.BaselineCommit
		}
		if v.BaselineRecorded {
			line += " (recorded)"
		}
		if v.BaselineAlready {
			line += " (already recorded)"
		}
		fmt.Fprintln(out, line)
	}
	if v.BaselineNote != "" {
		fmt.Fprintf(out, "note:     %s\n", v.BaselineNote)
	}
	if v.AlreadyFrozen {
		fmt.Fprintln(out, "note:     already frozen (idempotent)")
	}
	if v.ChangeID != "" {
		fmt.Fprintf(out, "change:   %s (%s)\n", v.ChangeID, v.ChangeDoc)
	}
	if len(v.PendingChangeIDs) > 0 {
		fmt.Fprintf(out, "pending:  %s\n", strings.Join(v.PendingChangeIDs, ", "))
	}
	if v.Blocked {
		fmt.Fprintf(out, "blocked:  %s\n", strings.Join(v.BlockReasons, ", "))
		for _, f := range v.Findings {
			loc := f.File
			if f.Line > 0 {
				loc = fmt.Sprintf("%s:%d", f.File, f.Line)
			}
			fmt.Fprintf(out, "  [%-28s] %s — %s\n", f.Kind, loc, f.Detail)
		}
	}
	return nil
}

func newTruthCreateCmd() *cobra.Command {
	var project, version, slug string
	var asJSON bool
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a draft feature package (four-piece skeleton + store registration)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			eng := orchestrator.NewEngine(st)
			v, err := eng.CreateTruthPackage(project, version, slug)
			if err != nil {
				return err
			}
			return printTruthView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path (must be registered)")
	c.Flags().StringVar(&version, "version", "", "docs version (default: the project's current version)")
	c.Flags().StringVar(&slug, "slug", "", "feature package slug (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("slug")
	return c
}

// newTruthFreezeCmd builds freeze (refreeze=false) and refreeze
// (refreeze=true): the same precheck + write path in the engine.
func newTruthFreezeCmd(refreeze bool) *cobra.Command {
	var project, feature, version, by string
	var asJSON bool
	use, short := "freeze", "Freeze a draft package (completeness + integrity precheck, then status: frozen)"
	if refreeze {
		use, short = "refreeze", "Refreeze a change-pending package, closing its pending change(s)"
	}
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
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
			eng := orchestrator.NewEngine(st)
			var v orchestrator.TruthPackageView
			if refreeze {
				v, err = eng.RefreezeTruthPackage(ctx, by)
			} else {
				v, err = eng.FreezeTruthPackage(ctx, by)
			}
			if v.Slug != "" {
				if perr := printTruthView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&by, "by", "cli", "actor identity (recorded on the trail + the integrity evaluation)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthChangeCmd() *cobra.Command {
	var project, feature, version, summary, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "change",
		Short: "Register a change against a frozen package (writes changes/<id>.md; the truth body is untouched)",
		Args:  cobra.NoArgs,
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
			eng := orchestrator.NewEngine(st)
			v, err := eng.RequestTruthChange(ctx, summary, by)
			if v.Slug != "" {
				if perr := printTruthView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&summary, "summary", "", "what the change is (required)")
	c.Flags().StringVar(&by, "by", "cli", "who requests the change")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("summary")
	return c
}

// newTruthBaselineCmd is `truth baseline` (S1, AC-139): verify
// the frozen body's commit state on the canonical branch and
// record the baseline (baseline_commit + body sha) on the trail.
// Freeze attempts the same recording in passing when the body is
// already committed; this command is the explicit path after the
// author commits. Idempotent.
func newTruthBaselineCmd() *cobra.Command {
	var project, feature, version, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "baseline",
		Short: "Record the committed baseline of a frozen package (AC-139: frozen becomes consumable once committed on the canonical branch)",
		Args:  cobra.NoArgs,
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
			eng := orchestrator.NewEngine(st)
			v, err := eng.BaselineTruth(ctx, by)
			if v.Slug != "" {
				if perr := printTruthView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&by, "by", "cli", "actor identity (recorded on the trail)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthShowCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Show one package's state (read-only; data via the spine feature contexts)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "truth show", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			eng := orchestrator.NewEngine(st)
			v, err := eng.TruthPackage(ctx)
			if err != nil {
				return err
			}
			return printTruthView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthListCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List feature packages (read-only; optionally of one project)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				if asJSON {
					return printJSON(cmd, map[string]any{"features": []orchestrator.TruthPackageView{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no packages")
				return nil
			}
			defer st.Close()
			root := ""
			if project != "" {
				root = store.ResolveRootPath(project)
			}
			ctxs, err := st.ListFeatureContexts(root)
			if err != nil {
				return err
			}
			eng := orchestrator.NewEngine(st)
			views := []orchestrator.TruthPackageView{}
			for _, ctx := range ctxs {
				v, err := eng.TruthPackage(ctx)
				if err != nil {
					return err
				}
				views = append(views, v)
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"features": views})
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no packages")
				return nil
			}
			for _, v := range views {
				fmt.Fprintf(cmd.OutOrStdout(), "%-42s %-8s %s/%s\n", v.Slug, v.Version, v.Status, v.LifecycleStage)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", "", "project root path (default: all projects)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
