// Program admission + route (P2d, G-12): per-lake entry checks and
// the read-only orchestration view.
//
// Admission reuses the engine's existing derivations — there is no
// second frozen/completed judgement: a lake's truth must be frozen
// under the same rule the exit gates use (featureFrozen), and a
// dependency counts as closed when the lake is completed under the
// same rule the route uses (featureCompleted) or already carries a
// passing admission record on this plan.
package orchestrator

import (
	"sort"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/store"
)

// ProgramAdmission judges every lake of the run's program plan (or
// one named lake) and records the verdicts on the plan. A lake is
// admitted when its feature package exists, its truth is frozen and
// every dependency is closed (completed or already admitted).
func (e *Engine) ProgramAdmission(runID, onlyLake string) (ProgramAdmissionView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramAdmissionView{}, err
	}
	if m.Program == nil {
		return ProgramAdmissionView{}, core.NewError(core.ReasonProgramPlanMissing, "program.Admission",
			"run "+runID+" has no program plan")
	}
	if onlyLake != "" && m.Program.lake(onlyLake) == nil {
		return ProgramAdmissionView{}, core.NewError(core.ReasonProgramLakeNotInPlan, "program.Admission",
			"lake "+onlyLake+" is not in program "+m.Program.Slug)
	}
	view := ProgramAdmissionView{RunID: runID, ProgramSlug: m.Program.Slug}
	payload := []map[string]any{}
	// Judge in topological order: a dependency admitted earlier in
	// this same pass counts as closed for its dependents.
	order, _ := TopoOrder(m.Program.Lakes)
	for _, slug := range order {
		lake := m.Program.lake(slug)
		if onlyLake != "" && lake.Slug != onlyLake {
			continue
		}
		adm := e.judgeLakeAdmission(run, m.Program, lake.Slug)
		lake.Admission = &adm
		view.Lakes = append(view.Lakes, adm)
		payload = append(payload, map[string]any{
			"lake": adm.Lake, "admitted": adm.Admitted, "reason_codes": adm.ReasonCodes,
		})
	}
	// AllAdmitted speaks about the whole plan, judged or not.
	view.AllAdmitted = true
	for _, l := range m.Program.Lakes {
		if l.Admission == nil || !l.Admission.Admitted {
			view.AllAdmitted = false
		}
	}
	if err := e.appendEvent(run, "program_admission_evaluated", map[string]any{
		"program_slug": m.Program.Slug, "lakes": payload,
	}); err != nil {
		return ProgramAdmissionView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramAdmissionView{}, err
	}
	return view, nil
}

// judgeLakeAdmission evaluates one lake's three admission items.
func (e *Engine) judgeLakeAdmission(run store.Run, plan *ProgramState, slug string) LakeAdmission {
	adm := LakeAdmission{Lake: slug, Admitted: true, EvaluatedAt: nowRFC()}
	fail := func(check, code, detail string) {
		adm.Items = append(adm.Items, LakeAdmissionItem{Check: check, ReasonCode: code, Detail: detail})
		adm.ReasonCodes = append(adm.ReasonCodes, code)
		adm.Admitted = false
	}
	ok := func(check, detail string) {
		adm.Items = append(adm.Items, LakeAdmissionItem{Check: check, OK: true, Detail: detail})
	}
	feat, err := e.lakeFeature(run, slug)
	if err != nil {
		fail("feature_exists", core.ReasonProgramLakeFeatureMiss,
			"no feature package "+slug+" in the host run's project version")
		// Without a feature the remaining items cannot hold either.
		fail("truth_frozen", core.ReasonProgramTruthNotFrozen, "feature package missing")
	} else if e.featureFrozen(feat.ID) {
		ok("feature_exists", "feature package present")
		ok("truth_frozen", "truth frozen")
	} else {
		ok("feature_exists", "feature package present")
		fail("truth_frozen", core.ReasonProgramTruthNotFrozen,
			"lake truth is not frozen (fill != freeze)")
	}
	lake := plan.lake(slug)
	var unclosed []string
	for _, dep := range lake.DependsOn {
		if plan.lakeAdmitted(dep) {
			continue
		}
		depFeat, err := e.lakeFeature(run, dep)
		if err != nil {
			unclosed = append(unclosed, dep)
			continue
		}
		if done, _ := e.featureCompleted(depFeat.ID); !done {
			unclosed = append(unclosed, dep)
		}
	}
	if len(unclosed) > 0 {
		sort.Strings(unclosed)
		fail("dependencies_closed", core.ReasonProgramDependencyUnclos,
			"dependencies not completed or admitted: "+strings.Join(unclosed, ", "))
	} else {
		ok("dependencies_closed", "all dependencies completed or admitted")
	}
	return adm
}

