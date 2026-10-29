---
slug: agent-desktop-workspace
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/agent-desktop-workspace/truth.md
  tests: docs/beacon/v2.0.0/features/agent-desktop-workspace/tests.md
  tasks: docs/beacon/v2.0.0/features/agent-desktop-workspace/tasks.md
  evidence: docs/beacon/v2.0.0/features/agent-desktop-workspace/evidence.md
---

# Requirement Truth: agent-desktop-workspace

## 人话

给本机开发者：一个原生桌面工作台，左栏是作战室，中栏是交互与步骤计划，右栏是需求真相与证据；数据全部来自真实账本，没登记就显示为空而不是编造。界面只有一种形态（Wails），不再有浏览器副本。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "要一个桌面工作台，能看到真实 feature、truth 和执行步骤"
goal: "桌面单壳渲染三栏，数据来自账本与磁盘事实，操作能真正创建 run 并返回门禁裁决"
non_goals: "不做浏览器副本；不臆造未登记的 feature；不做第二套后端"
constraints: "单一渲染路径；未登记项目显示空；发布按钮必须返回真实裁决"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 给本机开发者：桌面端展示真实需求包与门禁状态并能创建 run；完成=三栏可见、数据来自账本、操作有真实返回值 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「桌面三栏」 | `TestShellScriptParses` |
| AC-002 | 目标「真实账本数据流」 | `TestUnregisteredProjectIsEmptyNotInvented` |
| AC-003 | 目标「Wails 工程化」 | `TestWailsConfigValid` |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 单壳渲染三栏工作台：作战室/组织、交互与步骤计划、需求真相与证据面板 |
| AC-002 | 数据来自账本：feature 列表与详情由 store 解析；未登记项目返回空集并标记 registered=false，不臆造内容 |
| AC-003 | Wails 工程可复现：wails.json 完整，`desktop` 入口可构建，后端能力以结构化绑定暴露 |

## FSM 五元组

Not required: domain_required is false.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Not required: scope_mode is focused.

| Capability | Covered | Evidence |
|------------|---------|----------|
