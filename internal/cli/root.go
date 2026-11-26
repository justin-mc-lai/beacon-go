// Package cli is the Go counterpart of the frozen v2 design's beacon-cli
// crate: the main CLI binary surface. Dependency direction (AC-023,
// translated): cli -> {gate, orchestrator, store, core, harness};
// store -> core; orchestrator -> {store, core}; no cycles.
package cli

import (
	"github.com/spf13/cobra"

	"beacon-go/internal/harness"
	"beacon-go/internal/version"
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
// v2.0.0-go.15 adds SDD docs-version iteration: greenfield v0.0.1,
// explicit projects bump, current/latest resolution, and docs/runtime
// axis separation.
// v2.0.0-go.12 adds program-level sea delivery (drive --until-done +
// one program complete), release shadow (auto_shadow), feature maps,
// skills drift verification, and monotonic lifecycle guards.
// v2.0.0-go.11 namespaces installed skills (beacon-go-<id>) so the Go track
// can coexist with the Python v1 set in every host, adds the opencode host,
// and ships a generated never-stale beacon-go router skill.
// v2.0.0-go.10 closes the 2026-10-29 audit: the fake verify-harness is
// now a real ledger+hash check, the default ledger is the Go track's own
// beacon-go.db, migrations snapshot first, long runs honour SIGINT,
// streaming forwards real deltas, benchmark and file-size gates exist,
// and the frozen feature packages are reconciled (docs/changes/change-audit-2026-10-29.md).
// Development builds between exits carried a dev suffix.
//
// Historical note: v2.0.0-go.1..17 were prereleases of 2.0.0. As of
// 2026-11-24 the version is plain SemVer from internal/version
// (docs/version-scheme.md); no new -go.N versions are issued.
const Version = version.Version

// hidden keeps an internal maintenance command executable but out of the
// user-facing help menu. The seven groups stay visible.
func hidden(c *cobra.Command) *cobra.Command {
	c.Hidden = true
	return c
}

// NewRootCmd builds the CLI from harness.Abilities — the single canonical
// list of seven command groups. Everything else is a maintenance surface,
// hidden from help but still executable.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "beacon",
		Short: "Beacon - requirement-truth kernel with an intent tree",
		Long: "Beacon 3.x requirement-truth kernel.\n" +
			"Seven groups: plan | truth | change | implement | verify | release | goal; plus skills.\n" +
			"Truth is an intent tree in the repo; SQLite is a rebuildable runtime cache.\n" +
			"Internal maintenance commands are hidden from this help.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("Beacon CLI v{{.Version}}\n")

	// The seven groups come from the one canonical list. A missing
	// constructor is a programming error, never a silently absent group.
	constructors := map[string]func() *cobra.Command{
		"plan":      newPlanCmd,
		"truth":     newTruthCmd,
		"change":    newChangeCmd,
		"implement": newImplementCmd,
		"verify":    newVerifyCmd,
		"release":   newReleaseCmd,
		"goal":      newGoalCmd,
	}
	for _, ability := range harness.Abilities {
		ctor, ok := constructors[ability.ID]
		if !ok {
			panic("no CLI constructor for ability " + ability.ID)
		}
		root.AddCommand(ctor())
	}
	// Maintenance surfaces: executable but hidden from help.
	root.AddCommand(
		hidden(newSpineCmd()), hidden(newProjectsCmd()),
		hidden(newGateCmd()), hidden(newMigrateCmd()), hidden(newDoctorCmd()),
		newSkillsCmd(), hidden(newReviewCmd()), hidden(newArchiveCmd()),
		hidden(newHooksCmd()), hidden(newFSMCmd()), hidden(newVersionCmd()),
	)
	return root
}
