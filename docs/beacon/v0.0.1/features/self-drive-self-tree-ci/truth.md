---
slug: self-drive-self-tree-ci
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_e11ba0e98451
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
CI 仅覆盖 build/test/version-consistency，未校验本仓意图树指纹、索引与 SQLite 投影一致，无法 dogfood 自己的树。

## 目标用户
beacon-go 维护者与 CI 流水线，需在每次变更后确认树一致性。

## 场景
CI 在 ubuntu runner 上执行 tree 重建与全量测试，期望 doctor rebuild-tree 退出 0 且 rebuilt 数大于 0，同时 go test ./... -count=1 全绿。

## 范围
定义 CI 必须执行的树一致性校验：doctor rebuild-tree --project . 退出 0 且 rebuilt 数等于索引节点数，并运行全量测试。现有落点 .github/workflows/ci.yml 仅含 build/test/version-consistency，无树指纹 CI，当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不实现 tree 指纹 diff/added/changed/removed 报告（属 requirement-level-drift）；不实现 AGENTS.md 漂移检测（属 agents-md-sync-guard）；不实现 gate verdict 可视化（属 gate-verdict-panel）。

## 验收标准
`beacon doctor rebuild-tree --project . --json | jq -e '.rebuilt > 0'` 退出 0；且 `go test ./... -count=1` 全绿。前者命令当前尚不存在，属待实现能力。
