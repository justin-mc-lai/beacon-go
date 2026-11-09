# AC-002 证据：replay 不重复生效，resume 输出已证明阶段清单

## 判据（原文）

replay 不重复生效；resume 输出已证明阶段清单。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `control_plane/effect_runtime*` 与 `settlement_driver.py`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/effectjournal -run TestReplaySkipsSucceededEffectAndResumeListsProvenStages -count=1` |
| TC-004 | `go test ./internal/effectjournal -run TestFailedOrIncompleteRecordsAreNotProvenStages -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

