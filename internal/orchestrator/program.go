// Program orchestration (P2d, G-12): the program_* family (2102
// lines: graph, admission, orchestrator, driver, freeze gate, change
// intent) re-based from project files onto the store.
//
// Storage argument: a program is anchored on ONE host goal run —
// the plan (lakes, dependency edges, freeze state, admission
// records, change log) lives in that run's metadata_json, and lakes
// are feature slugs in the host run's project version. No program
// table is added: runs already anchors identity and the event
// trail, and each lake's progress is derived live from that lake
// feature's own lifecycle/runs (ProgramRoute caches nothing). The
// program-level long-run driver is NOT a second driver: G-06's
// driver pushes one lake's goal run at a time, and the route view
// emits the per-lake driver commands as hints.
package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ---- Pure graph validation (program_graph.py) ----

// GraphProblem is one structural problem of a program lake graph.
type GraphProblem struct {
	ReasonCode string `json:"reason_code"`
	Detail     string `json:"detail"`
}

// ValidateProgramGraph checks a lake list structurally: non-empty,
// slugs present and unique, dependencies known and acyclic, no
// self-dependency. Pure function (program_graph.py's
// validate_feature_graph_payload + the cycle check the file-based
// family spread across the freeze gate's structure fingerprint).
func ValidateProgramGraph(lakes []ProgramLakeInput) []GraphProblem {
	var problems []GraphProblem
	if len(lakes) == 0 {
		return []GraphProblem{{ReasonCode: core.ReasonProgramLakesMissing,
			Detail: "a program needs at least one lake"}}
	}
	seen := map[string]bool{}
	for i, l := range lakes {
		if strings.TrimSpace(l.Slug) == "" {
			problems = append(problems, GraphProblem{core.ReasonProgramLakeSlugRequired,
				fmt.Sprintf("lakes[%d] has an empty slug", i)})
			continue
		}
		if seen[l.Slug] {
			problems = append(problems, GraphProblem{core.ReasonProgramLakeDuplicate,
				"lake slug appears twice: " + l.Slug})
		}
		seen[l.Slug] = true
	}
	for _, l := range lakes {
		for _, dep := range l.DependsOn {
			switch {
			case dep == l.Slug:
				problems = append(problems, GraphProblem{core.ReasonProgramDependencySelf,
					"lake " + l.Slug + " depends on itself"})
			case !seen[dep]:
				problems = append(problems, GraphProblem{core.ReasonProgramDependencyMissing,
					"lake " + l.Slug + " depends on unknown lake " + dep})
			}
		}
	}
	if len(problems) > 0 {
		return problems
	}
	if cycle := findCycle(lakes); cycle != nil {
		problems = append(problems, GraphProblem{core.ReasonProgramDependencyCycle,
			"dependency cycle: " + strings.Join(cycle, " -> ")})
	}
	return problems
}

