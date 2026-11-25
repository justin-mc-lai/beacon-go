package intenttree

import (
	"strings"
	"testing"
)

func TestRenderHTMLTreeShowsRequirementBodies(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "m", ParentID: "r", Kind: KindModule, Status: StatusFrozen, Title: "登录模块", Path: "root/login-module", Fingerprint: "mm"},
		{ID: "f", ParentID: "m", Kind: KindFeature, Status: StatusFrozen, Title: "登录流", Path: "root/login-module/login-flow", Fingerprint: "cc"},
		{ID: "t", ParentID: "f", Kind: KindTouchpoint, Status: StatusFrozen, Title: "login", Path: "root/login-module/login-flow/login", Fingerprint: "bb"},
	}}
	tp := Node{ID: "t", Kind: KindTouchpoint, Title: "login", Intent: "用户能登录",
		ACs:          []AC{{ID: "t-AC1", Text: "正确凭证返回会话"}},
		Verification: &Verification{Type: "test", Spec: "当凭证正确时返回会话"}}
	f := Node{ID: "f", Kind: KindFeature, Title: "登录流", Intent: "登录状态机", DomainFSM: &DomainFSM{
		States: []string{"draft", "frozen"}, Transitions: []FSMTransition{{From: "draft", To: "frozen", Event: "freeze"}},
		Illegal: []FSMTransition{{From: "frozen", To: "draft"}}}}
	html := string(RenderHTMLTree(idx, map[string]Node{"r": {ID: "r", Kind: KindIntent, Intent: "根意图"}, "t": tp, "f": f}))
	for _, want := range []string{"用户能登录", "正确凭证返回会话", "当凭证正确时返回会话", "验收标准", "业务状态机", "freeze", "frozen→draft", "data-panel", "data-node", "fsm-canvas", "转移明细"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "没有意图说明") {
		t.Fatal("body intent was not rendered")
	}
}
