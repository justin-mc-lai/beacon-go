package intenttree

import (
	"fmt"
	"strings"

	"beacon-go/internal/core"
)

func writeNodePanel(b *strings.Builder, n Node, rootID string, truth map[string]TruthLinkStatus, rc RenderContext) {
	if n.ID == "" {
		return
	}
	hidden := ""
	if n.ID != rootID {
		hidden = " hidden"
	}
	fmt.Fprintf(b, `<section class="node-panel%s" data-panel="%s">`, hidden, esc(n.ID))
	kind := map[Kind]string{KindIntent: "项目意图", KindModule: "模块", KindFeature: "功能", KindTouchpoint: "触达点"}[n.Kind]
	if n.Constraint {
		kind = "质量约束"
	}
	fmt.Fprintf(b, `<div class="crumb">%s</div><h1>%s</h1><div class="badges"><span class="badge">%s</span>`, kind, esc(n.Title), esc(kind))
	if n.DomainFSM != nil {
		b.WriteString(`<span class="badge fsm">状态机</span>`)
	}
	if n.Constraint {
		b.WriteString(`<span class="badge warn">约束</span>`)
	}
	fmt.Fprintf(b, `<span class="badge">%s</span></div>`, esc(statusLabel(n.Status)))
	if n.Kind == KindFeature {
		if strings.TrimSpace(n.Doc) == "" {
			b.WriteString(`<div class="doc-missing"><b>需求正文缺失</b>：这是 legacy/工程节点；PRD 必须有 问题、目标用户、场景、范围、非目标、验收标准。</div>`)
		} else {
			b.WriteString(`<section class="prd"><h2>需求正文</h2>` + renderMarkdown(n.Doc) + `</section>`)
		}
	}
	if n.Intent != "" {
		fmt.Fprintf(b, `<p class="intent">%s</p>`, esc(n.Intent))
	}
	if n.EffectiveTruthPath() == "" && n.Kind == KindFeature {
		b.WriteString(`<div class="verify"><b>需求源：</b>树原生（无 legacy 包链）</div>`)
	}
	if n.EffectiveTruthPath() != "" {
		b.WriteString(`<div class="verify"><b>需求包（legacy）：</b>` + esc(n.EffectiveTruthPath()))
		if st, ok := truth[n.ID]; ok {
			if st.PackageStatus != "" {
				b.WriteString(` · 包状态：` + esc(st.PackageStatus))
			}
			b.WriteString(` · 节点：` + esc(string(n.Status)))
			if !st.Match || st.Error != "" {
				b.WriteString(` · <span style="color:#9a3412">状态不一致</span>`)
			}
			if st.Error != "" {
				b.WriteString(` · ` + esc(st.Error))
			}
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<section class="gate-panel"><h2>Gate 判定</h2><table><tr><th>Gate</th><th>判定</th><th>reason code</th></tr>`)
	verdicts := rc.GateVerdicts[n.ID]
	if len(verdicts) == 0 {
		fmt.Fprintf(b, `<tr><td>gate</td><td>未评估</td><td>%s</td></tr>`, core.ReasonGateNoResult)
	}
	for _, v := range verdicts {
		fmt.Fprintf(b, `<tr><td>%s</td><td>%s</td><td>%s</td></tr>`, esc(v.GateType), esc(v.Verdict), esc(strings.Join(v.ReasonCodes, ", ")))
	}
	b.WriteString(`</table></section>`)
	if len(n.ACs) > 0 {
		b.WriteString(`<h2>验收标准</h2><ul class="acs">`)
		for _, ac := range n.ACs {
			fmt.Fprintf(b, `<li>%s</li>`, esc(ac.Text))
		}
		b.WriteString(`</ul>`)
	}
	if n.Verification != nil || len(n.TestCases) > 0 {
		b.WriteString(`<details><summary><b>测试与验证</b></summary>`)
		if n.Verification != nil {
			fmt.Fprintf(b, `<div class="verify"><b>验证方式：</b>%s</div>`, esc(n.Verification.Spec))
		}
		if len(n.TestCases) > 0 {
			acText := map[string]string{}
			for _, ac := range n.ACs {
				acText[ac.ID] = ac.Text
			}
			b.WriteString(`<table class="bind-ac-table"><tr><th>验收标准</th><th>用例</th><th>状态</th></tr>`)
			for _, tc := range n.TestCases {
				text := acText[tc.ACID]
				if text == "" {
					text = tc.ACID
				}
				fmt.Fprintf(b, `<tr class="bind-ac-row" data-bind-ac="%s" data-bind-node="%s" data-bind-testcase="%s" data-bind-ref="%s" data-bind-kind="%s" data-bind-status="%s"><td>%s</td><td class="ref">%s</td><td><span class="badge">%s</span></td></tr>`,
					esc(tc.ACID), esc(n.ID), esc(tc.ID), esc(tc.Ref), esc(tc.Kind), esc(tc.Status), esc(text), esc(tc.Ref), esc(testStatusLabel(tc.Status)))
			}
			b.WriteString(`</table>`)
			if n.Status != StatusDraft && n.Status != StatusChangePending {
				b.WriteString(`<p class="bind-ac-hint">冻结合同只读；须先经授权变更流程后才能修改绑定。</p>`)
			} else if rc.BindingURL == "" || rc.BindingToken == "" || rc.SnapshotID != "" {
				b.WriteString(`<p class="bind-ac-hint">静态导出与历史快照只读；绑定操作请在本地工作台进行。</p>`)
			} else {
				fmt.Fprintf(b, `<form class="bind-ac-form" data-bind-form="%s" action="%s" method="post" aria-label="AC 与用例绑定"><input type="hidden" name="node" value="%s"><input type="hidden" name="base_fingerprint" value="%s"><input type="hidden" name="csrf_token" value="%s">`, esc(n.ID), esc(rc.BindingURL), esc(n.ID), esc(n.Fingerprint), esc(rc.BindingToken))
				fmt.Fprintf(b, `<label for="bind-tc-%s">选择用例</label><select id="bind-tc-%s" name="test_case_id" required><option value="">请选择用例</option>`, esc(n.ID), esc(n.ID))
				for _, tc := range n.TestCases {
					fmt.Fprintf(b, `<option value="%s">%s · %s</option>`, esc(tc.ID), esc(tc.ID), esc(tc.Ref))
				}
				fmt.Fprintf(b, `</select><label for="bind-ac-%s">绑定到验收标准</label><select id="bind-ac-%s" name="ac_id" required><option value="">请选择 AC</option>`, esc(n.ID), esc(n.ID))
				for _, ac := range n.ACs {
					fmt.Fprintf(b, `<option value="%s">%s · %s</option>`, esc(ac.ID), esc(ac.ID), esc(ac.Text))
				}
				b.WriteString(`</select><button type="submit">保存绑定</button><p role="status" aria-live="polite" data-bind-result></p></form>`)
			}
		}
		b.WriteString(`</details>`)
	}
	if revs := rc.NodeHistory[n.ID]; len(revs) > 0 {
		b.WriteString(`<details class="tech"><summary><b>历史版本</b></summary>`)
		for _, rev := range revs {
			fmt.Fprintf(b, `<a class="history-link" href="%s">%s · %s · %s</a>`, esc(rev.URL), esc(rev.Label), esc(rev.Status), esc(shortHash(rev.Fingerprint)))
		}
		b.WriteString(`</details>`)
	}
	b.WriteString(`<details class="tech"><summary>技术信息</summary><dl>`)
	fmt.Fprintf(b, `<dt>ID</dt><dd>%s</dd><dt>路径</dt><dd>%s</dd><dt>状态</dt><dd>%s</dd><dt>指纹</dt><dd>%s · %s</dd>`, esc(n.ID), esc(n.Path), esc(statusLabel(n.Status)), esc(n.FingerprintAlg), esc(n.Fingerprint))
	if n.VerificationEvidence != nil {
		fmt.Fprintf(b, `<dt>证据</dt><dd>%s</dd>`, esc(n.VerificationEvidence.ArtifactID))
	}
	b.WriteString(`</dl></details></section>`)
}
