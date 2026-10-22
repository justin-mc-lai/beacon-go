// Goal P2d command surfaces: hotfix-plan (G-13), multi-version-plan
// (G-14), memory (G-15) and axis (G-16).
//
// Naming notes (vs the v1 goal.py surface): v1 packs actions into
// single commands with an action argument (memory reindex|search,
// axis generate|validate) or attaches validation to the build
// (hotfix-plan). The Go surface uses subcommands for the action —
// the same shape P2b/P2c already established (driver start|status,
// design-stage status|bind) — with identical action words. The
// hotfix build still refuses what v1 only flagged, per the task's
// build contract (see orchestrator/hotfix.go).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

// ---- Hotfix (G-13) ----

func newGoalHotfixPlanCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "hotfix-plan",
		Short: "Hotfix fast lane: build a fix plan and validate it for release (human release always required)",
	}
	c.AddCommand(newGoalHotfixBuildCmd(), newGoalHotfixValidateCmd())
	return c
}

func newGoalHotfixBuildCmd() *cobra.Command {
	var runID, description, fromVersion, targetVersion, rollback string
	var fixPoints []string
	var rollbackExecutable, asJSON bool
	c := &cobra.Command{
		Use:   "build",
		Short: "Build and record the hotfix plan on a run (baseline, forward target, fix points, rollback — all required)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			plan, err := eng.BuildHotfixPlan(runID, orchestrator.HotfixPlanInput{
				Description: description, FromVersion: fromVersion, TargetVersion: targetVersion,
				FixPoints: fixPoints, RollbackPlan: rollback,
				RollbackDeclaredExecutable: rollbackExecutable,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, plan)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"hotfix plan for run %s: %s -> %s, %d fix point(s), rollback recorded, human release required: %v\n",
				plan.RunID, plan.FromVersion, plan.TargetVersion, len(plan.FixPoints), plan.RequiresHumanRelease)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&description, "description", "", "the incident / fault description (required)")
	c.Flags().StringVar(&fromVersion, "from-version", "", "baseline version to fix from (required)")
	c.Flags().StringVar(&targetVersion, "target-version", "", "forward-port target version (required)")
	c.Flags().StringArrayVar(&fixPoints, "fix-point", nil, "one concrete fix point (repeatable, at least one)")
	c.Flags().StringVar(&rollback, "rollback", "", "rollback plan / notes (required — no plan without one)")
	c.Flags().BoolVar(&rollbackExecutable, "rollback-executable", false, "declare the rollback executable as written")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalHotfixValidateCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate the run's hotfix plan for release (completeness + executable rollback + qa pass)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ValidateHotfixForRelease(runID)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "hotfix release validation: ok=%v blocked=%v codes=%s (human release required: %v)\n",
					v.OK, v.Blocked, strings.Join(v.ReasonCodes, ", "), v.RequiresHumanRelease)
			}
			if v.Blocked {
				return fail(v.ReasonCodes[0], "goal hotfix-plan validate", "hotfix plan is not release-ready")
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- Multi-version plan (G-14) ----

func newGoalMultiVersionPlanCmd() *cobra.Command {
	var runID string
	var segments, segDeps, completed, dropped []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "multi-version-plan",
		Short: "Cut one goal into per-version segments; unfinished items must carry over or be dropped explicitly",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			in, err := parseMultiVersionInput(segments, segDeps, completed, dropped)
			if err != nil {
				return err
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			plan, err := eng.PlanMultiVersion(runID, in)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, plan); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				for _, seg := range plan.Segments {
					fmt.Fprintf(out, "%s: items=%s completed=%s dropped=%s carried_in=%s\n",
						seg.Version, strings.Join(seg.Items, ","), strings.Join(seg.Completed, ","),
						strings.Join(seg.Dropped, ","), strings.Join(seg.CarriedIn, ","))
				}
				for _, p := range plan.Problems {
					fmt.Fprintf(out, "problem: %s — %s\n", p.ReasonCode, p.Detail)
				}
				fmt.Fprintf(out, "valid: %v\n", plan.Valid)
			}
			if !plan.Valid && len(plan.Problems) > 0 {
				return fail(plan.Problems[0].ReasonCode, "goal multi-version-plan", plan.Problems[0].Detail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringArrayVar(&segments, "segment", nil, "segment as version=item1,item2 (repeatable, versions must increase)")
	c.Flags().StringArrayVar(&segDeps, "segment-dep", nil, "segment dependency as version=earlier-version (repeatable)")
	c.Flags().StringArrayVar(&completed, "completed", nil, "finished item as version:item (repeatable)")
	c.Flags().StringArrayVar(&dropped, "dropped", nil, "explicitly dropped item as version:item (repeatable)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func parseMultiVersionInput(segments, segDeps, completed, dropped []string) (orchestrator.MultiVersionInput, error) {
	in := orchestrator.MultiVersionInput{
		Completed: map[string][]string{}, Dropped: map[string][]string{},
	}
	deps := map[string][]string{}
	for _, d := range segDeps {
		ver, dep, found := strings.Cut(d, "=")
		if !found || ver == "" || dep == "" {
			return in, fail("invalid_segment_dep_spec", "goal multi-version-plan", "--segment-dep must be version=earlier-version, got "+d)
		}
		deps[ver] = append(deps[ver], dep)
	}
	for _, s := range segments {
		ver, items, found := strings.Cut(s, "=")
		if !found || ver == "" {
			return in, fail("invalid_segment_spec", "goal multi-version-plan", "--segment must be version=item1,item2, got "+s)
		}
		var list []string
		for _, it := range strings.Split(items, ",") {
			if it = strings.TrimSpace(it); it != "" {
				list = append(list, it)
			}
		}
		in.Segments = append(in.Segments, orchestrator.VersionSegmentInput{
			Version: ver, Items: list, DependsOn: deps[ver],
		})
	}
	for _, spec := range completed {
		ver, item, found := strings.Cut(spec, ":")
		if !found || ver == "" || item == "" {
			return in, fail("invalid_completed_spec", "goal multi-version-plan", "--completed must be version:item, got "+spec)
		}
		in.Completed[ver] = append(in.Completed[ver], item)
	}
	for _, spec := range dropped {
		ver, item, found := strings.Cut(spec, ":")
		if !found || ver == "" || item == "" {
			return in, fail("invalid_dropped_spec", "goal multi-version-plan", "--dropped must be version:item, got "+spec)
		}
		in.Dropped[ver] = append(in.Dropped[ver], item)
	}
	return in, nil
}

// ---- Memory (G-15) ----

func newGoalMemoryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "memory",
		Short: "Goal memory over the run's store trail (reference only — never completion evidence)",
		Long: "Reindex and search a run's own lifecycle events, artifact pointers and\n" +
			"checkpoints. Results are support_advisory and reference_only: they can\n" +
			"never complete a goal, pass admission, or replace the verifier.",
	}
	c.AddCommand(newGoalMemoryReindexCmd(), newGoalMemorySearchCmd())
	return c
}

func newGoalMemoryReindexCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "reindex",
		Short: "Rebuild the run's memory index from its store trail",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.MemoryReindex(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "indexed %d entries for run %s (authority: %s, reference only)\n",
				v.Indexed, v.RunID, v.Authority)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalMemorySearchCmd() *cobra.Command {
	var runID, query string
	var limit int
	var asJSON bool
	c := &cobra.Command{
		Use:   "search",
		Short: "Search the run's memory index (every term must match; hits are reference_only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.MemorySearch(runID, query, limit)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "status: %s hits: %d (authority: %s, can_complete: %v)\n",
				v.Status, len(v.Hits), v.Authority, v.CanComplete)
			for _, h := range v.Hits {
				fmt.Fprintf(out, "  [%s/%s] %s\n", h.SourceKind, h.Label, h.Snippet)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVarP(&query, "query", "q", "", "search words (required)")
	c.Flags().IntVar(&limit, "limit", 10, "max hits")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("query")
	return c
}

// ---- Axis (G-16) ----

func newGoalAxisCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "axis",
		Short: "Main axis: generate the run's spine description and validate it against drift",
	}
	c.AddCommand(newGoalAxisGenerateCmd(), newGoalAxisValidateCmd())
	return c
}

func newGoalAxisGenerateCmd() *cobra.Command {
	var runID, intent string
	var asJSON bool
	c := &cobra.Command{
		Use:   "generate",
		Short: "Generate the main axis from the run's stage plan (and program plan, when present)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			axis, err := eng.GenerateMainAxis(runID, intent)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, axis)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "axis for run %s: %s\n", axis.RunID, axis.Objective)
			for _, s := range axis.Stages {
				fmt.Fprintf(out, "  %-10s %s\n", s.Stage, s.Criterion)
			}
			if len(axis.Lakes) > 0 {
				fmt.Fprintf(out, "lakes: %s\n", strings.Join(axis.Lakes, " -> "))
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&intent, "intent", "", "one-line objective (default: the run's utterance)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalAxisValidateCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate the stored axis against the current stage plan and program plan (drift report)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ValidateMainAxis(runID)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "axis ok: %v\n", v.OK)
				for _, d := range v.Drifts {
					fmt.Fprintf(out, "  drift %s: %s\n", d.Kind, d.Detail)
				}
			}
			if !v.OK {
				return fail(v.ReasonCode, "goal axis validate", "main axis drift detected")
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
