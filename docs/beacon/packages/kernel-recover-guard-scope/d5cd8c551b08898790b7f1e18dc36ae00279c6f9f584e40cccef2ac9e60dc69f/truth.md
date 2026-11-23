---
slug: kernel-recover-guard-scope
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: complete
materials_status: current
intent_confirmed: true
traceability_required: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-recover-guard-scope/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-recover-guard-scope/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-recover-guard-scope/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence.md
---

# Requirement Truth: kernel-recover-guard-scope

## 人话

给 Beacon 内核维护者与依赖 goal status 判断「是否在等人类」的房间：`beacon goal recover` 目前无条件清除任何阻断，连「发布必须由人确认」这类人类门禁也能被任意身份擦掉，status 随即显示 blocked:false，误导后续 bot 与监控。本包要求 recover 只清可恢复类阻断，遇人类门禁原因返回明确原因码且运行状态零改动；人类门禁只由带身份的明确确认动作清除；成功与拒绝的 recover 都留可审计事件；release 等待人类时 status 必须如实显示 blocked 与 last_block_reason。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "MausBot 转达的内核缺陷：羽球营地 run_aa702f8da68fe33f23214ae3 在 release 阶段记录 stage_exit_blocked（reason_code=release_requires_human_confirmation）后，一条 goal_recovered 把阻断擦掉，status 显示 blocked:false。要求 recover 遇人类门禁原因必须拒绝且零改动，人类门禁只能由明确确认动作清除，recover 成功与拒绝都留痕，status 如实显示阻断与原因。"
goal: "recover 只清可恢复类阻断；人类门禁类阻断由带身份的确认动作独占清除；recover 的成功与拒绝都有可审计事件；release 等待人类时 status 输出 blocked=true、blocked_streak、last_block_reason 与 paused 运行状态，历史事件流已丢失快照时也能还原真实阻断。"
non_goals: "不改 kernel-external-stage-evidence 已冻结包与已合入实现；不改版本号与对外发布说明；不清洗历史事件与历史虚假通过记录；不改 release 门的四项完成条件；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核，工作区 beacon-go，canonical 分支 main；truth 与控制面写入 canonical-only；每条 AC 配两条执行层 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级；不削弱既有 gate 与已冻结 AC。"
scope_mode: focused
intent_confirmed: false
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | MausBot 派单原话：羽球营地 run_aa702f8da68fe33f23214ae3 在 11:38:37 记录 release_requires_human_confirmation 阻断，11:38:48 一条 goal_recovered 把它擦掉，status 显示 blocked:false。完成标准是 recover 拒绝人类门禁原因且零改动、确认动作独占清除、成功与拒绝都留痕、status 如实显示阻断与原因。 |
| Q2 | 哪些阻断原因属于人类门禁类，recover 遇到时必须拒绝？ | 用封闭清单常量化，清单外按可恢复类处理 | answered | 人类门禁类至少包含 release_requires_human_confirmation、release_awaiting_complete、driver_awaiting_human、self_heal_requires_human_confirmation、planner_review_human_gate_p1；该集合在 reason catalog 中封闭声明，新增同类原因必须走 truth change。清单外的阻断保留现有 recover 清除语义。 |
| Q3 | 人类门禁类阻断由哪个动作清除？ | 由带身份的 release 确认动作清除，recover 不得代劳 | answered | beacon goal complete --human-confirmed --confirmed-by 写入 release_confirmed 事件，当前有效阻断为人类门禁时同时写 human_gate_cleared 并清空 Blocked、BlockedStreak、LastBlockReason；recover 不写 release_confirmed，不修改 release_human_confirmed。 |
| Q4 | status 在快照已被历史 recover 擦掉时如何如实显示？ | 快照与事件流取并集，人类门禁阻断以最新未确认事件为准 | answered | 快照 blocked=true 时直接显示快照原因；快照 blocked=false 但事件流中最新一条人类门禁 stage_exit_blocked 晚于最新 release_confirmed 时，status 显示 blocked=true、paused、该原因与对应 streak。历史 goal_recovered 不覆盖人类门禁事件，因为它正是要修的缺陷。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：recover 遇人类门禁必须拒绝且 state 零改动 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r1-recover-scope.md |
| AC-002 | 用户原话：人类门禁类阻断只能由明确确认动作清除 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md |
| AC-003 | 用户原话：recover 留痕，被拒绝的尝试也要留拒绝事件 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r3-recover-trail.md |
| AC-004 | 用户原话：release 等待人类时 status 必须如实显示阻断与原因 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r4-status-truth.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `beacon goal recover` 在有效阻断原因属于人类门禁类时拒绝执行，返回原因码 `goal_recovery_human_gate_forbidden`，运行快照的 blocked、blocked_streak、last_block_reason、stage plan 状态、LastTickKey、release_human_confirmed 全部保持请求前的取值；有效阻断原因不在人类门禁集合内时保持现有清零语义。 |
| AC-002 | 人类门禁类阻断只由带身份的明确确认动作清除；`beacon goal complete --human-confirmed --confirmed-by <identity>` 写入 `release_confirmed` 事件并在当前有效阻断为人类门禁时写 `human_gate_cleared` 且清空该阻断，空身份被拒绝；`beacon goal recover` 不写 `release_confirmed`，不修改 `release_human_confirmed`。 |
| AC-003 | 成功的 recover 写入 `goal_recovered` 事件，payload 含 by、stage、prior_reason_code、prior_blocked_streak、blocked_after=false、blocked_streak_after=0、decision=allowed；因人类门禁被拒绝的 recover 写入 `goal_recover_rejected` 事件，payload 含 by、stage、attempted_reason_code、refusal_reason_code、blocked_before、blocked_after、blocked_streak_before、blocked_streak_after、human_confirmation_required=true。 |
| AC-004 | release 等待人类时，`beacon goal status` 输出 blocked=true、run_status=paused、blocked_streak>0、last_block_reason 等于人类门禁原因码；被拒绝的 recover 与 attach-verifier 不得改变 blocked、blocked_streak、last_block_reason、run_status。快照已丢失该阻断时，status 以事件流中最新一条未被 `release_confirmed` 覆盖的人类门禁 `stage_exit_blocked` 为准。 |

