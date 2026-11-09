# AC-002 证据：连续 N 条 completed 且 drift=true 才提升 replan，非 completed 永不算已处理

## 判据（原文）

连续 N 条 completed 且 drift=true 才提升为既有 replan 义务；unknown/abstained/failed/stale 永不算已处理。

## 源锚点

loopx @ c9c0644，`loopx/control_plane/work_items/progress_review_policy.py` L19-L55 定义默认阈值 `PROGRESS_REVIEW_DEFAULT_DRIFT_THRESHOLD = 2`，只有状态为 completed 且 drift_signal 为真才累计计数并触发 assist 义务；`loopx/capabilities/progress_review/receipt.py` L28-L34 定义状态词表 `("completed", "abstained", "failed", "not_evaluated", "stale")`。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/progressreview -run TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation -count=1` |
| TC-004 | `go test ./internal/progressreview -run TestNonCompletedStatusesNeverCountAsHandled -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
