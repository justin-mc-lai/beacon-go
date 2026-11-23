package orchestrator

import (
	"beacon-go/internal/gate"
	"strings"
	"testing"
)

func TestSDDTransitionCoverageSeparatesAggregates(t *testing.T) {
	p := gate.FSMProjection{Transitions: []gate.FSMTransition{{Aggregate: "Order", State: "Pending", To: "Done", Event: "approve"}, {Aggregate: "Payment", State: "Pending", To: "Done", Event: "approve"}}}
	rows := []gate.TCRow{{TC: "TC-001"}}
	text := "## Transition Coverage\n| From | To | Trigger | TC ID |\n|---|---|---|---|\n| Pending | Done | approve | TC-001 |\n"
	if fs := sddTransitionCoverage(text, p, rows); len(fs) != 2 {
		t.Fatalf("ambiguous coverage accepted: %+v", fs)
	}
	text = "## Transition Coverage\n| Aggregate | From | To | Trigger | TC ID |\n|---|---|---|---|---|\n| Order | Pending | Done | approve | TC-001 |\n"
	if fs := sddTransitionCoverage(text, p, rows); len(fs) != 1 {
		t.Fatalf("aggregate coverage failed: %+v", fs)
	}
}

func TestSDDRejectsSuccessCommandSyntax(t *testing.T) {
	for _, command := range []string{"true;", ":;", "exit  0", "/usr/bin/true"} {
		text := strings.Replace(sddTruthFixture(1, 1, false, ""), "go test ./x -run T1 -count=1", command, 1)
		fs := sddFreezeFindings(gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Text: text}})
		if !findingCodes(fs)["sdd_tc_not_executable"] {
			t.Fatalf("accepted %q: %+v", command, fs)
		}
	}
}

func TestSDDOptionalMalformedModelRefused(t *testing.T) {
	text := sddTruthFixture(1, 1, true, "")
	fs := sddFreezeFindings(gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Text: text}})
	if !findingCodes(fs)["sdd_fsm_required"] {
		t.Fatalf("optional malformed FSM accepted: %+v", fs)
	}
}
