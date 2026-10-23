// QA intelligence faces (R2d): the qa9 strategy generator
// (M2) and the agent-eval line (M1) on the qa ability face,
// beside — never merged with — the skill score (`qa score`).
// `qa strategy` writes project configuration; the agent-eval
// commands read/write the project's .beacon/state/agent-eval/
// records. Both are file surfaces (Python parity): neither
// needs the runtime DB except agent-eval grade --run-id.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

// ---- qa strategy (M2) ----

func newQAStrategyCmd() *cobra.Command {
	var project, typesRaw, uiRunner string
	var write, asJSON bool
	c := &cobra.Command{
		Use:   "strategy",
		Short: "Generate the qa9 QA strategy for a project (type inference + applicability + command presets)",
		Long: "Ports qa9.build_qa_strategy: scan the project (language / framework /\n" +
			"API specs / skill markers), infer the project type, lay the nine QA\n" +
			"types over the applicability matrix and derive per-type command\n" +
			"presets. Written to .beacon/qa-strategy.json (unless --write=false),\n" +
			"where the autotest discovery chain reads its default command.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			var types []string
			if strings.TrimSpace(typesRaw) != "" {
				types = strings.Split(typesRaw, ",")
			}
			strategy, path, err := orchestrator.GenerateQAStrategy(root, types, uiRunner, write)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, strategy)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "project type: %s (%s/%s)\n", strategy.ProjectType,
				strategy.ProjectScan.Language, strategy.ProjectScan.Framework)
			fmt.Fprintf(out, "required:     %s\n", strings.Join(strategy.RequiredTypes, ", "))
			for _, t := range strategy.RequiredTypes {
				fmt.Fprintf(out, "  %-14s %s\n", t, strategy.CommandBaseline[t])
			}
			if path != "" {
				fmt.Fprintf(out, "written:      %s\n", path)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&typesRaw, "types", "", "comma-separated QA types to select (default: profile web-full)")
	c.Flags().StringVar(&uiRunner, "ui-runner", "playwright", "UI runner for functional/compatibility presets")
	c.Flags().BoolVar(&write, "write", true, "persist the strategy to .beacon/qa-strategy.json")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// ---- qa agent-score / agent-eval (M1) ----

// newQAAgentScoreCmd is the scorecard shortcut, the agent-side
// sibling of `qa score` (which scores SKILL packages — the two
// engines never share a card).
func newQAAgentScoreCmd() *cobra.Command {
	var project, agent string
	var asJSON bool
	c := &cobra.Command{
		Use:   "agent-score",
		Short: "Compute + persist an agent-eval scorecard for one agent (six dimensions; NOT the skill score)",
		Long: "Ports agent_eval_kit.build_agent_eval_scorecard: six weighted\n" +
			"dimensions over the agent's graded transcripts + the capability\n" +
			"catalog. A scorecard is runtime evidence only — it is not a QA or\n" +
			"release verdict. Below a 70 composite the card is blocked, and\n" +
			"executor profile selection refuses that profile.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			card, err := orchestrator.BuildAgentScorecard(root, agent)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, card)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "agent:     %s\n", card.Agent)
			fmt.Fprintf(out, "composite: %.2f (%s)\n", card.CompositeScore, card.Status)
			for _, dim := range core.AgentEvalDimensions {
				fmt.Fprintf(out, "  %-28s %6.2f (w %.2f)\n", dim, card.Dimensions[dim], card.Weights[dim])
			}
			fmt.Fprintf(out, "cases:     %d (failures %d)\n", card.CaseCount, card.FailureCount)
			fmt.Fprintf(out, "written:   %s\n", card.Path)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&agent, "agent", "", "agent / executor profile name (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("agent")
	return c
}

func newQAAgentEvalCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "agent-eval",
		Short: "Agent evaluation line: grade transcripts, inspect the catalog, check profile admission",
	}
	c.AddCommand(newQAAgentEvalGradeCmd(), newQAAgentEvalHistoryCmd(),
		newQAAgentEvalCatalogCmd(), newQAAgentEvalAdmissionCmd())
	return c
}

