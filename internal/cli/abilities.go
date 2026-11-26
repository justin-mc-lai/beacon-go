// Single-purpose command faces over the shared engine.
//
// The command groups are the public surface (see harness.Abilities).
// The faces here (implement / release) are either the same
// constructors the goal/gate groups use, or thin wrappers that fix one
// argument of the shared gate evaluation. Gate verdicts are not decided
// here: facts, verdicts and persistence belong to Engine.EvaluateGate.
// `verify-harness` is a read-only ledger check that writes no verdict.
//
// Relationship to `goal`: goal is the orchestration entry (one utterance
// walked stage by stage to the release door). These faces are the
// single-purpose tool entries over the same machinery. Both entries
// call the same implementation, so their verdicts always agree.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/orchestrator"
)

// newImplementCmd is the implement ability face: lane plan, the
// shared admission checker, the ralph delegation pair and the
// worktree group — the same constructors as under `goal`.
func newImplementCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "implement",
		Short: "Implementation: admit / worktree / report — no built-in agent executor",
		Long: "Implementation face: consume frozen truth and an admitted\n" +
			"worktree. The host does the work and records it with\n" +
			"`implement report`; the kernel runs no agent and harvests artifacts\n" +
			"under one admission rule.",
	}
	c.AddCommand(
		newGoalImplementPlanCmd(), newGoalAdmitCmd(),
		newGoalReceiveResultCmd(), newGoalWorktreeCmd(),
		newImplementTasksCmd(), newImplementReportCmd(),
	)
	return c
}

// newReleaseCmd is the release ability face: the release gate
// evaluation (shared service, type fixed) and the completion entry
// — the same constructor as `goal complete`, human confirmation
// invariant included.
func newReleaseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "release",
		Short: "Evaluate the release gate / complete a run (same implementation as `gate evaluate release` and `goal complete`)",
		Long: "Final go/no-go gate, one of the seven command groups. `release\n" +
			"evaluate` is `gate evaluate release` (the composed verdict over the\n" +
			"four earlier gates + the human confirmation on record); `release\n" +
			"complete` is `goal complete` — the run still completes only at\n" +
			"release_gate, with a human confirmation, an attached verifier and\n" +
			"a passing verification gate. Human confirmation is an invariant,\n" +
			"not a flag that can be bypassed from either entry.",
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
