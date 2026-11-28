package gate

import (
	"regexp"
	"strings"
)

var negativeTCID = regexp.MustCompile(`^TC-[0-9]+$`)

func slotToken(s string) string { return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "`")) }

// domainIllegalProjection uses the graph parser's directional, scoped result.
// In particular, governance prose is never reinterpreted as an edge here.
func domainIllegalProjection(truth string) []DomainIllegalSlot {
	var out []DomainIllegalSlot
	for _, s := range parseIllegalSlots(truth) {
		out = append(out, DomainIllegalSlot{Description: s.Description, Aggregate: slotToken(s.Aggregate), From: slotToken(s.From), To: slotToken(s.To), Source: "truth"})
	}
	return out
}

func uncoveredIllegalSlots(p DomainFSMProjection, tests string) []DomainFSMFinding {
	// Execution, validation and this query consume the same normalized TC rows.
	refs := ParseCoverageRows(tests)
	var findings []DomainFSMFinding
	for _, slot := range p.IllegalSlots {
		covered := false
		for _, ref := range refs {
			if ref.DuplicateTC || ref.Layer != "negative" || !negativeTCID.MatchString(ref.TC) || ref.Aggregate != slot.Aggregate {
				continue
			}
			if slot.From != "" && slot.To != "" {
				covered = ref.From == slot.From && ref.To == slot.To
			} else {
				covered = ref.IllegalSlot != "" && ref.IllegalSlot == slotToken(slot.Description)
			}
			if covered {
				break
			}
		}
		if !covered {
			findings = append(findings, DomainFSMFinding{Kind: "illegal_transition_slot_uncovered", ReasonCode: "uncovered", Aggregate: slot.Aggregate, Text: "illegal slot uncovered: aggregate=" + slot.Aggregate + " from=" + slot.From + " to=" + slot.To + " description=" + slot.Description})
		}
	}
	return findings
}
