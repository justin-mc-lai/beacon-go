// Brainstorm question generation (R2d, parity audit M4): the
// pure half of Python utils/brainstorm.py's run_beacon_brainstorm
// — the mode vocabulary, the Socratic question set, the
// first-principles breakdown and the redundancy cuts. In Python
// these are fixed deterministic lists (no model call); the Go
// port keeps them verbatim and feeds the questions into the
// truth package's ask ledger (orchestrator/brainstorm.go),
// where they coexist with hand-asked questions under a source
// marker.
package core

import "strings"

// BrainstormModes is brainstorm.MODES.
var BrainstormModes = []string{
	"requirement-convergence",
	"project-evolution",
	"implementation-readiness",
	"scope-simplification",
}

// brainstormModeAliases is brainstorm._MODE_ALIASES.
var brainstormModeAliases = map[string]string{
	"frozen-truth-delta":    "implementation-readiness",
	"evolution-exploration": "project-evolution",
	"redundancy-pruning":    "scope-simplification",
}

// CanonicalBrainstormMode resolves an alias to its canonical
// mode; ok=false for an unsupported mode (Python raises
// ValueError("unsupported brainstorm mode")).
func CanonicalBrainstormMode(mode string) (string, bool) {
	m := strings.TrimSpace(mode)
	if canon, ok := brainstormModeAliases[m]; ok {
		m = canon
	}
	for _, known := range BrainstormModes {
		if m == known {
			return m, true
		}
	}
	return "", false
}

// BrainstormSocraticQuestions is the socratic list of
// run_beacon_brainstorm, verbatim.
var BrainstormSocraticQuestions = []string{
	"What exact user outcome would fail if this idea is not implemented?",
	"Which frozen acceptance criterion proves this idea is required?",
	"What evidence would falsify this proposed direction?",
	"If this is an evolution, what is the smallest safe requirement delta?",
}

// BrainstormFirstPrinciples is the first_principles_breakdown
// list, verbatim.
var BrainstormFirstPrinciples = []string{
	"用户价值: identify the irreducible user outcome before proposing mechanism.",
	"证据约束: every implementation idea must trace back to frozen PRD/user-story/test-case evidence.",
	"闭环约束: if evidence is missing or changing, return to requirement/change routing instead of implementing.",
}

// BrainstormRedundancyCuts is the redundancy_cuts list, verbatim.
var BrainstormRedundancyCuts = []string{
	"Remove implementation mechanisms that cannot be tied to frozen acceptance evidence.",
	"Remove duplicate document-to-document reconciliation; use canonical Beacon truth or route a change.",
	"Remove broad roadmap speculation unless it produces a concrete next Beacon route.",
}

// BrainstormModeRecommendation ports _mode_recommendation: the
// converged recommendation text and the next route for one
// canonical mode.
func BrainstormModeRecommendation(canonicalMode string) (recommendation, nextRoute string) {
	switch canonicalMode {
	case "project-evolution":
		return "Treat this as an evolution candidate. Capture options in brainstorm output first, then route accepted scope through beacon-prd or beacon-change.",
			"beacon-prd or beacon-change"
	case "implementation-readiness":
		return "Use this only as a pre-implementation reasoning check. If frozen PRD/user-story/test-case drift is found, stop and route through beacon-change before implement.",
			"beacon-implement after drift is resolved"
	case "scope-simplification":
		return "Cut any idea that does not preserve the frozen user outcome or acceptance evidence. Prefer the smallest requirement delta.",
			"beacon-change if scope changes are needed"
	default: // requirement-convergence
		return "Converge missing or ambiguous requirement points into explicit questions and candidate decisions before editing frozen truth.",
			"beacon-prd, beacon-user-story, beacon-test-case, or beacon-change"
	}
}
