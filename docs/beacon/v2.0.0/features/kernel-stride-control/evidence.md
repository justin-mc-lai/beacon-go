# Evidence Index: kernel-stride-control

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| freeze | acceptance_truth | freeze-log.md | recorded | truth |
| stride record | support_advisory | ac-001-stride-record.md | recorded | qa |
| semantic ceiling | support_advisory | ac-002-semantic-ceiling.md | recorded | qa |
| layer boundaries | support_advisory | ac-003-layer-boundaries.md | recorded | qa |
| scheduler wake | support_advisory | ac-004-scheduler-wake.md | recorded | qa |
| source anchors | support_advisory | source-anchors.md | recorded | qa |
| full suite | acceptance_truth | go-test-all.txt | recorded | qa |
| QA contract | acceptance_truth | qa-contract.md | recorded | qa |

## Coverage Plan

每份 ac-00x 证据文件是一份活文档：冻结轮写入判据原文与 loopx 源锚点，实现轮把对应 TC 的实跑输出追加到 `执行记录` 小节。tests.md 覆盖矩阵每行的 Evidence 列指向这些包内文件，因此 AC 到 TC 到证据的链路在冻结时即可核对。

三份独立 QA verdict 由验收员、QA 验收 · agy、QA 验收 · 回归各自写入包内 qa 目录，命名与检查面见 qa-contract.md；maker 不自证。

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. 状态在底层文件真实存在后才从 planned 变成 recorded；没有底层 artifact 的 verdict 行不算证据。
