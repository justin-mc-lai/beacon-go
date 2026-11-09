# AC-002 证据：qa 门拒收录制态 stage_executed

## 判据（原文）

qa 门只接受真实 stage_executed 事件作为 qa_stage_executed 依据；录制执行器产生的录制态事件不得让 qa 门通过，依靠该事件通过的历史路径一律 fail-closed。

## 源锚点

- internal/orchestrator/gates_facts.go L227-L241：evalQA 只判 event_type=stage_executed、stage=qa、outcome!=failed。
- internal/gate/evaluate.go L258-L282：qa_stage_executed 是三项 check 之一。
- 实跑：09:37:04 的录制态事件让 09:37:52 的 qa 门变为 pass，详见 source-anchors.md。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | go test ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1 |
| TC-004 | go test ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1 |

## 执行记录（实现轮）

- 执行时间：2026-11-09T10:53Z（UTC，本机 18:53 +08:00）
- 分支：`feat/kernel-external-stage-evidence`
- 实现提交：`794433f`（feat(orchestrator): record real external stage evidence and reject recording placeholders）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

实现要点：`evalQA` 只在 `placeholder` 字段存在且为 `false`、`recording` 缺省或为 `false` 时才计为已执行（`internal/orchestrator/gates_facts.go:252`）。字段缺失按录制态处理，录制执行器写的 `placeholder:true` 一律不计，原先 09:37 那条假通过路径被关闭。Subprocess 与 Ralph 执行器本来写 `placeholder:false, real:true`，真实执行路径不受影响。

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-003 | `go test ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.435s` | 0 |
| TC-004 | `go test ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.392s` | 0 |

原始输出：

```text
$ go test ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1 -v
=== RUN   TestQAGateRejectsRecordingExecutorStageEvent
--- PASS: TestQAGateRejectsRecordingExecutorStageEvent (0.06s)
PASS
ok  	beacon-go/internal/orchestrator	0.435s
EXIT=0
$ go test ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1 -v
=== RUN   TestQAGateAcceptsRealExternalStageEvent
--- PASS: TestQAGateAcceptsRealExternalStageEvent (0.05s)
PASS
ok  	beacon-go/internal/orchestrator	0.392s
EXIT=0
```
