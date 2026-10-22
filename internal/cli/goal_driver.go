// Goal long-run care commands (P2b): `goal driver` (G-06),
// `goal supervise` + `goal longrun-doctor` (G-07). All state lives in
// the global store via the orchestrator engine; the recording executor
// remains an explicit placeholder opt-in (--executor recording).
package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

// ---- driver (G-06) ----

func newGoalDriverCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "driver",
		Short: "Long-run driver: pushes a goal run turn by turn, never completes or releases it",
	}
	c.AddCommand(newGoalDriverStartCmd(), newGoalDriverStatusCmd(), newGoalDriverStopCmd(),
		newGoalDriverRunOnceCmd(), newGoalDriverRunLoopCmd())
	return c
}

func printDriverView(cmd *cobra.Command, dv orchestrator.DriverView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, dv)
	}
	state := dv.State
	turns := 0
	if dv.Driver != nil {
		turns = dv.Driver.TurnN
	}
	fmt.Fprintf(cmd.OutOrStdout(), "run:    %s\ndriver: %s (turn %d, lease_active %v)\nstatus: %s (loop_stage %s)\nstage:  %s",
		dv.RunID, state, turns, dv.LeaseActive, dv.Run.Status, dv.Run.LoopStage, dv.Run.CurrentStage)
	if dv.Driver != nil && dv.Driver.HaltReason != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nhalt:   %s", dv.Driver.HaltReason)
	}
	if dv.Driver != nil && dv.Driver.NextPrompt != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nnext:   %s", dv.Driver.NextPrompt)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

