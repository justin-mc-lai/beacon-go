// goal artifacts (P6 E2E fix): the manual registration face for run
// artifacts, folded into the goal group (no new top-level command).
// `register` lands a real file in artifact_refs with its sha256 —
// the write path whose absence stopped the P6 end-to-end chain at
// the qa gate (stage agents produce a QA report on disk, but only
// registered pointers count as gate evidence). `list` is the
// read-only twin of `spine artifacts` over the same presentation.
//
// Registration is a declaration, not a judgement: it records who
// registered what, with the hash at registration time. Gates keep
// their own defence — every evaluation re-hashes the file, so a
// tampered artifact turns stale and blocks.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newGoalArtifactsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "artifacts",
		Short: "Register and list a goal run's artifact pointers (artifact_refs)",
		Long: "Artifacts are the pointers the formal gates trust as evidence.\n" +
			"register declares that a real file belongs to a run and records\n" +
			"its sha256 + size at registration time; it judges nothing — gate\n" +
			"evaluations re-hash the file on every read, so tampering turns\n" +
			"the pointer stale and blocks. Kinds are a controlled vocabulary:\n" +
			"qa, qa_*, scorecard, test_result(s) (the qa gate's QA family),\n" +
			"plus evidence, implement_artifact, report, log, artifact.",
	}
	c.AddCommand(newGoalArtifactsRegisterCmd(), newGoalArtifactsListCmd())
	return c
}

func newGoalArtifactsRegisterCmd() *cobra.Command {
	var runID, feature, project, version, scopeKind, scopeID, kind, path, by string
	var asJSON bool
	c := &cobra.Command{
		Use:   "register",
		Short: "Register a real file as a run or feature artifact (computes sha256, writes artifact_refs)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scopes := 0
			if strings.TrimSpace(runID) != "" {
				scopes++
			}
			if strings.TrimSpace(feature) != "" {
				scopes++
			}
			if strings.TrimSpace(scopeKind) != "" || strings.TrimSpace(scopeID) != "" {
				scopes++
			}
			if scopes != 1 {
				return fail("artifact_scope_required", "goal.artifacts.register",
					"exactly one scope is required: --run-id, --feature, or --scope-kind/--scope-id")
			}
			eng, closeFn, err := goalEngine("")
			if err != nil {
				return err
			}
			defer closeFn()
			var res orchestrator.ArtifactRegistration
			if scopeKind != "" || scopeID != "" {
				res, err = eng.RegisterScopedArtifact(scopeKind, scopeID, kind, path, by)
			} else if feature != "" {
				// AC-002: feature-scoped registration needs no run, so the
				// freeze -> artifact -> run cycle is broken here.
				ctx, cerr := resolveGateFeature(eng.Store(), project, feature, version)
				if cerr != nil {
					return cerr
				}
				res, err = eng.RegisterFeatureArtifact(ctx, kind, path, by)
			} else {
				res, err = eng.RegisterArtifact(runID, kind, path, by)
			}
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(cmd, res)
			}
			a := res.Artifact
			scope := "run:      " + a.RunID
			if feature != "" {
				scope = "feature:  " + feature + " (no run)"
			} else if scopeKind != "" {
				scope = scopeKind + ": " + scopeID
			}
			fmt.Fprintf(cmd.OutOrStdout(), "artifact: %s\nkind:     %s\npath:     %s\nhash:     %s\nsize:     %d\n%s\n",
				a.ID, a.Kind, a.AbsPath, a.ContentHash, a.SizeBytes, scope)
			if res.AlreadyRegistered {
				fmt.Fprintln(cmd.OutOrStdout(), "note:     already registered with the same hash (idempotent, nothing written)")
			}
			if res.Supersedes != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "note:     supersedes %s (file content changed since that registration)\n", res.Supersedes)
			}
			return nil
		},
	}
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (scope: run; give this or --feature)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (scope: feature; registers with no run)")
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path (with --feature)")
	c.Flags().StringVar(&version, "version", "", "docs version filter (with --feature)")
	c.Flags().StringVar(&kind, "kind", "", "artifact kind from the controlled vocabulary, e.g. qa_report (required)")
	c.Flags().StringVar(&path, "path", "", "path of the real file to register (required)")
	c.Flags().StringVar(&scopeKind, "scope-kind", "", "explicit evidence scope kind: feature|node")
	c.Flags().StringVar(&scopeID, "scope-id", "", "explicit evidence scope id (feature id or node id)")
	c.Flags().StringVar(&by, "by", "", "registrant identity (required)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("kind")
	_ = c.MarkFlagRequired("path")
	_ = c.MarkFlagRequired("by")
	return c
}

func newGoalArtifactsListCmd() *cobra.Command {
	var runID, feature, project, version, scopeKind, scopeID string
	var asJSON bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List a run's or feature's registered artifacts with stale flags (same view as `spine artifacts`)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			scopes := 0
			if strings.TrimSpace(runID) != "" {
				scopes++
			}
			if strings.TrimSpace(feature) != "" {
				scopes++
			}
			if strings.TrimSpace(scopeKind) != "" || strings.TrimSpace(scopeID) != "" {
				scopes++
			}
			if scopes != 1 {
				return fail("artifact_scope_required", "goal.artifacts.list",
					"exactly one scope is required: --run-id, --feature, or --scope-kind/--scope-id")
			}
			eng, closeFn, err := goalReadEngine()
			if err != nil {
				return err
			}
			defer closeFn()
			var views []store.ArtifactView
			label := "run " + runID
			if scopeKind != "" || scopeID != "" {
				views, err = eng.ListScopedArtifacts(scopeKind, scopeID)
				label = scopeKind + " " + scopeID
			} else if feature != "" {
				ctx, cerr := resolveGateFeature(eng.Store(), project, feature, version)
				if cerr != nil {
					return cerr
				}
				views, err = eng.ListFeatureArtifacts(ctx)
				label = "feature " + feature
			} else {
				views, err = eng.ListRunArtifacts(runID)
			}
			if err != nil {
				return err
			}
			if asJSON {
				out := map[string]any{"artifact_refs": views}
				if feature != "" {
					out["feature"] = feature
				} else if scopeKind != "" {
					out["scope_kind"] = scopeKind
					out["scope_id"] = scopeID
				} else {
					out["run_id"] = runID
				}
				return printJSON(cmd, out)
			}
			if len(views) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no artifacts for %s\n", label)
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
	c.Flags().StringVar(&runID, "run-id", "", "goal run id (scope: run; give this or --feature)")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (scope: feature)")
	c.Flags().StringVarP(&project, "project", "p", ".", "project root path (with --feature)")
	c.Flags().StringVar(&version, "version", "", "docs version filter (with --feature)")
	c.Flags().StringVar(&scopeKind, "scope-kind", "", "explicit evidence scope kind: feature|node")
	c.Flags().StringVar(&scopeID, "scope-id", "", "explicit evidence scope id (feature id or node id)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
