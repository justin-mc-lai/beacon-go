# AC-001 证据：external 与 manual run 留下真实 stage_executed

## 判据（原文）

external 与 manual run 记录阶段完成时，内核写入一条 stage_executed 事件，payload 含 source=external、executor=external、by 身份与录制标记字段 false，外部调用者无法自行伪造该事件。

## 源锚点

- internal/orchestrator/tick.go L113-L200：manual 与 external 分支只写 recordStageExit 与 stage_advanced。
- internal/orchestrator/delivery_paradigm.go L203-L240：stage_exit_evaluated 的 executor 与 completed_by 由内核填写。
- internal/orchestrator/executor.go L49-L72：录制执行器写 placeholder=true 的 stage_executed。
- 实跑 run_aa702f8da68fe33f23214ae3 全程只有 1 条 stage_executed，且是录制态事件，详见 source-anchors.md。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | go test ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1 |
| TC-002 | go test ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1 |

## 执行记录（实现轮）

- 执行时间：2026-11-09T10:53Z（UTC，本机 18:53 +08:00）
- 分支：`feat/kernel-external-stage-evidence`
- 实现提交：`794433f`（feat(orchestrator): record real external stage evidence and reject recording placeholders）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

实现要点：manual/external 分支在 tick 的推进事务内调用 `recordManualStageExecuted`（`internal/orchestrator/external_stage_evidence.go:24`），写入字段全部由内核生成，只有 `by` 取自调用方（`internal/orchestrator/tick.go:271`）。事件与 `stage_exit_evaluated`、`stage_advanced` 同一事务提交，阶段完成不会只留推进记录。

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-001 | `go test ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.377s` | 0 |
| TC-002 | `go test ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.433s` | 0 |

原始输出：

```text
$ go test ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1 -v
=== RUN   TestExternalStageCompletionRecordsRealStageExecuted
--- PASS: TestExternalStageCompletionRecordsRealStageExecuted (0.07s)
PASS
ok  	beacon-go/internal/orchestrator	0.377s
EXIT=0
$ go test ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1 -v
=== RUN   TestExternalStageEventCannotBeForgedExternally
--- PASS: TestExternalStageEventCannotBeForgedExternally (0.11s)
PASS
ok  	beacon-go/internal/orchestrator	0.433s
EXIT=0
```
