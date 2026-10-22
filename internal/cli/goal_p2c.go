// Goal P2c command surfaces: implement lanes (G-22), delegate-ralph
// (G-10), parallel-plan (G-11), design-stage (G-18) and admit (G-04).
//
// Honesty rules on these surfaces: the ralph executor is NOT
// implemented in P2c (real integration is P3) — a delegated execution
// only ever happens through the recording placeholder and is labelled
// simulated everywhere; parallel-plan is planned-only (no worktree
// is created); design-stage records primitive bindings and generates
// no prototype files.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalImplementCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "implement",
		Short: "Implement lanes: single / team / ralph under one admission and one harvest rule",
	}
	c.AddCommand(newGoalImplementPlanCmd())
	return c
}

func newGoalImplementPlanCmd() *cobra.Command {
	var runID, lane, scale string
	var asJSON bool
	c := &cobra.Command{
		Use:   "plan",
		Short: "Show the lane routing decision + shared admission verdict for a run (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.PlanImplement(runID, orchestrator.LaneRequest{Explicit: lane, Scale: scale})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "run:       %s\nlane:      %s (source %s — %s)\nadmission: %s (%s)\n",
				v.RunID, v.Decision.Lane, v.Decision.Source, v.Decision.Reason,
				v.Admission.Status, v.Admission.ReasonCode)
			for _, it := range v.Admission.Items {
				fmt.Fprintf(out, "  %-16s ok=%v %s\n", it.Check, it.OK, it.Detail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&lane, "lane", "", "explicit lane: single|team|ralph")
	c.Flags().StringVar(&scale, "scale", "", "task scale override (large routes to ralph)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalAdmitCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "admit",
		Short: "Run the shared implement admission checker for a run (read-only, per-item verdicts)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.CheckImplementAdmission(runID)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "admission: %s (%s) for run %s\n", v.Status, v.ReasonCode, v.RunID)
				for _, it := range v.Items {
					fmt.Fprintf(out, "  %-16s ok=%v %s\n", it.Check, it.OK, it.Detail)
				}
			}
			if !v.AllowImplement {
				return fail(v.ReasonCode, "goal admit", "implement admission blocked for run "+runID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDelegateRalphCmd() *cobra.Command {
	var runID string
	var simulate, asJSON bool
	var ef execFlags
	c := &cobra.Command{
		Use:   "delegate-ralph",
		Short: "Pack the implement stage and delegate it to the ralph lane (real loop when a ralph command is configured)",
		Long: "Delegate the run's whole implement stage to the ralph lane: the pack " +
			"(requirement package, stage plan, admission verdict, acceptance and " +
			"rollback rules) is recorded on the run. With a ralph command configured " +
			"(--ralph-cmd here, or recorded by goal run --ralph-cmd) the REAL ralph " +
			"loop runs (P3c): iterations execute until the RALPH_DONE completion " +
			"marker, a failure, or the iteration ceiling, and the return is collected " +
			"by the same harvest as any other return (simulated=false everywhere). " +
			"--simulate instead runs the recording placeholder and labels every " +
			"output simulated. With neither, the pack is recorded and no execution " +
			"happens (P2c dispatch face).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("recording")
			if err != nil {
				return err
			}
			defer closeFn()
			cfg, err := eng.ExecutorConfigFor(runID, ef.config())
			if err != nil {
				return err
			}
			var v orchestrator.DelegationView
			switch {
			case simulate:
				v, err = eng.DelegateRalph(cmd.Context(), runID, true)
			case orchestrator.RalphConfigured(cfg):
				v, err = eng.DelegateRalphReal(cmd.Context(), runID, cfg.RalphArgv, cfg.TimeoutSec, cfg.RalphMaxIter)
			default:
				v, err = eng.DelegateRalph(cmd.Context(), runID, false)
			}
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			d := v.Delegation
			fmt.Fprintf(cmd.OutOrStdout(),
				"delegation: %s\nlane: %s status: %s simulated: %v\ntruth: %s\nworktree: %s\n",
				d.ID, d.Lane, d.Status, d.Simulated, d.Package.TruthPath, d.Package.WorktreeRef)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&simulate, "simulate", false, "run the recording placeholder executor now (labelled simulated; no real ralph work)")
	addExecFlags(c, &ef, false)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalReceiveResultCmd() *cobra.Command {
	var runID, delegationID, status, summary, kind string
	var artifacts []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "receive-result",
		Short: "Collect a delegation's return under the shared harvest rule (artifacts + implement evidence)",
		Long: "Receive the return of a delegation created by goal delegate-ralph. " +
			"(The v1 surface had no receive command — returns landed via the host " +
			"loop; the name is new in P2c and noted in the report.) A done return " +
			"must bring artifact files (--artifact, absolute paths) and the " +
			"implement evidence produced after the delegation went out.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var inputs []orchestrator.ArtifactInput
			for _, p := range artifacts {
				inputs = append(inputs, orchestrator.ArtifactInput{Kind: kind, AbsPath: p})
			}
			v, err := eng.ReceiveRalphResult(runID, delegationID, orchestrator.LaneResultInput{
				Status: status, Summary: summary, Artifacts: inputs,
			})
			if v.RunID != "" {
				if asJSON {
					if perr := printJSON(cmd, v); perr != nil {
						return perr
					}
				} else {
					d := v.Delegation
					fmt.Fprintf(cmd.OutOrStdout(), "delegation: %s status: %s simulated: %v\n",
						d.ID, d.Status, d.Simulated)
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&delegationID, "delegation-id", "", "delegation id from goal delegate-ralph (required)")
	c.Flags().StringVar(&status, "status", "done", "return status: done|failed")
	c.Flags().StringVar(&summary, "summary", "", "return summary")
	c.Flags().StringArrayVar(&artifacts, "artifact", nil, "artifact file absolute path (repeatable)")
	c.Flags().StringVar(&kind, "kind", "implement_artifact", "artifact kind for --artifact pointers")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("delegation-id")
	return c
}

func newGoalParallelPlanCmd() *cobra.Command {
	var runID string
	var lakes, touches []string
	var parallel, asJSON bool
	c := &cobra.Command{
		Use:   "parallel-plan",
		Short: "Build and record the parallel-lakes plan for a run (planned-only: no worktree is created)",
		Long: "Plan how the run's lakes would work in parallel: one lake is a " +
			"single worktree; several lakes form a main-star/satellite pool " +
			"(satellites merge only into the main star and may never write " +
			"truth on the main line). Lakes declaring the same touched surface " +
			"are flagged as conflicts requiring serial work or a human " +
			"decision. P2c records the plan only — no worktree is created.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			byLake := map[string][]string{}
			var order []string
			for _, s := range lakes {
				if _, ok := byLake[s]; !ok {
					order = append(order, s)
				}
				byLake[s] = byLake[s]
			}
			for _, t := range touches {
				slug, surface, found := strings.Cut(t, "=")
				if !found || slug == "" || surface == "" {
					return fail("invalid_touch_spec", "goal parallel-plan", "--touch must be lake=surface, got "+t)
				}
				if _, ok := byLake[slug]; !ok {
					order = append(order, slug)
				}
				byLake[slug] = append(byLake[slug], surface)
			}
			var inputs []orchestrator.LakeInput
			for _, s := range order {
				inputs = append(inputs, orchestrator.LakeInput{Slug: s, Touches: byLake[s]})
			}
			plan, _, err := eng.ApplyParallelPlan(runID, inputs, parallel)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, plan)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "mode: %s (planned_only=%v, intent %s)\n", plan.Mode, plan.PlannedOnly, plan.IntentSource)
			for _, s := range plan.Satellites {
				fmt.Fprintf(out, "  satellite %s -> %s (merge to %s, truth on main: %v)\n",
					s.Lake, s.Worktree, s.MergeTarget, s.MayWriteTruthOnMain)
			}
			for _, cf := range plan.Conflicts {
				fmt.Fprintf(out, "  conflict: %s touched by %s — serial or human decision required\n",
					cf.Surface, strings.Join(cf.Lakes, ", "))
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "lake slug (repeatable)")
	c.Flags().StringArrayVar(&touches, "touch", nil, "declared touch as lake=surface (repeatable)")
	c.Flags().BoolVar(&parallel, "parallel", false, "explicitly request the satellite pool")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDesignStageCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "design-stage",
		Short: "Design stage: primitive binding status and binding (no prototypes are generated)",
		Long: "The design stage minds only whether the design primitives (the frozen " +
			"visual rules) are bound to the run — unbound UX work may not start " +
			"implementing. Prototype generation is delegated to the host agent " +
			"via the design bridge (P3); nothing is drawn or generated here.",
	}
	c.AddCommand(newGoalDesignStageStatusCmd(), newGoalDesignStageBindCmd())
	return c
}

func newGoalDesignStageStatusCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show whether design is required and the primitives are bound (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.DesignStageStatus(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "required: %v bound: %v stage: %s\nbridge: %s\n",
				v.Required, v.Bound, v.StageStatus, v.BridgeNote)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDesignStageBindCmd() *cobra.Command {
	var runID, primitiveSet, version, sourceRef, boundBy string
	var asJSON bool
	c := &cobra.Command{
		Use:   "bind",
		Short: "Bind a design primitive set to the run (records the binding; generates nothing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.BindDesignPrimitive(runID, orchestrator.DesignBinding{
				PrimitiveSet: primitiveSet, Version: version, SourceRef: sourceRef, BoundBy: boundBy,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "bound: %v primitives: %s (%s)\n", v.Bound,
				v.Binding.PrimitiveSet, v.Binding.Version)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&primitiveSet, "primitive-set", "", "design primitive set name (required)")
	c.Flags().StringVar(&version, "version", "", "primitive set version")
	c.Flags().StringVar(&sourceRef, "source-ref", "", "where the primitives live (reference only)")
	c.Flags().StringVar(&boundBy, "bound-by", "cli", "who binds")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("primitive-set")
	return c
}