// findCycle returns one dependency cycle as a slug path, or nil.
func findCycle(lakes []ProgramLakeInput) []string {
	deps := map[string][]string{}
	for _, l := range lakes {
		deps[l.Slug] = l.DependsOn
	}
	const (
		white = 0
		grey  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var cycle []string
	var visit func(slug string) bool
	visit = func(slug string) bool {
		color[slug] = grey
		stack = append(stack, slug)
		for _, dep := range deps[slug] {
			switch color[dep] {
			case grey:
				i := 0
				for stack[i] != dep {
					i++
				}
				cycle = append(append([]string{}, stack[i:]...), dep)
				return true
			case white:
				if visit(dep) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[slug] = black
		return false
	}
	for _, l := range lakes {
		if color[l.Slug] == white && visit(l.Slug) {
			return cycle
		}
	}
	return nil
}

// TopoOrder returns lake slugs in dependency order (dependencies
// first). ok=false when the graph has a cycle. Pure function.
func TopoOrder(lakes []ProgramLake) ([]string, bool) {
	inputs := make([]ProgramLakeInput, len(lakes))
	for i, l := range lakes {
		inputs[i] = ProgramLakeInput{Slug: l.Slug, DependsOn: l.DependsOn}
	}
	if findCycle(inputs) != nil {
		return nil, false
	}
	done := map[string]bool{}
	var out []string
	for len(out) < len(lakes) {
		progressed := false
		for _, l := range lakes {
			if done[l.Slug] {
				continue
			}
			ready := true
			for _, dep := range l.DependsOn {
				if !done[dep] {
					ready = false
					break
				}
			}
			if ready {
				done[l.Slug] = true
				out = append(out, l.Slug)
				progressed = true
			}
		}
		if !progressed {
			return nil, false
		}
	}
	return out, true
}

// ---- Lake facts (derived live from the store, never cached) ----

// featureFrozen reads the ONE shared frozen determination of
// this engine (Engine.FrozenFact, frozen_fact.go — the same fact
// admission and the truth exit gate consume).
func (e *Engine) featureFrozen(featureID string) bool {
	ff, err := e.frozenFactByID(featureID)
	return err == nil && ff.Frozen
}

// featureCompleted reports whether a lake feature is done: lifecycle
// at Release/Completed, or one of its goal runs succeeded. The
// second return value is the feature's latest goal run id ("" when
// it has none), which the route view turns into driver hints.
func (e *Engine) featureCompleted(featureID string) (bool, string) {
	completed := false
	if lc, err := e.store.Features().GetLifecycle(featureID); err == nil {
		if lc.Stage == core.StageRelease && lc.Status == core.StatusCompleted {
			completed = true
		}
	}
	latest := ""
	if runs, err := e.store.Runs().ListByFeature(featureID); err == nil {
		for _, r := range runs {
			if r.RunKind != "goal" {
				continue
			}
			latest = r.ID
			if r.Status == core.RunSucceeded {
				completed = true
			}
		}
	}
	return completed, latest
}

// lakeFeature resolves a lake slug inside the host run's project
// version (the program's binding rule).
func (e *Engine) lakeFeature(run store.Run, slug string) (store.FeaturePackage, error) {
	host, err := e.store.Features().GetByID(run.FeatureID)
	if err != nil {
		return store.FeaturePackage{}, err
	}
	return e.store.Features().GetBySlug(host.ProjectVersionID, slug)
}

// ---- Plan / freeze / change (program_freeze_gate + change intent) ----

// ProgramPlanInput is the ProgramPlan request.
type ProgramPlanInput struct {
	Slug   string
	Goal   string
	Lakes  []ProgramLakeInput
	Freeze bool
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func programLakesOf(inputs []ProgramLakeInput, carry map[string]*LakeAdmission) []ProgramLake {
	out := make([]ProgramLake, 0, len(inputs))
	for _, in := range inputs {
		l := ProgramLake{Slug: in.Slug, Goal: in.Goal, DependsOn: in.DependsOn, Touches: in.Touches}
		if carry != nil {
			if adm, ok := carry[in.Slug]; ok {
				l.Admission = adm
			}
		}
		out = append(out, l)
	}
	return out
}

// ProgramPlan creates (or, while unfrozen, replaces) the program
// plan on a host goal run. A frozen plan refuses direct edits with
// program_plan_frozen — changes must go through ProgramChange so
// they leave a record (program_freeze_gate + program_change_intent
// core semantics).
func (e *Engine) ProgramPlan(runID string, in ProgramPlanInput) (ProgramState, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramState{}, err
	}
	if run.RunKind != "goal" {
		return ProgramState{}, core.NewError(core.ReasonNotAGoalRun, "program.Plan",
			"a program anchors on a goal run, not a "+run.RunKind+" row")
	}
	if problems := ValidateProgramGraph(in.Lakes); len(problems) > 0 {
		p := problems[0]
		return ProgramState{}, core.NewError(p.ReasonCode, "program.Plan", p.Detail)
	}
	if m.Program != nil && m.Program.Frozen {
		return *m.Program, core.NewError(core.ReasonProgramPlanFrozen, "program.Plan",
			"program plan is frozen; record a change with goal program change instead")
	}
	now := nowRFC()
	st := &ProgramState{Slug: in.Slug, Goal: in.Goal, CreatedAt: now}
	event := "program_plan_created"
	if m.Program != nil {
		st = m.Program
		st.Slug, st.Goal = in.Slug, in.Goal
		st.UpdatedAt = now
		event = "program_plan_updated"
		carry := map[string]*LakeAdmission{}
		for i := range st.Lakes {
			if st.Lakes[i].Admission != nil {
				carry[st.Lakes[i].Slug] = st.Lakes[i].Admission
			}
		}
		st.Lakes = programLakesOf(in.Lakes, carry)
	} else {
		st.Lakes = programLakesOf(in.Lakes, nil)
	}
	m.Program = st
	if err := e.appendEvent(run, event, map[string]any{
		"program_slug": st.Slug, "lakes": st.lakeSlugs(),
	}); err != nil {
		return ProgramState{}, err
	}
	if in.Freeze {
		st.Frozen = true
		st.FrozenAt = now
		if err := e.appendEvent(run, "program_plan_frozen", map[string]any{
			"program_slug": st.Slug, "lakes": st.lakeSlugs(),
		}); err != nil {
			return ProgramState{}, err
		}
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramState{}, err
	}
	return *st, nil
}

// ProgramFreeze freezes the run's existing program plan.
func (e *Engine) ProgramFreeze(runID string) (ProgramState, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramState{}, err
	}
	if m.Program == nil {
		return ProgramState{}, core.NewError(core.ReasonProgramPlanMissing, "program.Freeze",
			"run "+runID+" has no program plan")
	}
	if m.Program.Frozen {
		return *m.Program, nil // already frozen: idempotent
	}
	m.Program.Frozen = true
	m.Program.FrozenAt = nowRFC()
	if err := e.appendEvent(run, "program_plan_frozen", map[string]any{
		"program_slug": m.Program.Slug, "lakes": m.Program.lakeSlugs(),
	}); err != nil {
		return ProgramState{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramState{}, err
	}
	return *m.Program, nil
}

// ProgramChange records a change intent against the plan and — when
// a replacement lake list is supplied — applies it. This is the ONLY
// way to alter a frozen plan's lake list: the change (note + before
// /after) lands in the plan snapshot and as a program_plan_changed
// event. Replacing the list clears admission records: a structural
// change invalidates earlier per-lake verdicts (the source family
// reruns its freeze gate after a structural refreeze for the same
// reason).
func (e *Engine) ProgramChange(runID, note string, lakes []ProgramLakeInput, replace bool) (ProgramState, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramState{}, err
	}
	if strings.TrimSpace(note) == "" {
		return ProgramState{}, core.NewError(core.ReasonProgramChangeNoteReq, "program.Change",
			"a program change must carry a note (the change intent record)")
	}
	if m.Program == nil {
		return ProgramState{}, core.NewError(core.ReasonProgramPlanMissing, "program.Change",
			"run "+runID+" has no program plan")
	}
	before := m.Program.lakeSlugs()
	after := before
	if replace {
		if problems := ValidateProgramGraph(lakes); len(problems) > 0 {
			p := problems[0]
			return *m.Program, core.NewError(p.ReasonCode, "program.Change", p.Detail)
		}
		m.Program.Lakes = programLakesOf(lakes, nil) // admissions cleared
		after = m.Program.lakeSlugs()
	}
	m.Program.Changes = append(m.Program.Changes, ProgramChange{
		At: nowRFC(), Note: note, LakesBefore: before, LakesAfter: after,
	})
	m.Program.UpdatedAt = nowRFC()
	if err := e.appendEvent(run, "program_plan_changed", map[string]any{
		"program_slug": m.Program.Slug, "note": note,
		"lakes_before": before, "lakes_after": after,
	}); err != nil {
		return ProgramState{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramState{}, err
	}
	return *m.Program, nil
}
