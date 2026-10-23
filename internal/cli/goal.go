// Goal command group (P2a): the user-facing entry is `goal` (charter
// decision 5, 2026-10-21 — the frozen name "loop" is superseded; the
// gen-change bookkeeping for the frozen package is a pre-P3 todo, see
// docs/scope-charter.md section 6). Subcommands mirror the minimum set
// of the v1 beacon/commands/goal.py surface over the orchestrator
// engine. All state lives in the global store; --json emits exactly
// one JSON document with snake_case fields.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

// goalReasonRunNotFound is the CLI surface code for a missing run; the
// engine's own code is product_goal_run_not_found (source constant).
const goalReasonRunNotFound = orchestrator.ReasonRunNotFound

func newGoalCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "goal",
		Short: "Goal runs: the loop ability's formal entry (create, tick, complete, resume)",
		Long: "Goal is the formal user entry for the loop ability (charter decision 5).\n" +
			"A goal run walks plan -> truth -> (design) -> implement -> qa -> release\n" +
			"under the loop-goal state machine; release completion always needs\n" +
			"human confirmation, an attached verifier and a passing qa gate.",
	}
	c.AddCommand(
		newGoalRunCmd(), newGoalStatusCmd(), newGoalTickCmd(false), newGoalTickCmd(true),
		newGoalBlockCmd(), newGoalAttachVerifierCmd(), newGoalCompleteCmd(),
		newGoalCheckpointCmd(), newGoalHeartbeatCmd(), newGoalResumeCmd(), newGoalTraceCmd(),
		// P4a debt clearance: entry-contract (G-01) + resume-pack
		// (G-08) — the two P2 sub-surfaces that had no commands of
		// their own.
		newGoalEntryContractCmd(), newGoalResumePackCmd(),
		// P2b long-run care: driver (G-06), supervise + longrun-doctor
		// (G-07), queue + consume-queue (G-09), self-heal (G-19),
		// steer + todo (G-21).
		newGoalDriverCmd(), newGoalSuperviseCmd(), newGoalLongrunDoctorCmd(),
		newGoalQueueCmd(), newGoalConsumeQueueCmd(), newGoalSelfHealCmd(),
		newGoalSteerCmd(), newGoalTodoCmd(),
		// P2c delegation & parallel: implement lanes (G-22), admit
		// (G-04), delegate-ralph + receive-result (G-10),
		// parallel-plan (G-11), design-stage (G-18).
		newGoalImplementCmd(), newGoalAdmitCmd(), newGoalDelegateRalphCmd(),
		newGoalReceiveResultCmd(), newGoalParallelPlanCmd(), newGoalDesignStageCmd(),
		// P2d orchestration extensions: program (G-12), hotfix-plan
		// (G-13), multi-version-plan (G-14), memory (G-15),
		// axis (G-16).
		newGoalProgramCmd(), newGoalHotfixPlanCmd(), newGoalMultiVersionPlanCmd(),
		newGoalMemoryCmd(), newGoalAxisCmd(),
		// P3c real integration: worktree (AC-099), planner-review
		// (AC-102), acp-probe (AC-046 probe level only).
		newGoalWorktreeCmd(), newGoalPlannerReviewCmd(), newGoalACPProbeCmd(),
		// P6 E2E fix: manual artifact registration (the write path
		// whose absence stopped the end-to-end chain at the qa
		// gate); folded into goal, no new top-level command.
		newGoalArtifactsCmd(),
		// S2: the standard execution flow (AC-138) + admission
		// evidence bundle (AC-142), folded into goal as the flow
		// subgroup — no new top-level command.
		newGoalFlowCmd(),
		newGoalTopologyCmd(),
		// R2b: quota (H6), orchestration records (M8 — decide /
		// gate / fork / error-recovery) and the spend ledger (M9).
		newGoalQuotaCmd(), newGoalDecideCmd(), newGoalGateCmd(), newGoalForkCmd(),
		newGoalErrorRecoveryCmd(), newGoalSpendCmd(),
	)
	return c
}

// goalEngine builds an engine over a write store. The recording
// executor is registered only when explicitly requested via
// --executor recording: it is a placeholder that records calls and
// writes evidence events, never a real agent backend.
func goalEngine(executor string) (*orchestrator.Engine, func(), error) {
	st, err := openWriteStore()
	if err != nil {
		return nil, nil, err
	}
	eng := orchestrator.NewEngine(st)
	if executor == "recording" {
		rec := orchestrator.NewRecordingExecutor(st, "recording")
		eng.RegisterExecutorForAllStages(rec)
		eng.RegisterLaneExecutorForAllLanes(rec)
	}
	return eng, func() { st.Close() }, nil
}

// goalReadEngine builds an engine over a read-only store for the pure
// read commands (status / resume / trace).
func goalReadEngine() (*orchestrator.Engine, func(), error) {
	st, exists, err := openReadStore()
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, fail(goalReasonRunNotFound, "goal", "no runtime DB yet; run not found")
	}
	return orchestrator.NewEngine(st), func() { st.Close() }, nil
}

