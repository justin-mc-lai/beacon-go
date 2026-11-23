---
slug: artifact-upsert
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
  truth: docs/beacon/v2.0.0/features/artifact-upsert/truth.md
  tests: docs/beacon/v2.0.0/features/artifact-upsert/tests.md
  tasks: docs/beacon/v2.0.0/features/artifact-upsert/tasks.md
  evidence: docs/beacon/v2.0.0/features/artifact-upsert/evidence.md
---

# Requirement Truth: artifact-upsert

## 人话

同一个文件被注册两次，`artifact_refs` 就多一行。重跑一次阶段，文件内容变了，
旧那一行的 hash 立刻对不上，qa 门就永远判 `qa_artifact_stale`——除非有人手工去库里删行。

重跑阶段是正常操作，不该把门弄坏。所以：同一路径的注册应当是**更新**，不是追加；
校验时每个路径只看**最新**那一条，历史行不再参与判定。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "artifact 重复注册导致 qa 永远 stale（beacon-go#5）"
goal: "同路径注册改为就地更新；校验取最新记录"
non_goals: "不迁移数据；不改 schema；不改 kind 语义"
constraints: "Go；schema 不变；历史重复行无需人工清理"
scope_mode: "lake"
```

## 判定规则

| 情况 | 结果 |
|------|------|
| 同一 run 同一路径重复注册，内容未变 | 仍一行，hash/时间刷新 |
| 同一 run 同一路径重复注册，内容已变 | 仍一行，hash 变为新值，id 不变 |
| 校验时存在历史重复行 | 只取 created_at 最新的一行参与校验 |
| 最新记录的文件不在盘上或 hash 不符 | 仍判 stale |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `Artifacts().Add` 对同一 (run_id, abs_path) 执行就地更新：content_hash、size_bytes、created_at 刷新，行数不增加 |
| AC-002 | 就地更新保留原有 id，外部引用不失效 |
| AC-003 | qa 证据判定对同一路径只取 created_at 最新的一条记录，历史重复行不参与 stale 判定 |
| AC-004 | 存在历史重复行的库无需迁移即可通过 qa 判定（读取路径去重），且最新记录真实存在时判 pass |
| AC-005 | 同一路径重新注册后，qa 门不再出现 qa_artifact_stale（真机：注册→改文件→再注册→qa pass） |
| AC-006 | 既有测试全绿；schema 不新增列、不引入迁移 |

## Non-goals（冻结边界）

- 不删除历史数据（读取时去重即可）
- 不改 ListByRun 的返回排序
- 不改 artifact kind 语义

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
