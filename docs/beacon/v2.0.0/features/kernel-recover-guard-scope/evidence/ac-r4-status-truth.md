# AC-004 证据：release 等待人类时 status 如实显示阻断与原因

## 判据（原文）

release 等待人类时，`beacon goal status` 输出 blocked=true、run_status=paused、blocked_streak>0、last_block_reason 等于人类门禁原因码；被拒绝的 recover 与 attach-verifier 不得改变 blocked、blocked_streak、last_block_reason、run_status。快照已丢失该阻断时，status 以事件流中最新一条未被 `release_confirmed` 覆盖的人类门禁 `stage_exit_blocked` 为准。

## 源锚点

- internal/orchestrator/engine.go L236-L245：runStatusFor 把 Blocked 映射为 paused，把 error_recovery 映射为 repairing。
- internal/orchestrator/engine.go L267-L276：viewOf 输出 Blocked 与 BlockedStreak，没有 last_block_reason 字段。
- internal/orchestrator/exitgates.go L298-L320：release 阶段永不自动退出；没有 release 门 verdict 时固定阻断 release_requires_human_confirmation。
- internal/orchestrator/complete.go L34-L70：AttachVerifier 固定写 cleared_block=false 与 block_pending，不改变快照阻断。
- internal/orchestrator/complete.go L82-L112：Recover 清空 Blocked 与 LastBlockReason，是快照丢失的根因。

## 只读实跑事实

- 羽球营地 run 在 11:38:48 之后 goal status 显示 status=running、blocked=false、current_stage=release；同一时刻 release 门没有 verdict，complete 仍要求人类确认。
- 事件流里 11:38:37 的 stage_exit_blocked 仍然存在，reason_code 与 stage 都能读到；快照与事件流已经不一致。
- 现有 status 输出没有 last_block_reason，调用方无法从 status 直接判断「在等人类」还是「真的没人管」。

## 验收方式

| TC | Command |
|----|---------|
| TC-007 | go test ./internal/orchestrator -run '^TestStatusShowsHumanGateBlockTruthfully$' -count=1 |
| TC-008 | go test ./internal/orchestrator -run '^TestRecoverAndAttachVerifierDoNotHideHumanGateBlock$' -count=1 |

## 执行记录（实现轮）

- 分支 `feat/kernel-recover-guard-scope`，实现提交 `8ad7fed`，实跑时间 `2026-11-09T20:06:08+0800`（Asia/Shanghai）。
- `go test ./internal/orchestrator -run '^TestStatusShowsHumanGateBlockTruthfully$' -count=1 -v` → `--- PASS` (0.48s)，`ok beacon-go/internal/orchestrator 0.802s`，`EXIT=0`。
- `go test ./internal/orchestrator -run '^TestRecoverAndAttachVerifierDoNotHideHumanGateBlock$' -count=1 -v` → `--- PASS` (0.48s)，`ok beacon-go/internal/orchestrator 0.802s`，`EXIT=0`。
- 断言落点：TC-007 在 release 门阻断下断言 status 为 `blocked=true`、`run_status=paused`、`blocked_streak=1`、`last_block_reason=release_requires_human_confirmation`；随后模拟历史缺陷把快照三个字段清零并保存，再读 status，断言仍还原出同样的 blocked/paused/streak/reason 且 `reason_code` 一致。TC-008 记录拒绝 recover 前的 status，再执行被拒 recover + `attach-verifier`，断言 blocked、streak、last_block_reason、run_status 四项前后完全相同且仍如实报告人类门禁。
 - 全量回归（实现轮）：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:04:10+0800`，17 包 ok、1 包无测试文件）。

 ## 执行记录（spec/AC QA 返修轮 · 重复确认后 status 不得误报）

 - 触发：spec/AC 面独立 verdict FAIL（验收员，bind `21047a4`），实测路径 external run → 首次确认 → 挂 verifier → 换身份 tick 重新阻断 → 二次确认 complete 成功 → status 报 `blocked=true / Paused`，违反 AC-004「status 不得误报阻断」。
 - 修复点：`complete.go` 去掉 `writeConfirmation := !m.ReleaseHumanConfirmed` 门槛后，每次确认都写 `release_confirmed`，status 重建时旧的 `stage_exit_blocked` 会被最新确认事实覆盖；`effectiveBlockView` 逻辑未改。
 - 返修提交 `83e4c8d`，实跑时间 `2026-11-09T20:34:32+0800`（Asia/Shanghai）。
 - TC-009 `go test ./internal/orchestrator -run '^TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked$' -count=1 -v` → `--- PASS` (0.52s)，`ok beacon-go/internal/orchestrator 0.819s`，`EXIT=0`。
 - 断言落点：完整 6 步复现后断言 `status=completed`、`blocked=false`、`last_block_reason` 为空、`run_status=Succeeded`，并要求 `release_confirmed` 与 `human_gate_cleared` 各 2 条；修复前同一 TC `EXIT=1`，报 `completed run reported as blocked`。
 - 全量回归（返修后重跑）：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:37:32+0800` 起跑，`2026-11-09T20:39:08+0800` 结束，96s，17 包 ok、1 包无测试文件），全文见 `evidence/go-test-all.txt`。
