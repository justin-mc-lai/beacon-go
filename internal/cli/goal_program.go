// Goal P2d program command group (G-12): plan / admit / route /
// change / freeze. Mirrors the v1 program surface's minimum set
// (validate graph, admission, orchestrate route) re-based on the
// store: the plan anchors on a host goal run, lakes are feature
// slugs in that run's project version, and progress is derived live
// — the route view caches nothing.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newGoalProgramCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "program",
		Short: "Program orchestration: split a big goal into lakes with dependencies (G-12)",
		Long: "A program splits one big goal into lakes (feature-sized pieces) with a\n" +
			"dependency graph. plan records the split on a host goal run; admit judges\n" +
			"each lake's entry (package exists, truth frozen, dependencies closed);\n" +
			"route shows the topological next batch and per-lake driver hints — the\n" +
			"program has no driver of its own, `goal driver` pushes one lake at a time.\n" +
			"A frozen plan refuses direct edits: use change, which leaves a record.",
	}
	c.AddCommand(newGoalProgramPlanCmd(), newGoalProgramAdmitCmd(), newGoalProgramRouteCmd(),
		newGoalProgramChangeCmd(), newGoalProgramFreezeCmd())
	return c
}

// parseProgramLakes folds the repeatable --lake / --lake-goal /
// --depends / --touch flags into lake inputs (the parallel-plan
// flag shape, extended with goals and dependency edges).
func parseProgramLakes(lakes, lakeGoals, depends, touches []string) ([]orchestrator.ProgramLakeInput, error) {
	bySlug := map[string]*orchestrator.ProgramLakeInput{}
	var order []string
	get := func(slug string) *orchestrator.ProgramLakeInput {
		if _, ok := bySlug[slug]; !ok {
			bySlug[slug] = &orchestrator.ProgramLakeInput{Slug: slug}
			order = append(order, slug)
		}
		return bySlug[slug]
	}
	for _, s := range lakes {
		get(s)
	}
	for _, g := range lakeGoals {
		slug, text, found := strings.Cut(g, "=")
		if !found || slug == "" {
			return nil, fail("invalid_lake_goal_spec", "goal program", "--lake-goal must be slug=text, got "+g)
		}
		get(slug).Goal = text
	}
	for _, d := range depends {
		slug, deps, found := strings.Cut(d, "=")
		if !found || slug == "" || deps == "" {
			return nil, fail("invalid_depends_spec", "goal program", "--depends must be lake=dep1,dep2, got "+d)
		}
		for _, dep := range strings.Split(deps, ",") {
			if dep = strings.TrimSpace(dep); dep != "" {
				get(slug).DependsOn = append(get(slug).DependsOn, dep)
			}
		}
	}
	for _, t := range touches {
		slug, surface, found := strings.Cut(t, "=")
		if !found || slug == "" || surface == "" {
			return nil, fail("invalid_touch_spec", "goal program", "--touch must be lake=surface, got "+t)
		}
		get(slug).Touches = append(get(slug).Touches, surface)
	}
	out := make([]orchestrator.ProgramLakeInput, 0, len(order))
	for _, slug := range order {
		out = append(out, *bySlug[slug])
	}
	return out, nil
}

func printProgramState(cmd *cobra.Command, st orchestrator.ProgramState, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, st)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "program: %s (frozen=%v)\nlakes:   %s\nchanges: %d\n",
		st.Slug, st.Frozen, strings.Join(lakeSlugsOf(st), ", "), len(st.Changes))
	return nil
}

func lakeSlugsOf(st orchestrator.ProgramState) []string {
	out := []string{}
	for _, l := range st.Lakes {
		out = append(out, l.Slug)
	}
	return out
}

