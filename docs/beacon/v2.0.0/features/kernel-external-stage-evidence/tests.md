---
slug: kernel-external-stage-evidence
version: v2.0.0
materials_status: current
---

# Tests: kernel-external-stage-evidence

每条 AC 配两条执行层 TC：一条验证正向证据成立，一条验证拒绝路径 fail-closed。Command 是可直接运行的 go test 选择器；实现轮按此命名落地测试，并把实跑输出追加到对应的包内证据文件。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | go test ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1 | external run 的记录阶段完成路径写出 stage_executed，payload 的 source 为 external、executor 为 external、by 为调用方身份、占位标记为 false | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e1-external-stage-event.md |
| AC-001 | TC-002 | negative | go test ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1 | 调用方自带的 payload 字段不进入内核写入路径；伪造的 executor 与 by 取值不改变内核写下的真实事件字段 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e1-external-stage-event.md |
| AC-002 | TC-003 | negative | go test ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1 | 只有 placeholder 为 true 的 qa stage_executed 时，qa 门 blocked 且 qa_stage_executed 为 false | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e2-recording-rejected.md |
| AC-002 | TC-004 | behavior | go test ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1 | placeholder 为 false 的真实 external qa stage_executed 让 qa_stage_executed 为 true | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e2-recording-rejected.md |
| AC-003 | TC-005 | behavior | go test ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1 | receive-result 外部完成记录加已注册且 hash 校验通过的 QA artifacts 让 qa 门通过 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e3-external-completion.md |
| AC-003 | TC-006 | negative | go test ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1 | 未注册 artifact、hash 不符、完成记录缺失 source=external 时 qa 门 blocked | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e3-external-completion.md |
| AC-004 | TC-007 | negative | go test ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1 | 阻塞原因未消除时 attach-verifier 不清除阻断，不写 cleared_block=true，streak 保持不变 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e4-streak-verifier.md |
| AC-004 | TC-008 | behavior | go test ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1 | streak 达到 3 时 continue 返回 product_goal_blocked_streak 且不推进；原因消除并经显式恢复后 streak 清零且 tick 推进 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e4-streak-verifier.md |

## 回归与门禁

实现轮除上述 8 条 TC 外，还需跑全量 go test ./... -count=1 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。
