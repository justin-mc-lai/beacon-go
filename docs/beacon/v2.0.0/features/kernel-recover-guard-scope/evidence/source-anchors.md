# 源锚点：kernel-recover-guard-scope

- 仓库：/Users/apple/Developer/Personal/products/beacon-go
- 分支：main
- 基线 commit：ffdfced12656d46a075ee7400a82d9f5fa847a86
- 核查时间：2026-11-09T19:52+08:00
- 核查方式：读代码 + 只读查询 sqlite ~/.beacon/beacon-go.db 的 lifecycle_events 表（payload_json 列）
- 参考 run：run_aa702f8da68fe33f23214ae3（feature=ui-rebuild-v3）

## AC-001 recover 的作用域

代码位置：

- internal/orchestrator/complete.go L82-L112：Recover 只校验 by 非空，随后无条件执行 Blocked=false、BlockedStreak=0、LastBlockReason=""、LastTickKey=""，并把当前 stage 置回 active；不读取 LastBlockReason 的分类。
- internal/orchestrator/tick.go L50-L80：applyBlockOn 写 Blocked=true、累加 BlockedStreak、把原因写进 LastBlockReason，streak 达到 3 时改写为 product_goal_blocked_streak。
- internal/core/loopgoal.go L46：ReasonReleaseRequiresHuman 的值为 release_requires_human_confirmation。
- internal/core/reasons.go L75 driver_awaiting_human、L162 self_heal_requires_human_confirmation、L261 release_awaiting_complete、L557 planner_review_human_gate_p1。
- internal/orchestrator/engine.go L236-L245：runStatusFor 只在 Blocked 为真时映射 paused。

## AC-002 人类确认的独占清除权

代码位置：

- internal/orchestrator/complete.go L126-L165：Complete 在 opts.HumanConfirmed 且尚未确认时写 release_confirmed（payload 只有 by），随后才校验 ReleaseHumanConfirmed。
- internal/cli/goal.go L463-L511：goal complete 暴露 --human-confirmed 与 --confirmed-by，--force 注释写明不绕过四项完成条件。
- internal/orchestrator/gates_facts.go L319-L348：release 事实读取 run 快照的 ReleaseHumanConfirmed，缺失时回退到事件流里的 release_confirmed。
- internal/orchestrator/complete.go L189：Complete 成功路径清 LastBlockReason，但当前代码没有 human_gate_cleared 事件，确认动作与阻断清除尚未绑定。

## AC-003 recover 留痕

代码位置：

- internal/orchestrator/complete.go L100-L110：goal_recovered payload 只有 by、note、stage、prior_blocked、prior_blocked_streak、prior_reason_code，没有 blocked_after、blocked_streak_after、decision。
- internal/orchestrator/complete.go L88-L90：空身份直接返回 goal_recovery_identity_required，不写事件。
- internal/cli/goal.go L394-L428：goal recover 暴露 --by 与 --note，--by 标记必填。
- 当前代码没有 goal_recover_rejected 事件类型，被拒绝的尝试无法留痕。

## AC-004 status 的如实呈现

代码位置：

- internal/orchestrator/engine.go L118-L163：RunView 有 blocked、run_status、blocked_streak、reason_code，没有 last_block_reason 字段。
- internal/orchestrator/engine.go L267-L276：viewOf 输出 Blocked 与 BlockedStreak，不从事件流回退。
- internal/orchestrator/engine.go L296-L302：reason_code 只在 Blocked 为真且 LastBlockReason 非空时输出。
- internal/orchestrator/exitgates.go L298-L320：release 阶段永不自动退出；release 门 pass 时阻断 release_awaiting_complete，否则阻断 release_requires_human_confirmation。
- internal/orchestrator/complete.go L34-L70：AttachVerifier 固定写 cleared_block=false 与 block_pending，不改变快照阻断。

## 只读实跑事实

查询：

    sqlite3 ~/.beacon/beacon-go.db "select created_at,event_type,payload_json from lifecycle_events where run_id='run_aa702f8da68fe33f23214ae3' order by created_at asc;"

关键事件：

- 2026-11-09T11:38:37.375502000Z stage_exit_blocked {"blocked_streak":1,"reason_code":"release_requires_human_confirmation","stage":"release"}
- 2026-11-09T11:38:48.878858000Z goal_recovered {"by":"羽球营地 前端","note":"","prior_blocked":true,"prior_blocked_streak":1,"prior_reason_code":"release_requires_human_confirmation","stage":"release"}
- 2026-11-09T11:45:48.627784000Z goal_blocked {"blocked_streak":1,"reason_code":"release_requires_human_confirmation","stage":"release"}

同一条 run 的事件流里没有任何 release_confirmed；11:38:48 之后快照显示 blocked=false，而 11:38:37 的阻断事件仍在流里，快照与事件流不一致。
