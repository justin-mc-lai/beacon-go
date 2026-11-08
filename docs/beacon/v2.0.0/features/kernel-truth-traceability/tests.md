---
slug: kernel-truth-traceability
version: v2.0.0
materials_status: current
---

# Tests: kernel-truth-traceability

每条 AC 至少一条执行层 TC。Command 是 go test 选择器，TASK 在实现分支上写入对应测试后可直接运行；Evidence 列指向实现与 QA 轮次产出的包内证据，conformance 的 `ac_tc_evidence_coverage` 检查要求该文件存在且位于本包 `evidence/` 目录内。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestConformanceTraceabilityAllCovered -count=1` | 完整 fixture（2 条 AC、2 条非软 TC、2 个存在的 evidence 文件）的 ac_tc_evidence_coverage 为 pass，整体 conformance pass | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-covered.json |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestConformanceTraceabilityMissingTC -count=1` | 删除 AC-002 的 TC 行后检查 fail，reason code 为 traceability_missing_tc，detail 精确包含 AC-002 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-missing-tc.json |
| AC-002 | TC-003 | behavior | `go test ./internal/orchestrator -run TestConformanceTraceabilityMissingEvidence -count=1` | Evidence 文件不存在时检查 fail，reason code 为 traceability_missing_evidence，detail 包含 AC-001/TC-001 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-missing-evidence.json |
| AC-002 | TC-004 | behavior | `go test ./internal/cli -run TestGoalConformanceTraceabilityExitCode -count=1` | 缺 TC fixture 上 goal conformance 退出码非 0，stdout 文本与 --json 输出都带 traceability_missing_tc，failed 数组含 ac_tc_evidence_coverage | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-cli-exit.txt |
| AC-002 | TC-005 | negative | `go test ./internal/orchestrator -run TestConformanceTraceabilityFailClosed -count=1` | tests.md 缺失、覆盖矩阵无 AC ID 表头、truth 无 AC 定义三种 fixture 均 fail，不出现 pass 或 not_required | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-failclosed.json |
| AC-003 | TC-006 | regression | `go test ./internal/orchestrator -run TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict -count=1` | 未声明 traceability_required 的冻结包新增项 status 为 not_required，退出判定仍由 truth_frozen、gates_evaluated、verifier_attached、state_exported 四项决定 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-not-required.json |
| AC-004 | TC-007 | behavior | `go test ./internal/orchestrator -run TestRefreezeRecordsACDiff -count=1` | 改 AC-002 文本并新增 AC-004 后 refreeze，change 文档 ## AC Diff 的 changed 含 AC-002、added 含 AC-004、removed 为空，changed 行含 before 与 after 两个不同的 SHA256 前缀 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/change-ac-diff.md |
| AC-004 | TC-008 | behavior | `go test ./internal/cli -run TestTruthShowExposesChangeACDiff -count=1` | refreeze 后 truth show --json 的 change_ac_diff 与 change 文档 ## AC Diff 的 added/changed/removed 一致 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/truth-show-change-ac-diff.json |
| AC-004 | TC-009 | negative | `go test ./internal/orchestrator -run TestRefreezeACDiffFailClosed -count=1` | change 文档不可写时 refreeze 返回非空 error，包状态保持 change_pending，事件流不出现 truth_refrozen | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/refreeze-ac-diff-failclosed.txt |
| AC-005 | TC-010 | regression | `go test ./internal/cli -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision' -count=1` | 三个 AC-009 提案门测试原样通过，无测试文件改写 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/ac009-regression.txt |
| AC-005 | TC-011 | e2e | `go test ./internal/orchestrator -run TestDeliveryParadigmEndToEnd -count=1` | delivery-paradigm 端到端测试原样通过，四项 conformance 行为不变 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/delivery-paradigm-e2e.txt |
| AC-005 | TC-012 | regression | `go test ./... -count=1` | 全仓测试退出码 0，无新增失败用例 | docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/go-test-all.txt |
