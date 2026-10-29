// Thin ability faces (P4a): design / implement / qa / release as
// top-level entries of the six-ability surface.
//
// These groups are ROUTERS, not second implementations. Every
// subcommand below is either the very constructor the goal/gate
// groups use (a fresh instance of the same command — same flags,
// same engine calls, same verdicts) or a three-line wrapper that
// fixes one argument of the shared gate evaluation. No judgement
// logic lives in this file.
//
// Relationship to `goal`: goal is the orchestration entry — one
// utterance in, a run walked stage by stage to the release door.
// The ability faces are the single-purpose tool entries over the
// same machinery: check a binding, run one admission, read one
// gate, complete one run. Both entries always agree because there
// is only one implementation underneath.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/orchestrator"
)

// newDesignCmd is the design ability face: binding status + bind,
// the same commands as `goal design-stage …` (charter 3.4 slim
// boundary: the face minds bound-or-not, it never draws).
func newDesignCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "design",
		Short: "Design ability (thin face): primitive binding status / bind — same implementation as `goal design-stage`",
		Long: "Design face of the six abilities. It answers only whether the design\n" +
			"primitives (the frozen visual rules) are bound to a run — the gate that\n" +
			"keeps unbound UX work out of implement. Prototype generation belongs\n" +
			"to the host agent via the design bridge; nothing is drawn here.\n" +
			"These are the same commands as `goal design-stage status|bind`:\n" +
			"goal orchestrates, this face is the single-purpose tool entry.",
	}
	c.AddCommand(newGoalDesignStageStatusCmd(), newGoalDesignStageBindCmd())
	return c
}

// newImplementCmd is the implement ability face: lane plan, the
// shared admission checker, the ralph delegation pair and the
// worktree group — the same constructors as under `goal`.
func newImplementCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "implement",
		Short: "Implement ability (thin face): plan / admit / delegate-ralph / receive-result / worktree — same implementation as under `goal`",
		Long: "Implement face of the six abilities: consume frozen truth + a bound\n" +
			"design + an admitted worktree, then run the agent/backend loop.\n" +
			"Subcommands are the same constructors as `goal implement plan`,\n" +
			"`goal admit`, `goal delegate-ralph`, `goal receive-result` and\n" +
			"`goal worktree …` — one admission checker and one harvest rule\n" +
			"under both entries.",
	}
	c.AddCommand(
		newGoalImplementPlanCmd(), newGoalAdmitCmd(), newGoalDelegateRalphCmd(),
		newGoalReceiveResultCmd(), newGoalWorktreeCmd(),
	)
	return c
}

// newQACmd is the qa ability face: gate status (read), the qa gate
// evaluation (the shared gate service with the type fixed to qa),
// planner review, and the unified skill score (P4b) folded in —
// score / score-history live here, not on a new top-level command.
func newQACmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "qa",
		Short: "QA ability (thin face): gate status / evaluate qa / planner-review / score / score-history — same implementations as `gate`, `goal planner-review` and the one score engine",
		Long: "QA face of the six abilities: behavior evidence, the confidence chain\n" +
			"and the defect repair chain, judged by the ONE gate service.\n" +
			"`qa status` is `gate status`; `qa evaluate` is `gate evaluate qa`\n" +
			"(verdicts persist to gate_results); `qa planner-review` is\n" +
			"`goal planner-review`.\n\n" +
			"`qa score` is the unified skill-score engine (AC-151..156): a health\n" +
			"check over skill packages, recorded under gate_type=skill_score and\n" +
			"listed by `qa score-history`, apart from the gates. A score is runtime\n" +
			"evidence only (AC-153): it never substitutes this face's formal qa\n" +
			"gate, and scoring never edits the skill — a human edits, then re-scores.",
	}
	c.AddCommand(newGateStatusCmd(), newGateEvaluateFixedCmd("qa"), newGoalPlannerReviewCmd(),
		newQAScoreCmd(), newQAScoreHistoryCmd(), newQAAutoTestCmd(),
		// R2d (M2/M1): qa9 strategy generation + the agent-eval
		// line (a separate engine from the skill score above).
		newQAStrategyCmd(), newQAAgentScoreCmd(), newQAAgentEvalCmd(),
		newQAVerifyHarnessCmd())
	return c
}

