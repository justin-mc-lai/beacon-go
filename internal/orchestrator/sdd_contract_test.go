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
	b.WriteString("---\nslug: demo\nversion: v0.0.1\npackage_contract: truth-tasks\ntraceability_required: true\nfsm_required: false\nnegative_required: false\n---\n\n")
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
	text := sddTruthFixture(1, 1, false, "")
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}}
	if fs := sddFreezeFindings(pkg); len(fs) != 0 {
		t.Fatalf("small stateless contract blocked: %+v", fs)
	}
}

func TestSDDContractRejectsDuplicateTC(t *testing.T) {
	text := strings.Replace(sddTruthFixture(5, 2, true, "business-analyst product-manager domain-fsm-architect devops-engineer security-engineer test-automator task-distributor architect-reviewer"), "TC-012", "TC-011", 1)
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}}
	if !findingCodes(sddFreezeFindings(pkg))["sdd_tc_duplicate"] {
		t.Fatal("duplicate TC identity must be refused")
	}
}

func TestSDDContractRejectsTrivialCommand(t *testing.T) {
	text := strings.ReplaceAll(sddTruthFixture(5, 2, true, "business-analyst product-manager domain-fsm-architect devops-engineer security-engineer test-automator task-distributor architect-reviewer"), "go test ./x -run T1 -count=1", "true")
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}}
	if !findingCodes(sddFreezeFindings(pkg))["sdd_tc_not_executable"] {
		t.Fatal("a success-only command is not verification")
	}
}

func TestSDDContractRequiresFSM(t *testing.T) {
	text := strings.Replace(sddTruthFixture(5, 2, false, ""), "fsm_required: false", "fsm_required: true", 1)
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDFSMRequired] {
		t.Fatal("declared stateful work must have a model")
	}
}

func TestSDDContractRejectsPlaceholder(t *testing.T) {
	files := truthSkeletonSDD("demo", "v0.0.1", "docs/demo")
	text := strings.Replace(files["truth.md"], `utterance: ""`, `utterance: "round prices"`, 1)
	pkg := gate.PackageFiles{Contract: SDDContract, Truth: gate.PackageFile{Path: "truth.md", Present: true, Text: text}, Tasks: gate.PackageFile{Text: files["tasks.md"]}}
	if !findingCodes(sddFreezeFindings(pkg))[core.ReasonSDDPlaceholder] {
		t.Fatal("recording intent must not make generated scaffold freezable")
	}
}
