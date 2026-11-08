# AC-001 证据：每次 turn 记录三层 stride 边界与停止原因

## 判据（原文）

每次 turn 记录 `stride{effect,delivery,authority}` 的边界与停止原因。

## 源锚点

loopx @ c9c0644，docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md 的 ## 1. Decision Summary（L24-L35）与 ## 5. The Three-Layer Model（L192-L245）。三层定义见 truth.md 的 Stride 记录契约。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/stride -run TestRecordStrideBoundariesAndStopReason -count=1` |
| TC-002 | `go test ./internal/orchestrator -run TestTurnRecordCarriesStrideRecord -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。
