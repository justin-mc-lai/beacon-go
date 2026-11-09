# Spec/AC QA Verdict: kernel-stride-control

- **verdict**: PASS
- **current_stage**: qa / Freeze/Completed
- **target_commit**: `c217994b72d9f8b13518ff1a498e5dfbff2ec706`（分支 `feat/kernel-stride-control`，已含 main `8303a35`）
- **evaluator**: 验收员（独立 spec/AC 验收，maker≠checker）
- **evaluated_at**: 2026-11-09T08:2x+08:00
- **scope**: 只判 AC 判据、映射与冻结包完整性；运行时行为与负向回归分别由 QA agy、QA 回归独立判定

---

## 1. AC 原文映射核对

| 上游编号 | 本包 AC ID | 上游原文 | truth.md 判据 | 结论 |
|---|---|---|---|---|
| AC-S1 | AC-001 | 每次 turn 记录 stride{effect,delivery,authority} 的边界与停止原因。 | 逐字相同 | 一致，无扩张 |
| AC-S2 | AC-002 | 数值上限只作安全上限，触顶不算完成；完成必须有语义 closeout。 | 逐字相同 | 一致，无扩张 |
| AC-S3 | AC-003 | 内层工具循环不得跨越权限/接受边界；外层连续不得跳过 settlement。 | 逐字相同 | 一致，无扩张 |
| AC-S4 | AC-004 | 调度唤醒不算语义进展。 | 逐字相同 | 一致，无扩张 |

`truth.md` 的 "AC ID 映射" 表与 "Acceptance Criteria" 表两条链路一一对应，不存在多出或缺失的 AC。附加的 "Stride 记录契约" 只是把 AC-001 要求的记录字段具体化（turn_id / layer / boundary / stop_reason / closeout / outcome），未新增判据、未放宽判据。

## 2. 冻结包完整性（相对 `53342bd`）

`git diff --name-status 53342bd HEAD -- docs/.../kernel-stride-control/` 只有两条新增：

```
A  evidence/go-test-stride-all.txt
A  evidence/impl-stride-run.md
```

四件套 blob 逐字节一致：

| 文件 | 53342bd | HEAD |
|---|---|---|
| truth.md | `03ed94bc…` | `03ed94bc…` |
| tests.md | `f5e6c157…` | `f5e6c157…` |
| tasks.md | `a1dfb6e0…` | `a1dfb6e0…` |
| evidence.md | `c97886d1…` | `c97886d1…` |

- `tasks.md` 仅 TASK-000（冻结轮自身）为 `[x]`，TASK-001..010 全部未勾选，实现轮未产生任何新勾选。
- 12 条 TASK 的 `evidence=` 全部指向包内已存在普通文件（freeze-log、ac-001..004、go-test-all、qa-contract），无一指向自身或尚未产生的 verdict。

## 3. 8 条冻结 TC 实跑（按 tests.md Command 原样）

| TC | AC | 包 | EXIT | RUN | PASS | FAIL |
|---|---|---|---|---|---|---|
| TC-001 TestRecordStrideBoundariesAndStopReason | AC-001 | internal/stride | 0 | 1 | 1 | 0 |
| TC-002 TestTurnRecordCarriesStrideRecord | AC-001 | internal/orchestrator | 0 | 1 | 1 | 0 |
| TC-003 TestNumericCeilingIsNotCompletion | AC-002 | internal/stride | 0 | 1 | 1 | 0 |
| TC-004 TestSemanticCloseoutRequiredForCompletion | AC-002 | internal/stride | 0 | 1 | 1 | 0 |
| TC-005 TestInnerLoopCannotCrossAuthorityBoundary | AC-003 | internal/stride | 0 | 1 | 1 | 0 |
| TC-006 TestOuterContinuityCannotSkipSettlement | AC-003 | internal/orchestrator | 0 | 1 | 1 | 0 |
| TC-007 TestSchedulerWakeIsNotProgress | AC-004 | internal/stride | 0 | 1 | 1 | 0 |
| TC-008 TestStateAdvanceCountsAsProgress | AC-004 | internal/stride | 0 | 1 | 1 | 0 |

