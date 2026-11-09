# Evidence Index: kernel-effect-journal

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | draft | truth |
| tests | test_truth | tests.md | draft | qa |
| tasks | support_advisory | tasks.md | present | truth |
| idempotency key | support_advisory | ac-j1-idempotency-key.md | recorded | qa |
| replay + resume proof | support_advisory | ac-j2-replay-resume.md | recorded | qa |
| missing journal | support_advisory | ac-j3-missing-journal.md | recorded | qa |
| source anchors | support_advisory | source-anchors.md | pending | qa |
| freeze | acceptance_truth | freeze-log.md | pending | truth |
| full suite | acceptance_truth | go-test-all.txt | pending | qa |
| QA contract | acceptance_truth | qa-contract.md | recorded | qa |

## Coverage Plan

每份 ac-jx 证据文件是一份活文档：冻结轮写入判据原文与 loopx 源锚点，实现轮把对应 TC 的实跑输出追加到 `执行记录` 小节。tests.md 覆盖矩阵每行的 Evidence 列指向这些包内文件，因此 AC 到 TC 到证据的链路在冻结时即可核对。

三份独立 QA verdict 由验收员、QA 验收 · agy、QA 验收 · 回归各自写入包内 qa 目录，命名与检查面见 qa-contract.md；maker 不自证。

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. 状态在底层文件真实存在后才从 planned 变成 recorded；没有底层 artifact 的 verdict 行不算证据。

