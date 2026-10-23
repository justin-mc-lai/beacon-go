// Operator Spine commands (AC-017..020): show / list / artifacts / gate.
// All four are read-only DB views (AC-015 DB-first); none of them reads
// project JSON files as authority (AC-006) and `spine gate` never
// evaluates or writes — it displays the gate_results rows that the
// formal evaluation surface (`beacon gate evaluate` / `gate
// integrity`, P3a) writes.
package cli

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/store"
)

func newSpineCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "spine",
		Short: "Operator Spine read views over the global runtime DB (AC-017..020)",
	}
	c.AddCommand(newSpineShowCmd(), newSpineListCmd(), newSpineArtifactsCmd(), newSpineGateCmd())
	return c
}

// resolveSearchRoot decides which project root scopes a spine query:
// an explicit --project must be registered; otherwise the cwd project
// when registered; otherwise "" (global) when a feature selector can
// still scope the query, or an error when a project is required.
func resolveSearchRoot(cmd *cobra.Command, st *store.Store, projectFlag string, projectRequired bool) (string, error) {
	if cmd.Flags().Changed("project") {
		root := store.ResolveRootPath(projectFlag)
		if _, err := st.Projects().GetByRootPath(root); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// R2c (H10): recognize a repo-carried state
				// snapshot instead of refusing the project.
				if wst, werr := openWriteStore(); werr == nil {
					autoRecognizeProject(wst, root)
					wst.Close()
					if _, rerr := st.Projects().GetByRootPath(root); rerr == nil {
						return root, nil
					}
				}
				return "", fail(ReasonProjectNotRegistered, "spine", "project not registered: "+root)
			}
			return "", err
		}
		return root, nil
	}
	cwd := store.ResolveRootPath(".")
	if _, err := st.Projects().GetByRootPath(cwd); err == nil {
		return cwd, nil
	}
	if !projectRequired {
		return "", nil
	}
	projects, err := st.Projects().List()
	if err != nil {
		return "", err
	}
	if len(projects) == 1 {
		return projects[0].RootPath, nil
	}
	return "", fail(ReasonProjectNotRegistered, "spine", "no project registered for cwd "+cwd+" (pass --project)")
}

// pickShowFeature chooses the feature for `spine show` without --feature:
// the one with the newest lifecycle snapshot wins; ties keep list order
// (current version first, then slug).
func pickShowFeature(st *store.Store, list []store.FeatureContext) store.FeatureContext {
	best := list[0]
	bestUpdated := ""
	for _, ctx := range list {
		lc, err := st.Features().GetLifecycle(ctx.Feature.ID)
		if err != nil {
			continue
		}
		if lc.UpdatedAt > bestUpdated {
			best, bestUpdated = ctx, lc.UpdatedAt
		}
	}
	return best
}

func newSpineShowCmd() *cobra.Command {
	var project, feature string
	var asJSON bool
	c := &cobra.Command{
		Use:   "show",
		Short: "Show one feature's SpineSnapshot (AC-017)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				if feature != "" {
					return fail(ReasonFeatureNotFound, "spine show", "no runtime DB yet, feature not found: "+feature)
				}
				return fail(ReasonProjectNotRegistered, "spine show", "no runtime DB yet, no project registered")
			}
			defer st.Close()
			root, err := resolveSearchRoot(cmd, st, project, feature == "")
			if err != nil {
				return err
			}
			var ctx store.FeatureContext
			if feature != "" {
				found, err := st.ResolveFeature(root, feature, "")
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fail(ReasonFeatureNotFound, "spine show", "feature not found: "+feature)
					}
					return err
				}
				ctx = found
			} else {
				list, err := st.ListFeatureContexts(root)
				if err != nil {
					return err
				}
				if len(list) == 0 {
					return fail(ReasonFeatureNotFound, "spine show", "no features registered for project "+root)
				}
				ctx = pickShowFeature(st, list)
			}
			snap, err := st.Spine().Snapshot(ctx.Feature.ID)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, snap)
			}
			printSnapshotHuman(cmd, snap)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func printSnapshotHuman(cmd *cobra.Command, s store.SpineSnapshot) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "feature:        %s\n", s.FeatureSlug)
	fmt.Fprintf(out, "project:        %s (%s)\n", s.ProjectRoot, s.ProjectKey)
	fmt.Fprintf(out, "version:        %s\n", s.Version)
	fmt.Fprintf(out, "lifecycle:      %s / %s\n", s.Stage, s.Status)
	fmt.Fprintf(out, "phase:          %s (progress %s, promoted %v)\n", s.Phase, s.ProgressStatus, s.PhasePromoted)
	fmt.Fprintf(out, "next:           %s -> %s\n", s.NextHarness, s.NextAction)
	fmt.Fprintf(out, "run:            %s (status %s, iteration %d, %d runs)\n", s.RunID, s.RunStatus, s.Iteration, s.RunCount)
	if len(s.ReasonCodes) > 0 {
		fmt.Fprintf(out, "blocking:       %v\n", s.ReasonCodes)
	}
	for _, g := range s.LatestGates {
		fmt.Fprintf(out, "gate %-10s   %s %v\n", g.GateType, g.Verdict, g.ReasonCodes)
	}
	fmt.Fprintf(out, "artifacts:      %d (%d stale)\n", s.ArtifactCount, countStale(s.ArtifactRefs))
	fmt.Fprintf(out, "updated_at:     %s\n", s.UpdatedAt)
}