func newGoalDriverStartCmd() *cobra.Command {
	var runID string
	var maxTurns, turnTimeoutSec, maxDurationSec, intervalSec int
	var supervise, asJSON bool
	c := &cobra.Command{
		Use:   "start",
		Short: "Start the run's driver (one live lease per run; a second start is refused)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			dv, err := eng.DriverStart(runID, orchestrator.DriverStartOptions{
				MaxTurns: maxTurns, TurnTimeoutSec: turnTimeoutSec, MaxDurationSec: maxDurationSec,
				AttachSupervisor: supervise, SupervisorIntervalSec: intervalSec,
			})
			if dv.RunID != "" {
				if perr := printDriverView(cmd, dv, asJSON); perr != nil {
					return perr
				}
			}
			return err
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().IntVar(&maxTurns, "max-turns", 0, "budget: max driver turns (default 32)")
	c.Flags().IntVar(&turnTimeoutSec, "turn-timeout-sec", 0, "per-turn timeout recorded for the host bridge (default 3600)")
	c.Flags().IntVar(&maxDurationSec, "max-duration-sec", 0, "budget: max total driver seconds (0 = unlimited)")
	c.Flags().BoolVar(&supervise, "supervise", true, "attach the durable supervisor (source parity)")
	c.Flags().IntVar(&intervalSec, "interval-sec", 0, "supervisor heartbeat interval seconds (default 120, min 30)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDriverStatusCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Show the run's driver state (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			dv, err := eng.DriverStatus(runID)
			if err != nil {
				return err
			}
			return printDriverView(cmd, dv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDriverStopCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "stop",
		Short: "Stop the run's driver (agent_driver_user_stop) and release its lease",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			dv, err := eng.DriverStop(runID)
			if err != nil {
				return err
			}
			return printDriverView(cmd, dv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDriverRunOnceCmd() *cobra.Command {
	var runID, executor string
	var asJSON bool
	var ef execFlags
	c := &cobra.Command{
		Use:   "run-once",
		Short: "Push exactly one driver turn (tick + heartbeat + checkpoint + next prompt)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := realGoalEngine(runID, executor, &ef)
			if err != nil {
				return err
			}
			defer closeFn()
			dv, err := eng.DriverRunOnce(cmd.Context(), runID)
			if err != nil {
				return err
			}
			return printDriverView(cmd, dv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&executor, "executor", "", "stage executor: recording (placeholder) | subprocess | subprocess:<profile> | ralph (real; unconfigured subprocess refuses, never falls back)")
	addExecFlags(c, &ef, false)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalDriverRunLoopCmd() *cobra.Command {
	var runID, executor string
	var maxRounds, intervalMS int
	var asJSON bool
	var ef execFlags
	c := &cobra.Command{
		Use:   "run-loop",
		Short: "Push driver turns until halt (budget / terminal / release gate) or --max-rounds",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := realGoalEngine(runID, executor, &ef)
			if err != nil {
				return err
			}
			defer closeFn()
			var sleep func()
			if intervalMS > 0 {
				sleep = func() { time.Sleep(time.Duration(intervalMS) * time.Millisecond) }
			}
			lv, err := eng.DriverRunLoop(cmd.Context(), runID, maxRounds, sleep)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, lv)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run:    %s\nrounds: %d\nfinal:  %s\nstatus: %s (loop_stage %s)\n",
				lv.RunID, lv.Rounds, lv.FinalState, lv.Run.Status, lv.Run.LoopStage)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().IntVar(&maxRounds, "max-rounds", 0, "max turns this loop invocation (default: driver max turns)")
	c.Flags().IntVar(&intervalMS, "interval-ms", 0, "sleep between turns in milliseconds (0 = none)")
	c.Flags().StringVar(&executor, "executor", "", "stage executor: recording (placeholder) | subprocess | subprocess:<profile> | ralph (real; unconfigured subprocess refuses, never falls back)")
	addExecFlags(c, &ef, false)
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- supervise + longrun-doctor (G-07) ----

func newGoalSuperviseCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "supervise",
		Short: "Durable supervisor: opt-in liveness watchdog for a goal run",
	}
	c.AddCommand(newGoalSuperviseAttachCmd(), newGoalSuperviseDetachCmd(), newGoalSuperviseOnceCmd())
	return c
}

func printSuperviseView(cmd *cobra.Command, sv orchestrator.SuperviseView, asJSON bool) error {
	if asJSON {
		return printJSON(cmd, sv)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "run:      %s\nattached: %v\nstall:    %v (heartbeat_age %ds, threshold %ds)",
		sv.RunID, sv.Attached, sv.StallDetected, sv.HeartbeatAgeSec, sv.ThresholdSec)
	if sv.ReasonCode != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\nreason:   %s", sv.ReasonCode)
	}
	fmt.Fprintln(cmd.OutOrStdout())
	return nil
}

func newGoalSuperviseAttachCmd() *cobra.Command {
	var runID string
	var intervalSec int
	var asJSON bool
	c := &cobra.Command{
		Use:   "attach",
		Short: "Attach the supervisor to a run (opt-in; never automatic on run creation)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			sv, err := eng.AttachSupervisor(runID, intervalSec)
			if err != nil {
				return err
			}
			return printSuperviseView(cmd, sv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().IntVar(&intervalSec, "interval-sec", 0, "heartbeat interval seconds (default 120, min 30)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalSuperviseDetachCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "detach",
		Short: "Detach the supervisor from a run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			sv, err := eng.DetachSupervisor(runID)
			if err != nil {
				return err
			}
			return printSuperviseView(cmd, sv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalSuperviseOnceCmd() *cobra.Command {
	var runID string
	var thresholdSec int
	var asJSON bool
	c := &cobra.Command{
		Use:   "once",
		Short: "Run one supervision poll: detect a stall and mark the run host-interrupted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			sv, err := eng.SuperviseOnce(runID, thresholdSec)
			if err != nil {
				return err
			}
			return printSuperviseView(cmd, sv, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().IntVar(&thresholdSec, "threshold-sec", 0, "stall threshold seconds (default: recorded heartbeat interval, else 1800)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalLongrunDoctorCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "longrun-doctor",
		Short: "Per-run long-run health report: driver rounds, heartbeat age, stalls, streaks (read-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			rep, err := eng.LongrunDoctor(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run:       %s\nstatus:    %s (loop_stage %s, stage %s)\ndriver:    %s (rounds %d)\nheartbeat: %ds old\nstalls:    %d (interrupts %d)\nstreak:    %d, repair loops %d\nhealthy:   %v %v\n",
				rep.RunID, rep.Status, rep.LoopStage, rep.CurrentStage, rep.DriverState, rep.DriverRounds,
				rep.HeartbeatAgeSec, rep.StallCount, rep.InterruptCount, rep.BlockedStreak, rep.RepairLoopCount,
				rep.Healthy, rep.Issues)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", true, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}
