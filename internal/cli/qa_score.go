// Unified skill score commands (P4b, AC-151..156/169/170),
// folded into the qa ability face per the charter anti-bloat
// rule: `qa score` runs the ONE score engine
// (orchestrator.ScoreSkill) and `qa score-history` reads the
// record back.
//
// Boundary (AC-153), printed on every card: a score is runtime
// evidence only — a health check, not a passport. It never
// substitutes the formal qa gate or the human release decision,
// and no gate/admission/completion path reads it.
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
)

// parseHumanScores turns repeatable --human-score key=value flags
// into the engine's map, failing closed on malformed input.
func parseHumanScores(raw []string) (map[string]float64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := map[string]float64{}
	for _, kv := range raw {
		key, val, ok := strings.Cut(kv, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fail(core.ReasonSkillHumanScoreInvalid, "qa score",
				"--human-score must be key=value (effectiveness or a corpus rubric dimension), got "+kv)
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return nil, fail(core.ReasonSkillHumanScoreInvalid, "qa score",
				"--human-score "+key+" is not a number: "+val)
		}
		out[strings.TrimSpace(key)] = f
	}
	return out, nil
}

func newQAScoreCmd() *cobra.Command {
	var project, feature, version, evaluator, corpusPath string
	var humanRaw []string
	var apply, asJSON bool
	c := &cobra.Command{
		Use:   "score <skill>",
		Short: "Score one skill package with the unified engine (structural static checks + human-graded effectiveness); record only, never edits the skill",
		Long: "Score a skill package (a SKILL.md path, a package directory, or a skill\n" +
			"name resolved under the project) with the unified skill-score engine\n" +
			"(schema skill-score.v1, corpus skill-score-corpus.v1).\n\n" +
			"Structural quality is checked mechanically; effectiveness is graded by\n" +
			"a human against the corpus test-prompts and supplied via --human-score\n" +
			"(effectiveness=<0-100>, or per-rubric values). The verdict is written\n" +
			"to gate_results as gate_type=skill_score plus a skill_scored event.\n\n" +
			"BOUNDARY (AC-153): a score is runtime evidence only — a health check,\n" +
			"not a passport. It does NOT substitute the formal qa gate or the human\n" +
			"release decision, and nothing in the gate/admission/complete paths\n" +
			"reads it. Scoring never edits the skill (AC-154): suggestions on the\n" +
			"card are advisory; a human edits, then re-scores.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if apply {
				return fail(core.ReasonSkillAutoEditForbidden, "qa score",
					"scoring never applies suggestions or edits SKILL.md (AC-154/170: score-only, human-edits); edit the package yourself, then re-score")
			}
			human, err := parseHumanScores(humanRaw)
			if err != nil {
				return err
			}
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
			card, err := eng.ScoreSkill(ctx, orchestrator.ScoreOptions{
				Target: args[0], Evaluator: evaluator,
				HumanScores: human, CorpusPath: corpusPath,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, card)
			}
			printScorecardHuman(cmd, card)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug the score record is filed under (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&evaluator, "evaluator", "", "evaluator identity (required; unattributed scores are refused)")
	c.Flags().StringArrayVar(&humanRaw, "human-score", nil, "human effectiveness grade key=value (effectiveness or a corpus rubric dimension, 0-100); repeatable")
	c.Flags().StringVar(&corpusPath, "corpus", "", "external skill-score-corpus.v1 JSON (default: the corpus embedded in the binary; a bad path fails closed)")
	c.Flags().BoolVar(&apply, "apply", false, "forbidden: applying suggestions / editing the skill is refused (skill_auto_edit_forbidden)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func printScorecardHuman(cmd *cobra.Command, card orchestrator.Scorecard) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "target:    %s (%s)\n", card.TargetName, card.TargetPath)
	fmt.Fprintf(out, "sha256:    %s\nevaluator: %s\n", card.TargetSHA256, card.Evaluator)
	for _, d := range card.Dimensions {
		if d.Scored {
			fmt.Fprintf(out, "%-14s %6.2f / 100 (weight %.0f, %s)\n", d.Dimension, *d.Score, d.Weight, d.Source)
		} else {
			fmt.Fprintf(out, "%-14s not scored (%s)\n", d.Dimension, d.Source)
		}
		for _, it := range d.Items {
			if !it.Applicable {
				fmt.Fprintf(out, "  [n/a  ] %-24s %s\n", it.Check, it.Detail)
				continue
			}
			mark := "ok"
			if it.Points < it.MaxPoints {
				mark = "weak"
			}
			fmt.Fprintf(out, "  [%-5s] %-24s %g/%g  %s\n", mark, it.Check, it.Points, it.MaxPoints, it.Detail)
		}
		if d.Note != "" {
			fmt.Fprintf(out, "  note: %s\n", d.Note)
		}
	}
	if card.TotalScore != nil {
		fmt.Fprintf(out, "total:     %.2f  grade: %s\n", *card.TotalScore, card.Grade)
	}
	if len(card.Incomplete) > 0 {
		fmt.Fprintf(out, "incomplete: %s (total covers scored dimensions only)\n", strings.Join(card.Incomplete, ", "))
	}
	if len(card.Weaknesses) > 0 {
		fmt.Fprintf(out, "weaknesses: %s\n", strings.Join(card.Weaknesses, ", "))
	}
	for _, s := range card.Suggestions {
		fmt.Fprintf(out, "  suggest [%s/%s] %s (advisory only — a human edits)\n", s.Dimension, s.Check, s.Advice)
	}
	fmt.Fprintf(out, "corpus:    %s %s (%s, sha256 %s, %d cases)\n", card.CorpusSchema, card.CorpusVersion, card.CorpusSource, card.CorpusSHA256, card.CorpusCases)
	fmt.Fprintf(out, "boundary:  %s\n", card.Boundary)
	fmt.Fprintf(out, "recorded:  %s (%s) under feature %s — score history: `beacon qa score-history`\n", card.GateResultID, card.ScoredAt, card.FeatureSlug)
}

func newQAScoreHistoryCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "score-history",
		Short: "Read a feature's unified skill-score history, oldest first (gate_type=skill_score records only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "qa score-history", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			eng := orchestrator.NewEngine(st)
			cards, err := eng.ScoreHistory(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"feature_slug": ctx.Feature.Slug,
					"feature_id":   ctx.Feature.ID,
					"scores":       cards,
				})
			}
			out := cmd.OutOrStdout()
			if len(cards) == 0 {
				fmt.Fprintf(out, "no skill scores recorded for feature %s\n", ctx.Feature.Slug)
				return nil
			}
			for _, card := range cards {
				total := "—"
				if card.TotalScore != nil {
					total = fmt.Sprintf("%.2f", *card.TotalScore)
				}
				fmt.Fprintf(out, "%s  %-24s grade %-2s total %-7s evaluator %-12s %s\n",
					card.ScoredAt, card.TargetName, card.Grade, total, card.Evaluator, card.GateResultID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
