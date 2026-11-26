// Package harness owns the one canonical list of first-class command groups.
//
// The seven groups are the public surface of the requirement-truth kernel:
//
//	plan | truth | change | implement | verify | release | goal
//
// Requirement truth is the intent tree; SQLite is a rebuildable cache. Every
// other command (gate, spine, projects, migrate, doctor, review, archive,
// hooks, fsm, version) is a maintenance surface, hidden from help. design
// was removed entirely (AGENTS.md truth source), as was the old six-ability
// set; neither is a second source of truth.
package harness

// Ability is one first-class command group.
type Ability struct {
	ID          string
	Description string
}

// Abilities is the closed public command surface, in pipeline order.
// This is the single list the CLI root builds from; do not keep a second
// copy in cli/root.go.
var Abilities = []Ability{
	{ID: "plan", Description: "Create and expand the intent tree"},
	{ID: "truth", Description: "Freeze nodes, record intent, change and refreeze"},
	{ID: "change", Description: "Invalidate a frozen subtree and refreeze bottom-up"},
	{ID: "implement", Description: "Admit a leaf and record execution evidence"},
	{ID: "verify", Description: "Register evidence, verify leaves, roll up and audit"},
	{ID: "release", Description: "Final go/no-go gate; human confirmation is an invariant"},
	{ID: "goal", Description: "One staged delivery run from create to release"},
}

// IDs returns the seven group ids in pipeline order.
func IDs() []string {
	ids := make([]string, len(Abilities))
	for i, a := range Abilities {
		ids[i] = a.ID
	}
	return ids
}
