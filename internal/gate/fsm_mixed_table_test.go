package gate

import "testing"

func TestFSMMixedValidAndMalformedTablesRefused(t *testing.T) {
	text := "---\nfsm_required: false\n---\n## FSM\n### Order\n| From | To | Trigger | Guard |\n|---|---|---|---|\n| A | B | approve | true |\n### Payment\n| From | To | Trigger |\n|---|---|---|\n| P | Q | pay |\n"
	_, findings, _, blocked := CheckTruthFSM(text)
	if !blocked || len(findings) == 0 {
		t.Fatal("valid table concealed malformed sibling")
	}
}
