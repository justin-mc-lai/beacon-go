# AC-003 证据：safe_fix 只允许一次有界修复，越界 fail-closed

## 判据（原文）

`safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `capabilities/change_quality/`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/changequality -run TestSafeFixAllowsOneBoundedRepairWithinAuthorization -count=1` |
| TC-006 | `go test ./internal/changequality -run TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

