package orchestrator

import (
	"regexp"
	"strings"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
	"beacon-go/internal/sddroles"
)

// SDDContract marks the two-file SDD package: truth.md is the Spec
// (including the AC/TC matrix) and tasks.md is the DAG. tests.md,
// evidence.md and restatement.md are optional.
const SDDContract = "truth-tasks"

// SDD quality floors (owner decision 2026-11-23).
const (
	sddMinAC          = 5
	sddMinTCPerAC     = 3
	sddMinNegativePer = 2
	sddMinTransitions = 3
)

func packageContract(truthText string) string {
	scalars, _, ok := gate.ParseFrontmatter(truthText)
	if !ok {
		return ""
	}
	return strings.TrimSpace(scalars["package_contract"])
}

var (
	sddPlaceholders = []string{"（一句话说清", "待补齐", "见 truth.md", "TODO", "TBD", "占位"}
	sddFSMHeading   = regexp.MustCompile(`(?im)^##\s+(FSM|状态机)\b`)
	sddFromToHeader = regexp.MustCompile(`(?im)^\|[^\n]*\bfrom\b[^\n]*\bto\b[^\n]*\|`)
)

// sddFreezeFindings enforces the SDD contract the four-piece gate does
// not: intent depth, test density with negative cases, a complete FSM
// (diagram + transitions), and an on-demand subagent plan.
func sddFreezeFindings(pkg gate.PackageFiles) []gate.Finding {
	var out []gate.Finding
	add := func(code, file, detail string) {
		out = append(out, gate.Finding{Kind: code, ReasonCode: code, File: file, Detail: detail, RepairRoute: gate.DefaultRepairRoute})
	}
	truthPath := pkg.Truth.Path

	for _, phrase := range sddPlaceholders {
		if strings.Contains(pkg.Truth.Text, phrase) || strings.Contains(pkg.Tasks.Text, phrase) {
			add(core.ReasonSDDPlaceholder, truthPath, "package still contains the placeholder phrase "+phrase)
			break
		}
	}

	defined := gate.DefinedACs(pkg.Truth.Text)
	if len(defined) < sddMinAC {
		add(core.ReasonSDDACMin, truthPath, "SDD package needs at least 5 acceptance criteria")
	}

	rows, _, matrixOK := gate.ParseCoverageMatrix(pkg.Truth.Text)
	if !matrixOK {
		add(core.ReasonSDDMatrixMissing, truthPath, "truth.md has no AC/TC coverage matrix (AC ID header)")
		return out
	}
	tcByAC := map[string]int{}
	negByAC := map[string]int{}
	for _, row := range rows {
		if row.IsSoft() {
			continue
		}
		tcByAC[row.AC]++
		if strings.EqualFold(strings.TrimSpace(row.Layer), "negative") {
			negByAC[row.AC]++
		}
	}
	for ac := range defined {
		if tcByAC[ac] < sddMinTCPerAC {
			add(core.ReasonSDDTCPerAC, truthPath, ac+" needs at least 3 executable TCs, has "+itoa(tcByAC[ac]))
		}
		if negByAC[ac] < sddMinNegativePer {
			add(core.ReasonSDDNegativeMin, truthPath, ac+" needs at least 2 negative TCs, has "+itoa(negByAC[ac]))
		}
	}

	if !sddFSMHeading.MatchString(pkg.Truth.Text) {
		add(core.ReasonSDDFSMRequired, truthPath, "SDD package needs a ## FSM section")
	} else {
		if !strings.Contains(pkg.Truth.Text, "stateDiagram") {
			add(core.ReasonSDDFSMRequired, truthPath, "FSM section needs a Mermaid stateDiagram block")
		}
		header := sddFromToHeader.FindString(pkg.Truth.Text)
		if header == "" || transitionRows(pkg.Truth.Text) < sddMinTransitions {
			add(core.ReasonSDDFSMRequired, truthPath, "FSM needs a From/To transition table with at least 3 transitions")
		}
	}

	if !strings.Contains(pkg.Truth.Text, "## Subagent Plan") {
		add(core.ReasonSDDRolesRequired, truthPath, "SDD package needs a ## Subagent Plan section")
	} else if utterance := sddUtterance(pkg.Truth.Text); utterance == "" {
		add(core.ReasonSDDRolesRequired, truthPath, "utterance is empty; the subagent plan cannot be judged")
	} else {
		plan := sddroles.Select(utterance)
		if !plan.Coverage.Ready {
			add(core.ReasonSDDRolesRequired, truthPath, "subagent plan coverage is not ready")
		}
		for _, stage := range plan.Stages {
			for _, role := range stage.Roles {
				if !strings.Contains(pkg.Truth.Text, role.ID) {
					add(core.ReasonSDDRolesRequired, truthPath, "planned role "+role.ID+" is missing from the Subagent Plan")
				}
			}
		}
	}
	return out
}

func sddUtterance(truth string) string {
	for _, line := range strings.Split(truth, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(s), "utterance:") {
			v := strings.TrimSpace(s[len("utterance:"):])
			v = strings.Trim(v, `"'`)
			return v
		}
	}
	return ""
}

func transitionRows(truth string) int {
	lines := strings.Split(truth, "\n")
	in, n := false, 0
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if sddFromToHeader.MatchString(s) {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(s, "|") {
			if s != "" {
				break
			}
			continue
		}
		if strings.Contains(s, "---") || strings.Contains(s, ":--") {
			continue
		}
		n++
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
