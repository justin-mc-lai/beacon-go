// Goal watch commands (P2b): `goal queue` + `goal consume-queue`
// (G-09), `goal self-heal` (G-19), `goal steer` + `goal todo` (G-21).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/orchestrator"
)

// ---- discovery queue (G-09) ----

func newGoalQueueCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "queue",
		Short: "Discovery queue: loop/support discoveries wait here for goal consumption (never product-complete)",
	}
	c.AddCommand(newGoalQueueEnqueueCmd(), newGoalQueueListCmd(),
		newGoalConsumeQueueCmd(), newGoalQueueMarkCompleteCmd())
	return c
}

// newGoalConsumeQueueCmd is registered both as `goal queue consume`
// and as the v1-parity top-level `goal consume-queue`: one
// implementation, two names (the frozen v1 surface name plus the
// grouped surface).
func newGoalConsumeQueueCmd() *cobra.Command {
	var consumer string
	var asJSON bool
	c := &cobra.Command{
		Use:   "consume-queue",
		Short: "Consume the oldest queued discovery (FIFO) and start a formal goal run from it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			res, err := eng.ConsumeNext(consumer)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			if res.Status == "empty" {
				fmt.Fprintln(cmd.OutOrStdout(), "queue empty (goal_queue_empty); nothing consumed")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "consumed: %s -> run %s (remaining queued: %d)\n",
				res.Item.ItemID, res.Run.RunID, res.QueueDepth)
			return nil
		},
	}
	c.Flags().StringVar(&consumer, "consumer", "beacon-goal", "consumer identity recorded on the item")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newGoalQueueEnqueueCmd() *cobra.Command {
	var project, version, feature, source, priority string
	var asJSON bool
	c := &cobra.Command{
		Use:   "enqueue [utterance]",
		Short: "Enqueue one support discovery for goal consumption (the discover surface; starts no delivery)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			item, depth, err := eng.EnqueueDiscovery(orchestrator.EnqueueOptions{
				ProjectRoot: project, Version: version, FeatureSlug: feature,
				Utterance: args[0], Source: source, Priority: priority,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"item": item, "queue_depth": depth})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "queued: %s (depth %d) — discovery only; consume with `beacon goal consume-queue`\n",
				item.ItemID, depth)
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().StringVarP(&version, "version", "v", "", "docs version (default: current)")
	c.Flags().StringVarP(&feature, "feature", "f", "", "feature slug (default: first of the project)")
	c.Flags().StringVar(&source, "source", "beacon-loop-goal", "discovery source identity")
	c.Flags().StringVar(&priority, "priority", "normal", "queue priority label")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newGoalQueueListCmd() *cobra.Command {
	var status string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List discovery queue items, oldest first",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			items, err := eng.ListQueue(status)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"items": items, "count": len(items)})
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-8s %-12s %s\n", it.ItemID, it.Status, it.FeatureSlug, it.Utterance)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "count: %d\n", len(items))
			return nil
		},
	}
	c.Flags().StringVar(&status, "status", orchestrator.QueueQueued, "queued | consumed | \"\" for all")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// newGoalQueueMarkCompleteCmd is the loop_goal.py mark-complete port:
// an explicit surface whose only behaviour is rejection. Loop/queue
// surfaces are support discovery; product completion belongs to
// `beacon goal complete` on a real goal run.
func newGoalQueueMarkCompleteCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "mark-complete",
		Short: "Always rejected: queue/loop surfaces cannot mark product complete",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := orchestrator.RejectQueueComplete("beacon-goal-queue")
			if asJSON {
				rc, _ := core.ReasonCodeOf(err)
				_ = printJSON(cmd, map[string]any{
					"allowed": false, "blocked": true, "status": "blocked",
					"reason_code": rc, "redirect": "beacon goal complete",
					"message": "loop/queue surfaces are support discovery only; use `beacon goal complete` on a goal run",
				})
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "blocked: %s\nloop/queue surfaces are support discovery only; use `beacon goal complete` on a goal run\n",
					core.ReasonLoopCannotComplete)
			}
			return err
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// ---- self-heal (G-19) ----