func printGoalView(cmd *cobra.Command, v orchestrator.RunView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, v)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "run:    %s\nstatus: %s (loop_stage %s)\nstage:  %s\nblocked: %v",
		v.RunID, v.Status, v.LoopStage, v.CurrentStage, v.Blocked)
	if v.ReasonCode != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nreason: %s", v.ReasonCode)
	}
	if v.NextSkill != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nnext:   %s", v.NextSkill)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

// blockedErr turns a blocked view into a reason-coded CLI failure
// (v1 goal commands exit non-zero when blocked).
func blockedErr(v orchestrator.RunView) error {
	if !v.Blocked {
		return nil
	}
	reason := v.ReasonCode
	if reason == "" {
		reason = "goal_blocked"
	}
	return fail(reason, "goal", "run "+v.RunID+" is blocked")
}

func newGoalRunCmd() *cobra.Command {
	var project, version, feature, executorID, scale string
	var lakes []string
	var ux, asJSON bool
	var ef execFlags
	c := &cobra.Command{
		Use:   "run [utterance]",
		Short: "Create a goal run for a registered feature (build_stage_order plan)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			utterance := ""
			if len(args) > 0 {
				utterance = args[0]
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.CreateRun(orchestrator.CreateRunOptions{
				ProjectRoot: project, Version: version, FeatureSlug: feature,
				Utterance: utterance, UXRequired: ux, ExecutorID: executorID,
				Lakes: lakes, Scale: scale, Exec: ef.config(),
			})
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().StringVarP(&version, "version", "v", "", "docs version (default: current)")
	c.Flags().StringVarP(&feature, "feature", "f", "", "feature slug (default: first of the project)")
	c.Flags().BoolVar(&ux, "ux", false, "UX/design work required (inserts the design stage; explicit signal, no guessing)")
	c.Flags().StringArrayVar(&lakes, "lake", nil, "lake slugs (recorded on the run; lane routing and parallel plans consult them)")
	c.Flags().StringVar(&scale, "scale", "", "task scale field (large routes the implement lane to ralph)")
	c.Flags().StringVar(&executorID, "executor-id", "", "identity of the executing agent (maker, for maker!=checker)")
	addExecFlags(c, &ef, true)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newGoalStatusCmd() *cobra.Command {
	var runID, project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show one goal run's status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.Status(runID)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalTickCmd(cont bool) *cobra.Command {
	var runID, project, executor, note string
	var asJSON bool
	var ef execFlags
	name, short := "tick", "Advance the run by one step (exit-gate judged, idempotent)"
	if cont {
		name, short = "continue", "Unblock a soft-blocked run (streak < 3) and advance one step"
	}
	c := &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := realGoalEngine(runID, executor, &ef)
			if err != nil {
				return err
			}
			defer closeFn()
			var v orchestrator.RunView
			if cont {
				v, err = eng.Continue(cmd.Context(), runID)
			} else {
				v, err = eng.Tick(cmd.Context(), runID)
			}
			if err != nil {
				return err
			}
			if perr := printGoalView(cmd, v, asJSON); perr != nil {
				return perr
			}
			return blockedErr(v)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().StringVar(&executor, "executor", "", "stage executor: recording (placeholder, no real work) | subprocess (real, needs --exec-cmd or a recorded one) | subprocess:<profile> (real harness profile) | ralph (real loop, implement lane)")
	c.Flags().StringVar(&note, "note", "", "free-form note (accepted for v1 parity)")
	addExecFlags(c, &ef, false)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalBlockCmd() *cobra.Command {
	var runID, project, reasonCode string
	var asJSON bool
	c := &cobra.Command{
		Use:   "block",
		Short: "Record a blocker on the run (reason_code lands in the lifecycle snapshot)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.Block(runID, reasonCode)
			if err != nil {
				return err
			}
			if perr := printGoalView(cmd, v, asJSON); perr != nil {
				return perr
			}
			return blockedErr(v)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&reasonCode, "reason-code", "manual_block", "blocker reason code")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalAttachVerifierCmd() *cobra.Command {
	var runID, project, source, executorID string
	var refs []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "attach-verifier",
		Short: "Attach an independent verifier (maker != checker) to the run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.AttachVerifier(runID, refs, source, executorID)
			if v.RunID != "" {
				if perr := printGoalView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringArrayVar(&refs, "ref", nil, "verifier evidence refs (repeatable, at least one required)")
	c.Flags().StringVar(&source, "source", "beacon-eval-qa", "verifier identity (checker; must differ from the executor)")
	c.Flags().StringVar(&executorID, "executor-id", "", "executor identity override for the maker!=checker check")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalCompleteCmd() *cobra.Command {
	var runID, project, confirmedBy string
	var humanConfirmed, force, asJSON bool
	c := &cobra.Command{
		Use:   "complete",
		Short: "Complete the run at release_gate (human confirmed + verifier + qa pass required)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.Complete(runID, orchestrator.CompleteOptions{
				HumanConfirmed: humanConfirmed, ConfirmedBy: confirmedBy,
			})
			if v.RunID != "" {
				if perr := printGoalView(cmd, v, asJSON); perr != nil {
					return perr
				}
			}
			if err == nil {
				// R2c (M12): completion triggers the project's
				// post_release hooks. Outcomes ride the run's
				// event trail; a hook outcome never un-completes
				// the run, and manual_confirmed hooks are only
				// ever reported as awaiting confirmation here.
				if summary, herr := eng.RunPostReleaseHooks(runID); herr == nil && len(summary.Hooks) > 0 {
					for _, h := range summary.Hooks {
						line := fmt.Sprintf("hook %s: %s (%s)", h.HookID, h.Status, h.Action)
						if asJSON {
							fmt.Fprintln(cmd.ErrOrStderr(), line)
						} else {
							fmt.Fprintln(cmd.OutOrStdout(), line)
						}
					}
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&humanConfirmed, "human-confirmed", false, "this invocation carries the human release confirmation")
	c.Flags().StringVar(&confirmedBy, "confirmed-by", "cli", "who confirms the release")
	c.Flags().BoolVar(&force, "force", false, "accepted for v1 parity; never bypasses the four complete conditions")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalCheckpointCmd() *cobra.Command {
	var runID, project, nextAction, substep string
	var asJSON bool
	c := &cobra.Command{
		Use:   "checkpoint",
		Short: "Write a resume checkpoint (store checkpoints, iteration strictly increasing)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			ckpt, v, err := eng.Checkpoint(runID, nextAction, substep)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"checkpoint": ckpt, "run": v})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "checkpoint %s iteration %d for run %s (next: %s)\n",
				ckpt.ID, ckpt.Iteration, v.RunID, nextAction)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&nextAction, "next-action", "", "next action recorded in the checkpoint (required)")
	c.Flags().StringVar(&substep, "substep", "", "current substep")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("next-action")
	return c
}

func newGoalHeartbeatCmd() *cobra.Command {
	var runID, project, summary, substep string
	var asJSON bool
	c := &cobra.Command{
		Use:   "heartbeat",
		Short: "Record a heartbeat for the run (updates last_heartbeat_at)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.Heartbeat(runID, summary, substep)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&summary, "summary", "", "heartbeat summary (required)")
	c.Flags().StringVar(&substep, "substep", "", "current substep")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("summary")
	return c
}

func newGoalResumeCmd() *cobra.Command {
	var runID, project, utterance string
	var asJSON bool
	c := &cobra.Command{
		Use:   "resume",
		Short: "Decide how a run resumes (load_next_skill / await_human_confirmation / replay_substep / none)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resume is not a pure read: it clears soft blocks and
			// consumes a pending interrupt, so it needs a write store.
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			rv, err := eng.Resume(runID, utterance)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rv)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run:    %s\nstage:  %s (loop %s)\naction: %s\nnext:   %s\n",
				rv.RunID, rv.CurrentStage, rv.LoopStage, rv.ResumeAction, rv.NextSkill)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&utterance, "utterance", "", "resume utterance (short words 继续/continue/resume/go map to load_next_skill)")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// newGoalEntryContractCmd is the G-01 debt from P2: the run's
// public entry contract as its own read-only command.
func newGoalEntryContractCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "entry-contract",
		Short: "Show the run's entry contract: public surface + stage plan + per-stage skills (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.EntryContract(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "run:     %s (%s)\nentry:   %s (single_entry=%v)\nstage:   %s (loop %s)\n",
				v.RunID, v.FeatureSlug, v.PublicCLI, v.SingleEntry, v.CurrentStage, v.LoopStage)
			for _, s := range v.Stages {
				fmt.Fprintf(out, "  %-10s %-8s skills=%v\n", s.Stage, s.Status, s.Skills)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// newGoalResumePackCmd is the G-08 debt from P2: the full recovery
// pack as its own read-only command (Resume itself stays the
// acting entry — the pack consumes and clears nothing).
func newGoalResumePackCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "resume-pack",
		Short: "Show the run's full recovery pack: snapshot + resume decision + latest checkpoint + open blockers (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := eng.ResumePack(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, v)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "run:       %s (%s)\naction:    %s\nnext:      %s\nblockers:  %v\n",
				v.Run.RunID, v.Run.Status, v.Resume.ResumeAction, v.Resume.NextAction, v.OpenBlockers)
			if v.LatestCheckpoint != nil {
				fmt.Fprintf(out, "checkpoint: %s (iteration %d)\n", v.LatestCheckpoint.ID, v.LatestCheckpoint.Iteration)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalTraceCmd() *cobra.Command {
	var runID, project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "trace",
		Short: "Aggregate the run's full lifecycle event trail in time order (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			bundle, err := eng.Trace(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, bundle)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run %s (%s) events: %d\n", bundle.RunID, bundle.Status, bundle.EventCount)
			for _, ev := range bundle.Events {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-18s %s\n", ev.CreatedAt, ev.EventType, string(ev.Payload))
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVarP(&project, "project", "p", "", "project root path (accepted for v1 parity)")
	c.Flags().BoolVar(&asJSON, "json", true, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
