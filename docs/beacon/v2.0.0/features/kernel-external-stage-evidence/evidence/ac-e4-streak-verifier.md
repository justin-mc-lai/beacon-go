# AC-004 证据：blocked_streak 与 attach-verifier 行为一致

## 判据（原文）

unresolved 阻断不得仅靠 attach-verifier 清除；blocked_streak 达到 3 后 continue 的拒绝语义与阻塞原因被真实消除后的恢复路径保持一致，attach-verifier 只在恢复条件同时满足时解除阻断。

## 源锚点

- internal/orchestrator/tick.go L40-L70：applyBlock 累加 streak，达到 3 时固化 product_goal_blocked_streak。
- internal/orchestrator/tick.go L348-L388：ContinueBy 在 streak 达到 3 时直接返回 ReasonBlockedStreak。
- internal/orchestrator/complete.go L32-L56：AttachVerifier 无条件写 cleared_block=true。
- internal/orchestrator/admission.go L166-L173：streak 达到 3 且无修复循环时拒绝 implement 入场。
- 实跑：09:34:51 verifier_attached（cleared_block=true）之后 qa 门仍 blocked；09:36:34 streak 被重置为 1，详见 source-anchors.md。

## 验收方式

| TC | Command |
|----|---------|
| TC-007 | go test ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1 |
| TC-008 | go test ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1 |

## 执行记录（实现轮）

- 执行时间：2026-11-09T10:53Z（UTC，本机 18:53 +08:00）
- 分支：`feat/kernel-external-stage-evidence`
- 实现提交：`794433f`（feat(orchestrator): record real external stage evidence and reject recording placeholders）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

实现要点：`AttachVerifier` 只登记独立验收方，不再清除阻断，事件固定写 `cleared_block:false` 并附 `block_pending`（`internal/orchestrator/complete.go:34`）。新增显式恢复路径 `Engine.Recover`（`internal/orchestrator/complete.go:82`）与 `beacon goal recover --run-id --by [--note]`（`internal/cli/goal.go:394`），要求给出断言原因已消除的身份，清零 `BlockedStreak` 并写 `goal_recovered`，记录恢复前的 streak 与 reason。`ContinueBy` 在 streak 达到 3 时仍直接返回 `product_goal_blocked_streak`（`internal/orchestrator/tick.go:381`），恢复后再 tick 才推进。

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-007 | `go test ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.394s` | 0 |
| TC-008 | `go test ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.421s` | 0 |

原始输出：

```text
$ go test ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1 -v
=== RUN   TestAttachVerifierDoesNotClearUnresolvedBlock
--- PASS: TestAttachVerifierDoesNotClearUnresolvedBlock (0.04s)
PASS
ok  	beacon-go/internal/orchestrator	0.394s
EXIT=0
$ go test ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1 -v
=== RUN   TestHardBlockedContinueResumesAfterCauseCleared
--- PASS: TestHardBlockedContinueResumesAfterCauseCleared (0.09s)
PASS
ok  	beacon-go/internal/orchestrator	0.421s
EXIT=0
```
