// Parallel lakes plan (P2c, G-11): the Go port of
// product_goal_runtime.build_parallel_plan, at plan-only scope.
//
// One lake, one workstation: a single lake works in the project root
// (single_worktree); several lakes form a main-star / satellite pool —
// each lake gets its own satellite worktree, satellites merge ONLY
// into the main star (never the reverse), and a satellite may never
// write truth on the main line. The plan states these constraints
// explicitly; P2c never creates the worktrees (real creation is P3),
// so every plan carries planned_only=true.
//
// detect_parallel_intent is deliberately the minimal version: v1
// scanned the utterance for marker words (parallel / 并行 / satellite
// / ...). P2c does NO text guessing — parallel is decided by the
// explicit flag or by lake count > 1, exactly the non-text branches
// of v1's build_parallel_plan mode resolution.
package orchestrator

import (
	"sort"
	"strings"

	"beacon-go/internal/core"
)

// DetectParallelIntent decides parallel mode from explicit signals
// only: the caller's flag, or more than one lake.
func DetectParallelIntent(explicitParallel bool, lakeCount int) bool {
	return explicitParallel || lakeCount > 1
}

// satelliteConstraints are spelled out on every satellite: merge
// direction is one-way and truth on the main line is off limits.
func satelliteConstraints() []string {
	return []string{
		"merge_direction: satellite -> main_star only (never reverse)",
		"satellite_may_write_truth_on_main: false (" + core.ReasonSatelliteTruthWrite + ")",
		"planned_only: worktree is not created in P2c (real creation is P3)",
	}
}

// BuildParallelPlan is a pure function: lake inputs in, plan out.
// Touch declarations are compared verbatim (after trimming); a
// surface declared by two or more lakes is a conflict — those lakes
// must be serialized or decided by a human, never silently parallel.
func BuildParallelPlan(lakes []LakeInput, explicitParallel bool) ParallelPlan {
	seen := map[string]bool{}
	var slugs []string
	touchByLake := map[string][]string{}
	for _, l := range lakes {
		slug := strings.TrimSpace(l.Slug)
		if slug == "" || seen[slug] {
			continue
		}
		seen[slug] = true
		slugs = append(slugs, slug)
		for _, t := range l.Touches {
			if t = strings.TrimSpace(t); t != "" {
				touchByLake[slug] = append(touchByLake[slug], t)
			}
		}
	}
	parallel := DetectParallelIntent(explicitParallel, len(slugs))
	intent := "single_default"
	if explicitParallel {
		intent = "explicit_flag"
	} else if len(slugs) > 1 {
		intent = "lake_count"
	}
	plan := ParallelPlan{
		Schema: "goal-parallel-plan.v1",
		Mode:   ParallelSingle,
		Lakes:  slugs,
		MainStar: MainStarPlan{
			Role: "main_star", Worktree: ".",
			MergePolicy: "satellites_merge_to_star_only",
		},
		PlannedOnly:  true,
		IntentSource: intent,
	}
	if parallel {
		plan.Mode = ParallelPool
		plan.OneLakeOneSatellite = true
		sats := slugs
		if len(sats) == 0 {
			sats = []string{"lake-placeholder"} // v1 parity for forced pool
		}
		for _, lake := range sats {
			plan.Satellites = append(plan.Satellites, SatellitePlan{
				Lake: lake, Role: "satellite",
				Worktree:            ".beacon/worktrees/satellite/" + lake,
				MergeTarget:         "main_star",
				MayWriteTruthOnMain: false,
				Constraints:         satelliteConstraints(),
			})
		}
	}
	// Touch conflicts: one surface, two or more lakes.
	byTouch := map[string][]string{}
	for _, slug := range slugs {
		for _, t := range touchByLake[slug] {
			byTouch[t] = appendUniqueStr(byTouch[t], slug)
		}
	}
	var surfaces []string
	for surface, ls := range byTouch {
		if len(ls) > 1 {
			surfaces = append(surfaces, surface)
		}
	}
	sort.Strings(surfaces)
	for _, surface := range surfaces {
		plan.Conflicts = append(plan.Conflicts, TouchConflict{Surface: surface, Lakes: byTouch[surface]})
	}
	plan.RequiresDecision = len(plan.Conflicts) > 0
	return plan
}

func appendUniqueStr(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// expectedWorktree is the worktree ref admission expects this run to
// be bound to, derived from the run's parallel plan: the satellite
// worktree of the run's own lake in pool mode, the main star (".")
// otherwise — including when no plan exists (single_worktree default,
// the project root itself).
func expectedWorktree(m *runMeta, featureSlug string) string {
	if m.ParallelPlan == nil || m.ParallelPlan.Mode != ParallelPool {
		return "."
	}
	for _, s := range m.ParallelPlan.Satellites {
		if s.Lake == featureSlug {
			return s.Worktree
		}
	}
	return "."
}

// ApplyParallelPlan builds the plan, records it on the run snapshot,
// binds the run's worktree ref to the plan's workstation for its own
// lake, and writes the parallel_plan_recorded event. Planning only —
// no worktree is created.
func (e *Engine) ApplyParallelPlan(runID string, lakes []LakeInput, explicitParallel bool) (ParallelPlan, RunView, error) {
	run, m, slug, err := e.loadRun(runID)
	if err != nil {
		return ParallelPlan{}, RunView{}, err
	}
	plan := BuildParallelPlan(lakes, explicitParallel)
	cp := plan
	m.ParallelPlan = &cp
	if m.Implement == nil {
		m.Implement = &ImplementState{}
	}
	m.Implement.WorktreeRef = expectedWorktree(m, slug)
	if err := e.appendEvent(run, "parallel_plan_recorded", map[string]any{
		"mode": plan.Mode, "lakes": plan.Lakes, "intent_source": plan.IntentSource,
		"conflicts": len(plan.Conflicts), "requires_decision": plan.RequiresDecision,
		"planned_only": true, "worktree_ref": m.Implement.WorktreeRef,
	}); err != nil {
		return ParallelPlan{}, RunView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ParallelPlan{}, RunView{}, err
	}
	return plan, viewOf(run, m, slug), nil
}
