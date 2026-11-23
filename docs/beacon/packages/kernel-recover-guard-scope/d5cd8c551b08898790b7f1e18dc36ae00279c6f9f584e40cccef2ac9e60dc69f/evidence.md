# Evidence Index: kernel-recover-guard-scope

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | recorded | truth |
| tests | test_truth | tests.md | recorded | qa |
| tasks | support_advisory | tasks.md | present | truth |
| recover scope | support_advisory | ac-r1-recover-scope.md | recorded | qa |
| human confirmation | support_advisory | ac-r2-human-confirmation.md | recorded | qa |
| recover trail | support_advisory | ac-r3-recover-trail.md | recorded | qa |
| status truth | support_advisory | ac-r4-status-truth.md | recorded | qa |
| source anchors | support_advisory | source-anchors.md | recorded | qa |
| freeze | acceptance_truth | freeze-log.md | recorded | truth |
| full suite | acceptance_truth | go-test-all.txt | recorded | qa |
| QA contract | acceptance_truth | qa-contract.md | recorded | qa |

## Coverage Plan

每份 ac-r 证据文件是一份活文档：冻结轮写入判据原文、代码锚点与只读实跑事实，实现轮把对应 TC 的实跑输出追加到「执行记录」小节。tests.md 覆盖矩阵每行的 Evidence 列指向这些包内文件，因此 AC 到 TC 到证据的链路在冻结时即可核对。

三份独立 QA verdict 由验收员、QA 验收 · agy、QA 验收 · 回归各自写入包内 qa 目录，命名与检查面见 qa-contract.md；maker 不自证。

## Boundary

evidence.md is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. 状态在底层文件真实存在后才从 planned 变成 recorded；没有底层 artifact 的 verdict 行不算证据。
