# Evidence Index: kernel-truth-traceability

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| freeze | acceptance_truth | freeze-log.md | recorded | truth |
| coverage pass | support_advisory | traceability-covered.json | recorded | qa |
| missing TC | support_advisory | traceability-missing-tc.json | recorded | qa |
| missing evidence | support_advisory | traceability-missing-evidence.json | recorded | qa |
| fail closed | support_advisory | traceability-failclosed.json | recorded | qa |
| legacy compat | support_advisory | traceability-not-required.json | recorded | qa |
| change diff | support_advisory | change-ac-diff.md | recorded | qa |
| truth show diff | support_advisory | truth-show-change-ac-diff.json | recorded | qa |
| CLI exit code | support_advisory | traceability-cli-exit.txt | recorded | qa |
| refreeze fail closed | support_advisory | refreeze-ac-diff-failclosed.txt | recorded | qa |
| AC-009 regression | acceptance_truth | ac009-regression.txt | recorded | qa |
| delivery e2e | acceptance_truth | delivery-paradigm-e2e.txt | recorded | qa |
| full suite | acceptance_truth | go-test-all.txt | recorded | qa |
| QA verdict spec | acceptance_truth | qa-spec.md | planned | qa |
| QA verdict runtime | acceptance_truth | qa-runtime.md | planned | qa |
| QA verdict regression | acceptance_truth | qa-regression.md | planned | qa |

## Coverage Plan

`ac_tc_evidence_coverage` 的 evidence 契约固定为：tests.md 覆盖矩阵每行的 Evidence 列给出项目相对路径，路径位于本包 evidence 目录内且文件存在。实现轮次与三份 QA 轮次把上述 artifact 落到 evidence 目录，qa-spec、qa-runtime、qa-regression 三个 verdict 分别由验收员、QA 验收 · agy、QA 验收 · 回归独立产出，maker 不自证。

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. Statuses move from planned to recorded only when the backing file exists; a verdict line without a backing artifact is not evidence.
