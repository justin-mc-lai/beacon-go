// hooks (R2c, M12): the project lifecycle hook face — list / plan /
// run / disable / status over the contract files in
// docs/beacon/hooks (and .beacon/hooks), mirroring Python's
// commands/hooks.py surface and flags.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
)

func newHooksCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "hooks",
		Short: "Project lifecycle hooks: contracts, admission, confirmed execution, post-release trigger",
	}
	c.AddCommand(newHooksListCmd(), newHooksPlanCmd(), newHooksRunCmd(), newHooksDisableCmd(), newHooksStatusCmd())
	return c
}

func newHooksListCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List project hook contracts (docs/beacon/hooks + .beacon/hooks)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := absProject(project)
			hooks := orchestrator.ListHookContracts(root)
			if hooks == nil {
				hooks = []orchestrator.HookContract{}
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"schema_version": "1.0", "status": "pass", "project_root": root,
					"hook_count": len(hooks), "hooks": hooks,
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Beacon Project Hooks")
			for _, h := range hooks {
				fmt.Fprintf(out, "- %s: %s (%s)\n", h.HookID, h.HookCode, h.Path)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newHooksPlanCmd() *cobra.Command {
	var project, version, feature, hookCode string
	var asJSON bool
	c := &cobra.Command{
		Use:   "plan <hook-id>",
		Short: "Plan a hook without executing side effects",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan := orchestrator.PlanHook(absProject(project), args[0], version, feature, hookCode)
			if asJSON {
				return printJSON(cmd, plan)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "status: %s\n", plan.Status)
			for _, d := range plan.Admission.Diagnostics {
				fmt.Fprintf(out, "  %s: %s\n", d.ReasonCode, d.Message)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().StringVarP(&version, "version", "v", "", "docs version for evidence context")
	c.Flags().StringVarP(&feature, "feature", "f", "", "feature context")
	c.Flags().StringVar(&hookCode, "hook-code", "", "expected hook code")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newHooksRunCmd() *cobra.Command {
	var project, version, feature, hookCode string
	var dryRun, confirm, asJSON bool
	c := &cobra.Command{
		Use:   "run <hook-id>",
		Short: "Run a hook through admission, dry-run, or manual confirmation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res := orchestrator.RunHook(context.Background(), absProject(project), args[0], orchestrator.HookRunOptions{
				Version: version, Feature: feature, HookCode: hookCode,
				DryRun: dryRun, Confirm: confirm,
			})
			if asJSON {
				return printJSON(cmd, res)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "status: %s\n", res.Status)
			if res.ReasonCode != "" {
				fmt.Fprintf(out, "reason_code: %s\n", res.ReasonCode)
			}
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().StringVar(&project, "project-root", ".", "project root path (Python flag alias)")
	c.Flags().StringVarP(&version, "version", "v", "", "docs version for evidence context")
	c.Flags().StringVarP(&feature, "feature", "f", "", "feature context")
	c.Flags().StringVar(&hookCode, "hook-code", "", "expected hook code")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "plan and write dry-run evidence only")
	c.Flags().BoolVar(&confirm, "confirm", false, "explicitly confirm side-effect hook execution")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newHooksDisableCmd() *cobra.Command {
	var project string
	var asJSON bool
	c := &cobra.Command{
		Use:   "disable <hook-id>",
		Short: "Disable a project hook contract (enabled: false)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, code, err := orchestrator.DisableHook(absProject(project), args[0])
			if err != nil {
				return err
			}
			if code != "" {
				return fail(code, "hooks disable", "hook contract not found: "+args[0])
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"schema_version": "1.0", "hook_id": args[0], "status": "disabled", "path": rel,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "status: disabled\npath: %s\n", rel)
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newHooksStatusCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status <hook-id>",
		Short: "Show hook admission and latest evidence read model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := orchestrator.HookStatus(absProject(project), args[0], feature)
			if asJSON {
				return printJSON(cmd, payload)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "status: %s\n", payload["status"])
			return nil
		},
	}
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path")
	c.Flags().StringVarP(&feature, "feature", "f", "", "feature context")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
