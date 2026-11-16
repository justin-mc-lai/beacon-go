---
slug: beacon-plan-pln-fsm-integration
surface_strict: true
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
surfaces:
  - cli
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/beacon-plan-pln-fsm-integration/truth.md
  tests: docs/beacon/v2.0.0/features/beacon-plan-pln-fsm-integration/tests.md
  tasks: docs/beacon/v2.0.0/features/beacon-plan-pln-fsm-integration/tasks.md
  evidence: docs/beacon/v2.0.0/features/beacon-plan-pln-fsm-integration/evidence.md
---

# Requirement Truth: beacon-plan-pln-fsm-integration

## 人话

在已有的 beacon-plan 技能中增加强制性 pln-fsm 模式，凡是涉及工作流、状态扭转、调度履约的需求，必须强制拆解实体前置序、聚合根转移矩阵与非法状态转移槽位，并将拆解产物与内核接口严格对齐。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "在原有的skill上补充即可，即beacon plan skill调用，强制新增fsm调用"
goal: "在现有 beacon-plan 技能中深度集成 pln-fsm 模式与 HARD GATE 强卡约束"
non_goals: "不破坏既有 brainstorm/review/source 模式；不向内核源码硬编码特定业务名称"
constraints: "保持 1+6 门禁严格单测；通过多客户端环境 verify 检验"
scope_mode: focused
```

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 模式列表扩充 | skills/plan/SKILL.md 声明 pln-fsm 模式 restates=1 |
| AC-002 | 硬门禁约束 | skills/plan/SKILL.md HARD GATE 规定工作流需求必选 pln-fsm restates=2 |
| AC-003 | 领域示范落地 | skills/plan/examples/domain-fsm-decomposition.md 提供跑腿拆解范例 restates=3 |
| AC-004 | 技能编译与同步 | beacon-go 内嵌单测全绿，完成 host 技能环境 install 与 verify restates=4 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 在 `skills/plan/SKILL.md` 与 `references/core.md` 中新增 `pln-fsm` 模式并映射至 `beacon truth fsm check` / `beacon fsm transition` CLI 接口 restates=1 |
| AC-002 | 在 `skills/plan/SKILL.md` 的 HARD GATE 中明确约束：凡涉及状态流转工作流需求，必须强制进行 FSM 实体序、状态矩阵与非法跃迁拆解 restates=2 |
| AC-003 | 新增 `skills/plan/examples/domain-fsm-decomposition.md` 范例，完整演示实体前置序、聚合根主表与非法转移槽位的规范写法 restates=3 |
| AC-004 | 运行 `go test ./skills/...` 全部通过，并通过 `beacon-go skills install --force` 刷新多端技能目录并通过 `beacon-go skills verify` 校验 restates=4 |
