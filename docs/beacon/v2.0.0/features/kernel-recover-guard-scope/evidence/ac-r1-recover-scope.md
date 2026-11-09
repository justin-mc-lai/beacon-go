# AC-001 证据：recover 只清可恢复类阻断

## 判据（原文）

`beacon goal recover` 在有效阻断原因属于人类门禁类时拒绝执行，返回原因码 `goal_recovery_human_gate_forbidden`，运行快照的 blocked、blocked_streak、last_block_reason、stage plan 状态、LastTickKey、release_human_confirmed 全部保持请求前的取值；有效阻断原因不在人类门禁集合内时保持现有清零语义。

## 源锚点

- internal/orchestrator/complete.go L82-L112：Recover 只检查 by 非空，随后无条件执行 Blocked=false、BlockedStreak=0、LastBlockReason=""、LastTickKey=""，不读取 LastBlockReason 的分类。
- internal/orchestrator/tick.go L50-L80：applyBlockOn 把阻断原因写进 LastBlockReason，streak 达到 3 时改写为 product_goal_blocked_streak，这是 recover 判定所需的快照事实。
- internal/core/reasons.go L75、L162、L261、L557：driver_awaiting_human、self_heal_requires_human_confirmation、release_awaiting_complete、planner_review_human_gate_p1 四个常量；release_requires_human_confirmation 定义在 internal/core/loopgoal.go L46。
- internal/orchestrator/engine.go L242-L245：runStatusFor 只在 Blocked 为真时映射 paused，recover 擦掉 Blocked 即让运行状态回到 running。

## 只读实跑事实

- 羽球营地 run_aa702f8da68fe33f23214ae3 在 11:38:37 记录 stage_exit_blocked，reason_code=release_requires_human_confirmation，blocked_streak=1。
- 11:38:48 的 goal_recovered 由「羽球营地 前端」触发，payload 记录 prior_reason_code=release_requires_human_confirmation，随后 goal status 的 blocked 变为 false。
- 该 run 的 release 阶段没有任何人类确认事件；阻断被擦除后，complete 仍因 ReleaseHumanConfirmed=false 拒绝，但 status 已经把「正在等人类」显示成未阻断。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | go test ./internal/orchestrator -run '^TestRecoverClearsRecoverableBlockAndResetsStreak$' -count=1 |
| TC-002 | go test ./internal/orchestrator -run '^TestRecoverRefusesHumanGateReasonsWithoutStateChange$' -count=1 |

## 执行记录（实现轮）

- 分支 `feat/kernel-recover-guard-scope`，实现提交 `8ad7fed`，实跑时间 `2026-11-09T20:06:08+0800`（Asia/Shanghai）。
- `go test ./internal/orchestrator -run '^TestRecoverClearsRecoverableBlockAndResetsStreak$' -count=1 -v` → `--- PASS` (0.03s)，`ok beacon-go/internal/orchestrator 0.348s`，`EXIT=0`。
- `go test ./internal/orchestrator -run '^TestRecoverRefusesHumanGateReasonsWithoutStateChange$' -count=1 -v` → 五个子用例 `release_requires_human_confirmation`、`release_awaiting_complete`、`driver_awaiting_human`、`self_heal_requires_human_confirmation`、`planner_review_human_gate_p1` 全部 `--- PASS` (0.12s)，`ok beacon-go/internal/orchestrator 0.434s`，`EXIT=0`。
- 断言落点：TC-001 校验可恢复阻断被清空，快照的 `Blocked`、`BlockedStreak`、`LastBlockReason`、`LastTickKey` 全部归零且阶段 plan 重新 active；TC-002 对五种人类门禁逐一校验返回 `goal_recovery_human_gate_forbidden`，并比对拒绝前后 run snapshot metadata 完全一致、`ReleaseHumanConfirmed` 仍为 false。
- 全量回归：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:04:10+0800`，17 包 ok、1 包无测试文件），全文见 `evidence/go-test-all.txt`。