func newGoalProgramPlanCmd() *cobra.Command {
	var runID, program, goal string
	var lakes, lakeGoals, depends, touches []string
	var freeze, asJSON bool
	c := &cobra.Command{
		Use:   "plan",
		Short: "Record (or, while unfrozen, replace) the program plan on a host goal run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputs, err := parseProgramLakes(lakes, lakeGoals, depends, touches)
			if err != nil {
				return err
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			st, err := eng.ProgramPlan(runID, orchestrator.ProgramPlanInput{
				Slug: program, Goal: goal, Lakes: inputs, Freeze: freeze,
			})
			if err != nil {
				return err
			}
			return printProgramState(cmd, st, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&program, "program", "", "program slug (required)")
	c.Flags().StringVar(&goal, "goal", "", "the big goal in one line")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "lake slug = feature slug (repeatable)")
	c.Flags().StringArrayVar(&lakeGoals, "lake-goal", nil, "lake goal as slug=text (repeatable)")
	c.Flags().StringArrayVar(&depends, "depends", nil, "dependencies as lake=dep1,dep2 (repeatable)")
	c.Flags().StringArrayVar(&touches, "touch", nil, "declared touch as lake=surface (repeatable)")
	c.Flags().BoolVar(&freeze, "freeze", false, "freeze the plan as recorded (later edits need change)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("program")
	return c
}

func newGoalProgramChangeCmd() *cobra.Command {
	var runID, note string
	var lakes, lakeGoals, depends, touches []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "change",
		Short: "Record a change intent against the plan (the only way to edit a frozen plan's lakes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			replace := len(lakes)+len(lakeGoals)+len(depends)+len(touches) > 0
			var inputs []orchestrator.ProgramLakeInput
			if replace {
				var err error
				inputs, err = parseProgramLakes(lakes, lakeGoals, depends, touches)
				if err != nil {
					return err
				}
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			st, err := eng.ProgramChange(runID, note, inputs, replace)
			if err != nil {
				return err
			}
			return printProgramState(cmd, st, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&note, "note", "", "why the plan changes (required; the change record)")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "replacement lake slug (repeatable)")
	c.Flags().StringArrayVar(&lakeGoals, "lake-goal", nil, "lake goal as slug=text (repeatable)")
	c.Flags().StringArrayVar(&depends, "depends", nil, "dependencies as lake=dep1,dep2 (repeatable)")
	c.Flags().StringArrayVar(&touches, "touch", nil, "declared touch as lake=surface (repeatable)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("note")
	return c
}

func newGoalProgramFreezeCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "freeze",
		Short: "Freeze the run's program plan (direct edits refused afterwards; use change)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			st, err := eng.ProgramFreeze(runID)
			if err != nil {
				return err
			}
			return printProgramState(cmd, st, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramAdmitCmd() *cobra.Command {
	var runID, lake string
	var asJSON bool
	c := &cobra.Command{
		Use:   "admit",
		Short: "Judge each lake's entry (package exists, truth frozen, dependencies closed); verdicts are recorded",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramAdmission(runID, lake)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				for _, adm := range v.Lakes {
					fmt.Fprintf(out, "lake %-16s admitted=%v %s\n", adm.Lake, adm.Admitted,
						strings.Join(adm.ReasonCodes, ", "))
				}
				fmt.Fprintf(out, "all admitted: %v\n", v.AllAdmitted)
			}
			if !v.AllAdmitted {
				for _, adm := range v.Lakes {
					if !adm.Admitted && len(adm.ReasonCodes) > 0 {
						return fail(adm.ReasonCodes[0], "goal program admit",
							"lake "+adm.Lake+" is not admitted")
					}
				}
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "judge only this lake")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramRouteCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "route",
		Short: "Show the live route: per-lake status, topological next batch, driver hints (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramRoute(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "program: %s (frozen=%v, all_complete=%v)\n", v.ProgramSlug, v.Frozen, v.AllComplete)
			for _, row := range v.Lakes {
				fmt.Fprintf(out, "  %-16s %-10s %s\n", row.Lake, row.Status, row.ReasonCode)
				if row.DriverHint != "" {
					fmt.Fprintf(out, "    drive: %s\n", row.DriverHint)
				}
			}
			fmt.Fprintf(out, "next batch: %s\n", strings.Join(v.NextBatch, ", "))
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
