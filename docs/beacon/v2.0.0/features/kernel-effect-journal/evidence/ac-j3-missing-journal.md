# AC-003 证据：resume 缺 journal 时 fail-closed，不静默重跑

## 判据（原文）

resume 缺 journal 时 fail-closed，不静默重跑。

## 源锚点

见本包 `evidence/source-anchors.md`（上游 loopx `control_plane/effect_runtime*` 与 `settlement_driver.py`，由 BeaconGo 研究 落盘）。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/effectjournal -run TestResumeWithValidJournalProceedsFromProof -count=1` |
| TC-006 | `go test ./internal/effectjournal -run TestResumeWithMissingOrCorruptJournalFailsClosed -count=1` |

## 执行记录

冻结轮只登记判据与验收命令；实现轮运行上述两条命令后，把实际输出与退出码追加到本小节。