func countStale(refs []store.ArtifactView) int {
	n := 0
	for _, r := range refs {
		if r.Stale {
			n++
		}
	}
	return n
}

func newSpineListCmd() *cobra.Command {
	var project, status string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List SpineSnapshots, optionally filtered (AC-018/082)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				if asJSON {
					return printJSON(cmd, map[string]any{"snapshots": []store.SpineSnapshot{}})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no features (no runtime DB yet)")
				return nil
			}
			defer st.Close()
			filter := store.SpineListFilter{Status: status}
			if cmd.Flags().Changed("project") {
				root := store.ResolveRootPath(project)
				if _, err := st.Projects().GetByRootPath(root); err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						return fail(ReasonProjectNotRegistered, "spine list", "project not registered: "+root)
					}
					return err
				}
				filter.ProjectRoot = root
			}
			snaps, err := st.Spine().List(filter)
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"snapshots": snaps})
			}
			if len(snaps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no features match")
				return nil
			}
			for _, s := range snaps {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-28s %-10s %-12s run=%-10s %s\n",
					s.FeatureSlug, s.ProjectKey, s.Stage, s.Status, s.RunStatus, s.UpdatedAt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path filter")
	c.Flags().StringVar(&status, "status", "", "lifecycle or run status filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newSpineArtifactsCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "artifacts <run-id>",
		Short: "List a run's artifact pointers with stale flags (AC-019)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runID := args[0]
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonRunNotFound, "spine artifacts", "no runtime DB yet, run not found: "+runID)
			}
			defer st.Close()
			if _, err := st.Runs().GetByID(runID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fail(ReasonRunNotFound, "spine artifacts", "run not found: "+runID)
				}
				return err
			}
			arts, err := st.Artifacts().ListByRun(runID)
			if err != nil {
				return err
			}
			views := []store.ArtifactView{}
			for _, a := range arts {
				views = append(views, store.ViewOf(a))
			}
			if asJSON {
				return printJSON(cmd, map[string]any{"run_id": runID, "artifact_refs": views})
			}
			if len(views) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no artifacts for run %s\n", runID)
				return nil
			}
			for _, v := range views {
				mark := ""
				if v.Stale {
					mark = " stale(" + v.StaleReason + ")"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-10s %s hash=%s size=%d%s\n", v.Kind, v.AbsPath, v.ContentHash, v.SizeBytes, mark)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newSpineGateCmd() *cobra.Command {
	var version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "gate <feature>",
		Short: "Read-only latest gate verdicts for a feature (AC-020; data written by `beacon gate evaluate`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "spine gate", "no runtime DB yet, feature not found: "+slug)
			}
			defer st.Close()
			// DB only (AC-006): project files such as
			// .beacon/gate-report.json are never consulted here.
			ctx, err := st.ResolveFeature("", slug, version)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fail(ReasonFeatureNotFound, "spine gate", "feature not found: "+slug)
				}
				return err
			}
			gates, err := st.Gates().LatestByType(ctx.Feature.ID)
			if err != nil {
				return err
			}
			if gates == nil {
				gates = []store.GateResult{}
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"feature_slug": slug,
					"version":      ctx.Version.Version,
					"project_key":  store.ProjectKey(ctx.Project.RootPath),
					"gate_results": gates,
				})
			}
			if len(gates) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no gate results recorded for %s\n", slug)
				return nil
			}
			for _, g := range gates {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-10s %v (%s)\n", g.GateType, g.Verdict, g.ReasonCodes, g.EvaluatedAt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&version, "version", "", "docs version filter (e.g. v2.0.0)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
