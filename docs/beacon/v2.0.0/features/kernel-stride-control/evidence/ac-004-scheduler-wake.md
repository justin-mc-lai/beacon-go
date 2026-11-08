# AC-004 证据：调度唤醒不算语义进展

## 判据（原文）

调度唤醒不算语义进展。

## 源锚点

loopx @ c9c0644，docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md 的 ### 6.4 Scheduler cadence is not semantic cadence（L354-L368）：唤醒信号只说明宿主可以检查当前状态，不证明出现了新 Todo、切片结束或需要人工关注。

## 验收方式

| TC | Command |
|----|---------|
| TC-007 | `go test ./internal/stride -run TestSchedulerWakeIsNotProgress -count=1` |
| TC-008 | `go test ./internal/stride -run TestStateAdvanceCountsAsProgress -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
