// R2b goal surfaces: quota (H6), the orchestration records (M8 —
// decide / gate / fork / error-recovery) and the spend ledger
// (M9). All are thin wrappers over the engine; every judgement
// lives in core/orchestrator.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
)

// ---- quota (H6) ----

func newGoalQuotaCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "quota",
		Short: "Goal compute quota: five-segment should-run decision + time-window slot ledger (goal_quota.py)",
	}
	c.AddCommand(newGoalQuotaStatusCmd(), newGoalQuotaConfigureCmd())
	return c
}

func newGoalQuotaStatusCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the run's quota decision (health -> operator gate -> evidence -> focus -> slots)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.QuotaStatus(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "quota:    %s (spent %d / allowed %d slots)\n", v.Decision.Decision, v.Decision.SpentSlots, v.Decision.AllowedSlots)
			fmt.Fprintf(out, "state:    %s\n", v.Decision.State)
			if v.Decision.ReasonCode != "" {
				fmt.Fprintf(out, "reason:   %s\n", v.Decision.ReasonCode)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalQuotaConfigureCmd() *cobra.Command {
	var runID string
	var compute float64
	var windowHours, slotMinutes, allowedSlots int
	var asJSON bool
	c := &cobra.Command{
		Use:   "configure",
		Short: "Set the run's quota config (stored raw; every decision re-normalises it)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ConfigureQuota(runID, core.QuotaConfig{
				Compute: compute, WindowHours: windowHours,
				SlotMinutes: slotMinutes, AllowedSlots: allowedSlots,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "quota configured: allowed_slots=%d (compute=%g window=%dh slot=%dm)\n",
				v.Config.AllowedSlots, v.Config.Compute, v.Config.WindowHours, v.Config.SlotMinutes)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().Float64Var(&compute, "compute", 0, "compute factor (default 1.0)")
	c.Flags().IntVar(&windowHours, "window-hours", 0, "window length in hours (default 24)")
	c.Flags().IntVar(&slotMinutes, "slot-minutes", 0, "slot length in minutes (default 1)")
	c.Flags().IntVar(&allowedSlots, "allowed-slots", 0, "explicit slot allowance (default: derived from window/slot/compute)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- records (M8) ----

func newGoalDecideCmd() *cobra.Command {
	var runID, key, value string
	var asJSON bool
	c := &cobra.Command{
		Use:   "decide",
		Short: "Record a user decision on the run (cached: the same key is never asked twice)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.RecordUserDecision(runID, key, value)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&key, "key", "", "decision key (required), e.g. planner_review_user_decision")
	c.Flags().StringVar(&value, "value", "", "the user's answer (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("key")
	_ = c.MarkFlagRequired("value")
	return c
}

func newGoalGateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gate",
		Short: "User gate cards (goal_gates.py): build a card, resolve it once, assert it before advancing",
	}
	c.AddCommand(newGoalGateCardCmd(), newGoalGateResolveCmd(), newGoalGateCheckCmd())
	return c
}

func newGoalGateCardCmd() *cobra.Command {
	var runID, key, question, context, resumeAction string
	var asJSON bool
	c := &cobra.Command{
		Use:   "card",
		Short: "Build a gate card (a cached decision returns the cached answer instead)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			res, err := eng.BuildGateCard(runID, key, question, context, resumeAction)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			out := cmd.OutOrStdout()
			if res.Duplicate {
				fmt.Fprintf(out, "already decided: %s = %s\n", key, res.CachedAnswer)
				return nil
			}
			fmt.Fprintf(out, "gate card [%s]: %s\n", res.Card.DecisionKey, res.Card.Question)
			if res.Card.Context != "" {
				fmt.Fprintf(out, "context: %s\n", res.Card.Context)
			}
			fmt.Fprintf(out, "resume action: %s\n", res.Card.ResumeAction)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&key, "key", "", "decision key (required)")
	c.Flags().StringVar(&question, "question", "", "the concrete question for the human (required)")
	c.Flags().StringVar(&context, "context", "", "context shown with the question")
	c.Flags().StringVar(&resumeAction, "resume-action", "", "resume action after the answer (default await_human_confirmation)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("key")
	_ = c.MarkFlagRequired("question")
	return c
}

func newGoalGateResolveCmd() *cobra.Command {
	var runID, key, answer string
	var asJSON bool
	c := &cobra.Command{
		Use:   "resolve",
		Short: "Record the user's answer for a gate card (re-resolving is refused: already_resolved)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ResolveGate(runID, key, answer)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&key, "key", "", "decision key (required)")
	c.Flags().StringVar(&answer, "answer", "", "the user's answer (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("key")
	_ = c.MarkFlagRequired("answer")
	return c
}

func newGoalGateCheckCmd() *cobra.Command {
	var runID, key string
	var asJSON bool
	c := &cobra.Command{
		Use:   "check",
		Short: "Assert a gate is resolved before advancing (unresolved: gate_unresolved)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			ok, code, err := eng.AssertGateResolved(runID, key)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"resolved": ok, "reason_code": code})
			}
			if !ok {
				return fail(code, "goal gate check", "gate "+key+" is unresolved for run "+runID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "gate resolved: %s\n", key)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&key, "key", "", "decision key (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("key")
	return c
}

func newGoalForkCmd() *cobra.Command {
	var runID, stage, utterance, decision, reasonCode string
	var asJSON bool
	c := &cobra.Command{
		Use:   "fork",
		Short: "Record an intent fork at a routing decision point (record_intent_fork)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.RecordIntentFork(runID, stage, utterance, decision, reasonCode)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&stage, "stage", "", "loop stage at the fork (required)")
	c.Flags().StringVar(&utterance, "utterance", "", "the utterance slice that caused the fork")
	c.Flags().StringVar(&decision, "decision", "", "the decision taken (required)")
	c.Flags().StringVar(&reasonCode, "reason-code", "", "reason code of the decision")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("stage")
	_ = c.MarkFlagRequired("decision")
	return c
}

func newGoalErrorRecoveryCmd() *cobra.Command {
	var runID, reasonCode string
	var asJSON bool
	c := &cobra.Command{
		Use:   "error-recovery",
		Short: "Route the run to error_recovery through the one shared door (route_to_error_recovery)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.RouteToErrorRecovery(runID, reasonCode)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&reasonCode, "reason-code", "", "why the run is being sent to recovery")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- spend ledger (M9) ----

func newGoalSpendCmd() *cobra.Command {
	var runID, source, digest, ref string
	var validated, asJSON bool
	c := &cobra.Command{
		Use:   "spend",
		Short: "Charge one spend to the run's spend ledger (whitelist + writeback + idempotency guards)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			ledger, err := eng.RecordSpend(runID, source, digest, validated, ref)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, ledger)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "spent %d / %d (goal %s)\n", ledger.Spent, ledger.QuotaLimit, ledger.GoalID)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&source, "source", "", "spend source (required): delivery_completion|repair|self_repair|capability_repair")
	c.Flags().StringVar(&digest, "digest", "", "index digest — the idempotency key (required)")
	c.Flags().StringVar(&ref, "writeback-ref", "", "writeback reference recorded with the spend")
	c.Flags().BoolVar(&validated, "writeback-validated", false, "the turn's writeback was validated (required to spend)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("source")
	_ = c.MarkFlagRequired("digest")
	return c
}