## 人类门禁原因集合

该集合是封闭清单，常量定义在 core reason catalog。清单内任一原因使 recover 拒绝，使确认动作为唯一清除路径。

| Reason Code | 语义 | 清除动作 |
|-------------|------|----------|
| release_requires_human_confirmation | release 阶段需要人类确认才能退出，tick 永不自动退出 release | `goal complete --human-confirmed` 带身份确认 |
| release_awaiting_complete | release 门已 pass，人类执行完成动作后收尾 | `goal complete --human-confirmed` 带身份确认 |
| driver_awaiting_human | agent driver 停在等待人类输入的回合 | 人类在 driver 会话中回复并推进 |
| self_heal_requires_human_confirmation | 自愈方案需要人类确认后执行 | 人类确认自愈方案 |
| planner_review_human_gate_p1 | planner review 的 P1 人工门禁 | 人类完成 P1 review 裁决 |

## Recover 判定规则

1. recover 先要求 `--by` 非空，空身份返回 `goal_recovery_identity_required`，不写任何事件。
2. recover 计算有效阻断视图，算法与 status 一致：快照优先，快照未阻断时回退到事件流中最新一条人类门禁 `stage_exit_blocked`。
3. 有效阻断原因命中人类门禁集合时，recover 返回 `goal_recovery_human_gate_forbidden`，只追加 `goal_recover_rejected` 事件；run 快照、stage plan、生命周期、LastTickKey、release 确认字段保持不变。
4. 有效阻断原因不在人类门禁集合时，recover 保持现有语义：清空 Blocked、BlockedStreak、LastBlockReason 与 LastTickKey，把当前 stage 置为 active，写 `goal_recovered`。
5. recover 不读取也不修改 release_human_confirmed，不写 release_confirmed。

## 人类确认规则

`beacon goal complete --human-confirmed --confirmed-by <identity>` 是 release 人类门禁的确认动作。`--confirmed-by` 为空时返回 `release_confirmation_identity_required`，不写事件。

确认动作在一个事务内写 `release_confirmed`（payload 含 by）与 `human_gate_cleared`（payload 含 by、stage、prior_reason_code、prior_blocked_streak）。当前有效阻断为人类门禁时，同一事务清空 Blocked、BlockedStreak、LastBlockReason。当前有效阻断不是人类门禁时，确认动作只记录确认事实，不清除该阻断。

确认之后 complete 仍需通过 verifier、qa 门与 release 门检查；后续检查失败产生的新阻断使用新的原因码，不复用人类门禁原因。

## 留痕规范

