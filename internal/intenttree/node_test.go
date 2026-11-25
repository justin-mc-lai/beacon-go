package intenttree

import (
	"strings"
	"testing"
)

func TestFingerprintStableAndSensitive(t *testing.T) {
	n := Node{
		ID: "n1", Kind: KindTouchpoint, Title: "登录", ParentID: "n0", Path: "root/auth/login",
		Status: StatusDraft, ACs: []AC{{ID: "n1-AC1", Text: "用户可登录"}},
		Verification: &Verification{Type: "test", Spec: "当输入正确凭证时返回会话"},
	}
	a, err := Fingerprint(n)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Fingerprint(n)
	if a != b {
		t.Fatalf("fingerprint not stable: %s vs %s", a, b)
	}
	n.ACs[0].Text = "用户可登录并保持会话"
	c, _ := Fingerprint(n)
	if a == c {
		t.Fatal("fingerprint must change when AC text changes")
	}
}

func TestValidateChildMatrix(t *testing.T) {
	cases := []struct {
		parent, child Kind
		ok            bool
	}{
		{KindIntent, KindIntent, true},
		{KindIntent, KindModule, true},
		{KindIntent, KindFeature, false},
		{KindModule, KindModule, true},
		{KindModule, KindFeature, true},
		{KindModule, KindTouchpoint, false},
		{KindFeature, KindTouchpoint, true},
		{KindFeature, KindFeature, false},
		{KindTouchpoint, KindTouchpoint, false},
	}
	for _, c := range cases {
		err := ValidateChild(c.parent, c.child)
		if (err == nil) != c.ok {
			t.Errorf("%s -> %s: ok=%v err=%v", c.parent, c.child, c.ok, err)
		}
	}
}

func TestValidateIndexCatchesDegenerateTrees(t *testing.T) {
	base := Index{Schema: Schema, RootID: "root", Nodes: []IndexEntry{
		{ID: "root", Kind: KindIntent, Path: "root"},
		{ID: "m", ParentID: "root", Kind: KindModule, Path: "legacy"},
		{ID: "f", ParentID: "m", Kind: KindFeature, Path: "legacy/x"},
		{ID: "t", ParentID: "f", Kind: KindTouchpoint, Path: "legacy/x/t"},
	}}
	if f := ValidateIndex(base); len(f) != 0 {
		t.Fatalf("valid tree findings: %v", f)
	}
	dup := base
	dup.Nodes = append(append([]IndexEntry(nil), base.Nodes...), IndexEntry{ID: "t", ParentID: "f", Kind: KindTouchpoint})
	if f := ValidateIndex(dup); !hasCode(f, ReasonDuplicateID) {
		t.Fatalf("duplicate not caught: %v", f)
	}
	orphan := base
	orphan.Nodes = append(append([]IndexEntry(nil), base.Nodes...), IndexEntry{ID: "x", ParentID: "nope", Kind: KindFeature})
	if f := ValidateIndex(orphan); !hasCode(f, ReasonOrphanNode) {
		t.Fatalf("orphan not caught: %v", f)
	}
	badKind := base
	badKind.Nodes = append(append([]IndexEntry(nil), base.Nodes...), IndexEntry{ID: "bad", ParentID: "root", Kind: KindFeature})
	if f := ValidateIndex(badKind); !hasCode(f, ReasonInvalidChildKind) {
		t.Fatalf("bad kind not caught: %v", f)
	}
}

func hasCode(findings []string, code string) bool {
	for _, f := range findings {
		if strings.Contains(f, code) {
			return true
		}
	}
	return false
}

func TestValidateDomainFSM(t *testing.T) {
	ok := &DomainFSM{States: []string{"draft", "frozen"}, Transitions: []FSMTransition{{From: "draft", To: "frozen", Event: "freeze"}}, Illegal: []FSMTransition{{From: "frozen", To: "draft"}}}
	if got := ValidateDomainFSM(ok); len(got) != 0 {
		t.Fatalf("valid FSM rejected: %v", got)
	}
	bad := &DomainFSM{States: []string{"draft", "draft", "verified"}, Transitions: []FSMTransition{{From: "draft", To: "verified"}}, Illegal: []FSMTransition{{From: "draft", To: "verified"}}}
	got := strings.Join(ValidateDomainFSM(bad), "; ")
	for _, want := range []string{ReasonFSMDuplicate, ReasonFSMConflict} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %q", want, got)
		}
	}
}

func TestValidateTestCases(t *testing.T) {
	acs := []AC{{ID: "a1"}, {ID: "a2"}}
	bad := []TestCase{{ID: "t1", ACID: "a1", Kind: "unit", Ref: "x", Status: "existing"}, {ID: "t1", ACID: "a9", Kind: "e2e", Ref: "", Status: "planned"}}
	got := strings.Join(ValidateTestCases(acs, bad), "; ")
	for _, want := range []string{ReasonTestCaseInvalid, ReasonACCoverage} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %q", want, got)
		}
	}
	ok := []TestCase{{ID: "t1", ACID: "a1", Kind: "unit", Ref: "x", Status: "existing"}, {ID: "t2", ACID: "a2", Kind: "manual", Ref: "y", Status: "planned"}}
	if got := ValidateTestCases(acs, ok); len(got) != 0 {
		t.Fatalf("valid cases rejected: %v", got)
	}
}
