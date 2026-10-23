// Gate commands (P3a): the formal evaluation surface of beacon-gate.
//
//   - evaluate <gate-type>  formally evaluate one of the five stage
//     gates and PERSIST the verdict (gate_results + lifecycle_events).
//     An --evaluator identity is mandatory; a blocked verdict prints
//     the evaluation and exits non-zero with the headline reason code.
//   - status                read-only latest verdicts (the same rows
//     `spine gate` shows; data source since P3a is `gate evaluate`).
//   - integrity             the Truth Integrity Gate (AC-101) as its
//     own entry: evaluates AND persists under gate_type=
//     truth_integrity. Default mode follows AC-101: an already-frozen
//     package is audit_only (reported, not blocking).
package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/store"
)

func newGateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gate",
		Short: "Formal gate evaluation: evaluate / status / integrity (writes gate_results)",
		Long: "Gate is the formal beacon-gate surface. Unlike `spine gate`\n" +
			"(a read-only view), `gate evaluate` and `gate integrity` READ\n" +
			"the facts, DECIDE, and WRITE the verdict to gate_results +\n" +
			"lifecycle_events. Every evaluation needs an --evaluator.",
	}
	c.AddCommand(newGateEvaluateCmd(), newGateStatusCmd(), newGateIntegrityCmd())
	return c
}

// resolveGateFeature resolves the feature context for a gate command.
func resolveGateFeature(st *store.Store, project, slug, version string) (store.FeatureContext, error) {
	root := store.ResolveRootPath(project)
	// R2c (H10): a repo carrying its own state snapshot is
	// auto-recognized instead of refused as unregistered. On a
	// read-only store the import simply fails and the normal
	// not-found path below reports as before.
	autoRecognizeProject(st, root)
	ctx, err := st.ResolveFeature(root, slug, version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.FeatureContext{}, fail(ReasonFeatureNotFound, "gate", "feature not found: "+slug)
		}
		return store.FeatureContext{}, err
	}
	return ctx, nil
}

func newGateEvaluateCmd() *cobra.Command {
	var project, feature, version, runID, evaluator string
	var asJSON bool
	c := &cobra.Command{
		Use:   "evaluate <gate-type>",
		Short: "Formally evaluate one gate (truth_freeze|design|implement|qa|release) and persist the verdict",
		Args:  cobra.ExactArgs(1),
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
			ev, err := eng.EvaluateGate(ctx, args[0], runID, evaluator)
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

func printEvaluationHuman(cmd *cobra.Command, ev gate.Evaluation) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "gate:      %s\nfeature:   %s\nverdict:   %s\n", ev.GateType, ev.FeatureSlug, ev.Verdict)
	if ev.RunID != "" {
		fmt.Fprintf(out, "run:       %s\n", ev.RunID)
	}
	fmt.Fprintf(out, "evaluator: %s\n", ev.Evaluator)
	for _, ch := range ev.Checks {
		mark := "ok"
		if ch.Applicable && !ch.OK {
			mark = "BLOCK"
		} else if !ch.Applicable {
			mark = "n/a"
		}
		fmt.Fprintf(out, "  [%-5s] %-24s %s\n", mark, ch.Check, ch.Detail)
	}
	if len(ev.ReasonCodes) > 0 {
		fmt.Fprintf(out, "reasons:   %s\n", strings.Join(ev.ReasonCodes, ", "))
	}
	fmt.Fprintf(out, "recorded:  %s (%s)\n", ev.GateResultID, ev.EvaluatedAt)
}

// gateStatusRow is one line of the `gate status` surface.
type gateStatusRow struct {
	GateType    string   `json:"gate_type"`
	Evaluated   bool     `json:"evaluated"`
	Verdict     string   `json:"verdict,omitempty"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	EvaluatedAt string   `json:"evaluated_at,omitempty"`
}

func newGateStatusCmd() *cobra.Command {
	var project, feature, version string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Read-only latest verdicts for all gate types (source: `gate evaluate` / `gate integrity` writes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, exists, err := openReadStore()
			if err != nil {
				return err
			}
			if !exists {
				return fail(ReasonFeatureNotFound, "gate status", "no runtime DB yet, feature not found: "+feature)
			}
			defer st.Close()
			ctx, err := resolveGateFeature(st, project, feature, version)
			if err != nil {
				return err
			}
			latest, err := st.Gates().LatestByType(ctx.Feature.ID)
			if err != nil {
				return err
			}
			byType := map[string]store.GateResult{}
			for _, g := range latest {
				byType[g.GateType] = g
			}
			rows := make([]gateStatusRow, 0, len(gate.OrderedGateTypes))
			for _, t := range gate.OrderedGateTypes {
				row := gateStatusRow{GateType: string(t)}
				if g, ok := byType[string(t)]; ok {
					row.Evaluated = true
					row.Verdict = g.Verdict
					row.ReasonCodes = g.ReasonCodes
					row.EvaluatedAt = g.EvaluatedAt
				}
				rows = append(rows, row)
			}
			if asJSON {
				return printJSON(cmd, map[string]any{
					"feature_slug": ctx.Feature.Slug,
					"feature_id":   ctx.Feature.ID,
					"version":      ctx.Version.Version,
					"gates":        rows,
				})
			}
			out := cmd.OutOrStdout()
			for _, r := range rows {
				if !r.Evaluated {
					fmt.Fprintf(out, "%-16s not_evaluated\n", r.GateType)
					continue
				}
				fmt.Fprintf(out, "%-16s %-10s %v (%s)\n", r.GateType, r.Verdict, r.ReasonCodes, r.EvaluatedAt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

func newGateIntegrityCmd() *cobra.Command {
	var project, feature, version, evaluator, mode string
	var asJSON bool
	c := &cobra.Command{
		Use:   "integrity",
		Short: "Run the Truth Integrity Gate (AC-101) for a feature package and persist the verdict",
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
			v, err := eng.EvaluateIntegrity(ctx, evaluator, mode)
			if err != nil {
				return err
			}
			if asJSON {
				if perr := printJSON(cmd, v); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "gate:      truth_integrity (mode %s)\nfeature:   %s\nverdict:   %s\n", v.Mode, v.FeatureSlug, v.Verdict)
				for _, f := range v.Findings {
					loc := f.File
					if f.Line > 0 {
						loc = fmt.Sprintf("%s:%d", f.File, f.Line)
					}
					fmt.Fprintf(out, "  [%-28s] %s — %s (repair: %s)\n", f.Kind, loc, f.Detail, f.RepairRoute)
				}
				if len(v.ReasonCodes) > 0 {
					fmt.Fprintf(out, "reasons:   %s\n", strings.Join(v.ReasonCodes, ", "))
				}
				fmt.Fprintf(out, "recorded:  %s (%s)\n", v.GateResultID, v.EvaluatedAt)
			}
			if v.Verdict == string(gate.VerdictBlocked) {
				code := core.ReasonIntegrityPackageIncomplete
				if len(v.ReasonCodes) > 0 {
					code = v.ReasonCodes[0]
				}
				return fail(code, "gate integrity", "truth integrity blocked for feature "+ctx.Feature.Slug+" (repair route: "+v.RepairRoute+")")
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "project root path")
	c.Flags().StringVar(&feature, "feature", "", "feature slug (required)")
	c.Flags().StringVar(&version, "version", "", "docs version filter")
	c.Flags().StringVar(&evaluator, "evaluator", "", "evaluator identity (required; unattributed verdicts are refused)")
	c.Flags().StringVar(&mode, "mode", "", "block | audit_only (default: audit_only for already-frozen packages, block otherwise)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}
