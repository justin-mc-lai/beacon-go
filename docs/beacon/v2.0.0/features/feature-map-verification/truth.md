---
slug: feature-map-verification
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
  truth: docs/beacon/v2.0.0/features/feature-map-verification/truth.md
  tests: docs/beacon/v2.0.0/features/feature-map-verification/tests.md
  tasks: docs/beacon/v2.0.0/features/feature-map-verification/tasks.md
  evidence: docs/beacon/v2.0.0/features/feature-map-verification/evidence.md
---

# Requirement Truth: feature-map-verification

## 人话

给 Beacon 内核的使用者：agent 拿到一个 bug 描述时，现在只能靠猜去哪里改、跑什么命令验证。本包把「每个 AC 住在哪个文件、用哪条命令证明它没坏」做成一张可生成的 `feature-map.md`；地图是需求包的单向投影，不改 truth，测试命令只能来自 tests.md，过期会被 verify 发现。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确保beacon-go 在需求真相跟qa验证足够稳定可靠"
goal: "让验证从每次重写脚本变成可复用、可过期检测的特征地图"
non_goals: "不自动修复 bug；不发明 tests.md 里没有的命令；不改 truth 正文"
constraints: "entry point 只来自 tasks.md 的 evidence=；验证命令只来自 tests.md；地图是单向投影"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 入口点从哪来？ | 只用 tasks.md 的 `evidence=` 文件路径，不猜 | answered | 采用；`.beacon/` 与 `docs/` 证据不算入口 |
| Q2 | 验证命令从哪来？ | 只取 tests.md 的 Command 列 | answered | 采用；缺失即 `missing_verification` |
| Q3 | 地图过期怎么处理？ | verify 报 `feature_map_stale` 并 blocked | answered | 采用；缺失只是 info |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「可复用的验证地图」 | internal/orchestrator/feature_map.go |
| AC-002 | 非目标「不改 truth」 | internal/orchestrator/feature_map.go:BuildFeatureMap |
| AC-003 | Q2 的答案 | internal/gate/integrity.go:ParseCoverageRows |
| AC-004 | Q3 的答案 | internal/orchestrator/truthmap.go:TruthMapVerify |
| AC-005 | Q1 的答案 | internal/orchestrator/feature_map.go:parseTaskBindings |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `beacon truth map build` 为每个 AC 写一行，含入口点与可执行验证命令，落到 `feature-map.md` |
| AC-002 | 生成前后 truth/tests/tasks/evidence 四件套字节不变 |
| AC-003 | 验证命令只来自 tests.md 的 Command 列；缺命令标 `missing_verification`，不凭空造命令 |
| AC-004 | `truth map verify` 输出 `feature_map_status`（fresh/stale/missing）；stale 时 verdict 为 blocked |
| AC-005 | 入口点只来自 tasks.md 的 `evidence=` 文件路径；缺入口标 `missing_entry` |

## Boundary

- 地图是只读投影，不参与 gate 判定（除了 verify 的过期诊断）。
- 不新增顶层命令；能力落在 `beacon truth map` 下。
