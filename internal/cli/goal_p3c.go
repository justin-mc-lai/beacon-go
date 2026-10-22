// Goal P3c command surfaces: real integration.
//
//   - goal run gains --exec-cmd / --ralph-cmd / --profile /
//     --reviewer-cmd (argv configuration, recorded on the run; the
//     command VALUE is one program path — extra arguments travel as
//     repeated --exec-arg / --ralph-arg values, never a shell
//     string).
//   - goal tick / continue and goal driver run-once / run-loop
//     accept --executor subprocess | subprocess:<profile> | ralph
//     in addition to the P2a recording placeholder. An unconfigured
//     subprocess selection refuses (executor_not_configured) — it
//     NEVER falls back to recording and calls it real.
//   - goal worktree create/merge/cleanup/status (real git worktrees
//     from the recorded parallel plan).
//   - goal planner-review (real reviewer subprocesses, AC-102).
//   - goal acp-probe (initialize handshake PROBE only; the help text
//     says probe, the output says probe, full ACP is P4).
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/harness"
	"beacon-go/internal/orchestrator"
)

// execFlags carries the shared executor-configuration flags.
type execFlags struct {
	execCmd     string
	execArgs    []string
	ralphCmd    string
	ralphArgs   []string
	ralphMax    int
	profile     string
	reviewerCmd string
	timeoutSec  int
}

func (f *execFlags) config() *orchestrator.ExecutorConfig {
	cfg := &orchestrator.ExecutorConfig{TimeoutSec: f.timeoutSec, RalphMaxIter: f.ralphMax}
	if strings.TrimSpace(f.execCmd) != "" {
		cfg.Argv = append([]string{f.execCmd}, f.execArgs...)
		cfg.ProfileArgv = cfg.Argv
	}
	if strings.TrimSpace(f.ralphCmd) != "" {
		cfg.RalphArgv = append([]string{f.ralphCmd}, f.ralphArgs...)
	}
	if f.profile != "" {
		cfg.Profile = f.profile
	}
	if strings.TrimSpace(f.reviewerCmd) != "" {
		cfg.ReviewerArgv = harness.ParseArgvFlag(f.reviewerCmd)
	}
	return cfg
}

func addExecFlags(c *cobra.Command, f *execFlags, withReviewer bool) {
	c.Flags().StringVar(&f.execCmd, "exec-cmd", "", "real executor command: ONE program path (argv execution, never a shell string)")
	c.Flags().StringArrayVar(&f.execArgs, "exec-arg", nil, "argument for --exec-cmd (repeatable; each value is one argv element)")
	c.Flags().StringVar(&f.ralphCmd, "ralph-cmd", "", "ralph loop command: ONE program path (real loop when set)")
	c.Flags().StringArrayVar(&f.ralphArgs, "ralph-arg", nil, "argument for --ralph-cmd (repeatable)")
	c.Flags().IntVar(&f.ralphMax, "ralph-max-iterations", 0, "ralph loop ceiling (default 8)")
	c.Flags().StringVar(&f.profile, "profile", "", "agent profile for subprocess:<profile> (codex-style|claude-style)")
	c.Flags().IntVar(&f.timeoutSec, "exec-timeout-sec", 0, "per-stage/iteration timeout seconds (default 3600)")
	if withReviewer {
		c.Flags().StringVar(&f.reviewerCmd, "reviewer-cmd", "", "planner-review reviewer command: ONE program path")
	}
}

// realGoalEngine builds an engine and resolves the --executor
// selector for the run. Selector semantics (see exec_resolve.go):
// "" leaves the engine executor-less (ticks block honestly) unless
// the driver path overrides it; "recording" is the placeholder;
// "subprocess"/"subprocess:<profile>"/"ralph" are real and refuse
// when unconfigured.
func realGoalEngine(runID, selector string, ef *execFlags) (*orchestrator.Engine, func(), error) {
	st, err := openWriteStore()
	if err != nil {
		return nil, nil, err
	}
	eng := orchestrator.NewEngine(st)
	closeFn := func() { st.Close() }
	if selector == "" {
		return eng, closeFn, nil
	}
	var inv *orchestrator.ExecutorConfig
	if ef != nil {
		inv = ef.config()
	}
	ex, err := eng.ResolveExecutorFor(runID, selector, inv)
	if err != nil {
		closeFn()
		return nil, nil, err
	}
	if ex != nil {
		eng.RegisterExecutorForAllStages(ex)
		eng.RegisterLaneExecutorForAllLanes(ex)
	}
	return eng, closeFn, nil
}

// ---- worktree (AC-099) ----

func newGoalWorktreeCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "worktree",
		Short: "Real git worktrees from the run's parallel plan (create / merge / cleanup / status)",
		Long: "Materialize the parallel plan: one git worktree + branch per satellite " +
			"lake under <project>/.beacon-worktrees/. Merges run satellite -> main_star " +
			"only; conflicts stop and are reported, never force-resolved; cleanup refuses " +
			"unmerged or dirty worktrees unless --discard is given (and records it).",
	}
	c.AddCommand(newGoalWorktreeCreateCmd(), newGoalWorktreeMergeCmd(),
		newGoalWorktreeCleanupCmd(), newGoalWorktreeStatusCmd())
	return c
}

func printWorktreeView(cmd *cobra.Command, wv orchestrator.WorktreeView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, wv)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "run:    %s\nroot:   %s\nmain:   %s (plan %s)\n", wv.RunID, wv.ProjectRoot, wv.MainBranch, wv.PlanMode)
	for _, r := range wv.Records {
		fmt.Fprintf(out, "  %-12s %-10s %s -> %s\n", r.Lake, r.State, r.Branch, r.Path)
	}
	for _, q := range wv.Quarantines {
		fmt.Fprintf(out, "  quarantine: %s in %s (%s, truth sha %s)\n", q.FeatureSlug, q.WorktreePath, q.Reason, q.TruthSHA256)
	}
	if wv.Note != "" {
		fmt.Fprintf(out, "note:   %s\n", wv.Note)
	}
	return nil
}

