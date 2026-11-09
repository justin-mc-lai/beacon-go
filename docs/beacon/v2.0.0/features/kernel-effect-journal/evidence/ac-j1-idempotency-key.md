# AC-001 证据：implement 副作用带 idempotency key 并写 effect journal

## 判据（原文）

implement 阶段每个副作用带 idempotency key 并写 effect journal。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `control_plane/effect_runtime*`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/effectjournal -run TestImplementEffectCarriesIdempotencyKeyAndWritesJournal -count=1` |
| TC-002 | `go test ./internal/effectjournal -run TestEffectWithoutIdempotencyKeyIsRefused -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

