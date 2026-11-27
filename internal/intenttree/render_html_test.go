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
	for _, want := range []string{"用户能登录", "正确凭证返回会话", "当凭证正确时返回会话", "验收标准", "业务状态机", "freeze", "frozen→draft", "data-panel", "data-node", "fsm-canvas", "转移明细", "tree-toggle", "status-badge", "全部展开", "localStorage", "权威源", "HTML 派生只读", "已交付"} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if strings.Contains(html, "没有意图说明") {
		t.Fatal("body intent was not rendered")
	}
}

func TestRenderHTMLTreeBindsTestCaseRowsToAC(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "n_bind", ParentID: "r", Kind: KindTouchpoint, Status: StatusFrozen, Title: "bind", Path: "root/bind", Fingerprint: "bb"},
	}}
	body := Node{ID: "n_bind", Kind: KindTouchpoint, Status: StatusFrozen, Title: "bind", Path: "root/bind", Fingerprint: "bb",
		ACs: []AC{{ID: "AC-001", Text: "点击即绑定"}, {ID: "AC-002", Text: "另一条绑定"}},
		TestCases: []TestCase{
			{ID: "tc-1", ACID: "AC-001", Kind: "unit", Ref: "go test ./internal/intenttree -run TestOne", Status: "planned"},
			{ID: "tc-2", ACID: "AC-002", Kind: "manual", Ref: "manual bind check", Status: "existing"},
		},
		Verification: &Verification{Type: "test", Spec: "点击行生成写回命令"}}
	out := string(RenderHTMLTree(idx, map[string]Node{"r": {ID: "r", Kind: KindIntent, Title: "root"}, "n_bind": body}))
	for _, want := range []string{
		`data-bind-ac="AC-001"`,
		`data-bind-ac="AC-002"`,
		`data-bind-testcase="tc-1"`,
		`data-bind-ref="go test ./internal/intenttree -run TestOne"`,
		`data-bind-form="n_bind"`,
		`data-bind-state`,
		`beacon change request --project . --node n_bind`,
		`beacon truth node update --project . --node n_bind --file /tmp/beacon-bind-n_bind.json`,
		`test_cases`,
		`addEventListener('click'`,
		`data-panel="n_bind"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q", want)
		}
	}
	if got := strings.Count(out, `class="bind-ac-row"`); got != 2 {
		t.Fatalf("bindable test case rows = %d, want 2", got)
	}
}

func TestRenderHTMLTreeShowsTruthLinkMismatch(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusFrozen, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "f", ParentID: "r", Kind: KindFeature, Status: StatusDraft, Title: "feature", Path: "root/feature", TruthPath: "docs/beacon/v0.1.0/features/feature/truth.md", Fingerprint: "bb"},
	}}
	body := Node{ID: "f", Kind: KindFeature, Title: "feature", Path: "root/feature", Status: StatusDraft, TruthPath: "docs/beacon/v0.1.0/features/feature/truth.md"}
	out := string(RenderHTMLTreeWithTruth(idx, map[string]Node{"r": {ID: "r", Kind: KindIntent}, "f": body}, map[string]TruthLinkStatus{
		"f": {Path: body.TruthPath, PackageStatus: "frozen", Match: false},
	}))
	for _, want := range []string{"需求包", "包状态：frozen", "状态不一致"} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLTreeCountsDelivered(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "r", Nodes: []IndexEntry{
		{ID: "r", Kind: KindIntent, Status: StatusVerified, Title: "root", Path: "root", Fingerprint: "aa"},
		{ID: "f", ParentID: "r", Kind: KindFeature, Status: StatusImplemented, Title: "feature", Path: "root/feature", Fingerprint: "bb"},
	}}
	body := Node{ID: "f", Kind: KindFeature, Title: "feature", Status: StatusImplemented, Implementation: &ImplementationRecord{By: "dev"}}
	out := string(RenderHTMLTree(idx, map[string]Node{"r": {ID: "r", Kind: KindIntent}, "f": body}))
	for _, want := range []string{"已交付/实现 <b>1</b>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("html missing %q", want)
		}
	}
}

func TestRenderHTMLTreeRendersFeatureDoc(t *testing.T) {
	idx := Index{Schema: Schema, Version: "v0.1.0", RootID: "f", Nodes: []IndexEntry{
		{ID: "f", Kind: KindFeature, Status: StatusFrozen, Title: "feature", Path: "root/feature", Fingerprint: "aa"},
	}}
	body := Node{ID: "f", Kind: KindFeature, Status: StatusFrozen, Doc: "## 问题\n用户拿不到结果。\n## 目标用户\n维护者。\n## 场景\n打开工作台。\n## 范围\n一个页面。\n## 非目标\n不做服务端。\n## 验收标准\n能看到正文。"}
	out := string(RenderHTMLTree(idx, map[string]Node{"f": body}))
	for _, want := range []string{"需求正文", "用户拿不到结果", "目标用户", "验收标准"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doc render missing %q", want)
		}
	}
}
