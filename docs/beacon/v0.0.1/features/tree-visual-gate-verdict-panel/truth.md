---
slug: tree-visual-gate-verdict-panel
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_5a4abb5210d0
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
意图树 HTML 渲染为静态展示，无法可视化 freeze/verify/gate 结果与 reason code，节点验收判定不可读。

## 目标用户
审阅需求树的开发者与评审者，需点击节点读取验收判定。

## 场景
生成树 HTML 后，页面包含 gate 判定区与 reason code 文本，审阅者点击节点即可查看对应验收判定。

## 范围
定义渲染产物必须包含 gate 判定区与 reason code 文本（'gate-panel' 标记）。现有落点 internal/intenttree/render_html.go 为静态渲染，无 verdict panel，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不实现 AC↔TestCase 交互绑定与回写（属 interactive-acceptance-binding）；不实现 gate verdict 计算逻辑本身（已有 gate 包）；不新增后端 API。

## 验收标准
`go test ./internal/intenttree -run TestRenderHTML -count=1` 通过，且输出 HTML 中 `grep -q 'gate-panel'` 命中。当前尚无 gate-panel 渲染，属待实现能力。
