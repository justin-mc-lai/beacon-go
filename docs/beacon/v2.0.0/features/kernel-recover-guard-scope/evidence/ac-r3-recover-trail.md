# AC-003 证据：recover 成功与拒绝都留可审计事件

## 判据（原文）

成功的 recover 写入 `goal_recovered` 事件，payload 含 by、stage、prior_reason_code、prior_blocked_streak、blocked_after=false、blocked_streak_after=0、decision=allowed；因人类门禁被拒绝的 recover 写入 `goal_recover_rejected` 事件，payload 含 by、stage、attempted_reason_code、refusal_reason_code、blocked_before、blocked_after、blocked_streak_before、blocked_streak_after、human_confirmation_required=true。

## 源锚点

- internal/orchestrator/complete.go L100-L110：goal_recovered 的 payload 只有 by、note、stage、prior_blocked、prior_blocked_streak、prior_reason_code，没有 blocked_after、blocked_streak_after、decision。
- internal/cli/goal.go L394-L427：goal recover 暴露 --run-id、--by、--note，--by 在 CLI 层必填。
- internal/orchestrator/complete.go L86-L90：空 by 返回 goal_recovery_identity_required，且不写事件；这是留痕之前的前置拒绝。
- internal/orchestrator/engine.go L254-L264：appendEventOn 是唯一追加事件的入口，payload 以 JSON 落 lifecycle_events。

## 只读实跑事实

- 羽球营地 run 的 11:38:48 goal_recovered 有 prior_reason_code 与 prior_blocked_streak，但没有清除后的 before/after 对照，也没有 decision 字段。
- 该 run 没有被拒绝的 recover 尝试事件；现有实现里人类门禁上的 recover 会直接成功，因此拒绝路径从未留下任何痕迹。
- 历史 goal_recovered 事件在被证明是误清之后仍无法从事件本身区分「合规清除」与「越权清除」。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | go test ./internal/orchestrator -run '^TestRecoverEventCarriesClearReceipt$' -count=1 |
| TC-006 | go test ./internal/orchestrator -run '^TestRejectedRecoverLeavesRejectionTrail$' -count=1 |

## 执行记录（实现轮）

- 分支 `feat/kernel-recover-guard-scope`，实现提交 `8ad7fed`，实跑时间 `2026-11-09T20:06:08+0800`（Asia/Shanghai）。
- `go test ./internal/orchestrator -run '^TestRecoverEventCarriesClearReceipt$' -count=1 -v` → `--- PASS` (0.03s)，`ok beacon-go/internal/orchestrator 0.359s`，`EXIT=0`。
- `go test ./internal/orchestrator -run '^TestRejectedRecoverLeavesRejectionTrail$' -count=1 -v` → `--- PASS` (0.03s)，`ok beacon-go/internal/orchestrator 0.351s`，`EXIT=0`。
- 断言落点：TC-005 读取唯一一条 `goal_recovered`，逐字段断言 `by=owner`、`stage=plan`、`prior_reason_code=manual_block`、`prior_blocked_streak=1`、`blocked_after=false`、`blocked_streak_after=0`、`decision=allowed` 且 `note` 存在。TC-006 读取唯一一条 `goal_recover_rejected`，逐字段断言 `by`、`stage`、`attempted_reason_code`、`refusal_reason_code`、`blocked_before`/`blocked_after`、`blocked_streak_before`/`blocked_streak_after`、`human_confirmation_required=true`、`decision=rejected`，并断言没有写入 `goal_recovered`。
- 全量回归：`go test ./... -count=1` → `EXIT=0`（`2026-11-09T20:04:10+0800`，17 包 ok、1 包无测试文件），全文见 `evidence/go-test-all.txt`。
