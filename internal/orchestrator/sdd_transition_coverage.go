package orchestrator

import (
	"beacon-go/internal/gate"
	"strings"
)

func sddTransitionCoverage(truth string, projection gate.FSMProjection, rows []gate.TCRow) []gate.Finding {
	tests := make(map[string]bool, len(rows))
	for _, r := range rows {
		tests[r.TC] = true
	}
	covered := map[string]bool{}
	triples := map[string]int{}
	for _, tr := range projection.Transitions {
		triples[tr.State+"\x00"+tr.To+"\x00"+tr.Event]++
	}
	inSection := false
	for _, line := range strings.Split(truth, "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), "Transition Coverage")
			continue
		}
		if !inSection || !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 4 && len(cells) != 5 {
			continue
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		aggregate := ""
		if len(cells) == 5 {
			aggregate = cells[0]
			cells = cells[1:]
		}
		key := cells[0] + "\x00" + cells[1] + "\x00" + cells[2]
		if tests[cells[3]] && (aggregate != "" || triples[key] == 1) {
			covered[aggregate+"\x00"+key] = true
		}
	}
	var findings []gate.Finding
	for _, tr := range projection.Transitions {
		key := tr.State + "\x00" + tr.To + "\x00" + tr.Event
		if !covered[tr.Aggregate+"\x00"+key] && !(triples[key] == 1 && covered["\x00"+key]) {
			findings = append(findings, gate.Finding{Kind: "sdd_transition_uncovered", ReasonCode: "sdd_transition_uncovered", File: "truth.md", Detail: "missing Transition Coverage TC binding: " + tr.State + " -> " + tr.To + " on " + tr.Event, RepairRoute: gate.DefaultRepairRoute})
		}
	}
	return findings
}
