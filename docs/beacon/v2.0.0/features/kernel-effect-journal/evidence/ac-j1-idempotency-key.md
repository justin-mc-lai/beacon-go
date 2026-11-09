# AC-001 证据：implement 副作用带 idempotency key 并写 effect journal

## 判据（原文）

implement 阶段每个副作用带 idempotency key 并写 effect journal。

## 源锚点

loopx @ c9c0644，`loopx/control_plane/turn_driver/turn_journal_effects.ts` L47-L53 定义 `journalEffectId` 幂等键，L140-L165 在 `commitTurnJournal` 中原子写入本地 journal；`settlement_driver.py` L1-L8、L49-L65 用 `effect_ids_match` 核验执行步骤与回执中的 effect id；`effect_program.ts` L683-L730 的 `settlementReceipt` 携带强类型 `effect_id`。摘录见本包 `evidence/source-anchors.md` AC-001 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/effectjournal -run TestImplementEffectCarriesIdempotencyKeyAndWritesJournal -count=1` |
| TC-002 | `go test ./internal/effectjournal -run TestEffectWithoutIdempotencyKeyIsRefused -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:18:55Z（UTC）
- 分支：`feat/kernel-effect-journal`
- 实现提交：`e7fd1ba`（feat(effectjournal): add idempotent effect journal with replay and resume proof）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-001 | `go test ./internal/effectjournal -run TestImplementEffectCarriesIdempotencyKeyAndWritesJournal -count=1` | `ok  	beacon-go/internal/effectjournal	0.269s` | 0 |
| TC-002 | `go test ./internal/effectjournal -run TestEffectWithoutIdempotencyKeyIsRefused -count=1` | `ok  	beacon-go/internal/effectjournal	0.247s` | 0 |

原始输出：

```text
$ go test ./internal/effectjournal -run TestImplementEffectCarriesIdempotencyKeyAndWritesJournal -count=1
ok  	beacon-go/internal/effectjournal	0.269s
EXIT=0
$ go test ./internal/effectjournal -run TestEffectWithoutIdempotencyKeyIsRefused -count=1
ok  	beacon-go/internal/effectjournal	0.247s
EXIT=0
```
