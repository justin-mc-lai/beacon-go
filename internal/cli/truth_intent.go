// Truth intent flow (R1: ask before sign): record the user's intent,
// drive an open-question ledger one question at a time, sign the
// intent, and self-check the frozen package for requirement drift.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/gate"
	"beacon-go/internal/orchestrator"
)

func printIntent(cmd *cobra.Command, v orchestrator.IntentView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "package:   %s\n", v.Slug)
	fmt.Fprintf(out, "confirmed: %v\n", v.Confirmed)
	fmt.Fprintf(out, "ready:     %v\n", v.Ready)
	fmt.Fprintf(out, "goal:      %s\n", v.Snapshot.Goal)
	fmt.Fprintf(out, "non-goals: %s\n", v.Snapshot.NonGoals)
	fmt.Fprintf(out, "constraints: %s\n", v.Snapshot.Constraints)
	fmt.Fprintf(out, "scope:     %s\n", v.Snapshot.ScopeMode)
	if len(v.OpenQuestions) > 0 {
		fmt.Fprintln(out, "questions:")
		for _, q := range v.OpenQuestions {
			fmt.Fprintf(out, "  %s [%s] %s\n", q.ID, q.Status, q.Question)
		}
	}
	for _, r := range v.BlockReasons {
		fmt.Fprintf(out, "blocked:   %s\n", r)
	}
	return nil
}

func newTruthAskCmd() *cobra.Command {
	var project, feature, version, question, recommend, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "ask",
		Short: "Add one open question (the ask half of ask-before-sign)",
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
			v, err := orchestrator.NewEngine(st).AskQuestion(ctx, question, recommend, by)
			if err != nil {
				return err
			}
			return printIntent(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&question, "question", "", "question to ask (required)")
	c.Flags().StringVar(&recommend, "recommend", "", "recommended answer (one line)")
	c.Flags().StringVar(&by, "by", "cli", "who asks")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("question")
	return c
}

func newTruthAnswerCmd() *cobra.Command {
	var project, feature, version, id, answer, by string
	var drop, asJSON bool
	c := &cobra.Command{
		Use:   "answer",
		Short: "Close one open question with an answer (or --drop)",
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
			status := ""
			if drop {
				status = "dropped"
				if strings.TrimSpace(answer) == "" {
					answer = "(dropped by author)"
				}
			}
			v, err := orchestrator.NewEngine(st).AnswerQuestion(ctx, id, answer, status, by)
			if err != nil {
				return err
			}
			return printIntent(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&id, "id", "", "question id (e.g. Q1; required)")
	c.Flags().StringVar(&answer, "answer", "", "answer text")
	c.Flags().BoolVar(&drop, "drop", false, "drop the question instead of answering")
	c.Flags().StringVar(&by, "by", "cli", "who answers")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("id")
	return c
}

func newTruthQuestionsCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "questions",
		Short: "List the open-question ledger (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "truth questions", "no runtime DB yet")
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			v, err := orchestrator.NewEngine(st).IntentState(ctx)
			if err != nil {
				return err
			}
			return printIntent(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthIntentCmd() *cobra.Command {
	var project, feature, version, utterance, goal, nonGoals, constraints, scopeMode, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "intent",
		Short: "Record the user-intent snapshot (goal / non-goals / constraints / scope)",
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
			snap := gate.IntentSnapshot{
				Utterance: utterance, Goal: goal, NonGoals: nonGoals,
				Constraints: constraints, ScopeMode: scopeMode,
			}
			v, err := orchestrator.NewEngine(st).UpdateIntent(ctx, snap, by)
			if err != nil {
				return err
			}
			return printIntent(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&utterance, "utterance", "", "the user's words (verbatim)")
	c.Flags().StringVar(&goal, "goal", "", "what outcome this serves")
	c.Flags().StringVar(&nonGoals, "non-goals", "", "what is explicitly out of scope")
	c.Flags().StringVar(&constraints, "constraints", "", "hard constraints / boundaries")
	c.Flags().StringVar(&scopeMode, "scope-mode", "", "scope mode: full_parity | mvp | patch | brownfield_takeover | research | lake | ocean | focused (empty = derive from the utterance)")
	c.Flags().StringVar(&by, "by", "cli", "who records the intent")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	c.MarkFlagRequired("goal")
	return c
}

func newTruthConfirmCmd() *cobra.Command {
	var project, feature, version, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "confirm",
		Short: "Sign the intent (refuses while any intent finding remains)",
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
			v, err := orchestrator.NewEngine(st).ConfirmIntent(ctx, by)
			if err != nil {
				return err
			}
			return printIntent(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&by, "by", "cli", "who signs")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthDriftCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "drift",
		Short: "Requirement-level self-check against the last freeze (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "truth drift", "no runtime DB yet")
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			v, err := orchestrator.NewEngine(st).TruthDrift(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "package: %s\nclean:   %v\n", v.Slug, v.Clean)
			if v.Note != "" {
				fmt.Fprintf(out, "note:    %s\n", v.Note)
			}
			fmt.Fprintf(out, "body drift: %v\n", v.BodyDrift)
			for _, id := range v.Added {
				fmt.Fprintf(out, "  + %s\n", id)
			}
			for _, id := range v.Changed {
				fmt.Fprintf(out, "  ~ %s\n", id)
			}
			for _, id := range v.Removed {
				fmt.Fprintf(out, "  - %s\n", id)
			}
			if v.BaseDiverged {
				fmt.Fprintf(out, "base diverged: %s\n", v.BaseDetail)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	return c
}

func newTruthRebaseCmd() *cobra.Command {
	var project, feature, version, changeID, note, by string
	var accept bool
	var asJSON bool
	c := &cobra.Command{
		Use:   "rebase",
		Short: "Accept the current frozen body as a pending change's base (explicit parallel-change merge)",
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
			v, err := orchestrator.NewEngine(st).RebaseChange(ctx, changeID, accept, note, by)
			if err != nil {
				return err
			}
			return printTruthView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&changeID, "change", "", "pending change id (required)")
	c.Flags().BoolVar(&accept, "accept-current", false, "required: accept the current frozen body as base")
	c.Flags().StringVar(&note, "note", "", "what was merged before accepting")
	c.Flags().StringVar(&by, "by", "cli", "who rebases")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("feature")
	_ = c.MarkFlagRequired("change")
	return c
}
