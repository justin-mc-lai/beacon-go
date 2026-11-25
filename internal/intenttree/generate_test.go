package intenttree

import (
	"strings"
	"testing"
)

func TestPlannedTestCasesCoverACAndFSM(t *testing.T) {
	n := Node{ID: "f", Kind: KindFeature, ACs: []AC{{ID: "a1", Text: "can submit"}},
		DomainFSM: &DomainFSM{
			States:      []string{"draft", "frozen"},
			Transitions: []FSMTransition{{From: "draft", To: "frozen", Event: "freeze"}},
			Illegal:     []FSMTransition{{From: "frozen", To: "draft"}},
		}}
	cases := PlannedTestCases(n)
	if len(cases) != 3 {
		t.Fatalf("cases = %d, want AC + legal + illegal", len(cases))
	}
	for _, tc := range cases {
		if tc.Status != "planned" || tc.Ref == "" || tc.ACID != "a1" {
			t.Fatalf("bad generated case: %+v", tc)
		}
	}
	if got := PlannedTestCases(Node{ID: "f", Kind: KindFeature, ACs: n.ACs, TestCases: cases, DomainFSM: n.DomainFSM}); len(got) != 0 {
		t.Fatalf("regeneration must be idempotent, got %d", len(got))
	}
	doc := GenerateDoc(n)
	for _, want := range []string{"## 状态迁移", "## 验收标准", "frozen -> draft 必须被拒绝"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("doc missing %q:\n%s", want, doc)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
