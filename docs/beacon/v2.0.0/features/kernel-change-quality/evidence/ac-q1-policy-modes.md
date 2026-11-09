# AC-001 证据：策略默认 off，可选 safe_fix 或 strict_receipt

## 判据（原文）

策略默认 off，可选 `safe_fix` 或 `strict_receipt`。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `capabilities/change_quality/`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/changequality -run TestChangeQualityPolicyDefaultsToOff -count=1` |
| TC-002 | `go test ./internal/changequality -run TestChangeQualityUnknownModeIsReportedAndNotUpgraded -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

