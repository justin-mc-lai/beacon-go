---
slug: program-drive
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
  truth: docs/beacon/v2.0.0/features/program-drive/truth.md
  tests: docs/beacon/v2.0.0/features/program-drive/tests.md
  tasks: docs/beacon/v2.0.0/features/program-drive/tasks.md
  evidence: docs/beacon/v2.0.0/features/program-drive/evidence.md
---

# Requirement Truth: program-drive

## 人话

给 Beacon 内核的使用者：多湖需求今天只有「计划」没有「执行」，人得手工建 run、手工切湖、手工搬状态。本包新增 `goal program drive`，每次调用把一个就绪湖推到它的 release gate，停下等人确认；湖↔run 的绑定会记住，失败会记原因，依赖它的湖不会被偷偷推进。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "把上面的推荐全部走 /beacon-go-plan 第一性原理 /beacon-go-goal 冻结并实现"
goal: "多湖需求可以按拓扑顺序自动接力，但每湖的 release 仍由人确认"
non_goals: "不自动 complete；不并行执行多个湖；不新增 loop 顶层命令；不做跨机器调度"
constraints: "复用 ProgramRoute 的拓扑批次与既有 driver；湖↔run 绑定写进 program 快照；不削弱 release 人工门"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 一次 drive 推进几个湖？ | 最多一个，到 release gate 就返回 | answered | 采用；符合 issue #7 AC-1 |
| Q2 | 湖失败时依赖湖怎么办？ | 依赖湖保持 待命，不跳步 | answered | 采用；blocked 会写 reason_code |
| Q3 | 已完成的湖会不会重跑？ | 不会；completed 直接跳过 | answered | 采用；绑定与终态都持久化 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q1 的答案 | internal/orchestrator/program_drive.go:ProgramDrive |
| AC-002 | 目标「绑定要记住」 | internal/orchestrator/meta_p2d.go:ProgramLake.RunID |
| AC-003 | 非目标「不自动 complete」 | internal/orchestrator/program_drive.go |
| AC-004 | Q2 的答案 | internal/orchestrator/program_drive_test.go |
| AC-005 | 目标「看得到状态」 | internal/orchestrator/program_route.go |
| AC-006 | Q3 的答案 | internal/orchestrator/program_drive.go |
| AC-007 | 目标「全部完成才算交付」 | internal/orchestrator/program_drive.go |
| AC-008 | 非目标「不新增顶层命令」 | internal/cli/goal_program.go |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `beacon goal program drive --run-id <host>` 存在；单次调用最多推进一个就绪湖到其 release gate |
| AC-002 | 湖↔run 绑定幂等：已有 running run 复用，绑定写入 program 快照 |
| AC-003 | 到达 release gate 后记录 `awaiting_release`；不自动 complete |
| AC-004 | 某湖失败时记录 `blocked` + reason_code + 湖 slug；依赖湖保持 待命 |
| AC-005 | `program route` 暴露每湖 run_id、当前 stage、program 级 rollup 与 program_status |
| AC-006 | drive 可恢复：已 completed 的湖不再执行 |
| AC-007 | 全部湖 complete → `delivered`；存在 blocked 湖时不得 `delivered` |
| AC-008 | 不新增顶层命令；`beacon loop` 文案不变 |

## Boundary

- 本包只做串行接力；并行与跨机器调度不在范围内。
- 不削弱 release 人工门：drive 永远停在 gate。
