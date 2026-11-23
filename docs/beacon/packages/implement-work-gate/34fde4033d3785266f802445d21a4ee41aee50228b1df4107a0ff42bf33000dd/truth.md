---
slug: implement-work-gate
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/implement-work-gate/truth.md
  tests: docs/beacon/v2.0.0/features/implement-work-gate/tests.md
  tasks: docs/beacon/v2.0.0/features/implement-work-gate/tasks.md
  evidence: docs/beacon/v2.0.0/features/implement-work-gate/evidence.md
---

# Requirement Truth: implement-work-gate

## 人话

`implement` 阶段的出口只看准入（真相冻结了没有、admission 通没通过），
从不看"这一阶段到底有没有产出"。于是一个什么都不做的执行器可以一路推到 release，
报告全绿——实际上仓库一行没变。truth 和 qa 都有硬事实，只有 implement 这一格是空的。

这个湖给 implement 补一条硬事实：**阶段结束时，工作树必须能证明动过**。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "implement 阶段空实施也能通过（上游 issue #4）"
goal: "implement 出口要有工作证据；没有就拒绝"
non_goals: "不判质量；不跑测试；不改别的门"
constraints: "Go；只读 git；非 git 不判红"
scope_mode: "lake"
```

## 判定规则

| 情况 | 判定 |
|------|------|
| 工作树 HEAD 相对基线已移动、工作树存在未提交改动 | 通过 |
| HEAD 未动且工作树干净 | 拒绝，reason=implement_no_work |
| 运行显式声明 `--allow-empty` | 检查项 not_applicable，reason=implement_empty_allowed |
| 工作目录不存在、非 git、git 读取失败 | not_applicable，reason=implement_work_unverified（不判红，也不判绿） |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | implement 阶段出口评估新增检查项 `implement_work_recorded`：工作树 HEAD 相对运行记录的基线提交已移动、工作树存在未提交改动时判 pass |
| AC-002 | HEAD 未移动且工作树干净时判 fail，reason_code=implement_no_work，implement 门整体 blocked |
| AC-003 | 运行带有 `implement_empty_allowed` 生命周期事件时该检查判 not_applicable，reason_code=implement_empty_allowed，其余准入项与门判定不受影响 |
| AC-004 | 工作目录不存在、不是 git 仓库、git 命令失败时检查判 not_applicable 并记 reason_code=implement_work_unverified；不得因此判 blocked，也不得判 pass 冒充已验证 |
| AC-005 | 新检查只在 implement 阶段出现；truth_freeze、qa、release 的检查项集合与判定不变；既有测试全绿 |
| AC-006 | 检查为只读：不写工作树、不创建提交、不修改运行状态；同一运行重复评估得到相同结论 |

## 配置项

| 键 | 默认 | 作用 |
|----|------|------|
| `implement_work_gate` | `on` | 总开关；`off` 时检查项 not_applicable，reason=implement_work_gate_off |

## Non-goals（冻结边界）

- 不判断改动是否符合需求（那是 qa 的事）
- 不检查提交信息、不强制提交
- 不把"读不到工作树"当成失败

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user utterance | truth.md#User-Intent-Snapshot |
| AC-002 | user utterance | truth.md#User-Intent-Snapshot |
| AC-003 | user utterance | truth.md#User-Intent-Snapshot |
| AC-004 | user utterance | truth.md#User-Intent-Snapshot |
| AC-005 | user utterance | truth.md#User-Intent-Snapshot |
| AC-006 | user utterance | truth.md#User-Intent-Snapshot |
