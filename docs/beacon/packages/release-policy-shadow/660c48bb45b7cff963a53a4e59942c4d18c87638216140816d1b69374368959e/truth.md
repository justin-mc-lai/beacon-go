---
slug: release-policy-shadow
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
  truth: docs/beacon/v2.0.0/features/release-policy-shadow/truth.md
  tests: docs/beacon/v2.0.0/features/release-policy-shadow/tests.md
  tasks: docs/beacon/v2.0.0/features/release-policy-shadow/tasks.md
  evidence: docs/beacon/v2.0.0/features/release-policy-shadow/evidence.md
---

# Requirement Truth: release-policy-shadow

## 人话

给 Beacon 内核的使用者：release 仍然必须人点头，但先加一个「影子判定」——如果将来允许自动放行，这次会不会通过、会被哪条卡住。影子只记录判断，不改放行规则；`auto` 真放行本包明确不做。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确认按照推荐方案做……确保稳定交付"
goal: "先积累自动放行的判断数据，但不降低 release 的人工门强度"
non_goals: "不实现 auto 真放行；不改 verifier 独立性；不自动发布到外部系统"
constraints: "默认 human；影子判定不写 ReleaseHumanConfirmed；复用既有四门与证据新鲜度判定"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 影子判定放在哪一层？ | 新增 `goal release-shadow`，复用既有 release facts，只记录事件 | answered | 采用独立命令，不污染正式 gate verdict |
| Q2 | 是否允许 `auto` 值？ | 不允许：显式拒绝并说明未启用 | answered | 只接受 `human` / `auto_shadow` |
| Q3 | 影子通过后 `goal complete` 是否需要人？ | 仍然需要 | answered | 影子不写确认标志，人工门不变 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q2 的答案 | internal/orchestrator/engine.go:CreateRunOptions |
| AC-002 | Q1 的答案 | internal/orchestrator/gates_facts.go:evalRelease |
| AC-003 | Q3 的答案 | internal/orchestrator/complete.go:106-118 |
| AC-004 | 目标「积累判断数据」 | internal/orchestrator/meta.go:runMeta |
| AC-005 | 非目标「不做 auto」 | internal/gate/evaluate.go:EvaluateRelease |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | run 级 `release_policy` 接受 `human`（默认）/ `auto_shadow`；`auto` 与未知值被拒绝且 reason code 稳定 |
| AC-002 | `goal release-shadow --run-id` 产出「若自动放行会通过 / 被哪条卡住」的判定并写事件；不写 `ReleaseHumanConfirmed` |
| AC-003 | `auto_shadow` 下 `goal complete` 仍要求人工确认（人工门强度不变） |
| AC-004 | `goal status` 暴露 `release_policy` 与最近一次影子判定 |
| AC-005 | 不实现 `auto`；错误文案明确说明「auto 尚未启用」 |

## Boundary

- 影子判定是只读判断 + 事件记录，不是正式 gate verdict。
- 不新增顶层命令；能力落在 `beacon goal` 下。
