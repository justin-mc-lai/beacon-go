---
slug: truth-binding-requirement-level-drift
version: v0.0.1
status: frozen
package_maturity: complete
source: tree
tree_node_id: n_679cb763d41a
parser_contract: beacon-feature-package-v2
truth_source_model: tree_derived
---
## 问题
意图树节点带有指纹，但缺少按 added/changed/removed 分级的需求漂移报告，无法直接回答"相对上次冻结，哪些需求正文变了"。报告只需覆盖需求正文，治理规则不属于需求漂移，不应混入。

## 目标用户
需要在提交或 refreeze 前查看需求级差异的开发者，以及审查树变更的 gate 使用方。

## 场景
改动某个节点正文后运行漂移检查，该节点被列为 changed，未改动节点不出现；新增或删除节点分别落入 added 与 removed 数组。报告只读，以 JSON 输出三个数组便于脚本消费。

## 范围
本包交付节点指纹级 added/changed/removed 需求漂移报告的定义，只覆盖需求正文。现有落点：internal/intenttree/disk.go:87 与 internal/intenttree/snapshot.go:181 对指纹不一致 fail-closed，internal/orchestrator/branch.go 的 ACDiff 记录 refreeze 的 AC 差异；但针对 plan 命令的 added/changed/removed 专用报告当前未实现，本包冻结的是需求定义，不是实现状态。

## 非目标
不把治理规则变更纳入需求漂移报告，这与 agents-md-sync-guard 的边界一致；不自动修复漂移节点，只输出报告。不负责版本一致性校验（runtime-version-source）与发布时 manifest 对齐（dual-axis-alignment）。

## 验收标准
改动节点正文后该节点在报告中显示为 changed，未改动节点不出现，并输出 added/changed/removed 三个数组。验收标准为 `go test ./internal/intenttree -run TestDrift -count=1 通过且命令 beacon plan drift --node <id> --json 输出 added/changed/removed 数组`：internal/intenttree 中当前没有 TestDrift，且 beacon plan drift 子命令当前尚不存在，均属待实现能力。
