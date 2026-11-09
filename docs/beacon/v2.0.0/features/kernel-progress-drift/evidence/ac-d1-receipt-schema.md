# AC-001 证据：progress_review_receipt_v0 schema 与 shadow 模式，默认 off

## 判据（原文）

progress_review_receipt_v0 schema + shadow 模式，默认 off。

## 源锚点

loopx @ c9c0644，`loopx/capabilities/progress_review/receipt.py` L27-L38 定义 `PROGRESS_REVIEW_RECEIPT_SCHEMA_VERSION = "progress_review_receipt_v0"`；`loopx/control_plane/work_items/progress_review_policy.py` L16-L23 定义模式词表 `("off", "shadow", "assist")` 与默认 `PROGRESS_REVIEW_DEFAULT_MODE = "off"`。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/progressreview -run TestProgressReviewReceiptSchemaAndDefaultMode -count=1` |
| TC-002 | `go test ./internal/progressreview -run TestShadowModeRecordsReceiptWithoutReplanObligation -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
