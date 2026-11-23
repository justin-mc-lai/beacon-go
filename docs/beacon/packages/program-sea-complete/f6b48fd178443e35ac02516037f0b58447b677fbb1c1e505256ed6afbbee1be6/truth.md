---
slug: program-sea-complete
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
  truth: docs/beacon/v2.0.0/features/program-sea-complete/truth.md
  tests: docs/beacon/v2.0.0/features/program-sea-complete/tests.md
  tasks: docs/beacon/v2.0.0/features/program-sea-complete/tasks.md
  evidence: docs/beacon/v2.0.0/features/program-sea-complete/evidence.md
---

# Requirement Truth: program-sea-complete

## 人话

给 Beacon 内核的使用者：现在多湖能一个湖一个湖地推，但每到一个湖的 release gate 就得人点一次，整片需求海没法一口气跑完。本包加两件事：`goal program drive --until-done` 自动把所有湖依次推到各自的 release gate（跳过已完成、遇阻就停）；最后用 `goal program complete` 做**一次**程序级人工确认，把整片海一次放行。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "我想要的多湖顺序执行到整片需求海做完的能力"
goal: "一条命令把多湖依次跑到各自 release gate，最后程序级签一次，整片海 delivered"
non_goals: "不做每湖自动 release（auto）；不跳过任何门的硬前置；不做跨机器调度"
constraints: "复用 ProgramDrive / ProgramRoute / Complete；程序级确认写事件并归因；被阻塞的湖不得被确认"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 程序级确认是几次？ | 一次，覆盖所有已到 gate 的湖 | answered | 采用；每湖的 release_confirmed 归因 program:<name> |
| Q2 | 已到 gate 的湖，循环要不要停？ | 不停，跳过并继续下一个湖 | answered | 采用；全部到 gate 后返回 awaiting_release |
| Q3 | 有湖 blocked 时怎么办？ | 立即停，记录原因，不确认任何湖 | answered | 采用；依赖湖保持待命 |
| Q4 | 上游湖到 gate 但还没最终确认，下游湖能不能开工？ | 能：视为 release-ready；整片海最后一起确认 | answered | 采用；否则“最后只签一次”永远串不起来 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | Q2 的答案 | internal/orchestrator/program_drive.go |
| AC-002 | 目标「一条命令跑到头」 | internal/cli/goal_program.go |
| AC-003 | Q3 的答案 | internal/orchestrator/program_drive.go |
| AC-004 | Q1 的答案 | internal/orchestrator/program_complete.go |
| AC-005 | 目标「归因可查」 | internal/orchestrator/program_complete.go |
| AC-006 | 非目标「不跳过硬前置」 | internal/orchestrator/complete.go |
| AC-007 | 目标「整片海状态可见」 | internal/orchestrator/program_route.go |
| AC-008 | 非目标「不新增顶层命令」 | internal/cli/goal_program.go |
| AC-009 | Q4 的答案 | internal/orchestrator/program_drive.go |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `goal program drive --until-done` 依次推进所有就绪湖；已 completed/awaiting_release 的湖跳过 |
| AC-002 | 湖到 release gate 记 `awaiting_release` 后，循环继续下一个湖，不要求每湖人工确认 |
| AC-003 | 任一湖 blocked 时循环立即停，记录 reason_code 与该湖 slug；依赖湖保持待命 |
| AC-004 | `goal program complete --run-id <host> --confirmed-by <name>` 是唯一一次程序级人工确认，完成所有在 release gate 的湖 |
| AC-005 | 程序级确认写 `program_release_confirmed` 事件，含确认人与各湖 run id；每湖 `release_confirmed` 归因 `program:<name>` |
| AC-006 | 有湖不在 release gate、缺 verifier、缺 qa pass 时，`program complete` 拒绝并给稳定 reason code |
| AC-007 | `program route` 在所有湖到 gate 时显示 `program_status=awaiting_release`，确认后显示 `delivered` |
| AC-008 | 不新增顶层命令；两个能力都落在 `beacon goal program` 下 |
| AC-009 | 在 `--until-done` 中，依赖湖处于 `awaiting_release` 视为可启动；下游湖继续推进，最终一起确认 |

## Boundary

- 程序级确认是**一次人工签字**，不是 auto；release 仍是人签，只是从 N 次变 1 次。
- 不跳过任何门的硬前置；blocked 湖永不被确认。
