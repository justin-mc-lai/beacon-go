// Main axis (P2d, G-16): goalrun_main_axis.py's generate_main_axis /
// validate_main_axis, re-based from rendered markdown files onto
// the store. The axis is a DESCRIPTION of the spine — objective,
// stage order with done-criteria, lakes in dependency order —
// derived from the run's stage plan and program plan at generation
// time. It introduces no new judgements: the criteria it states are
// the existing gate surfaces' rules (exit gates, admission,
// complete), and validation never completes or advances anything;
// it only reports drift between the stored axis and current facts
// (the source's "do not implement against stale axis" policy).
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
)

// stageCriterion states, in one line, when a product stage counts
// as done — a restatement of the gate that already judges it.
func stageCriterion(stage string) string {
	switch stage {
	case StagePlan:
		return "stage plan recorded; plan exit gate passes"
	case StageTruth:
		return "truth package frozen (lifecycle Freeze/Completed or truth_freeze gate pass)"
	case StageDesign:
		return "design primitives bound to the run; design evidence present"
	case StageImplement:
		return "implement admission passes; artifacts registered and stage evidence harvested"
	case StageQA:
		return "qa gate verdict = pass"
	case StageRelease:
		return "verifier attached and human release confirmation recorded (Complete)"
	default:
		return "stage exit gate passes"
	}
}

// GenerateMainAxis derives and records the run's main axis. The
// objective is the caller's intent line when given, else the run's
// own utterance. Program lakes are laid out in topological order.
func (e *Engine) GenerateMainAxis(runID, intent string) (MainAxis, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return MainAxis{}, err
	}
	objective := strings.TrimSpace(intent)
	if objective == "" {
		objective = m.Utterance
	}
	axis := MainAxis{Schema: "goalrun-main-axis.v1", RunID: runID, Objective: objective, GeneratedAt: nowRFC()}
	for _, sp := range m.StagePlan {
		axis.Stages = append(axis.Stages, AxisStage{Stage: sp.Stage, Criterion: stageCriterion(sp.Stage)})
	}
	if m.Program != nil {
		axis.ProgramSlug = m.Program.Slug
		order, ok := TopoOrder(m.Program.Lakes)
		if !ok {
			return MainAxis{}, core.NewError(core.ReasonProgramDependencyCycle, "goal.GenerateMainAxis",
				"recorded program graph has a cycle")
		}
		axis.Lakes = order
	}
	m.MainAxis = &axis
	if err := e.appendEvent(run, "main_axis_generated", map[string]any{
		"program_slug": axis.ProgramSlug, "stages": stageNames(axis.Stages), "lakes": axis.Lakes,
	}); err != nil {
		return MainAxis{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return MainAxis{}, err
	}
	return axis, nil
}

func stageNames(stages []AxisStage) []string {
	out := []string{}
	for _, s := range stages {
		out = append(out, s.Stage)
	}
	return out
}

// ValidateMainAxis re-derives the facts and reports every drift of
// the stored axis: stage order vs the run's current stage plan,
// lake set vs the program plan, lake order vs the dependency graph,
// and axis lake references vs the registered feature packages.
func (e *Engine) ValidateMainAxis(runID string) (AxisValidation, error) {
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return AxisValidation{}, err
	}
	if m.MainAxis == nil {
		return AxisValidation{}, core.NewError(core.ReasonMainAxisMissing, "goal.ValidateMainAxis",
			"run "+runID+" has no main axis; generate one first")
	}
	axis := m.MainAxis
	v := AxisValidation{RunID: runID, ProgramSlug: axis.ProgramSlug}
	// Stage order vs the live stage plan.
	current := []string{}
	for _, sp := range m.StagePlan {
		current = append(current, sp.Stage)
	}
	if strings.Join(stageNames(axis.Stages), ",") != strings.Join(current, ",") {
		v.Drifts = append(v.Drifts, AxisDrift{Kind: "stage_mismatch",
			Detail: "axis stages [" + strings.Join(stageNames(axis.Stages), ", ") +
				"] != current stage plan [" + strings.Join(current, ", ") + "]"})
	}
	if m.Program != nil {
		planSet := strSet(m.Program.lakeSlugs())
		axisSet := strSet(axis.Lakes)
		for _, slug := range m.Program.lakeSlugs() {
			if !axisSet[slug] {
				v.Drifts = append(v.Drifts, AxisDrift{Kind: "lake_missing",
					Detail: "program lake " + slug + " is not on the axis"})
			}
		}
		for _, slug := range axis.Lakes {
			if !planSet[slug] {
				v.Drifts = append(v.Drifts, AxisDrift{Kind: "lake_extra",
					Detail: "axis lake " + slug + " is not in the program plan"})
			}
		}
		// Order: every dependency must precede its dependent on the axis.
		pos := map[string]int{}
		for i, slug := range axis.Lakes {
			pos[slug] = i
		}
		for _, lake := range m.Program.Lakes {
			for _, dep := range lake.DependsOn {
				dpos, dok := pos[dep]
				lpos, lok := pos[lake.Slug]
				if dok && lok && dpos > lpos {
					v.Drifts = append(v.Drifts, AxisDrift{Kind: "order_conflict",
						Detail: "axis places " + lake.Slug + " before its dependency " + dep})
				}
			}
		}
	} else if len(axis.Lakes) > 0 {
		v.Drifts = append(v.Drifts, AxisDrift{Kind: "lake_extra",
			Detail: "axis carries lakes but the run has no program plan"})
	}
	// References: every axis lake must resolve to a feature package.
	for _, slug := range axis.Lakes {
		if _, err := e.lakeFeature(run, slug); err != nil {
			v.Drifts = append(v.Drifts, AxisDrift{Kind: "ref_missing",
				Detail: "axis lake " + slug + " has no feature package in the host project version"})
		}
	}
	v.OK = len(v.Drifts) == 0
	if !v.OK {
		v.ReasonCode = core.ReasonMainAxisDrift
	}
	return v, nil
}
