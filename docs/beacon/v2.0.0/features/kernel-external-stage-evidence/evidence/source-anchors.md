# 源锚点：kernel-external-stage-evidence

- 仓库：/Users/apple/Developer/Personal/products/beacon-go
- 分支：main
- 基线 commit：a3eab40
- 核查时间：2026-11-09T17:44+08:00
- 核查方式：读代码 + 只读查询 sqlite ~/.beacon/beacon-go.db 的 lifecycle_events 表
- 参考 run：run_aa702f8da68fe33f23214ae3（executor=external，executor_id=羽球营地 前端，feature=ui-rebuild-v3）

## AC-001 external 与 manual run 的阶段完成

代码位置：

- internal/orchestrator/tick.go L113-L200：TickBy 在 manual 与 external 分支只走 recordStageExit 与 stage_advanced，没有 append stage_executed。
- internal/orchestrator/delivery_paradigm.go L60-L62：isManualExecutor 判定 manual 与 external 两种执行模式。
- internal/orchestrator/delivery_paradigm.go L203-L240：recordStageExit 写入 stage_exit_evaluated，payload 里带 executor 与 completed_by，但事件类型不是 stage_executed。
- internal/orchestrator/executor.go L49-L72：RecordingExecutor 写 stage_executed，payload 固定 placeholder=true。

实跑事实（sqlite 查询 lifecycle_events，按 created_at 升序）：

- 09:15:53 至 09:31:30 之间 plan、truth、design、implement 四个阶段的 stage_exit_evaluated 全部存在，executor 字段均为 external。
- 整个 run 只有 1 条 stage_executed，时间 2026-11-09T09:37:04.038100000Z，payload 为 {"executor":"recording","outcome":"done","placeholder":true,"stage":"qa"}。
- 也就是说：external run 逐阶段推进时没有一条真实 stage_executed。

## AC-002 qa 门判定

代码位置：

- internal/orchestrator/gates_facts.go L227-L241：遍历事件，只判 event_type=stage_executed、stage=qa、outcome!=failed，代码不读 payload 的 placeholder 字段。
- internal/gate/evaluate.go L258-L282：qa 门三项 check 为 qa_artifacts_present、qa_stage_executed、qa_artifact_refs_valid。

实跑事实：

- 09:31:46、09:32:11、09:34:56 三次 qa 门 blocked，reason_codes 均为 qa_stage_not_executed。
- 09:36:34 goal_continued 之后，09:37:04 出现上述 placeholder=true 的 stage_executed，09:37:52 qa 门 verdict 变为 pass，reason_codes 为空。
- 该 pass 没有任何真实 qa 阶段执行支撑，属于录制态事件造成的虚假通过。

## AC-003 receive-result 外部完成

代码位置：

- internal/orchestrator/delivery_paradigm.go L258-L333：ReceiveExternalResult 要求 by 与 summary，至少一个 artifact，注册 artifact 后写 external_result_received 与 goal_completed（source=external，human_confirmed=false），并把 run 关闭为 done。
- internal/orchestrator/artifacts.go L46-L70：QA 类 artifact 判定为 qa_report 精确词条或 qa_ 前缀。
- internal/cli/goal_p2c.go L172-L205：CLI 入口为 goal receive-result，flag 为 --external、--by、--summary、--artifact。

现状：external_result_received 不影响 qa_stage_executed，qa 门仍 blocked；AC-003 要求把它与已注册且 hash 校验通过的 QA artifacts 组合成等价证据。

## AC-004 blocked_streak 与 attach-verifier

代码位置：

- internal/orchestrator/tick.go L40-L70：applyBlock 累加 BlockedStreak，达到 3 时把 LastBlockReason 固化为 product_goal_blocked_streak。
- internal/orchestrator/tick.go L348-L388：ContinueBy 在 BlockedStreak 达到 3 时直接返回 ReasonBlockedStreak，不推进阶段。
- internal/orchestrator/complete.go L32-L56：AttachVerifier 在 run 处于阻断时无条件写 cleared_block=true，并把 Blocked 置 false，BlockedStreak 保持不变。
- internal/orchestrator/admission.go L166-L173：BlockedStreak 达到 3 且 RepairLoopCount 为 0 时拒绝 implement 入场。

实跑事实：

- 09:32:56 一条 goal_continued（blocked_streak=2）后立刻出现 goal_blocked（blocked_streak=3，reason_code=product_goal_blocked_streak）。
- 09:34:51 verifier_attached 带 cleared_block=true。
- 09:34:56 紧接着 qa 门仍 blocked（qa_stage_not_executed），说明解除阻断并没有消除真实阻塞原因。
- 09:36:34 又出现 goal_continued（blocked_streak=1），streak 在没有修复原因的情况下被重置，与 continue 在 streak 达到 3 时拒绝的规则不一致。
