package intenttree

import (
	"fmt"
	"strings"
)

// PlannedTestCases derives reviewable test skeletons from the node's ACs
// and Domain FSM. It never claims these tests have run: every case is
// status=planned and its ref names the intended check.
func PlannedTestCases(n Node) []TestCase {
	seen := map[string]bool{}
	for _, tc := range n.TestCases {
		seen[tc.ID] = true
	}
	acs := n.ACs
	if len(acs) == 0 {
		return nil
	}
	var out []TestCase
	add := func(id, acID, kind, ref string) {
		if seen[id] {
			return
		}
		seen[id] = true
		out = append(out, TestCase{ID: id, ACID: acID, Kind: kind, Ref: ref, Status: "planned"})
	}
	acFor := func(i int) string {
		if len(acs) == 0 {
			return ""
		}
		return acs[i%len(acs)].ID
	}
	for i, ac := range acs {
		add(fmt.Sprintf("%s-tc-ac-%d", n.ID, i+1), ac.ID, "unit",
			"planned: exercise "+ac.ID+": "+strings.TrimSpace(ac.Text))
	}
	if n.DomainFSM != nil {
		for i, tr := range n.DomainFSM.Transitions {
			add(fmt.Sprintf("%s-tc-fsm-ok-%d", n.ID, i+1), acFor(i), "unit",
				fmt.Sprintf("planned: %s -> %s (%s) is allowed", tr.From, tr.To, tr.Event))
		}
		for i, tr := range n.DomainFSM.Illegal {
			add(fmt.Sprintf("%s-tc-fsm-bad-%d", n.ID, i+1), acFor(i), "unit",
				fmt.Sprintf("planned: %s -> %s is rejected", tr.From, tr.To))
		}
	}
	return out
}

// GenerateDoc renders a deterministic requirement-document skeleton from
// the node's intent, ACs and FSM. Semantic prose is the host agent's job;
// this function only fills the stable sections and evidence links.
func GenerateDoc(n Node) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", n.Title)
	if n.Intent != "" {
		fmt.Fprintf(&b, "## 意图\n\n%s\n\n", n.Intent)
	}
	b.WriteString("## 问题\n\n- [待补充：这个需求要解决的用户问题]\n\n")
	b.WriteString("## 目标用户\n\n- [待补充：谁在什么场景下使用]\n\n")
	b.WriteString("## 场景\n\n- [待补充：关键用户场景]\n\n")
	b.WriteString("## 范围\n\n- [待补充：本次做什么]\n\n")
	b.WriteString("## 非目标\n\n- [待补充：本次明确不做什么]\n\n")
	b.WriteString("## 用户流程\n\n1. [待补充：用户主动作]\n2. [待补充：系统响应]\n\n")
	b.WriteString("## 异常与边界\n\n- [待补充：失败、重试、取消或超时]\n")
	b.WriteString("\n## 验收标准\n\n- [待补充：用户可观察的结果]\n")
	if n.DomainFSM != nil {
		b.WriteString("\n## 状态迁移\n\n| From | To | Event |\n|---|---|---|\n")
		for _, tr := range n.DomainFSM.Transitions {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", tr.From, tr.To, tr.Event)
		}
		if len(n.DomainFSM.Illegal) > 0 {
			b.WriteString("\n### 非法迁移\n\n")
			for _, tr := range n.DomainFSM.Illegal {
				fmt.Fprintf(&b, "- %s -> %s 必须被拒绝\n", tr.From, tr.To)
			}
		}
	}
	if len(n.ACs) > 0 {
		b.WriteString("\n## 验收标准\n\n")
		for _, ac := range n.ACs {
			fmt.Fprintf(&b, "- **%s**：%s\n", ac.ID, ac.Text)
		}
	}
	if n.Verification != nil {
		fmt.Fprintf(&b, "\n## 验证方式\n\n%s\n", n.Verification.Spec)
	}
	b.WriteString("\n> 生成骨架：语义正文与真实测试由宿主 agent 补全；不得把占位符当作已验证结果。\n")
	return b.String()
}

// GeneratedReport is the dry-run/apply result for generate commands.
type GeneratedReport struct {
	NodeID    string     `json:"node_id"`
	Applied   bool       `json:"applied"`
	TestCases []TestCase `json:"test_cases,omitempty"`
	Doc       string     `json:"doc,omitempty"`
	Added     int        `json:"added,omitempty"`
}
