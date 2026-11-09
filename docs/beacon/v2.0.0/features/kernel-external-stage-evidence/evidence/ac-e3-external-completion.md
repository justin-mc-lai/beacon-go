# AC-003 证据：receive-result 外部完成加已注册 QA artifacts 作为等价证据

## 判据（原文）

beacon goal receive-result --external 记录的完成事实，连同已注册且内容 hash 校验通过的 QA artifacts，可作为 qa_stage_executed 的等价证据，事件 payload 明确 source=external，并与 AC-001 的逐阶段路径写明取舍关系。

## 源锚点

- internal/orchestrator/delivery_paradigm.go L258-L333：ReceiveExternalResult 注册 artifact，写 external_result_received 与 goal_completed（source=external）。
- internal/orchestrator/artifacts.go L46-L70：QA 类 artifact 判定为 qa_report 精确词条或 qa_ 前缀。
- internal/cli/goal_p2c.go L172-L205：CLI flag 为 --run-id、--external、--by、--summary、--artifact。
- 现状：external_result_received 不影响 qa_stage_executed，详见 source-anchors.md。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | go test ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1 |
| TC-006 | go test ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1 |

## 执行记录（实现轮）

- 执行时间：2026-11-09T10:53Z（UTC，本机 18:53 +08:00）
- 分支：`feat/kernel-external-stage-evidence`
- 实现提交：`794433f`（feat(orchestrator): record real external stage evidence and reject recording placeholders）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

实现要点：`evalQA` 记录带 `source=external` 的 `goal_completed`，并只对同一 run、且至少一份 QA 类 artifact 的 hash 仍与磁盘一致（`store.ArtifactStaleness` 为不 stale）的才算等价证据（`internal/orchestrator/gates_facts.go:279`）。`registerArtifacts` 在调用方未提供 hash 时从磁盘补算 sha256 与字节数（`internal/orchestrator/lanes.go:228`），否则 receive-result 这类只传路径的注册会把好证据误判为 stale。空 hash、文件缺失仍留在 stale 侧，等价路径 fail-closed。

与 AC-001 的取舍：逐阶段的真实 `stage_executed` 是首选路径，优先级更高；等价证据只在没有真实 qa 事件时作为整湖完成的兜底，不能覆盖或替代 AC-001 的逐阶段记录，也不能让未注册、hash 不符或非 QA 类 artifact 通过。

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-005 | `go test ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1` | `--- PASS` / `ok beacon-go/internal/orchestrator 0.531s` | 0 |
| TC-006 | `go test ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1` | `--- PASS`（3 个子用例）/ `ok beacon-go/internal/orchestrator 0.486s` | 0 |

原始输出：

```text
$ go test ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1 -v
=== RUN   TestExternalReceiveResultIsQAStageEvidence
--- PASS: TestExternalReceiveResultIsQAStageEvidence (0.06s)
PASS
ok  	beacon-go/internal/orchestrator	0.531s
EXIT=0
$ go test ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1 -v
=== RUN   TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts
=== RUN   TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/no_qa_artifact
=== RUN   TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/hash_mismatch
=== RUN   TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/missing_source_external
--- PASS: TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts (0.11s)
    --- PASS: TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/no_qa_artifact (0.04s)
    --- PASS: TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/hash_mismatch (0.04s)
    --- PASS: TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts/missing_source_external (0.04s)
PASS
ok  	beacon-go/internal/orchestrator	0.486s
EXIT=0
```
