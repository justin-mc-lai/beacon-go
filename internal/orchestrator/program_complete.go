// Program-level one-shot release confirmation (program-sea-complete).
// `program drive --until-done` brings every lake to its release gate
// without requiring a human per lake; `program complete` is the one
// human signature that releases the whole sea together. It refuses if
// any lake is blocked, not at its gate, lacks a verifier, or lacks a
// passing qa verdict.
package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"beacon-go/internal/core"
)

func (e *Engine) ProgramComplete(hostRunID, confirmedBy string) (ProgramDriveView, error) {
	if strings.TrimSpace(confirmedBy) == "" {
		return ProgramDriveView{}, core.NewError(core.ReasonGateEvaluatorRequired, "goal.ProgramComplete",
			"a human identity is required (--confirmed-by)")
	}
	run, m, _, err := e.loadRun(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	if m.Program == nil {
		return ProgramDriveView{}, core.NewError(core.ReasonProgramPlanMissing, "goal.ProgramComplete",
			"run "+hostRunID+" has no program plan")
	}
	route, err := e.ProgramRoute(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	if route.Rollup.Blocked > 0 {
		return ProgramDriveView{}, core.NewError(core.ReasonProgramBlocked, "goal.ProgramComplete",
			fmt.Sprintf("program %s has %d blocked lake(s); resolve them before the one-shot confirmation",
				m.Program.Slug, route.Rollup.Blocked))
	}

	// Two passes: validate every lake before completing any, so a bad
	// lake cannot leave the sea half-released.
	var pending []LakeRoute
	for _, row := range route.Lakes {
		if row.ProgramStatus == "completed" {
			continue
		}
		if row.ProgramStatus != "awaiting_release" {
			return ProgramDriveView{}, core.NewError(core.ReasonProgramLakeNotReady, "goal.ProgramComplete",
				"lake "+row.Lake+" is "+row.ProgramStatus+" (want awaiting_release)")
		}
		if row.GoalRunID == "" {
			return ProgramDriveView{}, core.NewError(core.ReasonProgramLakeNotReady, "goal.ProgramComplete",
				"lake "+row.Lake+" has no bound goal run")
		}
		pending = append(pending, row)
	}
	// Status validation is complete; now make sure every lake has an
	// independent verifier before releasing any of them.
	for _, row := range pending {
		if err := e.ensureVerifier(row.GoalRunID); err != nil {
			return ProgramDriveView{}, err
		}
	}
	var confirmed []map[string]string
	for _, row := range pending {
		if _, err := e.Complete(row.GoalRunID, CompleteOptions{
			HumanConfirmed: true,
			ConfirmedBy:    "program:" + confirmedBy,
		}); err != nil {
			return ProgramDriveView{}, err
		}
		confirmed = append(confirmed, map[string]string{"lake": row.Lake, "run_id": row.GoalRunID})
	}

	// Reload: Complete saved each lake run, not the host snapshot.
	run, m, _, err = e.loadRun(hostRunID)
	if err != nil {
		return ProgramDriveView{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range m.Program.Lakes {
		if m.Program.Lakes[i].ProgramStatus == "awaiting_release" {
			m.Program.Lakes[i].ProgramStatus = "completed"
		}
		m.Program.Lakes[i].UpdatedAt = now
	}
	m.Program.UpdatedAt = now
	if err := e.appendEvent(run, "program_release_confirmed", map[string]any{
		"by": confirmedBy, "lakes": confirmed,
	}); err != nil {
		return ProgramDriveView{}, err
	}
	if err := e.saveRun(run, m); err != nil {
		return ProgramDriveView{}, err
	}
	fresh, ferr := e.ProgramRoute(hostRunID)
	if ferr != nil {
		return ProgramDriveView{}, ferr
	}
	return ProgramDriveView{
		RunID: hostRunID, ProgramSlug: m.Program.Slug,
		ProgramStatus: "delivered", Route: fresh,
	}, nil
}
