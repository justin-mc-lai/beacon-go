# AC-002 证据：strict_receipt 在 premerge 校验回执与精确当前 diff SHA 一致

## 判据（原文）

`strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `capabilities/change_quality/`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/changequality -run TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA -count=1` |
| TC-004 | `go test ./internal/changequality -run TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

