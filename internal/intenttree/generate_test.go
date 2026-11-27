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

func TestGenerateDocCopiesNodeProseInsteadOfScaffold(t *testing.T) {
	n := Node{
		ID: "f", Kind: KindFeature, Title: "feature", Status: StatusFrozen,
		Doc: "## 问题\n\n真实的用户问题。\n",
		ACs: []AC{{ID: "AC-001", Text: "可提交"}},
		DomainFSM: &DomainFSM{States: []string{"draft", "frozen"},
			Transitions: []FSMTransition{{From: "draft", To: "frozen", Event: "freeze"}}},
	}
	doc := GenerateDoc(n)
	if !strings.Contains(doc, "真实的用户问题") {
		t.Fatalf("node prose must be the base of the document:\n%s", doc)
	}
	if strings.Contains(doc, scaffoldMarker) {
		t.Fatalf("a node with real prose must not get scaffold placeholders:\n%s", doc)
	}
	if !strings.Contains(doc, "## 状态迁移") {
		t.Fatalf("structured FSM section must be appended:\n%s", doc)
	}
	if again := GenerateDoc(Node{ID: n.ID, Kind: n.Kind, Title: n.Title, Status: n.Status,
		Doc: doc, ACs: n.ACs, DomainFSM: n.DomainFSM}); strings.Count(again, "## 状态迁移") != 1 {
		t.Fatalf("regeneration must not duplicate sections:\n%s", again)
	}
}

func TestGenerateDocScaffoldOnlyForMissingProse(t *testing.T) {
	doc := GenerateDoc(Node{ID: "t", Kind: KindTouchpoint, Title: "leaf", Status: StatusFrozen})
	if !strings.Contains(doc, scaffoldMarker) {
		t.Fatalf("a node without prose must get a scaffold:\n%s", doc)
	}
	if DocReady(Node{ID: "t", Doc: doc}) {
		t.Fatal("a generated scaffold must never report DocReady")
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