| 事件 | 触发 | 必含 payload |
|------|------|--------------|
| goal_recovered | 可恢复类阻断被显式清除 | by、note、stage、prior_reason_code、prior_blocked、prior_blocked_streak、blocked_after=false、blocked_streak_after=0、decision=allowed |
| goal_recover_rejected | 人类门禁类阻断上的 recover 尝试 | by、stage、attempted_reason_code、refusal_reason_code=goal_recovery_human_gate_forbidden、blocked_before=true、blocked_after=true、blocked_streak_before、blocked_streak_after、human_confirmation_required=true、decision=rejected |
| release_confirmed | 带身份的人类确认 | by |
| human_gate_cleared | 人类确认清除人类门禁阻断 | by、stage、prior_reason_code、prior_blocked_streak |

事件只追加，不改写历史。被拒绝的 recover 也必须留下 goal_recover_rejected，便于审计谁在什么时候尝试绕过人类门禁。

## status 呈现规则

`beacon goal status` 在 RunView 中输出 `last_block_reason` 字段，取值规则如下。

**终止事实优先**：run 的 LoopStage 为 done，或事件流中存在晚于最新人类门禁 `stage_exit_blocked` 的 `goal_completed` / `external_result_received` 时，status 输出 completed、blocked=false、last_block_reason 为空，不把历史人类门禁重建为当前阻断。external 交付收尾按设计不写 `release_confirmed`，不因此被误判为仍在等待人类。

1. 快照 blocked=true 时，last_block_reason 取快照的 LastBlockReason；blocked_streak 取快照值；run_status 为 paused。
2. 快照 blocked=false 时，扫描该 run 的事件流。取最新一条人类门禁 `stage_exit_blocked` 与最新一条 `release_confirmed`；前者晚于后者时，status 输出 blocked=true、run_status=paused、last_block_reason 为该事件的原因码、blocked_streak 为该事件的 blocked_streak。
3. 历史 `goal_recovered` 不解除人类门禁事件。attach-verifier 与 verifier_attached 同样不改变上述判定。
4. 快照与事件流都表明没有阻断时，blocked=false，last_block_reason 为空。

## AC 之间的关系

AC-001 管 recover 的作用域，关闭任意身份擦除人类门禁的路径。AC-002 管清除权的归属，确认动作独占人类门禁。AC-003 管可审计性，成功与拒绝都可追溯。AC-004 管对外呈现，快照被历史擦除也能从事件流还原真实状态。四条缺一不可：只拦 recover 而不修 status 仍会误导；只修 status 而不拦 recover 仍可伪造解除。

## 源锚点

本仓 main 分支 commit ffdfced 的代码与 sqlite 事件流。逐条文件、行号与实跑摘录写在包内 evidence/source-anchors.md。

| AC ID | 本仓文件 | 行号 |
|-------|----------|------|
| AC-001 | internal/orchestrator/complete.go | L82-L112（Recover 无条件清 Blocked、BlockedStreak、LastBlockReason） |
| AC-001 | internal/orchestrator/tick.go | L50-L80（applyBlockOn 累加 streak，达到 3 固化原因码） |
| AC-001 | internal/core/reasons.go | L75、L162、L261、L557（人类门禁类原因常量） |
| AC-002 | internal/orchestrator/complete.go | L149-L161（human-confirmed 分支写 release_confirmed） |
| AC-002 | internal/cli/goal.go | L463-L510（goal complete 的 --human-confirmed 与 --confirmed-by） |
| AC-002 | internal/orchestrator/gates_facts.go | L319-L348（release_confirmed 事件是 trail 侧确认事实） |
| AC-003 | internal/orchestrator/complete.go | L100-L110（goal_recovered payload 只有 prior 字段，没有 after 字段） |
| AC-003 | internal/cli/goal.go | L394-L427（goal recover 命令与 --by 必填） |
| AC-004 | internal/orchestrator/engine.go | L236-L245（runStatusFor 由 Blocked 映射 paused） |
| AC-004 | internal/orchestrator/engine.go | L267-L276（viewOf 输出 Blocked 与 BlockedStreak，没有 last_block_reason） |
| AC-004 | internal/orchestrator/exitgates.go | L298-L320（release 永不自动退出，固定阻断 release_requires_human_confirmation） |
| AC-004 | internal/orchestrator/complete.go | L34-L70（AttachVerifier 固定 cleared_block=false） |

## Non-Goals

- 不改 kernel-external-stage-evidence 已冻结包与已合入实现。
- 不改 kernel-change-quality、kernel-effect-journal 两个未合入分支。
- 不改版本号与对外发布说明。
- 不清洗历史事件与历史虚假通过记录；修复只影响判定与后续写入。
- 不改 release 门的四项完成条件（human confirmed、verifier、qa pass、release gate）。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
