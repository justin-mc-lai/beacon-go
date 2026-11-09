# AC-002 证据：replay 不重复生效，resume 输出已证明阶段清单

## 判据（原文）

replay 不重复生效；resume 输出已证明阶段清单。

## 源锚点

loopx @ c9c0644，`loopx/control_plane/turn_driver/turn_journal_effects.ts` L185-L195 命中 `operationId` 即返回 `replayed: true, appended: false`，不再执行副作用；`turn_journal.ts` L132-L146、L623-L630 用 `legalPhaseAdvances` 保证阶段单调、`completed_phases` 构成严格有序前缀；`recovery.py` L105-L125 的 `build_turn_recovery_audit` 对外输出 `completed_phases`。摘录见本包 `evidence/source-anchors.md` AC-002 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/effectjournal -run TestReplaySkipsSucceededEffectAndResumeListsProvenStages -count=1` |
| TC-004 | `go test ./internal/effectjournal -run TestFailedOrIncompleteRecordsAreNotProvenStages -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:18:55Z（UTC）
- 分支：`feat/kernel-effect-journal`
- 实现提交：`e7fd1ba`（feat(effectjournal): add idempotent effect journal with replay and resume proof）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-003 | `go test ./internal/effectjournal -run TestReplaySkipsSucceededEffectAndResumeListsProvenStages -count=1` | `ok  	beacon-go/internal/effectjournal	0.230s` | 0 |
| TC-004 | `go test ./internal/effectjournal -run TestFailedOrIncompleteRecordsAreNotProvenStages -count=1` | `ok  	beacon-go/internal/effectjournal	0.228s` | 0 |

原始输出：

```text
$ go test ./internal/effectjournal -run TestReplaySkipsSucceededEffectAndResumeListsProvenStages -count=1
ok  	beacon-go/internal/effectjournal	0.230s
EXIT=0
$ go test ./internal/effectjournal -run TestFailedOrIncompleteRecordsAreNotProvenStages -count=1
ok  	beacon-go/internal/effectjournal	0.228s
EXIT=0
```
