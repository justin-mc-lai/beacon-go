package gate

import (
	"testing"
)

func TestGeneralDomainKindExemption(t *testing.T) {
	// 1. Business domain with domain_required: true without user journey -> should fail
	bizTruth := `---
status: draft
domain_required: true
domain_kind: business
---
## Acceptance Criteria
| AC ID | Criterion |
|-------|-----------|
| AC-001 | Test criteria restates=1 |
`
	var findingsBiz []string
	addBiz := func(kind, code, detail string) {
		findingsBiz = append(findingsBiz, kind)
	}
	checkDFillDepth(PackageFiles{
		Truth: PackageFile{Path: "truth.md", Text: bizTruth, Present: true},
	}, map[string]string{"domain_required": "true", "domain_kind": "business"}, addBiz)

	hasJourneyMissing := false
	for _, f := range findingsBiz {
		if f == "journey_missing" {
			hasJourneyMissing = true
			break
		}
	}
	if !hasJourneyMissing {
		t.Fatalf("expected journey_missing finding for domain_kind: business, got %v", findingsBiz)
	}

	// 2. Platform / workflow / general / engine / cli / lifecycle / gate / meta with domain_required: true -> exempted
	exemptKinds := []string{"platform", "workflow", "general", "cli", "engine", "lifecycle", "gate", "meta"}
	for _, k := range exemptKinds {
		doc := "---\nstatus: draft\ndomain_required: true\ndomain_kind: " + k + "\n---\n## Acceptance Criteria\n| AC ID | Criterion |\n|-------|-----------|\n| AC-001 | Test criteria restates=1 |\n"
		var findingsExempt []string
		addExempt := func(kind, code, detail string) {
			findingsExempt = append(findingsExempt, kind)
		}
		checkDFillDepth(PackageFiles{
			Truth: PackageFile{Path: "truth.md", Text: doc, Present: true},
		}, map[string]string{"domain_required": "true", "domain_kind": k}, addExempt)

		for _, f := range findingsExempt {
			if f == "journey_missing" {
				t.Fatalf("domain_kind: %s should be exempted from journey_missing, got %v", k, findingsExempt)
			}
		}
	}
}
