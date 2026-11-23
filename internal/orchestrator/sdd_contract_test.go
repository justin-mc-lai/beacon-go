package orchestrator

import (
	"fmt"
	"strings"
	"testing"

	"beacon-go/internal/core"
	"beacon-go/internal/gate"
)

func sddTruthFixture(acs int, negPerAC int, withFSM bool, plan string) string {
	var b strings.Builder
	b.WriteString("---\nslug: demo\nversion: v0.0.1\npackage_contract: truth-tasks\ntraceability_required: true\n---\n\n")
	b.WriteString("# Requirement Truth: demo\n\n")
	b.WriteString("## User Intent Snapshot\n\n```yaml\nutterance: \"订单审批流程：从待支付到已发布，带发布部署\"\ngoal: g\nnon_goals: n\nconstraints: c\n```\n\n")
	if withFSM {
		b.WriteString("## FSM\n\n```mermaid\nstateDiagram-v2\n [*] --> A\n```\n\n| From | To | Trigger |\n|---|---|---|\n| A | B | go |\n| B | C | ok |\n| C | A | reset |\n\n")
	}
	b.WriteString("## Subagent Plan\n\n")
	b.WriteString(plan)
	b.WriteString("\n\n## Acceptance Criteria\n\n| AC ID | Criterion |\n|---|---|\n")
	for i := 1; i <= acs; i++ {
		fmt.Fprintf(&b, "| AC-%03d | criterion %d |\n", i, i)
	}
	b.WriteString("\n## Coverage Matrix\n\n| AC ID | TC ID | Layer | Command | Assertion |\n|---|---|---|---|---|\n")
	for i := 1; i <= acs; i++ {
		fmt.Fprintf(&b, "| AC-%03d | TC-%03d | behavior | `go test ./x -run T%d -count=1` | a |\n", i, i*10+1, i)
		fmt.Fprintf(&b, "| AC-%03d | TC-%03d | positive | `go test ./x -run P%d -count=1` | a |\n", i, i*10+2, i)
		for n := 0; n < negPerAC; n++ {
			fmt.Fprintf(&b, "| AC-%03d | TC-%03d | negative | `go test ./x -run N%d%d -count=1` | a |\n", i, i*10+3+n, i, n)
		}
	}
	return b.String()
}

func findingCodes(fs []gate.Finding) map[string]bool {
	out := map[string]bool{}
	for _, f := range fs {
		out[f.ReasonCode] = true
	}
	return out
}

func TestSDDContractPasses(t *testing.T) {
	plan := "business-analyst\nproduct-manager\ndomain-fsm-architect\ndevops-engineer\nsecurity-engineer\ntest-automator\ntask-distributor\narchitect-reviewer"
	pkg := gate.PackageFiles{
		Contract: SDDContract,
		Truth:    gate.PackageFile{Path: "truth.md", Present: true, Text: sddTruthFixture(5, 2, true, plan)},
		Tasks:    gate.PackageFile{Path: "tasks.md", Present: true, Text: "## Task Ledger\n"},
	}
	if fs := sddFreezeFindings(pkg); len(fs) != 0 {
		t.Fatalf("valid SDD package blocked: %+v", fs)
	}
}

func TestSDDContractRejectsThinCounts(t *testing.T) {
	plan := "business-analyst\nproduct-manager\ndomain-fsm-architect\ndevops-engineer\nsecurity-engineer\ntest-automator\ntask-distributor\narchitect-reviewer"
	pkg := gate.PackageFiles{Contract: SDDContract,
		Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: sddTruthFixture(4, 2, true, plan)}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDACMin] {
		t.Fatal("4 ACs must trip sdd_ac_min")
	}
	pkg.Truth.Text = sddTruthFixture(5, 1, true, plan)
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDNegativeMin] {
		t.Fatal("1 negative per AC must trip sdd_negative_tc_min")
	}
}

func TestSDDContractRequiresFSM(t *testing.T) {
	plan := "business-analyst\nproduct-manager\ndomain-fsm-architect\ndevops-engineer\nsecurity-engineer\ntest-automator\ntask-distributor\narchitect-reviewer"
	pkg := gate.PackageFiles{Contract: SDDContract,
		Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: sddTruthFixture(5, 2, false, plan)}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDFSMRequired] {
		t.Fatal("missing FSM must trip sdd_fsm_required")
	}
}

func TestSDDContractRejectsPlaceholder(t *testing.T) {
	text := strings.Replace(sddTruthFixture(5, 2, true, "business-analyst\nproduct-manager\ntest-automator\ntask-distributor\narchitect-reviewer"), "criterion 1", "（一句话说清）", 1)
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDPlaceholder] {
		t.Fatal("placeholder text must trip sdd_placeholder")
	}
}

func TestSDDContractRequiresPlannedRoles(t *testing.T) {
	pkg := gate.PackageFiles{Contract: SDDContract,
		Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: sddTruthFixture(5, 2, true, "business-analyst")}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDRolesRequired] {
		t.Fatal("missing planned roles must trip sdd_roles_required")
	}
}
