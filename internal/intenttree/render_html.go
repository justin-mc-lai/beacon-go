package intenttree

import (
	"fmt"
	"sort"
	"strings"
)

// RenderContext is the version identity of one rendered tree. Runtime,
// requirement line, snapshot and source hash are separate axes and must
// stay visible separately.
type HistoryLink struct {
	ID        string
	Label     string
	URL       string
	CreatedAt string
}

type RenderContext struct {
	RuntimeVersion     string
	RuntimeCommit      string
	BinarySHA          string
	RequirementVersion string
	SnapshotID         string
	SnapshotCreatedAt  string
	History            []HistoryLink
	Changes            []HistoryLink
	NodeHistory        map[string][]NodeRevision
	GateVerdicts       map[string][]GateVerdictStatus
	BackURL            string
	BindingURL         string
	BindingToken       string
}

// GateVerdictStatus is the read-only gate result shown in a node's
// verdict panel. The kernel decides the verdict; rendering only shows it.
type GateVerdictStatus struct {
	GateType    string   `json:"gate_type"`
	Verdict     string   `json:"verdict"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
	EvaluatedAt string   `json:"evaluated_at,omitempty"`
}

type NodeRevision struct {
	SnapshotID  string
	Label       string
	URL         string
	Status      string
	Fingerprint string
}

// RenderHTMLTree renders a three-pane reader: the full requirement tree
// on the left, the selected node's requirement text in the middle, and
// the related business FSM as a real flow diagram on the right.
func RenderHTMLTree(idx Index, bodies map[string]Node) []byte {
	return RenderHTMLTreeWithTruth(idx, bodies, nil)
}

// RenderHTMLTreeWithTruth adds the read-only package-link comparison to
// the node detail pane.
func RenderHTMLTreeWithTruth(idx Index, bodies map[string]Node, truth map[string]TruthLinkStatus) []byte {
	return RenderHTMLTreeWithContext(idx, bodies, truth, RenderContext{RequirementVersion: idx.Version})
}

// RenderHTMLTreeWithContext adds the runtime/requirement/snapshot identity.
func RenderHTMLTreeWithContext(idx Index, bodies map[string]Node, truth map[string]TruthLinkStatus, rc RenderContext) []byte {
	idx = SortedIndex(idx)
	children := map[string][]IndexEntry{}
	for _, n := range idx.Nodes {
		children[n.ParentID] = append(children[n.ParentID], n)
	}
	for k := range children {
		sort.Slice(children[k], func(i, j int) bool { return children[k][i].Path < children[k][j].Path })
	}
	ownFSM := map[string]string{}
	var walk func(id string)
	walk = func(id string) {
		n := bodies[id]
		if n.DomainFSM != nil {
			ownFSM[id] = id
		}
		for _, c := range children[id] {
			walk(c.ID)
		}
	}
	walk(idx.RootID)
	related := map[string]string{}
	firstFSM := map[string]string{}
	var first func(id string) string
	first = func(id string) string {
		if got, ok := firstFSM[id]; ok {
			return got
		}
		if ownFSM[id] != "" {
			firstFSM[id] = ownFSM[id]
			return ownFSM[id]
		}
		for _, c := range children[id] {
			if got := first(c.ID); got != "" {
				firstFSM[id] = got
				return got
			}
		}
		firstFSM[id] = ""
		return ""
	}
	defaultFSM := ""
	for _, id := range ownFSM {
		if bodies[id].Title == "freeze-change" {
			defaultFSM = id
		}
	}
	var assign func(id, inherited string)
	assign = func(id, inherited string) {
		owner := inherited
		if ownFSM[id] != "" {
			owner = ownFSM[id]
		} else if owner == "" {
			owner = first(id)
		}
		if id == idx.RootID && defaultFSM != "" {
			owner = defaultFSM
		}
		related[id] = owner
		for _, c := range children[id] {
			assign(c.ID, owner)
		}
	}
	assign(idx.RootID, "")
	var b strings.Builder
	writeHTMLHead(&b, idx, bodies, rc)
	b.WriteString(`<div class="app">`)
	writeTreePane(&b, idx, children, bodies, related, ownFSM, rc)
	writeDetailPane(&b, idx, children, bodies, truth, rc)
	writeFSMPane(&b, idx, children, bodies, ownFSM)
	b.WriteString(`</div>`)
	writeScript(&b, idx)
	fmt.Fprintf(&b, "</body></html>\n")
	return []byte(b.String())
}

func writeHTMLHead(b *strings.Builder, idx Index, bodies map[string]Node, rc RenderContext) {
	fmt.Fprintf(b, "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	fmt.Fprintf(b, "<!-- source: %s -->\n", SourceHash(idx))
	fmt.Fprintf(b, "<title>需求工作台 · %s</title><style>%s</style></head><body>\n", esc(idx.Version), css)
}

const css = `
:root{--ink:#1f2937;--muted:#6b7280;--line:#e5e7eb;--bg:#f7f6f3;--card:#fff;--accent:#2f6f4e;--accent-soft:#f1f8f4;--danger:#b23b3b;--warn:#8a5a12}
*{box-sizing:border-box}html,body{height:100%}body{margin:0;background:var(--bg);color:var(--ink);font:14px/1.65 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB",sans-serif}
.app{display:grid;grid-template-columns:280px minmax(360px,1fr) 430px;height:100vh;overflow:hidden}
.pane{overflow:auto;min-width:0}
.tree-pane{background:#fbfaf8;border-right:1px solid var(--line);padding:20px 14px 32px}
.detail-pane{background:var(--card);padding:30px 40px 56px}
.fsm-pane{background:#fbfaf8;border-left:1px solid var(--line);padding:24px 22px 40px}
.brand{font-weight:700;font-size:16px;letter-spacing:-.01em;margin:0 0 2px}
.sub{color:var(--muted);font-size:12px;margin:0 0 16px}
.filter{width:100%;padding:8px 10px;border:1px solid var(--line);border-radius:8px;background:#fff;font:12px/1.4 inherit;margin:0 0 14px}
.tree{list-style:none;margin:0;padding:0}.tree-item{margin:1px 0}.tree-children{list-style:none;margin:0;padding:0;display:none}.tree-item.open>.tree-children{display:block}.tree-row{display:flex;align-items:center;gap:2px}.tree-toggle{width:18px;height:22px;border:0;background:transparent;cursor:pointer;color:#6b7280;padding:0;font-size:13px}.tree-toggle:before{content:"▸"}.tree-item.open>.tree-row .tree-toggle:before{content:"▾"}.tree-spacer{width:18px;flex:none}.tree-tools{display:flex;gap:6px;margin:0 0 8px}.tree-tools button{border:1px solid var(--line);background:#fff;border-radius:6px;padding:3px 7px;font-size:11px;color:#374151;cursor:pointer}.tree-tools button:hover{background:var(--accent-soft)}.status-legend{display:flex;flex-wrap:wrap;gap:4px;margin:0 0 10px}.status-badge{font-size:10px;line-height:1.5;border-radius:999px;padding:0 5px;border:1px solid var(--line);color:#6b7280;white-space:nowrap}.status-draft{color:#6b7280;background:#f3f4f6}.status-frozen{color:#2f6f4e;background:#f1f8f4;border-color:#b7d7c5}.status-implemented{color:#1d4ed8;background:#eff6ff;border-color:#bfdbfe}.status-verified{color:#166534;background:#dcfce7;border-color:#86efac}.status-change_pending{color:#9a5b13;background:#fffaf1;border-color:#efd4ad}.status-deprecated{color:#6b7280;background:#f3f4f6}.status-blocked{color:#991b1b;background:#fef2f2;border-color:#fecaca}
.node{display:flex;align-items:center;gap:6px;width:100%;border:0;background:transparent;text-align:left;color:var(--ink);padding:4px 7px;border-radius:7px;cursor:pointer;font:13px/1.5 inherit}
.node:hover{background:#efede8}.node.active{background:#e8f0eb;color:#17412c;font-weight:600}
.node .label{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.dot{width:6px;height:6px;border-radius:50%;flex:none;background:#c7c7c7}.dot.frozen{background:#4b8b68}.dot.verified{background:#2f6f4e}.dot.change{background:#c58a2a}
.fsm-mark{font-size:9px;line-height:1;border:1px solid #b7d7c5;color:var(--accent);border-radius:999px;padding:1px 4px;flex:none}
.tree-kicker{font-size:11px;color:var(--muted);margin:12px 0 4px;letter-spacing:.04em}
.crumb{font-size:12px;color:var(--muted);margin-bottom:10px}
h1{font-size:26px;line-height:1.25;margin:0 0 8px;letter-spacing:-.02em}
h2{font-size:15px;margin:26px 0 8px;color:#374151}
.badges{display:flex;flex-wrap:wrap;gap:6px;margin:0 0 14px}
.badge{font-size:12px;border-radius:999px;padding:1px 8px;border:1px solid var(--line);color:var(--muted);background:#fafafa;white-space:nowrap}
.badge.fsm{color:var(--accent);border-color:#b7d7c5;background:var(--accent-soft)}
.badge.warn{color:var(--warn);border-color:#efd4ad;background:#fffaf1}
.intent{font-size:16px;color:#374151;margin:0 0 6px}
ul.acs{margin:0;padding-left:20px}ul.acs li{margin:5px 0}
.prd{background:#fcfbf8;border:1px solid var(--line);border-radius:10px;padding:10px 14px;margin:14px 0}.prd h2{margin:2px 0 8px;font-size:16px}.prd h3{font-size:14px;margin:14px 0 4px}.prd p{margin:6px 0}.prd ul{margin:4px 0 8px;padding-left:20px}.prd table{border-collapse:collapse;width:100%;font-size:12px}.prd td,.prd th{border-bottom:1px solid var(--line);padding:4px 6px;text-align:left}.doc-missing{border:1px solid #efd4ad;background:#fffaf1;color:#8a5a12;border-radius:8px;padding:8px 10px;margin:12px 0;font-size:12px}.verify{background:#fafaf8;border:1px solid var(--line);border-radius:10px;padding:10px 12px;margin:8px 0;font-size:13px}
details.tech{margin-top:22px;color:var(--muted);font-size:12px}details.tech summary{cursor:pointer}
.tech dl{display:grid;grid-template-columns:90px 1fr;gap:3px 10px;margin:10px 0}.tech dt{color:#9ca3af}.tech dd{margin:0;word-break:break-all}
table{border-collapse:collapse;width:100%;margin:6px 0;font-size:12px}th,td{text-align:left;border-bottom:1px solid var(--line);padding:5px 6px;vertical-align:top}th{color:var(--muted);font-weight:500}td.ref{color:#4b5563;word-break:break-word}td:last-child{white-space:nowrap}
.fsm-head{font-size:12px;color:var(--muted);margin:0 0 6px}.fsm-title{font-size:17px;font-weight:650;margin:0 0 4px}
.fsm-wrap{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:10px;overflow:auto}
.fsm-wrap svg{display:block;width:100%;height:auto}
.fsm-canvas{position:relative;margin:0 auto}.fsm-canvas .edge-layer{position:absolute;inset:0;width:100%;height:100%;overflow:visible}
.state{position:absolute;width:100px;height:40px;padding:0 4px;line-height:1.1;text-align:center;overflow-wrap:anywhere;border-radius:9px;background:#f4f8f5;border:1.4px solid #c9d5ce;display:flex;align-items:center;justify-content:center;font-size:11.5px;font-weight:600;color:#1f2937;white-space:nowrap;box-sizing:border-box}.state.start{border-color:#2f6f4e}
.legend{display:flex;gap:14px;flex-wrap:wrap;font-size:11px;color:var(--muted);margin:10px 0 0}
.legend i{display:inline-block;width:22px;height:0;border-top:2px solid #6b7280;vertical-align:middle;margin-right:5px}.legend .bad i{border-top:2px dashed var(--danger)}
.illegal-line{font-size:11px;color:var(--danger);margin:8px 0 0}
.fsm-toggle{margin:8px 0 0;border:1px solid var(--line);background:#fff;border-radius:7px;padding:5px 10px;font-size:11px;font-family:inherit;line-height:1.4;color:#374151;cursor:pointer;min-width:96px;text-align:center}.fsm-toggle:hover{background:var(--accent-soft)}
.fsm-slot:not(.show-exceptions) .exceptions{display:none}
.fsm-list{display:flex;flex-direction:column;gap:6px;margin-top:10px}
.fsm-link{text-align:left;border:1px solid var(--line);background:#fff;border-radius:8px;padding:7px 9px;cursor:pointer;font:12px/1.4 inherit;color:var(--ink)}
.fsm-link:hover{border-color:#b7d7c5;background:var(--accent-soft)}
.bind-ac-row{cursor:pointer}
.bind-ac-row:hover td{background:#f1f8f4}
.bind-ac-row.selected td{background:#e8f0eb;color:#17412c;font-weight:600}
.bind-ac-form{margin:10px 0 4px;border:1px solid var(--line);border-radius:10px;padding:8px 10px;background:#fcfbf8;font-size:12px}
.bind-ac-form label{display:block;margin-top:8px}
.bind-ac-form select,.bind-ac-form button{display:block;max-width:100%;min-height:48px;padding:8px;font:inherit;font-size:1rem}
.bind-ac-hint{margin:4px 0;color:var(--muted)}
.hidden{display:none!important}
.empty{color:var(--muted);font-size:12px}
@media (max-width:1100px){.app{grid-template-columns:240px 1fr 360px}}
@media (max-width:860px){.app{display:block;height:auto}.pane{overflow:visible;max-height:none}.tree-pane,.fsm-pane{border:0;border-bottom:1px solid var(--line)}.detail-pane{padding:22px}}
`

func writeTreePane(b *strings.Builder, idx Index, children map[string][]IndexEntry, bodies map[string]Node, related, own map[string]string, rc RenderContext) {
	root := bodies[idx.RootID]
	counts := map[Status]int{}
	for _, e := range idx.Nodes {
		counts[e.Status]++
	}
	b.WriteString(`<aside class="pane tree-pane">`)
	fmt.Fprintf(b, `<div class="brand">%s</div><div class="sub">需求树 · %d 节点</div>`, esc(root.Title), len(idx.Nodes))
	requirement := rc.RequirementVersion
	if requirement == "" {
		requirement = idx.Version
	}
	snapshot := rc.SnapshotID
	if snapshot == "" {
		snapshot = "当前工作树（未绑定快照）"
	}
	runtime := rc.RuntimeVersion
	if runtime == "" {
		runtime = "unknown"
	}
	commit := rc.RuntimeCommit
	if commit != "" {
		commit = " · " + shortHash(commit)
	}
	mode := "HTML 派生只读"
	if rc.BindingURL != "" && rc.BindingToken != "" && rc.SnapshotID == "" {
		mode = "本地工作台 · 绑定写回 canonical；冻结节点只读"
	}
	fmt.Fprintf(b, `<div class="source-note"><b>权威源</b> tree/index.json · %s<br><b>需求线</b> %s · <b>快照</b> %s<br><b>运行时</b> %s%s<br>%s</div>`, esc(shortHash(SourceHash(idx))), esc(requirement), esc(snapshot), esc(runtime), esc(commit), mode)
	if rc.BackURL != "" || len(rc.History) > 0 {
		b.WriteString(`<details class="history"><summary>版本历史`)
		if len(rc.History) > 0 {
			fmt.Fprintf(b, `（%d）`, len(rc.History))
		}
		b.WriteString(`</summary>`)
		if rc.BackURL != "" {
			fmt.Fprintf(b, `<a class="history-link" href="%s">← 当前树</a>`, esc(rc.BackURL))
		}
		for _, h := range rc.History {
			fmt.Fprintf(b, `<a class="history-link" href="%s">%s</a>`, esc(h.URL), esc(h.Label))
		}
		if len(rc.Changes) > 0 {
			b.WriteString(`<div class="history-kicker">变更记录</div>`)
			for _, h := range rc.Changes {
				fmt.Fprintf(b, `<span class="history-change">%s · %s</span>`, esc(h.CreatedAt), esc(h.Label))
			}
		}
		b.WriteString(`</details>`)
	}
	b.WriteString(`<input id="filter" class="filter" placeholder="搜索节点、AC、状态" autocomplete="off">`)
	b.WriteString(`<div class="status-legend"><button type="button" class="status-filter active" data-status-filter="">全部 <b>` + fmt.Sprint(len(idx.Nodes)) + `</b></button>`)
	for _, st := range []Status{StatusDraft, StatusFrozen, StatusImplemented, StatusVerified, StatusDelivered, StatusChangePending} {
		fmt.Fprintf(b, `<button type="button" class="status-filter status-badge status-%s" data-status-filter="%s">%s <b>%d</b></button>`, esc(string(st)), esc(string(st)), esc(statusLabel(st)), counts[st])
	}
	b.WriteString(`</div>`)
	b.WriteString(`<div class="tree-tools"><button type="button" id="tree-expand">全部展开</button><button type="button" id="tree-collapse">全部收起</button></div>`)
	b.WriteString(`<ul class="tree">`)
	writeTreeNode(b, idx.RootID, children, bodies, related, own)
	b.WriteString(`</ul>`)
	var constraints []Node
	for _, e := range idx.Nodes {
		if bodies[e.ID].Constraint {
			constraints = append(constraints, bodies[e.ID])
		}
	}
	if len(constraints) > 0 {
		sort.Slice(constraints, func(i, j int) bool { return constraints[i].Path < constraints[j].Path })
		fmt.Fprintf(b, `<details class="constraints"><summary>质量约束（%d）</summary>`, len(constraints))
		for _, n := range constraints {
			fmt.Fprintf(b, `<button type="button" class="constraint-link" data-select="%s">%s</button>`, esc(n.ID), esc(n.Title))
		}
		b.WriteString(`</details>`)
	}
	b.WriteString(`</aside>`)
}

func writeTreeNode(b *strings.Builder, id string, children map[string][]IndexEntry, bodies map[string]Node, related, own map[string]string) {
	n := bodies[id]
	if n.Constraint {
		return
	}
	owner := related[id]
	fsm := ""
	if own[id] != "" {
		fsm = `<span class="fsm-mark">FSM</span>`
	}
	kids := children[id]
	open := ""
	if n.Kind == KindIntent || n.Kind == KindModule {
		open = " open"
	}
	status := string(n.Status)
	if status == "" {
		status = "draft"
	}
	search := esc(strings.ToLower(n.Title + " " + n.Intent + " " + string(n.Kind) + " " + status))
	fmt.Fprintf(b, `<li class="tree-item%s" data-tree-id="%s"><div class="tree-row">`, open, esc(id))
	if len(kids) > 0 {
		fmt.Fprintf(b, `<button type="button" class="tree-toggle" data-toggle="%s" aria-label="展开或收起"></button>`, esc(id))
	} else {
		b.WriteString(`<span class="tree-spacer"></span>`)
	}
	fmt.Fprintf(b, `<button class="node" data-node="%s" data-fsm="%s" data-own="%s" data-kind="%s" data-status="%s" data-search="%s"><span class="dot %s"></span><span class="label">%s</span><span class="status-badge status-%s">%s</span>%s</button></div>`,
		esc(id), esc(owner), boolStr(own[id] != ""), string(n.Kind), esc(status), search, status, esc(n.Title), status, esc(statusLabel(n.Status)), fsm)
	if len(kids) > 0 {
		b.WriteString(`<ul class="tree-children">`)
		for _, c := range kids {
			writeTreeNode(b, c.ID, children, bodies, related, own)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`</li>`)
}

func writeDetailPane(b *strings.Builder, idx Index, children map[string][]IndexEntry, bodies map[string]Node, truth map[string]TruthLinkStatus, rc RenderContext) {
	b.WriteString(`<main class="pane detail-pane">`)
	var walk func(id string)
	walk = func(id string) {
		writeNodePanel(b, bodies[id], idx.RootID, truth, rc)
		for _, c := range children[id] {
			walk(c.ID)
		}
	}
	walk(idx.RootID)
	b.WriteString(`</main>`)
}

func renderMarkdown(s string) string {
	var b strings.Builder
	inList := false
	closeList := func() {
		if inList {
			b.WriteString("</ul>")
			inList = false
		}
	}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			closeList()
			continue
		}
		switch {
		case strings.HasPrefix(line, "### "):
			closeList()
			fmt.Fprintf(&b, "<h3>%s</h3>", esc(strings.TrimSpace(line[4:])))
		case strings.HasPrefix(line, "## "):
			closeList()
			fmt.Fprintf(&b, "<h3>%s</h3>", esc(strings.TrimSpace(line[3:])))
		case strings.HasPrefix(line, "# "):
			closeList()
			fmt.Fprintf(&b, "<h2>%s</h2>", esc(strings.TrimSpace(line[2:])))
		case strings.HasPrefix(line, "- "):
			if !inList {
				b.WriteString("<ul>")
				inList = true
			}
			fmt.Fprintf(&b, "<li>%s</li>", esc(strings.TrimSpace(line[2:])))
		case strings.HasPrefix(line, "|"):
			closeList()
			fmt.Fprintf(&b, "<p class=\"md-table\">%s</p>", esc(line))
		default:
			closeList()
			fmt.Fprintf(&b, "<p>%s</p>", esc(line))
		}
	}
	closeList()
	return b.String()
}

func writeFSMPane(b *strings.Builder, idx Index, children map[string][]IndexEntry, bodies map[string]Node, own map[string]string) {
	b.WriteString(`<aside class="pane fsm-pane"><div class="fsm-head" id="fsm-head">业务状态机</div><div id="fsm-title" class="fsm-title"></div>`)
	for _, e := range orderedFSMNodes(idx, bodies) {
		n := bodies[e.ID]
		fmt.Fprintf(b, `<div class="fsm-slot hidden" data-owner="%s" data-owner-title="%s">`, esc(n.ID), esc(n.Title))
		fmt.Fprintf(b, `<div class="fsm-wrap">%s</div>`, renderFSMSVG(n.DomainFSM, n.ID))
		b.WriteString(`<details class="transitions"><summary>转移明细</summary><table><tr><th>从</th><th>到</th><th>事件</th><th>类型</th></tr>`)
		for _, tr := range n.DomainFSM.Transitions {
			fmt.Fprintf(b, `<tr><td>%s</td><td>%s</td><td>%s</td><td>正向</td></tr>`, esc(tr.From), esc(tr.To), esc(tr.Event))
		}
		for _, tr := range n.DomainFSM.Illegal {
			fmt.Fprintf(b, `<tr><td>%s</td><td>%s</td><td>%s</td><td>禁止</td></tr>`, esc(tr.From), esc(tr.To), esc(tr.Event))
		}
		b.WriteString(`</table></details><button class="fsm-toggle" type="button">显示异常转移</button><div class="legend"><span><i></i>正向转移</span><span class="bad"><i></i>禁止转移</span></div>`)
		if len(n.DomainFSM.Illegal) > 0 {
			fmt.Fprintf(b, `<div class="illegal-line">禁止：%s</div>`, esc(joinIllegal(n.DomainFSM.Illegal)))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div id="fsm-empty" class="hidden"><p class="empty">该节点不承载状态机；选择带 FSM 标记的功能查看。</p><div class="fsm-list">`)
	for _, e := range orderedFSMNodes(idx, bodies) {
		n := bodies[e.ID]
		fmt.Fprintf(b, `<button class="fsm-link" data-select="%s">%s <span class="fsm-mark">FSM</span></button>`, esc(n.ID), esc(n.Title))
	}
	b.WriteString(`</div></div></aside>`)
}

func writeScript(b *strings.Builder, idx Index) {
	fmt.Fprintf(b, `<script>
const panels=[...document.querySelectorAll('[data-panel]')];
const buttons=[...document.querySelectorAll('.node')];
const byId=Object.fromEntries(buttons.map(b=>[b.dataset.node,b]));
const slots=[...document.querySelectorAll('.fsm-slot')];
const head=document.getElementById('fsm-head'), title=document.getElementById('fsm-title');
const empty=document.getElementById('fsm-empty');
function select(id){
 const btn=byId[id]; if(!btn)return;
 buttons.forEach(b=>b.classList.toggle('active',b===btn));
 panels.forEach(p=>p.classList.toggle('hidden',p.dataset.panel!==id));
 const owner=btn.dataset.fsm; const own=btn.dataset.own==='1';
 slots.forEach(s=>s.classList.toggle('hidden',s.dataset.owner!==owner));
 empty.classList.toggle('hidden',!!owner);
 head.textContent=own?'当前节点状态机':(btn.dataset.kind==='touchpoint'?'所属功能状态机':'相关状态机');
 title.textContent=owner?(document.querySelector('.fsm-slot[data-owner="'+owner+'"]')?.dataset.ownerTitle||''):'';
 if(location.hash.slice(1)!==id)history.replaceState(null,'','#'+id);
}
buttons.forEach(b=>b.addEventListener('click',()=>select(b.dataset.node)));
document.querySelectorAll('[data-select]').forEach(b=>b.addEventListener('click',()=>select(b.dataset.select)));
document.querySelectorAll('.fsm-toggle').forEach(b=>b.addEventListener('click',()=>{const slot=b.closest('.fsm-slot');slot.classList.toggle('show-exceptions');b.textContent=slot.classList.contains('show-exceptions')?'隐藏异常转移':'显示异常转移';}));
const bindForms=[...document.querySelectorAll('[data-bind-form]')];
bindForms.forEach(form=>{
 const cases=form.elements.namedItem('test_case_id'), ac=form.elements.namedItem('ac_id');
 const panel=form.closest('[data-panel]');
 cases.addEventListener('change',()=>{const row=[...panel.querySelectorAll('.bind-ac-row')].find(row=>row.dataset.bindTestcase===cases.value);ac.value=row?.dataset.bindAc||'';});
 form.addEventListener('submit',async event=>{
  event.preventDefault();
  if(form.dataset.saving==='1'||!form.reportValidity())return;
  const button=form.querySelector('button[type="submit"]'), result=form.querySelector('[data-bind-result]');
  form.dataset.saving='1';button.disabled=true;result.textContent='正在保存并回读…';
  try{
   const response=await fetch(form.action,{method:'POST',headers:{'Accept':'application/json'},body:new URLSearchParams(new FormData(form)),credentials:'same-origin'});
   if(!response.ok)throw new Error(await response.text());
   const node=await response.json();
   form.elements.namedItem('base_fingerprint').value=node.fingerprint;
   node.test_cases.forEach(tc=>{const row=[...panel.querySelectorAll('.bind-ac-row')].find(row=>row.dataset.bindTestcase===tc.id);if(row){row.dataset.bindAc=tc.ac_id;row.cells[0].textContent=node.acs.find(item=>item.id===tc.ac_id)?.text||tc.ac_id;}});
   result.textContent='已保存绑定，并从 canonical 回读确认。';
  }catch(error){result.textContent='保存未确认：'+error.message+'；请刷新后核对。';}
  finally{form.dataset.saving='0';button.disabled=false;}
 });
});
const filter=document.getElementById('filter');
let activeStatus='';
const statusButtons=[...document.querySelectorAll('[data-status-filter]')];
statusButtons.forEach(b=>b.addEventListener('click',()=>{activeStatus=b.dataset.statusFilter||'';statusButtons.forEach(x=>x.classList.toggle('active',x===b));applyFilter();}));
const TREE_KEY='beacon-tree-open-v1';
function treeItems(){return [...document.querySelectorAll('.tree-item')];}
function saveTree(){try{localStorage.setItem(TREE_KEY,JSON.stringify(treeItems().filter(i=>i.classList.contains('open')).map(i=>i.dataset.treeId)));}catch(e){}}
function restoreTree(){let saved=null;try{saved=JSON.parse(localStorage.getItem(TREE_KEY)||'null');}catch(e){}if(!saved)return;const set=new Set(saved);treeItems().forEach(i=>i.classList.toggle('open',set.has(i.dataset.treeId)));}
document.querySelectorAll('.tree-toggle').forEach(t=>t.addEventListener('click',()=>{const li=t.closest('.tree-item');li.classList.toggle('open');saveTree();}));
const expand=document.getElementById('tree-expand'), collapse=document.getElementById('tree-collapse');
if(expand){expand.addEventListener('click',()=>{treeItems().forEach(i=>i.classList.add('open'));saveTree();});}
if(collapse){collapse.addEventListener('click',()=>{treeItems().forEach(i=>i.classList.remove('open'));saveTree();});}
function ownMatch(li,text,status){const b=li.querySelector(':scope > .tree-row .node');if(!b)return false;if(status&&b.dataset.status!==status)return false;return !text||(b.dataset.search||'').includes(text);}
function applyFilter(){const raw=(filter&&filter.value.trim().toLowerCase())||'';let status=activeStatus,text=raw;if(raw.startsWith('status:')){status=raw.slice(7).trim();text='';}function walk(li){const kids=[...li.querySelectorAll(':scope > .tree-children > .tree-item')];let child=false;kids.forEach(k=>{if(walk(k))child=true;});const own=ownMatch(li,text,status);const show=own||child;li.classList.toggle('hidden',!show);if(child)li.classList.add('open');return show;}treeItems().filter(i=>!i.parentElement.closest('.tree-item')).forEach(walk);}
if(filter){filter.addEventListener('input',applyFilter);}
restoreTree();
select(location.hash.slice(1)||'%s');
</script>`, esc(idx.RootID))
}

func orderedFSMNodes(idx Index, bodies map[string]Node) []IndexEntry {
	var out []IndexEntry
	for _, e := range idx.Nodes {
		if n, ok := bodies[e.ID]; ok && n.DomainFSM != nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
