# 实现轮实跑记录：kernel-stride-control

- 分支：`feat/kernel-stride-control`
- 基线：`main` @ `210a995`（含冻结包 `53342bd`）
- 实现范围：`internal/stride`（记录结构、受控词表、停止原因判定、内层边界 Guard）、`internal/orchestrator`（turn 结算写入 stride 事实、只读查询、外层续跑 settlement 守卫）
- 实跑时间：2026-11-09 07:50 +08:00

## 冻结 TC 逐条实跑

每条先按 tests.md 的 Command 原样跑一次记退出码，再用同一选择器加 `-v` 跑一次，统计 `=== RUN` 与 `--- PASS` 行数，确认不是 0 匹配的假绿。

| TC | Command | EXIT | RUN | PASS |
|----|---------|------|-----|------|
| TC-001 | `go test ./internal/stride -run TestRecordStrideBoundariesAndStopReason -count=1` | 0 | 1 | 1 |
| TC-002 | `go test ./internal/orchestrator -run TestTurnRecordCarriesStrideRecord -count=1` | 0 | 1 | 1 |
| TC-003 | `go test ./internal/stride -run TestNumericCeilingIsNotCompletion -count=1` | 0 | 1 | 1 |
| TC-004 | `go test ./internal/stride -run TestSemanticCloseoutRequiredForCompletion -count=1` | 0 | 1 | 1 |
| TC-005 | `go test ./internal/stride -run TestInnerLoopCannotCrossAuthorityBoundary -count=1` | 0 | 1 | 1 |
| TC-006 | `go test ./internal/orchestrator -run TestOuterContinuityCannotSkipSettlement -count=1` | 0 | 1 | 1 |
| TC-007 | `go test ./internal/stride -run TestSchedulerWakeIsNotProgress -count=1` | 0 | 1 | 1 |
| TC-008 | `go test ./internal/stride -run TestStateAdvanceCountsAsProgress -count=1` | 0 | 1 | 1 |

## 实现落点

- `internal/stride/stride.go`
  - `Record{turn_id, layer, boundary, stop_reason, closeout, outcome}`，三层 `effect` / `delivery` / `authority`，停止原因受控词表 7 项。
  - `Evaluate`：`numeric_ceiling` 与 `scheduler_wake` 恒为 `incomplete`；只有 `semantic_closeout` 且 `closeout` 非空才为 `complete`；非语义停止携带 `closeout` 一律拒绝（`stride_closeout_not_allowed`）。
  - `Guard`：effect 层越权限/接受边界或续借外层授权即返回 blocked 记录加 `BoundaryError`，调用方不得继续。
  - `BuildTurn`：语义进展只统计已结算 turn 的 delivery 层 complete 记录；effect 层 closeout 不计外层进展。
- `internal/orchestrator/stride_control.go`
  - `StageTurnID` / `StageSettlementRef`：稳定 turn id 与 settlement 标识。
  - `settleStageAdvance`：stage 退出与三条 stride 记录在同一事务内写进 run 快照（`runMeta.StrideTurns`），随 `saveRunOn` 一并落盘；不改 `lifecycle_events` 顺序，既有 trace 断言不变。
  - `StrideTurns`：每次都重新加载 run 快照，不依赖进程内状态。
  - `ContinueOuterTurn`：最近一个 turn 无 settlement 时返回 `settlement_required`。
- `internal/orchestrator/tick.go`
  - Tick 执行前调用 `ContinueOuterTurn`，未结算则阻断并写 `settlement_boundary_blocked`，不写进展与完成判定。
  - stage 前进事务内调用 `settleStageAdvanceOn`，turn 记录与 stage_advanced 同批提交。

## 未做

- 未改冻结包 truth.md / tests.md / tasks.md / evidence.md 与既有 evidence 文件，未勾选任何 `[x]`。
- 未合并 main、未 push、未写三份独立 QA verdict。

## 全量回归

- `go test ./... -count=1` → `EXIT=0`，15 个包 ok、1 个 `no test files`（`cmd/beacon`），无 FAIL/panic。
- 完整输出：`evidence/go-test-stride-all.txt`。
- 首轮全量曾因 `TestTraceOrderAndBundle` 断言事件前缀失败；改为 run 快照持久化后该既有断言原样通过，未改任何既有测试。
