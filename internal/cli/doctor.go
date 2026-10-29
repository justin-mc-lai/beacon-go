// Doctor commands (P3b): `beacon doctor rebuild` — projection rebuild
// against on-disk facts (AC-011). The logic lives in
// internal/migration; this file is only the CLI surface.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"beacon-go/internal/benchgate"
	"beacon-go/internal/migration"
	"beacon-go/internal/store"
)

func newDoctorCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnostics and projection maintenance",
	}
	c.AddCommand(newDoctorRebuildCmd(), newDoctorBenchGateCmd())
	return c
}

func newDoctorRebuildCmd() *cobra.Command {
	var project string
	var apply, asJSON bool
	c := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild a project's store index from its on-disk MD + v1 JSON facts (AC-011)",
		Long: "Compare the store index of a registered project with its on-disk feature packages.\n" +
			"Default: report drift only (missing_in_store / missing_on_disk / status_mismatch).\n" +
			"--apply fills missing packages and derived lifecycles and corrects package status,\n" +
			"appending a lifecycle_events row per correction. Existing lifecycle snapshots and\n" +
			"gate results are never overwritten, and store rows are never deleted.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := store.ResolveRootPath(project)
			var rep *migration.RebuildReport
			if apply {
				st, err := openWriteStore()
				if err != nil {
					return err
				}
				defer st.Close()
				r, err := migration.Rebuild(st, root, true)
				if err != nil {
					return err
				}
				rep = r
			} else {
				st, exists, err := openReadStore()
				if err != nil {
					return err
				}
				if !exists {
					return fail(ReasonProjectNotRegistered, "doctor rebuild", "no store yet; nothing is registered")
				}
				defer st.Close()
				r, err := migration.Rebuild(st, root, false)
				if err != nil {
					return err
				}
				rep = r
			}
			if asJSON {
				return printJSON(cmd, rep)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "doctor rebuild: %s (applied=%v)\n", rep.ProjectRoot, rep.Applied)
			if len(rep.Drift) == 0 {
				fmt.Fprintln(out, "  no drift")
			}
			for _, d := range rep.Drift {
				fmt.Fprintf(out, "  drift [%s] %s/%s: %s\n", d.Kind, d.Version, d.Slug, d.Detail)
			}
			for _, c := range rep.Corrected {
				fmt.Fprintf(out, "  corrected: %s\n", c)
			}
			fmt.Fprintf(out, "  events_written=%d\n", rep.EventsWritten)
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", ".", "registered project root")
	c.Flags().BoolVar(&apply, "apply", false, "apply corrections (default: report drift only)")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	return c
}

// newDoctorBenchGateCmd is AC-095: the P6 performance regression gate.
// It compares a benchmark document against its baseline and fails when a
// shared acceptance criterion regressed by more than --tolerance. Across
// a different OS or CPU count it refuses to judge instead of producing a
// confident number from two different machines.
func newDoctorBenchGateCmd() *cobra.Command {
	var baselinePath, currentPath string
	var tolerance float64
	var asJSON bool
	c := &cobra.Command{
		Use:   "bench-gate",
		Short: "Compare a benchmark result document with its baseline (AC-095, default 10% regression blocks)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseRaw, err := os.ReadFile(baselinePath)
			if err != nil {
				return fail("bench_baseline_unreadable", "doctor bench-gate", err.Error())
			}
			curRaw, err := os.ReadFile(currentPath)
			if err != nil {
				return fail("bench_current_unreadable", "doctor bench-gate", err.Error())
			}
			rep, err := benchgate.Compare(baseRaw, curRaw, tolerance)
			if err != nil {
				return fail("bench_document_invalid", "doctor bench-gate", err.Error())
			}
			if asJSON {
				if perr := printJSON(cmd, rep); perr != nil {
					return perr
				}
			} else {
				out := cmd.OutOrStdout()
				if !rep.Comparable {
					fmt.Fprintf(out, "bench-gate: NOT COMPARABLE — %s\n", rep.Note)
				} else {
					fmt.Fprintf(out, "bench-gate: baseline %s vs candidate %s (tolerance %.0f%%)\n",
						baselinePath, currentPath, rep.TolerancePct)
					for _, r := range rep.Rows {
						mark := "ok"
						if r.Regressed {
							mark = "REGRESSED"
						}
						fmt.Fprintf(out, "  %-8s %-8s base=%8.2fms cur=%8.2fms %+7.1f%%\n",
							r.AC, mark, r.BaselineP95MS, r.CandidateP95MS, r.DeltaPct)
					}
				}
			}
			if rep.Failed {
				return fail("benchmark_regression", "doctor bench-gate",
					"regression over tolerance: "+strings.Join(rep.Regressions, ", "))
			}
			return nil
		},
	}
	c.Flags().StringVar(&baselinePath, "baseline", "", "baseline benchmark JSON (required)")
	c.Flags().StringVar(&currentPath, "current", "", "current benchmark JSON (required)")
	c.Flags().Float64Var(&tolerance, "tolerance", benchgate.DefaultTolerancePct, "allowed regression percentage")
	c.Flags().BoolVar(&asJSON, "json", false, "emit one JSON document")
	_ = c.MarkFlagRequired("baseline")
	_ = c.MarkFlagRequired("current")
	return c
}
