---
slug: kernel-stride-control
version: v2.0.0
materials_status: current
---

# Tests: kernel-stride-control

每条 AC 至少两条执行层 TC：一条在 `internal/stride` 验证记录与判定本身，一条在 turn 结算路径上验证集成行为。Command 一律是可直接运行的 `go test` 选择器；实现轮次按此命名落地测试，并把实跑输出追加到对应的包内证据文件。AC-S1 至 AC-S4 与 AC-001 至 AC-004 的对应关系见 truth.md 的 AC ID 映射表。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | `go test ./internal/stride -run TestRecordStrideBoundariesAndStopReason -count=1` | 一次记录同时给出 effect、delivery、authority 三层的 boundary 与 stop_reason，字段非空且 stop_reason 落在受控词表内 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-001-stride-record.md |
| AC-001 | TC-002 | e2e | `go test ./internal/orchestrator -run TestTurnRecordCarriesStrideRecord -count=1` | turn 结算后能从只读查询取到该 turn 的 stride 记录，turn_id 与本次 turn 一致，三层记录都在 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-001-stride-record.md |
| AC-002 | TC-003 | negative | `go test ./internal/stride -run TestNumericCeilingIsNotCompletion -count=1` | stop_reason=numeric_ceiling 的记录得到 outcome=incomplete，不因触顶被标记完成，closeout 为空 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-002-semantic-ceiling.md |
| AC-002 | TC-004 | behavior | `go test ./internal/stride -run TestSemanticCloseoutRequiredForCompletion -count=1` | stop_reason=semantic_closeout 且 closeout 非空才得到 outcome=complete；缺少 closeout 的同类记录判为未完成 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-002-semantic-ceiling.md |
| AC-003 | TC-005 | negative | `go test ./internal/stride -run TestInnerLoopCannotCrossAuthorityBoundary -count=1` | 内层 effect 层请求越过权限或接受边界时被拒，记录 stop_reason=authority_boundary 或 acceptance_boundary，不得续借外层授权 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-003-layer-boundaries.md |
| AC-003 | TC-006 | e2e | `go test ./internal/orchestrator -run TestOuterContinuityCannotSkipSettlement -count=1` | 外层连续续跑前没有 settlement 记录时被阻断，不得写入进展或完成判定 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-003-layer-boundaries.md |
| AC-004 | TC-007 | negative | `go test ./internal/stride -run TestSchedulerWakeIsNotProgress -count=1` | scheduler_wake 只触发状态检查，不增加语义进展计数，也不产生 complete 判定 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-004-scheduler-wake.md |
| AC-004 | TC-008 | behavior | `go test ./internal/stride -run TestStateAdvanceCountsAsProgress -count=1` | 状态实际推进并带 settlement 时才计为语义进展，与 scheduler_wake 的记录可区分 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-004-scheduler-wake.md |

## 回归与门禁

实现轮次除上述 8 条 TC 外，还需跑全量 `go test ./... -count=1` 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。
