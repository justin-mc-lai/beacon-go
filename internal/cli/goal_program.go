// Goal P2d program command group (G-12): plan / admit / route /
// change / freeze. Mirrors the v1 program surface's minimum set
// (validate graph, admission, orchestrate route) re-based on the
// store: the plan anchors on a host goal run, lakes are feature
// slugs in that run's project version, and progress is derived live
// — the route view caches nothing.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
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
		newGoalProgramChangeCmd(), newGoalProgramFreezeCmd(),
		newGoalProgramValidateCmd(), newGoalProgramImportCmd(),
		newGoalProgramDriveCmd(), newGoalProgramCompleteCmd(),
		newGoalProgramProposeCmd(), newGoalProgramSelectCmd())
	return c
}

func newGoalProgramDriveCmd() *cobra.Command {
	var runID, executor string
	var maxRounds int
	var asJSON, untilDone bool
	var ef execFlags
	c := &cobra.Command{
		Use:   "drive",
		Short: "Advance one ready lake to its release gate (never auto-completes, never releases)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramDrive(runID, orchestrator.ProgramDriveOptions{
				Executor: executor, Exec: ef.config(), MaxTurns: maxRounds, UntilDone: untilDone,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "program:   %s (%s)\n", v.ProgramSlug, v.ProgramStatus)
			if v.AdvancedLake != "" {
				fmt.Fprintf(out, "lake:      %s -> %s (run %s)\n", v.AdvancedLake, v.LakeStatus, v.LakeRunID)
			}
			if v.ReasonCode != "" {
				fmt.Fprintf(out, "reason:    %s\n", v.ReasonCode)
			}
			fmt.Fprintf(out, "rollup:    completed=%d awaiting_release=%d blocked=%d pending=%d\n",
				v.Route.Rollup.Completed, v.Route.Rollup.AwaitingRelease, v.Route.Rollup.Blocked, v.Route.Rollup.Pending)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&executor, "executor", "", "stage executor: recording | subprocess | subprocess:<profile> | ralph")
	c.Flags().IntVar(&maxRounds, "max-rounds", 0, "driver turn cap per lake (<=0 default)")
	c.Flags().BoolVar(&untilDone, "until-done", false, "keep advancing ready lakes until all are release-ready or one blocks")
	addExecFlags(c, &ef, true)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
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
	var runID, program, goal, fromProposal string
	var lakes, lakeGoals, depends, touches []string
	var freeze, asJSON bool
	c := &cobra.Command{
		Use:   "plan",
		Short: "Record (or, while unfrozen, replace) the program plan on a host goal run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			if fromProposal != "" {
				st, err := eng.ProgramPlanFromProposal(runID, fromProposal, freeze)
				if err != nil {
					return err
				}
				return printProgramState(cmd, st, asJSON)
			}
			hasPlan, err := eng.HasProgramPlan(runID)
			if err != nil {
				return err
			}
			if !hasPlan {
				return fail(core.ReasonLakeProposalRequired, "goal program",
					"a new program must start from a human-selected lake proposal: run `beacon goal program propose` then `beacon goal program select`, or pass --from-proposal")
			}
			if strings.TrimSpace(program) == "" {
				return fail("program_slug_required", "goal program",
					"--program is required unless --from-proposal is set")
			}
			inputs, err := parseProgramLakes(lakes, lakeGoals, depends, touches)
			if err != nil {
				return err
			}
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
	c.Flags().StringVar(&program, "program", "", "program slug (required unless --from-proposal)")
	c.Flags().StringVar(&goal, "goal", "", "the big goal in one line")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "lake slug = feature slug (repeatable)")
	c.Flags().StringArrayVar(&lakeGoals, "lake-goal", nil, "lake goal as slug=text (repeatable)")
	c.Flags().StringArrayVar(&depends, "depends", nil, "dependencies as lake=dep1,dep2 (repeatable)")
	c.Flags().StringArrayVar(&touches, "touch", nil, "declared touch as lake=surface (repeatable)")
	c.Flags().StringVar(&fromProposal, "from-proposal", "", "build the plan from a selected lake proposal id")
	c.Flags().BoolVar(&freeze, "freeze", false, "freeze the plan as recorded (later edits need change)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramProposeCmd() *cobra.Command {
	var runID, file string
	c := &cobra.Command{
		Use:   "propose",
		Short: "Validate and record an AI lake proposal (3-7 lakes, commercial lens)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			var p orchestrator.LakeProposal
			if err := json.Unmarshal(raw, &p); err != nil {
				return err
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramPropose(runID, p)
			if err != nil {
				return err
			}
			return printJSON(cmd, v)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&file, "file", "", "lake-proposal.v1 JSON file (required)")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("file")
	return c
}

func newGoalProgramSelectCmd() *cobra.Command {
	var runID, proposal, by string
	var lakes, rejects []string
	c := &cobra.Command{
		Use:   "select",
		Short: "Record the human multi-select (drafts are created for selected lakes only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramSelect(runID, proposal, lakes, rejects, by)
			if err != nil {
				return err
			}
			return printJSON(cmd, v)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&proposal, "proposal", "", "proposal id (required)")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "selected lake slug (repeatable)")
	c.Flags().StringArrayVar(&rejects, "reject", nil, "rejected lake slug (repeatable)")
	c.Flags().StringVar(&by, "by", "", "human identity confirming the selection (required)")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("proposal")
	_ = c.MarkFlagRequired("by")
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
			if v.ProposalID != "" {
				fmt.Fprintf(out, "proposal: %s selected=%d rejected=%d\n", v.ProposalID, v.ProposalSelected, v.ProposalRejected)
			}
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

func newGoalProgramValidateCmd() *cobra.Command {
	var file string
	var asJSON bool
	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate an on-disk program feature-graph.json (v1 program validate-graph)",
		Long: "Read docs/beacon/<version>/programs/<slug>/feature-graph.json and check the\n" +
			"v1 required keys plus the dependency/cycle rules. Non-zero exit when blocked.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, problems, err := orchestrator.LoadProgramGraphFile(file)
			if err != nil {
				return err
			}
			if asJSON {
				status := "pass"
				if len(problems) > 0 {
					status = "blocked"
				}
				if perr := printJSON(cmd, map[string]any{"status": status, "errors": problems, "graph": g}); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "program: %s (lakes=%d)\n", g.ProgramSlug, len(g.Lakes))
				for _, p := range problems {
					fmt.Fprintf(out, "  %s: %s\n", p.ReasonCode, p.Detail)
				}
			}
			if len(problems) > 0 {
				return fail(problems[0].ReasonCode, "goal program validate", problems[0].Detail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "program feature-graph.json path (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("file")
	return c
}

func newGoalProgramImportCmd() *cobra.Command {
	var file, runID string
	var freeze, asJSON bool
	c := &cobra.Command{
		Use:   "import",
		Short: "Validate an on-disk program graph and record it on a host goal run",
		Long: "Same validation as `goal program validate`, then record the file graph as the\n" +
			"run's program plan (parity_subset / status / scope_mode are kept). An invalid\n" +
			"graph writes nothing.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			st, err := eng.ProgramImportFile(runID, file, freeze)
			if err != nil {
				return err
			}
			return printProgramState(cmd, st, asJSON)
		},
	}
	c.Flags().StringVar(&file, "file", "", "program feature-graph.json path (required)")
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().BoolVar(&freeze, "freeze", false, "freeze the imported plan")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("file")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalProgramCompleteCmd() *cobra.Command {
	var runID, confirmedBy string
	var asJSON bool
	c := &cobra.Command{
		Use:   "complete",
		Short: "One program-level human confirmation: release every lake at its gate together",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ProgramComplete(runID, confirmedBy)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "program:   %s (%s)\n", v.ProgramSlug, v.ProgramStatus)
			fmt.Fprintf(out, "confirmed: %s\n", confirmedBy)
			fmt.Fprintf(out, "rollup:    completed=%d awaiting_release=%d blocked=%d pending=%d\n",
				v.Route.Rollup.Completed, v.Route.Rollup.AwaitingRelease, v.Route.Rollup.Blocked, v.Route.Rollup.Pending)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "host goal run id (required)")
	c.Flags().StringVar(&confirmedBy, "confirmed-by", "", "human identity confirming the whole sea (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("confirmed-by")
	return c
}
