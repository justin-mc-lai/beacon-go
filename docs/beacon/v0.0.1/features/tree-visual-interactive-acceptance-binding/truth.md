---
slug: tree-visual-interactive-acceptance-binding
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_39e50c0e0da2
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
需求树 HTML 中 AC 与 TestCase 无交互绑定入口，无法通过点选完成绑定并回写树，绑定复杂度暴露给用户。

## 目标用户
审阅需求树的开发者，需在 HTML 中点选完成 AC↔TestCase 绑定。

## 场景
生成树 HTML 后，页面存在 data-bind-ac 属性标记的 TestCase 行，用户点击后绑定写回树。

## 范围
定义渲染产物必须包含 data-bind-ac 属性以支持点击绑定与回写。现有落点 internal/intenttree/render_html.go 为静态渲染，无交互绑定/回写，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不实现 gate verdict 展示（属 gate-verdict-panel）；不实现 FSM illegal slot 覆盖（属 fsm-illegal-coverage）；不改变树指纹/索引投影（属 self-tree-ci）。

## 验收标准
`go test ./internal/intenttree -run TestRenderHTML -count=1` 通过，且输出 HTML 中 `grep -q data-bind-ac` 命中。当前尚无交互绑定，属待实现能力。