// newReleaseCmd is the release ability face: the release gate
// evaluation (shared service, type fixed) and the completion entry
// — the same constructor as `goal complete`, human confirmation
// invariant included.
func newReleaseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "release",
		Short: "Release ability (thin face): evaluate release gate / complete a run — same implementation as `gate evaluate release` and `goal complete`",
		Long: "Release face of the six abilities: the final go/no-go. `release\n" +
			"evaluate` is `gate evaluate release` (the composed verdict over the\n" +
			"four earlier gates + the human confirmation on record); `release\n" +
			"complete` is `goal complete` — the run still completes only at\n" +
			"release_gate, with a human confirmation, an attached verifier and\n" +
			"a passing qa gate. Human confirmation is an invariant, not a flag\n" +
			"that can be bypassed from either entry.",
	}
	c.AddCommand(newGateEvaluateFixedCmd("release"), newGoalCompleteCmd(), newReleaseCheckCmd())
	return c
}

// newGateEvaluateFixedCmd wraps the shared formal gate evaluation
// with the gate type fixed (the qa/release thin faces). It contains
// no judgement: facts, verdict and persistence are all
// Engine.EvaluateGate's, exactly as `gate evaluate <type>` runs
// them; a blocked verdict prints, then exits with its headline
// reason code.
func newGateEvaluateFixedCmd(gateType string) *cobra.Command {
	var project, feature, version, runID, evaluator string
	var asJSON bool
	c := &cobra.Command{
		Use:   "evaluate",
		Short: "Formally evaluate the " + gateType + " gate and persist the verdict (same as `gate evaluate " + gateType + "`)",
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
			ev, err := eng.EvaluateGate(ctx, gateType, runID, evaluator)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, ev); perr != nil {
					return perr
				}
			} else {
				printEvaluationHuman(cmd, ev)
			}
			if ev.Verdict == gate.VerdictBlocked {
				code := core.ReasonStageExitBlocked
				if len(ev.ReasonCodes) > 0 {
					code = ev.ReasonCodes[0]
				}
				return fail(code, "gate evaluate", "gate "+string(ev.GateType)+" blocked for feature "+ctx.Feature.Slug)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&runID, "run", "", "goal run id (binds run-scoped gates; default: newest goal run)")
	c.Flags().StringVar(&evaluator, "evaluator", "", "evaluator identity (required; unattributed verdicts are refused)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// newQAVerifyHarnessCmd exposes the machine-oriented verification lever
// (AC-AGENT-004). It answers from the ledger and the bytes on disk — QA
// artifacts recorded, qa stage executed, every blob hash still matching —
// and exits non-zero when anything is missing or stale. It never writes a
// gate result: formal verdicts belong to `gate evaluate qa`.
func newQAVerifyHarnessCmd() *cobra.Command {
	var jsonOutput bool
	var project, slug, version, evaluator string
	cmd := &cobra.Command{
		Use:   "verify-harness",
		Short: "Machine verification lever for agents (Build the Lever)",
		Long: "Verify one feature's QA evidence without writing a gate result.\n" +
			"Passing needs recorded QA artifacts, a qa stage execution on record, and\n" +
			"every artifact blob still matching its recorded SHA-256. Anything else\n" +
			"exits non-zero with the failing checks named.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openWriteStore()
			if err != nil {
				return err
			}
			defer st.Close()
			fctx, err := resolveGateFeature(st, project, slug, version)
			if err != nil {
				return err
			}
			eng := orchestrator.NewEngine(st)
			res, err := eng.VerifyHarness(fctx, evaluator)
			if err != nil {
				return err
			}
			if jsonOutput {
				if perr := printJSON(cmd, res); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "harness verify: %s → %s (exit %d)\n", res.Slug, res.Status, res.ExitCode)
				fmt.Fprintf(out, "  evidence: %d artifact(s) hash=%s\n", res.EvidenceCount, res.EvidenceHash)
				for _, f := range res.Checks {
					if f.Applicable && !f.OK {
						fmt.Fprintf(out, "  BLOCK [%s] %s\n", f.ReasonCode, f.Detail)
					}
				}
				for _, s := range res.Stale {
					fmt.Fprintf(out, "  stale: %s\n", s)
				}
			}
			if !res.Verified {
				code := core.ReasonStageExitQAEvidenceMiss
				if len(res.ReasonCodes) > 0 {
					code = res.ReasonCodes[0]
				}
				return fail(code, "qa verify-harness", "verification blocked for feature "+res.Slug)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", true, "Output structured JSON for machine agents")
	cmd.Flags().StringVar(&project, "project", ".", "project root path")
	cmd.Flags().StringVar(&slug, "slug", "", "Target feature slug to verify (required)")
	cmd.Flags().StringVar(&version, "version", "", "docs version filter")
	cmd.Flags().StringVar(&evaluator, "evaluator", "harness", "evaluator identity recorded in the answer")
	_ = cmd.MarkFlagRequired("slug")
	return cmd
}