func newGoalWorktreeCreateCmd() *cobra.Command {
	var runID, lake, mode string
	var isolated, allowDirty, asJSON bool
	c := &cobra.Command{
		Use:   "create",
		Short: "Create the plan's real worktrees (main tree must be clean unless --allow-dirty)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			wv, err := eng.WorktreeCreate(cmd.Context(), runID, orchestrator.WorktreeCreateOptions{
				Mode: mode, Lake: lake, Isolated: isolated, AllowDirty: allowDirty,
			})
			if wv.RunID != "" {
				if perr := printWorktreeView(cmd, wv, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "create only this lake's worktree (default: all satellites)")
	c.Flags().StringVar(&mode, "mode", "", "worktree mode: worktree (default) | current-branch | run-branch (AC-099)")
	c.Flags().BoolVar(&isolated, "isolated-worktree", false, "request isolated worktrees (with --mode current-branch this combination is refused, AC-099)")
	c.Flags().BoolVar(&allowDirty, "allow-dirty", false, "proceed despite a dirty main work tree (the override is recorded in the creation event)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalWorktreeMergeCmd() *cobra.Command {
	var runID, lake, from, to string
	var asJSON bool
	c := &cobra.Command{
		Use:   "merge",
		Short: "Merge one satellite into the main star (the only direction; conflicts stop and report)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			wv, err := eng.WorktreeMerge(cmd.Context(), runID, lake, from, to)
			if wv.RunID != "" {
				if perr := printWorktreeView(cmd, wv, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "satellite lake to merge (required)")
	c.Flags().StringVar(&from, "from", "", "merge source (must be the satellite; main_star here is refused)")
	c.Flags().StringVar(&to, "to", "", "merge target (only main_star is accepted)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("lake")
	return c
}

func newGoalWorktreeCleanupCmd() *cobra.Command {
	var runID, lake string
	var discard, asJSON bool
	c := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove one satellite worktree (refuses unmerged/dirty work unless --discard)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			wv, err := eng.WorktreeCleanup(cmd.Context(), runID, lake, discard)
			if wv.RunID != "" {
				if perr := printWorktreeView(cmd, wv, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&lake, "lake", "", "satellite lake to remove (required)")
	c.Flags().BoolVar(&discard, "discard", false, "discard unmerged/uncommitted work (recorded in the removal event)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("lake")
	return c
}

func newGoalWorktreeStatusCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the run's recorded worktrees + AC-140 stale-truth quarantines (records new findings)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// S1: status runs the quarantine scan and persists
			// NEW findings, so it needs the write engine (with no
			// new finding it writes nothing).
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			wv, err := eng.WorktreeStatus(runID)
			if err != nil {
				return err
			}
			return printWorktreeView(cmd, wv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", true, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- planner-review (AC-102) ----

func newGoalPlannerReviewCmd() *cobra.Command {
	var runID, summary string
	var ef execFlags
	var reviewers, reviewerProfiles []string
	var asJSON bool
	c := &cobra.Command{
		Use:   "planner-review",
		Short: "Multi-agent planner review: real reviewer subprocesses judge the stage plan (AC-102)",
		Long: "Run the configured reviewer profiles against the run's stage plan / truth " +
			"package. Each reviewer must print one JSON verdict line " +
			"({\"verdict\":\"pass\"|\"block\",\"reasons\":[...]}). Any valid block blocks " +
			"the review; invalid reviewers (timeout, bad output, non-zero exit) never " +
			"count as passes. Reviewers run sequentially — the output says so " +
			"(execution_mode=multi_agent_sequential); parallel execution is not claimed. " +
			"The aggregate is written to gate_results (gate_type=planner_review).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			cfg, err := eng.ExecutorConfigFor(runID, ef.config())
			if err != nil {
				return err
			}
			cmds := reviewers
			if len(cmds) == 0 && len(cfg.ReviewerArgv) > 0 {
				cmds = []string{strings.Join(cfg.ReviewerArgv, " ")}
			}
			if len(cmds) == 0 {
				return fail(core.ReasonPlannerReviewerUnconfig, "goal planner-review",
					"no reviewer configured: pass --reviewer-cmd (repeatable) or record one with goal run --reviewer-cmd")
			}
			var specs orchestrator.PlannerReviewerSpecs
			for i, rc := range cmds {
				profile := ""
				if i < len(reviewerProfiles) {
					profile = reviewerProfiles[i]
				}
				name := ""
				if profile != "" {
					name = profile
				}
				specs = append(specs, orchestrator.ReviewerSpec{
					Name: name, Profile: profile,
					Argv: harness.ParseArgvFlag(rc),
				})
			}
			view, err := eng.RunPlannerReview(cmd.Context(), runID, specs, summary, ef.timeoutSec)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, view); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "verdict: %s (%s) target %s mode %s\n", view.Verdict, view.ReasonCode, view.Target, view.ExecutionMode)
				for _, l := range view.ReviewerLanes {
					fmt.Fprintf(out, "  %-16s %-8s exit %d %v\n", l.Reviewer, l.Verdict, l.ExitCode, l.Reasons)
				}
				fmt.Fprintf(out, "synthesis: %s\n", view.Synthesis)
			}
			if view.Verdict != "pass" {
				return fail(view.ReasonCode, "goal planner-review", "planner review did not pass for run "+runID)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringArrayVar(&reviewers, "reviewer-cmd", nil, "reviewer command: ONE program path (repeatable — one lane per value)")
	c.Flags().StringArrayVar(&reviewerProfiles, "reviewer-profile", nil, "profile name per reviewer lane (codex-style|claude-style; position-matched with --reviewer-cmd)")
	c.Flags().StringVar(&summary, "summary", "", "one-line author summary stored on the review record")
	c.Flags().IntVar(&ef.timeoutSec, "exec-timeout-sec", 0, "per-reviewer timeout seconds (default 3600)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- acp-probe (AC-046, probe level only) ----

func newGoalACPProbeCmd() *cobra.Command {
	var target string
	var targetArgs []string
	var timeoutSec int
	var asJSON bool
	c := &cobra.Command{
		Use:   "acp-probe",
		Short: "PROBE ONLY: ACP initialize + session/new handshake against a target command (no ACP prompt support)",
		Long: "Probe an Agent Client Protocol target: spawn it, send an initialize " +
			"JSON-RPC request over stdio, then a session/new request, and report how " +
			"far the handshake got (protocol version, capabilities, session id, or " +
			"the step it stopped at). This is a PROBE, not an ACP backend: " +
			"session/prompt and any real iteration are not implemented — a real " +
			"iteration needs a real ACP agent target (P6 external verification, " +
			"see docs/p4c-implementation.md). Nothing on this surface claims ACP support.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			argv := harness.ParseArgvFlag(target)
			argv = append(argv, targetArgs...)
			timeout := time.Duration(timeoutSec) * time.Second
			res, err := harness.ProbeACP(cmd.Context(), argv, timeout)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "probe: %v responded: %v protocol: %d capabilities: %v session_created: %v session_id: %s stopped_at: %s\n%s\n",
				res.Probe, res.Responded, res.ProtocolVersion, res.HasAgentCapabilities, res.SessionCreated, res.SessionID, res.StoppedAt, res.Detail)
			return nil
		},
	}
	c.Flags().StringVar(&target, "target", "", "ACP target command: ONE program path (required)")
	c.Flags().StringArrayVar(&targetArgs, "target-arg", nil, "argument for the target command (repeatable)")
	c.Flags().IntVar(&timeoutSec, "timeout-sec", 10, "probe timeout seconds")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("target")
	return c
}
