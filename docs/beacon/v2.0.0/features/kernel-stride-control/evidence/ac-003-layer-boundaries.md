# AC-003 证据：内层不越权，外层不跳 settlement

## 判据（原文）

内层工具循环不得跨越权限/接受边界；外层连续不得跳过 settlement。

## 源锚点

loopx @ c9c0644，docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md 的 ### 6.2 Inner continuation cannot borrow outer authority（L339-L345）与 ### 6.3 Outer continuity cannot excuse missing settlement（L347-L352）。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/stride -run TestInnerLoopCannotCrossAuthorityBoundary -count=1` |
| TC-006 | `go test ./internal/orchestrator -run TestOuterContinuityCannotSkipSettlement -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
