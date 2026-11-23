package gate

import (
	"strings"
	"testing"
)

func TestNormalizedFSMDialects(t *testing.T) {
	for _, table := range []string{
		"| From | To | Trigger | Guard | Side effect |\n|---|---|---|---|---|\n| draft | approved | approve | reviewed | publish |",
		"| Action | To | Guard | Event | State |\n|---|---|---|---|---|\n| publish | approved | reviewed | approve | draft |",
	} {
		p := ParseTruthFSM("## FSM\n" + table + "\n### Illegal transitions\n- illegal: approved -> draft\n")
		if !p.FiveTuplePresent || len(p.Transitions) != 1 {
			t.Fatalf("projection: %+v", p)
		}
		tr := p.Transitions[0]
		if tr.State != "draft" || tr.To != "approved" || tr.Event != "approve" || tr.Guard != "reviewed" || tr.Action != "publish" {
			t.Fatalf("column normalization: %+v", tr)
		}
		if findings, blocked := ValidateTruthFSM(p); blocked {
			t.Fatalf("valid dialect rejected: %+v", findings)
		}
	}
}

func TestNormalizedFSMScopeAndMissingFields(t *testing.T) {
	table := "| From | To | Trigger | Guard | Side effect |\n|---|---|---|---|---|\n| draft | approved | approve | none | publish |\n"
	if p := ParseTruthFSM("## Examples\n" + table); p.FiveTuplePresent {
		t.Fatalf("unrelated table became model: %+v", p)
	}
	p := ParseTruthFSM("## FSM\n### Transitions\n" + strings.Replace(table, "| approve |", "| |", 1) + "## Illegal transitions\n- illegal: approved -> draft\n")
	findings, blocked := ValidateTruthFSM(p)
	if !blocked || !fsmKinds(findings)["transition_event_missing"] {
		t.Fatalf("missing event escaped validation: %+v", findings)
	}
	p = ParseTruthFSM("## FSM\n" + table + "## Illegal transitions\n| To | From | Reason |\n|---|---|---|\n| approved | draft | forbidden |\n")
	findings, blocked = ValidateTruthFSM(p)
	if !blocked || !fsmKinds(findings)["illegal_declared_as_legal"] {
		t.Fatalf("contradiction escaped validation: %+v", findings)
	}
}

func TestNormalizedFSMLegacyAggregate(t *testing.T) {
	p := ParseTruthFSM("## Domain FSM - Order\n| State | From | Guard |\n|---|---|---|\n| pending | - | - |\n| paid | pending | payment_confirmed |\n## Illegal transitions\n| Description | Aggregate | Source |\n|---|---|---|\n| paid -> pending (no rollback) | Order | manual |\n")
	if len(p.Transitions) != 1 || p.Transitions[0].State != "pending" || p.Transitions[0].To != "paid" || p.Transitions[0].Aggregate != "Order" {
		t.Fatalf("legacy direction: %+v", p)
	}
	if len(p.Illegal) != 1 || p.Illegal[0].From != "paid" || p.Illegal[0].To != "pending" || p.Illegal[0].Aggregate != "Order" {
		t.Fatalf("legacy illegal table: %+v", p.Illegal)
	}
}

func TestCheckTruthFSMApplicability(t *testing.T) {
	for _, tc := range []struct {
		name, truth         string
		applicable, blocked bool
	}{
		{"absent", "# Truth\n", true, true},
		{"required absent", "---\nfsm_required: true\n---\n", true, true},
		{"stateless", "---\nfsm_required: false\n---\n", false, false},
		{"empty table", "## FSM\n| State | Event | Guard | To | Action |\n|---|---|---|---|---|\n", true, true},
		{"present invalid despite optional", "---\nfsm_required: false\n---\n## FSM\n| From | To | Trigger | Guard |\n|---|---|---|---|\n| a | b | go | none |\n", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, applicable, blocked := CheckTruthFSM(tc.truth)
			if applicable != tc.applicable || blocked != tc.blocked {
				t.Fatalf("applicable=%v blocked=%v", applicable, blocked)
			}
		})
	}
}

func TestFSMCoverageTableIsNotModel(t *testing.T) {
	for _, heading := range []string{"## FSM Transition Coverage", "## FSM\n### Transition Coverage"} {
		p := ParseTruthFSM(heading + "\n| From | To | Trigger | TC ID |\n|---|---|---|---|\n| a | b | go | TC-001 |\n")
		if p.FiveTuplePresent || len(p.Transitions) != 0 {
			t.Fatalf("coverage became model: %+v", p)
		}
	}
}

func TestNormalizedFSMLegacyColumnsNotPositions(t *testing.T) {
	p := ParseTruthFSM("## Domain FSM - Order\n| Guard | From | To |\n|---|---|---|\n| confirmed | pending | paid |\n## Illegal transitions\n- illegal: paid -> pending\n")
	if len(p.Transitions) != 1 || p.Transitions[0].State != "pending" || p.Transitions[0].To != "paid" || p.Transitions[0].Guard != "confirmed" {
		t.Fatalf("legacy column mapping: %+v", p)
	}
}
