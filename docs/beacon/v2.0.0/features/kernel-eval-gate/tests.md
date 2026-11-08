---
slug: kernel-eval-gate
version: v2.0.0
materials_status: current
---

# Tests: kernel-eval-gate

每条 AC 至少一条执行层 TC。Command 一律是 `go test` 选择器，TASK 在实现分支上写入 `internal/evalgate` 的用例后可逐条运行；Evidence 列指向实现与 QA 轮次产出的包内证据，conformance 的 `ac_tc_evidence_coverage` 检查要求该文件存在且位于本包 `evidence/` 目录内。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | e2e | `go test ./internal/evalgate -count=1` | 四类用例全部执行且与期望一致，整集退出 0，无 skip | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-suite.txt |
| AC-001 | TC-002 | behavior | `go test ./internal/evalgate -run TestEvalRegistryCoverage -count=1` | 注册表覆盖 stage_gate、conformance、rejection、no_degradation 四类，每个用例 ID 唯一且带期望退出码与 reason code，未知 class 与缺失期望被判 fail | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-registry.json |
| AC-002 | TC-003 | behavior | `go test ./internal/evalgate -run TestEvalConformancePositive -count=1` | 完整 fixture 的 conformance pass，ac_tc_evidence_coverage 为 pass，退出码 0 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-positive.json |
| AC-002 | TC-004 | behavior | `go test ./internal/evalgate -run TestEvalConformanceMissingTC -count=1` | 缺 TC 的 fixture 退出码非 0，reason code 为 traceability_missing_tc，detail 点名被删的 AC | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-missing-tc.json |
| AC-002 | TC-005 | negative | `go test ./internal/evalgate -run TestEvalConformanceEvidence -count=1` | 证据文件不存在、路径越界、Evidence 列缺失三种 fixture 均非 0，reason code 为 traceability_missing_evidence 并点名 AC/TC | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-evidence.json |
| AC-002 | TC-006 | negative | `go test ./internal/evalgate -run TestEvalConformanceFailClosed -count=1` | tests.md 缺失、覆盖矩阵不可解析、truth 无 AC 定义三种 fixture 均非 0 且携带 reason code，文本与 --json 不出现 pass 与 not_required | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-failclosed.json |
| AC-002 | TC-007 | regression | `go test ./internal/evalgate -run TestEvalConformanceNotRequiredCompat -count=1` | 未声明 traceability_required 的历史冻结包新增项为 not_required，原有四项判定名称与退出码不变 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-not-required.json |
| AC-003 | TC-008 | behavior | `go test ./internal/evalgate -run TestEvalStageGate -count=1` | truth 未冻结时 implement 门 blocked 并携带 stage_exit_truth_not_frozen；事实齐备时 implement 门 pass | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-stage-gate.json |
| AC-003 | TC-009 | negative | `go test ./internal/evalgate -run TestEvalRejectionPaths -count=1` | 未知 gate type、缺 evaluator、未知 feature、未知 run 四种非法输入均非 0，reason code 分别为 gate_type_unknown、gate_evaluator_required、feature_not_found 与 conformance 的非零 reason，且不写入 pass 判定 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-rejection-paths.json |
| AC-004 | TC-010 | negative | `go test ./internal/evalgate -run TestEvalMismatchFailsLoudly -count=1` | 注入与真实结果不符的期望后评测集非 0 退出，failed 列表包含该 case ID，输出含 case=<ID> 与 expected/actual，不出现 pass 与 skip | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-mismatch-fails.json |
| AC-004 | TC-011 | e2e | `go test ./internal/evalgate -run TestEvalDeterministicOffline -count=1` | 连续两次运行的归一化报告逐字节一致；HTTP_PROXY、HTTPS_PROXY、ALL_PROXY 指向 127.0.0.1:1 时整集仍通过 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-deterministic-offline.txt |
| AC-005 | TC-012 | regression | `go test ./... -count=1` | 全仓测试退出码 0，无新增失败用例 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/go-test-all.txt |
| AC-005 | TC-013 | regression | `go test ./internal/cli ./internal/orchestrator -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision|TestDeliveryParadigmEndToEnd' -count=1` | AC-009 三个提案门测试与 delivery-paradigm 端到端测试原样通过，无测试文件改写 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/ac009-regression.txt |
