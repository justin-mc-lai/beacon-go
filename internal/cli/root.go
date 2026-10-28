// Package cli is the Go counterpart of the frozen v2 design's beacon-cli
// crate: the main CLI binary surface. Dependency direction (AC-023,
// translated): cli -> {gate, orchestrator, store, core, harness};
// store -> core; orchestrator -> {store, core}; no cycles.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"beacon-go/internal/harness"
)

// Version is the Go-track build version. The frozen kernel design targets
// v2.0.0; the -go.N suffix numbers the Go track's own releases under the
// charter's maintenance rules (section 5): v2.0.0-go.1 is the P2 exit —
// the first release whose goal workflow set (G-01..G-22) is fully in
// place; v2.0.0-go.2 is the P3 exit — gates write back (P3a), v1 data
// migrates in (P3b), and the integrations are real (P3c: subprocess /
// profile executors, the ralph loop, git worktrees, planner review);
// v2.0.0-go.3 is the P4 exit — six-ability surfaces converged and
// truth grew its own package lifecycle (P4a), the frozen-fact fix and
// the unified skill score landed (P4b), and the v1.6.3–1.6.6 leftover
// list reached final disposition (P4c: protected-path write guard,
// package body fingerprints, failure-mode catalog, loop kill switch,
// two-step ACP probe); v2.0.0-go.4 is the P6 exit — acceptance
// closeout: the benchmark gate exists and is archived (5 of 8
// thresholds pass on this machine; the three write-path misses are
// recorded, not hidden), the AC matrix is fully dispositioned, and
// the scripted E2E walks the whole chain up to the one step the
// QA-artifact registration gap blocks (see docs/p6-implementation.md);
// v2.0.0-go.5 is the P6 E2E fix — the manual artifact registration
// face (`goal artifacts register/list`) closes that gap and the
// scripted E2E passes all 11 steps (see
// docs/p6-fix-artifact-register.md);
// v2.0.0-go.6 is the write-path performance fix — multi-write call
// sites commit as single transactions (store.WithTx) and writable
// stores run WAL synchronous=NORMAL (see
// docs/p6-fix-write-perf.md);
// v2.0.0-go.7 completes the two standard groups: the standard flow
// (AC-136..142 — canonical branch discipline, committed baselines,
// the eight-step flow, the baseline-merged + dirty-policy admission
// items, the admission evidence bundle) and topology (AC-171..173 —
// GitScopeGraph -> BranchContext projection, the three-topology
// gate, mandatory topology consumption at the implement entry; see
// docs/stdflow-s1/s2/s3-implementation.md);
// v2.0.0-go.8 adds the ask-before-sign chain (truth ask/answer/intent/
// confirm), requirement-level drift, the parallel-change base guard,
// embedded public skills, and the macOS/Linux path canonicalization fix;
// v2.0.0-go.9 closes the 26 high/medium gaps from the Python v1.6.19
// parity audit (docs/parity-audit-python-v16.md): freeze quality
// (vague-term block, FSM graph validation, five integrity dimension
// groups, fill depth, deferral ledger, scope modes, stale materials),
// orchestration closure (review write-back, quota, version-level
// release checks, budget contract), execution & host (autotest,
// skills install, repo-carried state, archive/init/hooks), and the
// intelligence faces (agent eval, qa strategy, intent decomposition,
// brainstorm, code2truth, reopen). See docs/r2a..r2d reports.
// Development builds between exits carried a dev suffix.
const Version = "2.0.0-go.9"

// NewRootCmd builds the beacon root command with the six first-class
// abilities as real command groups (P4a convergence): goal (the loop
// ability's formal entry), truth (real package lifecycle), and the
// thin design / implement / qa / release faces that route to the
// same implementations goal and gate use. No placeholders remain.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "beacon",
		Short: "Beacon - Evidence-first Product Workflow Kernel (Go track)",
		Long: "Beacon v2 kernel (Go implementation of the frozen v2 design).\n" +
			"Six first-class abilities: goal | truth | design | implement | qa | release\n" +
			"(charter decision 5: the loop ability's user entry is `goal`; `loop` is a pointer).\n" +
			"Runtime state: global ~/.beacon/beacon.db + project blob pointers.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("Beacon CLI v{{.Version}}\n")

	for _, ability := range harness.Abilities {
		id := ability.ID // capture for closure
		switch id {
		case "loop":
			// Charter decision 5 (2026-10-21): the user entry is named
			// "goal", not "loop". The loop entry is kept as a
			// pointer only — no alias with its own behaviour — so the
			// surface cannot grow two divergent entries (anti-bloat
			// rule). The change is booked on the Go side in
			// docs/changes/change-goal-entry-rename.md (P3a); the
			// upstream frozen package is read-only and unmodified.
			root.AddCommand(&cobra.Command{
				Use:   "loop",
				Short: "Superseded by `beacon goal` (charter decision 5: the formal entry is goal)",
				RunE: func(cmd *cobra.Command, args []string) error {
					_, err := fmt.Fprintln(cmd.OutOrStdout(),
						"beacon loop: superseded — the formal entry for this ability is `beacon goal` (charter decision 5, 2026-10-21). Run `beacon goal --help`.")
					return err
				},
			})
		case "truth":
			root.AddCommand(newTruthCmd())
		case "design":
			root.AddCommand(newDesignCmd())
		case "implement":
			root.AddCommand(newImplementCmd())
		case "qa":
			root.AddCommand(newQACmd())
		case "release":
			root.AddCommand(newReleaseCmd())
		}
	}
	// P1 read surfaces (AC-017..021): Operator Spine + projects registry.
	// These are support surfaces, not first-class abilities.
	// P2a: the goal command group (the loop ability's formal entry).
	// P3a: the gate command group (formal evaluation write-back).
	// P3b: migrate v1 (AC-077..079) + doctor rebuild (AC-011).
	root.AddCommand(newSpineCmd(), newProjectsCmd(), newGoalCmd(), newGateCmd(), newMigrateCmd(), newDoctorCmd(), newSkillsCmd(), newReviewCmd(), newArchiveCmd(), newHooksCmd(), newUICmd())
	return root
}
