---
slug: kernel-recover-guard-scope
version: v2.0.0
materials_status: current
---

# Tests: kernel-recover-guard-scope

每条 AC 配两条执行层 TC：一条验证正向行为成立，一条验证拒绝路径 fail-closed。Command 是可直接运行的 go test 选择器；实现轮按此命名落地测试，并把实跑输出追加到对应的包内证据文件。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | go test ./internal/orchestrator -run '^TestRecoverClearsRecoverableBlockAndResetsStreak$' -count=1 | 可恢复类阻断上 recover 清空 blocked、blocked_streak、last_block_reason，stage 回到 active，写入 goal_recovered | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r1-recover-scope.md |
| AC-001 | TC-002 | negative | go test ./internal/orchestrator -run '^TestRecoverRefusesHumanGateReasonsWithoutStateChange$' -count=1 | 五种人类门禁原因逐一让 recover 返回 goal_recovery_human_gate_forbidden；run 快照、streak、stage plan、LastTickKey、release 确认字段与调用前逐字节一致 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r1-recover-scope.md |
| AC-002 | TC-003 | behavior | go test ./internal/orchestrator -run '^TestHumanConfirmationClearsHumanGateBlock$' -count=1 | complete --human-confirmed 带身份写 release_confirmed 与 human_gate_cleared，清空人类门禁阻断；空身份返回 release_confirmation_identity_required 且不写事件 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md |
| AC-002 | TC-004 | negative | go test ./internal/orchestrator -run '^TestRecoverCannotSubstituteForHumanConfirmation$' -count=1 | 被拒绝的 recover 不写 release_confirmed、不修改 release_human_confirmed，run 仍处于人类门禁阻断 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md |
| AC-003 | TC-005 | behavior | go test ./internal/orchestrator -run '^TestRecoverEventCarriesClearReceipt$' -count=1 | goal_recovered payload 含 by、stage、prior_reason_code、prior_blocked_streak、blocked_after=false、blocked_streak_after=0、decision=allowed | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r3-recover-trail.md |
| AC-003 | TC-006 | negative | go test ./internal/orchestrator -run '^TestRejectedRecoverLeavesRejectionTrail$' -count=1 | 被拒绝的 recover 写 goal_recover_rejected，payload 含 attempted_reason_code、refusal_reason_code、before/after 的 blocked 与 streak、human_confirmation_required=true，且没有 goal_recovered | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r3-recover-trail.md |
| AC-004 | TC-007 | behavior | go test ./internal/orchestrator -run '^TestStatusShowsHumanGateBlockTruthfully$' -count=1 | 快照阻断时 status 输出 blocked=true、paused、last_block_reason；快照被历史 recover 擦掉时从最新人类门禁 stage_exit_blocked 还原同样的 blocked、paused、reason、streak | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r4-status-truth.md |
| AC-004 | TC-008 | negative | go test ./internal/orchestrator -run '^TestRecoverAndAttachVerifierDoNotHideHumanGateBlock$' -count=1 | 被拒绝的 recover 与 attach-verifier 之后，status 的 blocked、blocked_streak、last_block_reason、run_status 与调用前一致 | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r4-status-truth.md |
| AC-002 | TC-009 | behavior | go test ./internal/orchestrator -run '^TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked$' -count=1 | 重复人类确认路径：首次确认后挂 verifier、换身份 tick 重新阻断、再次确认并 complete 成功，status 必须 completed、blocked=false、last_block_reason 为空；每次确认都留 release_confirmed 与 human_gate_cleared | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md |
| AC-004 | TC-010 | negative | go test ./internal/orchestrator -run '^TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted$' -count=1 | external run 在 release 被人类门禁阻断后由 receive-result --external 收尾，status 必须 completed、blocked=false、last_block_reason 为空、run_status=Succeeded；终止事件晚于人类门禁，不得重建 blocked | docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r4-status-truth.md |

## 回归与门禁

实现轮除上述 10 条 TC 外，还需跑全量 go test ./... -count=1 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。