// ProgramRoute computes the live orchestration view: lakes in
// topological order, each with its derived status, the next batch
// that may start, and the per-lake G-06 driver hint. Read-only —
// nothing is persisted here (the state lives in the plan snapshot
// and in each lake's own runs/lifecycle).
func (e *Engine) ProgramRoute(runID string) (ProgramRouteView, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return ProgramRouteView{}, err
	}
	if m.Program == nil {
		return ProgramRouteView{}, core.NewError(core.ReasonProgramPlanMissing, "program.Route",
			"run "+runID+" has no program plan")
	}
	plan := m.Program
	order, ok := TopoOrder(plan.Lakes)
	if !ok {
		return ProgramRouteView{}, core.NewError(core.ReasonProgramDependencyCycle, "program.Route",
			"recorded program graph has a cycle")
	}
	view := ProgramRouteView{RunID: runID, ProgramSlug: plan.Slug, Frozen: plan.Frozen, AllComplete: len(order) > 0}
	bySlug := map[string]LakeRoute{}
	for _, slug := range order {
		lake := plan.lake(slug)
		row := LakeRoute{Lake: slug, Goal: lake.Goal, DependsOn: lake.DependsOn}
		feat, ferr := e.lakeFeature(run, slug)
		completed := false
		if ferr == nil {
			row.FeatureID = feat.ID
			completed, row.GoalRunID = e.featureCompleted(feat.ID)
		}
		// Startability is stricter than admission: admission counts
		// a dependency closed when it is completed OR admitted (the
		// paperwork chain), but a lake may only START once its
		// dependencies are completed (the source orchestrator's
		// await_upstream_lake_closure). An admitted lake with open
		// dependencies therefore routes as waiting, not admitted.
		depsDone := true
		for _, dep := range lake.DependsOn {
			if bySlug[dep].Status != LakeCompleted {
				depsDone = false
			}
		}
		switch {
		case completed:
			row.Status = LakeCompleted
		case ferr != nil:
			row.Status = LakeBlocked
			row.ReasonCode = core.ReasonProgramLakeFeatureMiss
		case !e.featureFrozen(feat.ID):
			row.Status = LakeBlocked
			row.ReasonCode = core.ReasonProgramTruthNotFrozen
		case !depsDone:
			row.Status = LakeWaiting
			row.ReasonCode = core.ReasonProgramDependencyUnclos
		case plan.lakeAdmitted(slug):
			row.Status = LakeAdmitted
		default:
			row.Status = LakeReady
		}
		if row.Status == LakeAdmitted || row.Status == LakeReady {
			row.DriverHint = driverHintFor(runID, row)
			view.NextBatch = append(view.NextBatch, slug)
		}
		if row.Status != LakeCompleted {
			view.AllComplete = false
		}
		bySlug[slug] = row
		view.Lakes = append(view.Lakes, row)
	}
	return view, nil
}

// driverHintFor renders the per-lake long-run suggestion: program
// has no driver of its own — G-06's driver pushes the lake's own
// goal run (program_driver.py's per-lake drive, expressed as the
// equivalent goal commands).
func driverHintFor(hostRunID string, row LakeRoute) string {
	drive := ""
	if row.GoalRunID != "" {
		drive = "beacon goal driver start --run-id " + row.GoalRunID
	} else {
		drive = "beacon goal run --feature " + row.Lake + " (then: beacon goal driver start --run-id <lake run>)"
	}
	if row.Status == LakeReady {
		return "beacon goal program admit --run-id " + hostRunID + " --lake " + row.Lake + "; then " + drive
	}
	return drive
}
