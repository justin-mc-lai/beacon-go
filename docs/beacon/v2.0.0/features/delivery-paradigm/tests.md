---
slug: delivery-paradigm
version: v2.0.0
materials_status: current
---

# Tests: delivery-paradigm

每个 AC 至少一条执行层 TC。命令是 go test 选择器，TASK-2 在 fix/delivery-paradigm 上写入对应测试后可直接运行；断言列写的是可判定的观察点。

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestManualExecutorRunAdvancesViaTick -count=1» | manual 执行器 run 创建后 goal status 不报 stage_executor_not_registered；一次 goal tick --by room 记录完成并推进 stage |
| AC-001 | TC-002 | behavior | `go test ./internal/cli -run TestGoalRunManualExecutorPrintsTickHint -count=1» | goal run 创建输出包含 goal tick --by <identity> 提示，并说明该执行器不自动驱动 |
| AC-002 | TC-003 | behavior | `go test ./internal/cli -run TestGoalRunRejectsUnknownExecutor -count=1» | 退出码非 0，reason code 为 stage_executor_unsupported，store 中不出现该 run |
| AC-003 | TC-004 | behavior | `go test ./internal/orchestrator -run TestTickPersistsStageExitVerdict -count=1» | 每次推进前写入 evaluated=true 的出口记录；未评估时 blocked，reason code 为 stage_gate_not_evaluated |
| AC-003 | TC-005 | behavior | `go test ./internal/cli -run TestGateStatusShowsPlanAndImplementEvaluated -count=1» | gate status 的 plan 行与 implement 行均为 evaluated=true，不再是 not_evaluated |
| AC-004 | TC-006 | behavior | `go test ./internal/cli -run TestGoalReceiveResultExternalWithoutDelegation -count=1» | 不传 delegation-id 也能回写；artifact 绝对路径进入 artifact_refs；run 记录该湖完成 |
| AC-004 | TC-007 | behavior | `go test ./internal/orchestrator -run TestProgramRouteShowsLakeCompletedAfterExternalResult -count=1» | 回写后 program route 该湖 status=completed |
| AC-005 | TC-008 | behavior | `go test ./internal/orchestrator -run TestProgramRouteNextActionsTopological -count=1» | next actions 按拓扑顺序输出；truth 未冻结湖带 truth freeze 建议与湖 slug |
| AC-006 | TC-009 | behavior | `go test ./internal/gate -run TestIntegrityFlagsUxMismatch -count=1» | finding kind 为 truth_ux_requirement_mismatch，默认 verdict 为 audit |
| AC-006 | TC-010 | behavior | `go test ./internal/cli -run TestGateIntegrityStrictBlocksUxMismatch -count=1» | --strict 时 verdict 为 blocked，退出码非 0 |
| AC-007 | TC-011 | behavior | `go test ./internal/cli -run TestGoalConformanceAllPassAndFailure -count=1» | 四项全 pass 退出码 0；缺 verifier 与 state 时退出码非 0 并列出未过项 |
| AC-008 | TC-012 | e2e | `go test ./internal/orchestrator -run TestDeliveryParadigmEndToEnd -count=1» | plan→truth→design→implement→qa→release 全程推进；plan 出口记录与五个 gate 的 latest verdict 均 evaluated=true |
| AC-009 | TC-013 | regression | `go test ./internal/cli -run TestProgramPlanRequiresProposal -count=1» | 直接 plan 返回 lake_proposal_required，新 program 仍必须先 propose 再 select |
| AC-009 | TC-014 | regression | `go test ./internal/orchestrator -run TestProgramPlanFreezeChange -count=1» | 冻结后直接编辑被拒，change 记录可改，原测试无改动通过 |
| AC-009 | TC-015 | regression | `go test ./internal/orchestrator -run TestProgramSelectRecordsDecision -count=1» | 只有被选中的湖创建 draft，原测试无改动通过 |

