package cli

import (
	"strings"
	"testing"

	"beacon-go/internal/core"
)

func TestTruthFSMCheckUsesGraphValidator(t *testing.T) {
	base := "---\npackage_contract: truth-tasks\nfsm_required: true\n---\n## FSM\n| From | To | Trigger | Guard | Side effect |\n|---|---|---|---|---|\n| a | b | go | none | write |\n## Illegal transitions\n- illegal: b -> a\n"
	view := buildFSMCheckView(base, "feature", "id")
	if !view.Valid || !view.Applicable || len(view.Findings) != 0 {
		t.Fatalf("normalized SDD model: %+v", view)
	}
	for _, invalid := range []string{
		strings.Replace(base, "- illegal: b -> a", "- illegal: a -> b", 1),
		strings.Replace(base, "| a | b | go |", "| a | b | |", 1),
		strings.Split(base, "## Illegal")[0],
	} {
		view := buildFSMCheckView(invalid, "feature", "id")
		if view.Valid || !view.Applicable || len(view.GraphFindings) == 0 {
			t.Fatalf("invalid model accepted: %+v", view)
		}
	}
}

func TestTruthFSMCheckMissingAndStateless(t *testing.T) {
	for _, tc := range []struct {
		name, truth string
		applicable  bool
	}{
		{"missing", "---\npackage_contract: truth-tasks\nfsm_required: true\n---\n", true},
		{"undeclared", "# Truth\n", true},
		{"stateless", "---\npackage_contract: truth-tasks\nfsm_required: false\n---\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := buildFSMCheckView(tc.truth, "feature", "id")
			if view.Valid || view.Applicable != tc.applicable {
				t.Fatalf("empty projection must not be valid: %+v", view)
			}
		})
	}
}

func TestTruthFSMCheckRefusalIsTyped(t *testing.T) {
	missing := buildFSMCheckView("# Truth", "feature", "id")
	if code, ok := core.ReasonCodeOf(fsmCheckError(missing)); !ok || code != "fsm_check_blocked" {
		t.Fatalf("expected typed FSM rejection, view=%+v", missing)
	}
	stateless := buildFSMCheckView("---\npackage_contract: truth-tasks\nfsm_required: false\n---\n", "feature", "id")
	if err := fsmCheckError(stateless); err != nil {
		t.Fatalf("explicit not applicable rejected: %v", err)
	}
}
