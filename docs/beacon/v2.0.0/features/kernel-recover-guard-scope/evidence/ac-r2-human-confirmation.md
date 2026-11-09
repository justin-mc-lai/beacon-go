# AC-002 证据：人类门禁只由带身份的确认动作清除

## 判据（原文）

人类门禁类阻断只由带身份的明确确认动作清除；`beacon goal complete --human-confirmed --confirmed-by <identity>` 写入 `release_confirmed` 事件并在当前有效阻断为人类门禁时写 `human_gate_cleared` 且清空该阻断，空身份被拒绝；`beacon goal recover` 不写 `release_confirmed`，不修改 `release_human_confirmed`。

## 源锚点

- internal/orchestrator/complete.go L149-L161：HumanConfirmed 分支设置 ReleaseHumanConfirmed 与 HumanConfirmedBy 并写 release_confirmed；没有该标志时 complete 以 release_requires_human_confirmation 拒绝。
- internal/cli/goal.go L463-L510：goal complete 暴露 --human-confirmed 与 --confirmed-by，--confirmed-by 当前默认 cli；实现轮要把空身份改为拒绝。
- internal/orchestrator/gates_facts.go L319-L348：release 门读取 run 快照的确认标志，并回退到最新 release_confirmed 事件，确认事实是 trail 侧可核对的。
- internal/orchestrator/complete.go L82-L112：recover 当前既不清空 ReleaseHumanConfirmed，也不写 release_confirmed；它只是绕过 blocked 快照，导致确认事实与阻断状态脱节。

## 只读实跑事实

- 羽球营地 run 的 goal_recovered 之后，release_human_confirmed 仍为 false；recover 没有产生任何 release_confirmed 事件。
- release 门在该 run 上仍没有 verdict；complete 的 ReleaseHumanConfirmed 检查仍是硬门，说明发布本身没有被放开。
- 缺陷在信号层：阻断状态被非确认动作清掉，确认动作与解除动作不是同一件事。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | go test ./internal/orchestrator -run '^TestHumanConfirmationClearsHumanGateBlock$' -count=1 |
| TC-004 | go test ./internal/orchestrator -run '^TestRecoverCannotSubstituteForHumanConfirmation$' -count=1 |

## 执行记录（实现轮）

- 分支 `feat/kernel-recover-guard-scope`，实现提交 `8ad7fed`，实跑时间 `2026-11-09T20:06:08+0800`（Asia/Shanghai）。
- `go test ./internal/orchestrator -run '^TestHumanConfirmationClearsHumanGateBlock$' -count=1 -v` → `--- PASS` (0.46s)，`ok beacon-go/internal/orchestrator 0.790s`，`EXIT=0`。
- `go test ./internal/orchestrator -run '^TestRecoverCannotSubstituteForHumanConfirmation$' -count=1 -v` → `--- PASS` (0.04s)，`ok beacon-go/internal/orchestrator 0.363s`，`EXIT=0`。
- 断言落点：TC-003 先以空白 `ConfirmedBy` 调 `Complete`，断言返回 `release_confirmation_identity_required` 且事件库中 `release_confirmed`、`human_gate_cleared` 均为 0 条；再以 `owner` 确认，断言运行 completed、恰好一条 `release_confirmed{by:owner}` 与一条 `human_gate_cleared{by:owner,stage:release,prior_reason_code:release_requires_human_confirmation,prior_blocked_streak:1}`。TC-004 断言人类门禁上的 recover 被拒、无 `release_confirmed`、`ReleaseHumanConfirmed` 仍 false 且 `HumanConfirmedBy` 为空、阻断与 streak 未变。
- 全量回归：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:04:10+0800`，17 包 ok、1 包无测试文件），全文见 `evidence/go-test-all.txt`。

## 执行记录（spec/AC QA 返修轮 · 重复确认路径）

- 触发：spec/AC 面独立 verdict FAIL（验收员，bind `21047a4`），阻塞项为第二次人类确认不写 `release_confirmed`，已完成 run 被 status 报成 blocked。
- 返修提交 `83e4c8d`（fix(orchestrator): record every human release confirmation），实跑时间 `2026-11-09T20:34:25+0800`（Asia/Shanghai）。
- 缺陷根因：`complete.go` 的 `writeConfirmation := !m.ReleaseHumanConfirmed` 让第二次确认只写 `human_gate_cleared`；`engine.go` 的 `effectiveBlockView` 只比对 `stage_exit_blocked` 与 `release_confirmed`，于是旧的 `stage_exit_blocked` 把已完成的 run 重建为阻断。
- 修法（方案 a）：去掉 `!m.ReleaseHumanConfirmed` 门槛，每次带身份的确认都设置 `ReleaseHumanConfirmed`/`HumanConfirmedBy` 并在同一事务写 `release_confirmed`；仅在当前有效阻断为人类门禁时追加 `human_gate_cleared` 并清阻断。
- 新增 TC-009 `go test ./internal/orchestrator -run '^TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked$' -count=1 -v` → `--- PASS` (0.52s)，`ok beacon-go/internal/orchestrator 0.819s`，`EXIT=0`（20:34:32 +0800）。
- 对照：修复前同一条 TC 实跑 `EXIT=1`，失败信息 `completed run reported as blocked: ... Status:blocked RunStatus:Paused Blocked:true ReasonCode:release_requires_human_confirmation`；修复后事件流 `release_confirmed` 2 条、`human_gate_cleared` 2 条，status 为 `completed`、`blocked=false`、`last_block_reason=""`、`run_status=Succeeded`。
- 全量回归（返修后重跑）：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:37:32+0800` 起跑，`2026-11-09T20:39:08+0800` 结束，96s，17 包 ok、1 包无测试文件），全文见 `evidence/go-test-all.txt`。