func newQAAgentEvalGradeCmd() *cobra.Command {
	var project, agent, transcriptPath, runID, expectRoute string
	var asJSON bool
	c := &cobra.Command{
		Use:   "grade",
		Short: "Grade one host-behavior transcript (ROUTE probe + boundary phrases) and persist the record",
		Long: "Ports agent_eval_grader.grade_host_behavior_transcript. The\n" +
			"transcript comes from --transcript (a file, or - for stdin) or is\n" +
			"assembled from a run's event trail with --run-id.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var rec orchestrator.AgentGradeRecord
			var err error
			switch {
			case runID != "":
				eng, closeFn, gerr := goalEngine("")
				if gerr != nil {
					return gerr
				}
				defer closeFn()
				rec, err = eng.GradeRunTranscript(runID, agent, expectRoute)
			case transcriptPath != "":
				var raw []byte
				if transcriptPath == "-" {
					raw, err = io.ReadAll(cmd.InOrStdin())
				} else {
					raw, err = os.ReadFile(transcriptPath)
				}
				if err != nil {
					return err
				}
				root := store.ResolveRootPath(project)
				rec, err = orchestrator.GradeAgentTranscript(root, agent, string(raw), expectRoute, "transcript", "")
			default:
				return fail("agent_eval_source_required", "qa agent-eval grade",
					"give --transcript <file|-> or --run-id <id>")
			}
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rec)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "agent:   %s\npassed:  %v\n", rec.Agent, rec.Grade.Passed)
			if rec.Grade.ReasonCode != "" {
				fmt.Fprintf(out, "reason:  %s\n", rec.Grade.ReasonCode)
			}
			fmt.Fprintf(out, "route:   %s\n", rec.Grade.ParsedRoute)
			if len(rec.Grade.ForbiddenHits) > 0 {
				fmt.Fprintf(out, "forbidden: %s\n", strings.Join(rec.Grade.ForbiddenHits, ", "))
			}
			fmt.Fprintf(out, "written: %s\n", rec.Path)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&agent, "agent", "", "agent / profile name the transcript belongs to")
	c.Flags().StringVar(&transcriptPath, "transcript", "", "transcript file path (or - for stdin)")
	c.Flags().StringVar(&runID, "run-id", "", "grade a run's event trail instead of a file")
	c.Flags().StringVar(&expectRoute, "expect-route", "", "expected ROUTE surface (default for --run-id: the run's current stage skill)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newQAAgentEvalHistoryCmd() *cobra.Command {
	var project, agent string
	var asJSON bool
	c := &cobra.Command{
		Use:   "history",
		Short: "List persisted agent-eval scorecards (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			cards := orchestrator.ListAgentScorecards(root, agent)
			if asJSON {
				return printJSON(cmd, map[string]any{"scorecards": cards})
			}
			out := cmd.OutOrStdout()
			if len(cards) == 0 {
				fmt.Fprintln(out, "no agent-eval scorecards")
				return nil
			}
			for _, card := range cards {
				fmt.Fprintf(out, "%s  %-24s composite=%.2f %s\n",
					card.GeneratedAt, card.Agent, card.CompositeScore, card.Status)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&agent, "agent", "", "filter by agent / profile name")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newQAAgentEvalCatalogCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "catalog",
		Short: "Show the agent-eval capability catalog and its P0 coverage (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog := orchestrator.BuildAgentCatalog()
			if asJSON {
				return printJSON(cmd, catalog)
			}
			total, covered := catalog.CatalogCoverage()
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "entries: %d   p0 coverage: %d/%d\n", catalog.EntryCount, covered, total)
			for _, e := range catalog.Entries {
				present := ""
				if e.SkillPresent != nil {
					present = fmt.Sprintf(" present=%v", *e.SkillPresent)
				}
				fmt.Fprintf(out, "  %-46s %-4s %s%s\n", e.ID, e.Harness, e.Command, present)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newQAAgentEvalAdmissionCmd() *cobra.Command {
	var project, agent string
	var asJSON bool
	c := &cobra.Command{
		Use:   "admission",
		Short: "Show the profile-selection admission verdict for one agent/profile (read-only)",
		Long: "The same verdict ResolveExecutorFor applies when a profile is\n" +
			"selected: blocked (latest scorecard below 70, or a boundary\n" +
			"violation in the latest grade) refuses selection; warn (no record\n" +
			"on file) and admitted proceed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			v := orchestrator.AgentAdmission(root, agent)
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "agent:   %s\nstate:   %s\n", agent, v.State)
			if v.ReasonCode != "" {
				fmt.Fprintf(out, "reason:  %s\n", v.ReasonCode)
			}
			if v.Detail != "" {
				fmt.Fprintf(out, "detail:  %s\n", v.Detail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&agent, "agent", "", "agent / executor profile name (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("agent")
	return c
}