每条命令先按 tests.md 原样跑记退出码，再以 `-v` 复跑核对 `=== RUN` / `--- PASS` 计数，确认不是 `-run` 匹配 0 个的假绿。

## 4. AC-S1..S4 逐条结论

| AC | 结论 | 代码锚点 | 测试锚点 | 说明 |
|---|---|---|---|---|
| AC-S1 | PASS | `internal/stride/stride.go`（Record/Turn/BuildTurn，三层 Layer 与受控词表）；`internal/orchestrator/stride_control.go:33-60,85-93`（stage 结算写三层记录 + 只读查询）；`tick.go:275-281`（与 stage 推进同事务落快照） | TC-001、TC-002、TC-008 | 一次 turn 同时给出 effect/delivery/authority 三层 boundary 与 stop_reason；turn_id 一致；重建 Engine 后仍可查回，证明是持久事实而非内存态 |
| AC-S2 | PASS | `internal/stride/stride.go:134-166`（numeric_ceiling 恒 incomplete；只有 semantic_closeout 且 closeout 非空才 complete；非语义停止带 closeout 直接拒绝） | TC-003、TC-004 | 触顶不产生 complete、progress=0、closeout 为空；伪造 closeout 被 `stride_closeout_not_allowed` 拒绝，属增强而非放松 |
| AC-S3 | PASS | `internal/stride/stride.go:219-246`（Guard：越权/借外层授权/越接受边界 → blocked + 受控 reason）；`internal/orchestrator/stride_control.go:95-112`（ContinueOuterTurn 缺 settlement 阻断）；`tick.go:135-147`（Tick 前置阻断，不写进展/完成） | TC-005、TC-006 | 内层 effect 越权限与接受边界均被拒且不续借外层授权；外层缺 settlement 时 Tick 返回 blocked(settlement_required)、stage 不前进、不新增 turn |
| AC-S4 | PASS | `internal/stride/stride.go:160-161,195-208`（scheduler_wake 恒 incomplete；progress 只统计已结算的 complete delivery） | TC-007、TC-008 | 唤醒 progress=0、不产生 complete，且与带 settlement 的状态推进可区分 |

四条 AC 在 `c217994` 上均有真实代码锚点与对应非软测试；TC 断言覆盖判据本身，不是空跑通过。

## 5. truth 状态（`truth show --json --feature kernel-stride-control`）

| 字段 | 值 |
|---|---|
| status | frozen |
| change_pending | false |
| branch_check | mismatch（当前在 `feat/kernel-stride-control`，canonical 为 `main`，功能分支预期） |
| baseline_status | committed |
| baseline_commit | `53342bd0579ca8fa9ebe2137c30da28d6875e4c2` |

`truth drift`：`clean=true`、`body_drift=false`。

## 6. 观察项（不构成 FAIL，留待后续判断）

1. 阻断路径（外层缺 settlement）不写 stride 记录，停止原因以 run 的 `reason_code=settlement_required` 表达；本包 AC-003 判据（不得跳过 settlement）已满足，若后续要求"每次 tick 都有 stride 事实"需另立 AC。
2. `stride.SettleTurn` 作为通用记录 API 允许只传部分层的记录；生产路径 `settleStageAdvance` 恒写三层，TC-002 已按生产路径验证。若要求 API 层强制三层，需在后续包明确。
3. `stride.Turn.Complete` 在未结算 turn 上可读为 true（回归验收也提出同一观察）；完成判定实际走 Engine + settlement 守卫，当前无生产消费方。

## 7. 判定

**PASS** — AC-S1..AC-S4 与 AC-001..AC-004 逐字对应、无扩张；冻结包四件套字节未变、无新勾选、evidence 链真实且不自指；8 条冻结 TC 独立实跑全部 EXIT=0 且 RUN/PASS=1/1；工作区仅有 QA agy、QA 回归两份（及本份）未跟踪 verdict 文件，实现与冻结文本零改动。

本 verdict 不替代运行时与负向回归判定；三份全绿后方可进入合入判定。
