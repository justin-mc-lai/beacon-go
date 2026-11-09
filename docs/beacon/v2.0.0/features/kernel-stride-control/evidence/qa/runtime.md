# Runtime QA Verdict: kernel-stride-control

- **verdict**: PASS
- **current_stage**: qa / Freeze/Completed
- **target_commit**: `c217994b72d9f8b13518ff1a498e5dfbff2ec706` (branch `feat/kernel-stride-control`, merged with main `8303a35`)
- **evaluator**: QA 验收 · agy（视觉/端侧 & 独立运行时验收）
- **evaluated_at**: 2026-11-09T08:12:00+08:00

---

## 1. 冻结 TC 逐条实测（8 条全部独立实跑）

按 `tests.md` Command 原样实跑，并在 `-v` 模式下核对 `=== RUN` 与 `--- PASS` 真实计数，杜绝假绿。

| TC ID | AC ID | Package | Command | Exit Code | RUN Count | PASS Count | Duration | Verdict |
|-------|-------|---------|---------|-----------|-----------|------------|----------|---------|
| TC-001 | AC-001 | internal/stride | `go test ./internal/stride -run TestRecordStrideBoundariesAndStopReason -count=1` | 0 | 1 | 1 | 0.222s | PASS |
| TC-002 | AC-001 | internal/orchestrator | `go test ./internal/orchestrator -run TestTurnRecordCarriesStrideRecord -count=1` | 0 | 1 | 1 | 0.372s | PASS |
| TC-003 | AC-002 | internal/stride | `go test ./internal/stride -run TestNumericCeilingIsNotCompletion -count=1` | 0 | 1 | 1 | 0.234s | PASS |
| TC-004 | AC-002 | internal/stride | `go test ./internal/stride -run TestSemanticCloseoutRequiredForCompletion -count=1` | 0 | 1 | 1 | 0.247s | PASS |
| TC-005 | AC-003 | internal/stride | `go test ./internal/stride -run TestInnerLoopCannotCrossAuthorityBoundary -count=1` | 0 | 1 | 1 | 0.246s | PASS |
| TC-006 | AC-003 | internal/orchestrator | `go test ./internal/orchestrator -run TestOuterContinuityCannotSkipSettlement -count=1` | 0 | 1 | 1 | 0.351s | PASS |
| TC-007 | AC-004 | internal/stride | `go test ./internal/stride -run TestSchedulerWakeIsNotProgress -count=1` | 0 | 1 | 1 | 0.240s | PASS |
| TC-008 | AC-004 | internal/stride | `go test ./internal/stride -run TestStateAdvanceCountsAsProgress -count=1` | 0 | 1 | 1 | 0.245s | PASS |

---

## 2. 全量测试结果

执行命令：
```bash
go test ./... -count=1
```

- **Exit Code**: `0`
- **Total Packages**: 17
  - **ok packages**: 16
    - `beacon-go/desktop` (1.799s)
    - `beacon-go/internal/agentapi` (2.911s)
    - `beacon-go/internal/benchgate` (1.029s)
    - `beacon-go/internal/cli` (14.985s)
    - `beacon-go/internal/core` (0.818s)
    - `beacon-go/internal/evalgate` (51.835s)
    - `beacon-go/internal/gate` (1.265s)
    - `beacon-go/internal/harness` (11.085s)
    - `beacon-go/internal/llm` (2.220s)
    - `beacon-go/internal/migration` (2.597s)
    - `beacon-go/internal/orchestrator` (89.857s)
    - `beacon-go/internal/session` (2.356s)
    - `beacon-go/internal/store` (2.561s)
    - `beacon-go/internal/stride` (2.044s)
    - `beacon-go/internal/workspace` (2.886s)
    - `beacon-go/skills` (1.987s)
  - **no-test packages**: 1 (`beacon-go/cmd/beacon`)
  - **FAIL count**: 0
  - **panic count**: 0

### 性能与耗时对比
- `internal/evalgate`: 实测 51.835s（main 8303a35 基线约 55.9s，无劣化）
- `internal/orchestrator`: 实测 89.857s（main 8303a35 基线约 95.0s，无劣化）
- 整体测试耗时正常，未引入任何阻塞性延迟或锁争用。

---

## 3. 运行时行为抽查与核验

### 3.1 `internal/stride` 停止原因与完成判定
- **numeric_ceiling 触顶判定**：`Evaluate()` 遇到 `StopNumericCeiling` 时，强制设置 `Outcome = OutcomeIncomplete`；若携带非空 closeout 会直接抛出 `ErrCloseoutNotAllowed`（`stride_closeout_not_allowed`）。`BuildTurn` 计算的 `Complete` 恒为 `false`，`Progress` 恒为 `0`。
- **scheduler_wake 调度唤醒判定**：`Evaluate()` 遇到 `StopSchedulerWake` 时，强制设置 `Outcome = OutcomeIncomplete`，不计入进展，`Complete` 恒为 `false`。
- **semantic_closeout 完成判定**：只有当 `StopReason == StopSemanticCloseout` 且 `Closeout != ""` 时，`Outcome` 才被置为 `OutcomeComplete`。缺少 closeout 证明时即使原因为 `StopSemanticCloseout` 也判为 `OutcomeIncomplete`。
- **effect 层越权防护**：`Guard()` 对 `LayerEffect` 检查：
  - 发生 `CrossesAuthority` 或 `BorrowOuterAuthority` 时，记录被标记为 `StopAuthorityBoundary`，`OutcomeBlocked`，并返回 `*BoundaryError{Reason: "authority_boundary"}` 拒绝执行，严禁借用外层权限。
  - 发生 `CrossesAcceptance` 时，记录被标记为 `StopAcceptanceBoundary`，`OutcomeBlocked`，返回 `*BoundaryError{Reason: "acceptance_boundary"}`。

### 3.2 `internal/orchestrator` 状态持久化与阻断行为
- **stride 记录持久化与只读查回**：
  - `settleStageAdvance` / `SettleTurn` 在事务中将该 turn 的 stride 事实写入 `runMeta.StrideTurns`，并随 run snapshot 落地存储。
  - 通过新建 Engine 实例重新从底层 store 读取验证，`StrideTurns(runID)` 能完整还原该 turn 的 `TurnID`、`Settlement` 以及 3 层 stride 记录（`effect`、`delivery`、`authority`），证明数据真正持久化而非仅存进程内存。
- **缺失 settlement 时 Tick 阻断**：
  - `ContinueOuterTurn(runID)` 检查上一 turn 的 `Settled()` 状态；若缺少 settlement，立即返回 `ReasonSettlementRequired` 错误。
  - `TickBy` 在推进前调用 `ContinueOuterTurn`，遇缺少 settlement 时执行 `applyBlockBatched`，将 Run 置为 Blocked（`reason_code = settlement_required`），不前进 stage，不写入新 turn，不产生任何进展或完成判定。补全 settlement 后恢复放行。

---

## 4. 验收结论与纪律核验

- **Verdict**: **PASS**
- **只读纪律**:
  - 未执行切分支、commit、push、merge 等写分支行为。
  - 冻结包 `truth.md` / `tests.md` / `tasks.md` / `evidence.md` 及既有 evidence 逐字节未变。
  - 仅写入本文件 `evidence/qa/runtime.md`。
