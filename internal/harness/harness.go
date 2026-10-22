// Package harness is the Go counterpart of the frozen v2 design's
// beacon-harness contract: exactly six first-class abilities and nothing
// else (AC-059, AC-063, AC-109). Internal governance surfaces (doctor,
// router, hooks, metadata, status, archive, debug) stay internal modules
// and must never reappear as first-class abilities (AC-125).
package harness

// Ability is one first-class kernel ability.
type Ability struct {
	ID          string
	Description string
}

// Abilities is the closed v2 public kernel surface, in pipeline order.
var Abilities = []Ability{
	{ID: "loop", Description: "Cross-harness orchestration: truth -> design? -> implement -> qa-evidence -> release"},
	{ID: "truth", Description: "Pre-freeze intent capture, prompt snapshots, decision packet, freeze/refreeze"},
	{ID: "design", Description: "Design primitive contract (design_primitives.v1) gating implement when design is required"},
	{ID: "implement", Description: "Consume frozen truth + bound design + admitted worktree; run agent/backend loop"},
	{ID: "qa", Description: "qa-evidence: behavior evidence, confidence chain, business FSM coverage, defect repair chain"},
	{ID: "release", Description: "Final go/no-go gate; human confirmation is an invariant"},
}

// IDs returns the six ability ids in pipeline order.
func IDs() []string {
	ids := make([]string, len(Abilities))
	for i, a := range Abilities {
		ids[i] = a.ID
	}
	return ids
}
