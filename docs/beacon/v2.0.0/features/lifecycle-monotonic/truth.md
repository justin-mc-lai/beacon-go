---
slug: lifecycle-monotonic
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
  truth: docs/beacon/v2.0.0/features/lifecycle-monotonic/truth.md
  tests: docs/beacon/v2.0.0/features/lifecycle-monotonic/tests.md
  tasks: docs/beacon/v2.0.0/features/lifecycle-monotonic/tasks.md
  evidence: docs/beacon/v2.0.0/features/lifecycle-monotonic/evidence.md
---

# Requirement Truth: lifecycle-monotonic

## 人话

给 Beacon 内核的使用者：一个 feature 已经 Release/Completed 之后，如果又建一个新 run，或者完成后补文档再 refreeze，它的生命周期会被打回 `Prd/InProgress` 或 `Freeze/Completed`。本包把“完成”当成终态：后续的建 run、冻结、tick、阻塞只能升级，不能把已完成的 feature 降级。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "直接帮我现在修复下这两个账本工具，按照第一性原理分析"
goal: "生命周期对完成事实保持单调：Release/Completed 不被建 run / refreeze / tick 覆盖"
non_goals: "不改 truth 内容语义；不阻止显式 reopen 新修订；不改 gate 判定"
constraints: "一个集中守卫；所有生命周期写入复用；失败不静默"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 哪些动作不能降级？ | 建 run、freeze/refreeze、tick/block | answered | 用同一个守卫 |
| Q2 | 显式 reopen 要不要也拦住？ | 不拦：reopen 是明确的新修订意图 | answered | reopen 保留原行为 |
| Q3 | 守卫放哪一层？ | orchestrator 的一个 helper，接收 `*store.FeatureDao` | answered | 事务内外都能用 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q1 的答案 | internal/orchestrator/engine.go:CreateRun |
| AC-002 | Q1 的答案 | internal/orchestrator/truth.go:FreezeTruthPackage |
| AC-003 | Q1 的答案 | internal/orchestrator/tick.go |
| AC-004 | 非目标「不阻止 reopen」 | internal/orchestrator/truth_reopen.go |
| AC-005 | Q3 的答案 | internal/orchestrator/lifecycle.go |
| AC-006 | 目标「稳定」 | tests + e2e |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 对已 Release/Completed 的 feature 调 CreateRun，生命周期保持 Release/Completed |
| AC-002 | 对已 Release/Completed 的 feature 调 freeze 和 refreeze，生命周期保持 Release/Completed |
| AC-003 | 新 run 的 tick 和 block 不把已 Release/Completed 的 feature 降级 |
| AC-004 | 未完成的 feature 行为不变：建 run 仍进 InProgress，冻结仍进 Freeze/Completed |
| AC-005 | 只有一个集中守卫 `updateLifecycleMonotonic`，上述写入全部复用它 |
| AC-006 | 全仓 build/test/vet/e2e 通过 |

## Boundary

- 只保护 `Release/Completed` 终态；其他阶段之间的正常推进不变。
- 不放宽任何 gate；只修账本写入的单调性。
