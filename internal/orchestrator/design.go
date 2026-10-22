// Design stage wiring (P2c, G-18): the slim charter-3.4 boundary.
//
// The Go version minds only three questions about design: is it
// required for this run, are the design primitives (the frozen visual
// rules) bound, and is that enough to start implementing. It never
// draws anything: v1's run_design_stage_for_goal synthesized a design
// baseline and linted DESIGN.md files — the synthesis/templates are
// the charter-dropped machinery, and the lint's surviving minimum is
// exactly this bound-or-not check. Prototype generation is a host
// bridge point (the host agent does the drawing, like ralph does the
// looping), stated on every surface via DesignBridgeNote.
//
// The binding record lives in the run snapshot (runs.metadata_json):
// a binding is a property of THIS run (two runs of one feature may
// bind different primitive versions), and feature_packages has no
// binding column — adding one would touch the frozen schema.
// gate_results stay reserved for gate evaluations (write-back is P3).
package orchestrator

import (
	"strings"

	"beacon-go/internal/core"
)

// designRequired reports whether this run must bind design
// primitives before implementing: UX was an explicit create signal,
// or the stage plan carries a design stage.
func designRequired(m *runMeta) bool {
	if m.UXRequired {
		return true
	}
	for _, sp := range m.StagePlan {
		if sp.Stage == StageDesign {
			return true
		}
	}
	return false
}

// designStageEntry returns the design entry of the stage plan.
func designStageEntry(m *runMeta) (StagePlanEntry, bool) {
	for _, sp := range m.StagePlan {
		if sp.Stage == StageDesign {
			return sp, true
		}
	}
	return StagePlanEntry{}, false
}

// designBound reports whether a usable binding is recorded: a named
// primitive set is the minimum (an unnamed binding binds nothing).
func designBound(m *runMeta) bool {
	return m.DesignBinding != nil && strings.TrimSpace(m.DesignBinding.PrimitiveSet) != ""
}

// DesignStageStatus is the read-only design surface: required? bound?
// and the recorded binding when present.
func (e *Engine) DesignStageStatus(runID string) (DesignStageView, error) {
	_, m, _, err := e.loadRun(runID)
	if err != nil {
		return DesignStageView{}, err
	}
	v := DesignStageView{
		RunID: runID, Required: designRequired(m), Bound: designBound(m),
		Binding: m.DesignBinding, BridgeNote: DesignBridgeNote,
	}
	if entry, ok := designStageEntry(m); ok {
		v.StagePresent = true
		v.StageStatus = entry.Status
	}
	if v.Required && !v.Bound {
		v.ReasonCode = core.ReasonDesignPrimitiveNotBound
	}
	return v, nil
}

// BindDesignPrimitive records the binding (primitive set + version +
// source ref) on the run and writes the design_primitive_bound event.
// It creates no files. Binding is refused when the run has no design
// stage — there is nothing to bind to (fail closed).
func (e *Engine) BindDesignPrimitive(runID string, b DesignBinding) (DesignStageView, error) {
	if strings.TrimSpace(b.PrimitiveSet) == "" {
		return DesignStageView{}, core.NewError(core.ReasonDesignPrimitiveSetReq, "goal.BindDesignPrimitive",
			"a primitive set name is required to bind")
	}
	run, m, _, err := e.loadRun(runID)
	if err != nil {
		return DesignStageView{}, err
	}
	if _, ok := designStageEntry(m); !ok {
		return DesignStageView{}, core.NewError(core.ReasonDesignStageNotRequired, "goal.BindDesignPrimitive",
			"run "+runID+" has no design stage in its plan (create it with --ux)")
	}
	b.BoundAt = nowISO()
	m.DesignBinding = &b
	if err := e.appendEvent(run, "design_primitive_bound", map[string]any{
		"primitive_set": b.PrimitiveSet, "version": b.Version,
		"source_ref": b.SourceRef, "bound_by": b.BoundBy,
	}); err != nil {
		return DesignStageView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return DesignStageView{}, err
	}
	return e.DesignStageStatus(runID)
}
