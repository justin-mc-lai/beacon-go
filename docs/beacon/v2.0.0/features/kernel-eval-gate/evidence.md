# Evidence Index: kernel-eval-gate

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| freeze | acceptance_truth | freeze-log.md | recorded | truth |
| eval suite | support_advisory | evalgate-suite.txt | planned | qa |
| eval registry | support_advisory | evalgate-registry.json | planned | qa |
| conformance pass | support_advisory | evalgate-conformance-positive.json | planned | qa |
| conformance missing TC | support_advisory | evalgate-conformance-missing-tc.json | planned | qa |
| conformance evidence | support_advisory | evalgate-conformance-evidence.json | planned | qa |
| conformance fail closed | support_advisory | evalgate-conformance-failclosed.json | planned | qa |
| conformance compat | support_advisory | evalgate-conformance-not-required.json | planned | qa |
| stage gate | support_advisory | evalgate-stage-gate.json | planned | qa |
| rejection paths | support_advisory | evalgate-rejection-paths.json | planned | qa |
| no degradation | support_advisory | evalgate-mismatch-fails.json | planned | qa |
| determinism offline | support_advisory | evalgate-deterministic-offline.txt | planned | qa |
| full suite | acceptance_truth | go-test-all.txt | planned | qa |
| AC-009 regression | acceptance_truth | ac009-regression.txt | planned | qa |
| QA verdict spec | acceptance_truth | qa-spec.md | planned | qa |
| QA verdict runtime | acceptance_truth | qa-runtime.md | planned | qa |
| QA verdict regression | acceptance_truth | qa-regression.md | planned | qa |

## Coverage Plan

`ac_tc_evidence_coverage` 的 evidence 契约固定为：tests.md 覆盖矩阵每行的 Evidence 列给出项目相对路径，路径位于本包 evidence 目录内且文件存在。实现轮次把上述 artifact 落到 evidence 目录并替换 planned，三份 QA verdict 由验收员、QA 验收 · agy、QA 验收 · 回归各自独立写入，maker 不自证。

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. 状态在底层文件真实存在后才从 planned 变成 recorded；没有底层 artifact 的 verdict 行不算证据。
