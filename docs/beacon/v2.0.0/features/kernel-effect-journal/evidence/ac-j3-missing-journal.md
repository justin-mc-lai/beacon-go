# AC-003 证据：resume 缺 journal 时 fail-closed，不静默重跑

## 判据（原文）

resume 缺 journal 时 fail-closed，不静默重跑。

## 源锚点

loopx @ c9c0644，`loopx/control_plane/turn_driver/recovery.py` L19-L25、L85-L103 定义 `TurnRecoveryBlockedError` 并在状态不明或 journal 缺失时严禁重试；`turn_journal.ts` L355-L380 返回 `{action: "blocked", reason: "journal_inconsistent", reinvoke_host: false}`；`turn_journal_effects.ts` L165-L178 在既有 journal 冲突时抛 `EffectRuntimeConflictError`。摘录见本包 `evidence/source-anchors.md` AC-003 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/effectjournal -run TestResumeWithValidJournalProceedsFromProof -count=1` |
| TC-006 | `go test ./internal/effectjournal -run TestResumeWithMissingOrCorruptJournalFailsClosed -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:18:55Z（UTC）
- 分支：`feat/kernel-effect-journal`
- 实现提交：`e7fd1ba`（feat(effectjournal): add idempotent effect journal with replay and resume proof）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-005 | `go test ./internal/effectjournal -run TestResumeWithValidJournalProceedsFromProof -count=1` | `ok  	beacon-go/internal/effectjournal	0.237s` | 0 |
| TC-006 | `go test ./internal/effectjournal -run TestResumeWithMissingOrCorruptJournalFailsClosed -count=1` | `ok  	beacon-go/internal/effectjournal	0.237s` | 0 |

原始输出：

```text
$ go test ./internal/effectjournal -run TestResumeWithValidJournalProceedsFromProof -count=1
ok  	beacon-go/internal/effectjournal	0.237s
EXIT=0
$ go test ./internal/effectjournal -run TestResumeWithMissingOrCorruptJournalFailsClosed -count=1
ok  	beacon-go/internal/effectjournal	0.237s
EXIT=0
```