func newGoalSelfHealCmd() *cobra.Command {
	var runID, phase string
	var planOnly, asJSON bool
	c := &cobra.Command{
		Use:   "self-heal",
		Short: "Self-heal band (pre/mid/post): plan, or execute only the auto steps — the post-release step always needs human confirmation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var plan orchestrator.SelfHealPlan
			if planOnly {
				plan, err = eng.PlanSelfHeal(runID, phase)
			} else {
				plan, err = eng.ExecuteSelfHeal(runID, phase)
			}
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, plan)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "run: %s phase: %s executed: %v\n", plan.RunID, plan.Phase, plan.Executed)
			for _, s := range plan.Steps {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-26s %-22s %s\n", s.ID, s.Status, s.UserFacing)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&phase, "phase", "", "pre | mid | post (default: derived from run state)")
	c.Flags().BoolVar(&planOnly, "plan", false, "only show the plan, execute nothing")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- steering (G-21) ----

func newGoalSteerCmd() *cobra.Command {
	var runID, kind, text string
	var ack, asJSON bool
	c := &cobra.Command{
		Use:   "steer",
		Short: "Record a mid-run direction change (surfaced by tick/resume/status until --ack)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var v orchestrator.RunView
			if ack {
				v, err = eng.AckSteering(runID)
			} else {
				v, err = eng.Steer(runID, kind, text)
			}
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&text, "text", "", "steering text (required unless --ack)")
	c.Flags().StringVar(&kind, "kind", orchestrator.SteeringObjective, "objective (supersedes the run objective) | note (advisory)")
	c.Flags().BoolVar(&ack, "ack", false, "acknowledge all pending steering records")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

// ---- todos (G-21) ----

func newGoalTodoCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "todo",
		Short: "The run's own todo list (its summary is part of goal status)",
	}
	c.AddCommand(newGoalTodoAddCmd(), newGoalTodoListCmd(), newGoalTodoDoneCmd(), newGoalTodoRemoveCmd())
	return c
}

func newGoalTodoAddCmd() *cobra.Command {
	var runID, title string
	var asJSON bool
	c := &cobra.Command{
		Use:   "add",
		Short: "Add one todo to the run's list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			item, v, err := eng.TodoAdd(runID, title)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"todo": item, "run": v})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added: %s %s (todos: %d open / %d total)\n",
				item.ID, item.Title, v.Todos.Open, v.Todos.Total)
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&title, "title", "", "todo title (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("title")
	return c
}

func newGoalTodoListCmd() *cobra.Command {
	var runID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List the run's todos in order",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			items, err := eng.TodoList(runID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"run_id": runID, "todos": items})
			}
			for _, it := range items {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %-8s %s\n", it.ID, it.Status, it.Title)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	return c
}

func newGoalTodoDoneCmd() *cobra.Command {
	return newGoalTodoMutateCmd("done", "Mark one todo done", func(eng *orchestrator.Engine, runID, id string) (orchestrator.RunView, error) {
		return eng.TodoDone(runID, id)
	})
}

func newGoalTodoRemoveCmd() *cobra.Command {
	return newGoalTodoMutateCmd("remove", "Remove one todo from the run's list", func(eng *orchestrator.Engine, runID, id string) (orchestrator.RunView, error) {
		return eng.TodoRemove(runID, id)
	})
}

func newGoalTodoMutateCmd(name, short string, fn func(*orchestrator.Engine, string, string) (orchestrator.RunView, error)) *cobra.Command {
	var runID, id string
	var asJSON bool
	c := &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			v, err := fn(eng, runID, id)
			if err != nil {
				return err
			}
			return printGoalView(cmd, v, asJSON)
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (required)")
	c.Flags().StringVar(&id, "id", "", "todo id (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("run-id")
	_ = c.MarkFlagRequired("id")
	return c
}
