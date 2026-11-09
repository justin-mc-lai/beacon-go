---
slug: kernel-progress-drift
version: v2.0.0
materials_status: current
---

# Tests: kernel-progress-drift

每条 AC 至少两条执行层 TC：一条验证回执结构与模式，一条验证策略计数或 revision 绑定的负向行为。Command 一律是可直接运行的 `go test` 选择器；实现轮次按此命名落地测试，并把实跑输出追加到对应的包内证据文件。AC-D1 至 AC-D3 与 AC-001 至 AC-003 的对应关系见 truth.md 的 AC ID 映射表。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | `go test ./internal/progressreview -run TestProgressReviewReceiptSchemaAndDefaultMode -count=1` | 回执带 `schema_version=progress_review_receipt_v0` 与固定字段；模式词表为 off/shadow/assist，默认 off | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d1-receipt-schema.md |
| AC-001 | TC-002 | behavior | `go test ./internal/progressreview -run TestShadowModeRecordsReceiptWithoutReplanObligation -count=1` | shadow 模式记录回执，但不产生 replan 义务；off 模式不记录可驱动的回执 | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d1-receipt-schema.md |
| AC-002 | TC-003 | behavior | `go test ./internal/progressreview -run TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation -count=1` | 连续 N 条 completed 且 drift=true 才提升为 replan 义务，第 N-1 条不提升 | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d2-drift-promotion.md |
| AC-002 | TC-004 | negative | `go test ./internal/progressreview -run TestNonCompletedStatusesNeverCountAsHandled -count=1` | abstained/failed/not_evaluated/stale 与 unknown 都不计入已处理，并打断连续计数 | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d2-drift-promotion.md |
| AC-003 | TC-005 | behavior | `go test ./internal/progressreview -run TestReceiptBindsPinnedGoalRevision -count=1` | 回执绑定 goal 固定 revision 时参与计数，固定 revision 可复现 | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d3-revision-binding.md |
| AC-003 | TC-006 | negative | `go test ./internal/progressreview -run TestForeignRevisionIsNotCountedAndReportsRebindHint -count=1` | 未固定 revision 返回 contract_revision_unpinned；绑定到其它 revision 不计数并上报 rebind_hint | docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d3-revision-binding.md |

## 回归与门禁

实现轮次除上述 6 条 TC 外，还需跑全量 `go test ./... -count=1` 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。
