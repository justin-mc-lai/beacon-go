---
mode_id: qa-negative-regression
run_id: none
stage: qa / Freeze/Completed
bound_head: c217994b72d9f8b13518ff1a498e5dfbff2ec706
branch: feat/kernel-stride-control
verdict: PASS
---

# QA 独立负向/回归验收 — kernel-stride-control

- **verdict**: **PASS**
- **current_stage**: `qa / Freeze/Completed`
- **绑定 HEAD**: `c217994b72d9f8b13518ff1a498e5dfbff2ec706`（`feat/kernel-stride-control`，已含 main `8303a35`）
- **验收方**: QA 验收 · 回归（grok 独立），只读仓库，只写本文件
- **纪律**: 未切分支、未 commit、未 push、未合并；冻结包四件套与既有 evidence 零字节改动；临时负向测试用后已删，仓库无残留。

## 1. 四类负向场景实测（真跑代码，非看他人日志）

临时测试分别写入 `internal/stride/zz_qa_tmp_test.go`、`internal/orchestrator/zz_qa_tmp_test.go`，跑完删除。

### N-1 触顶 + closeout 假称完成 → 必须拒绝（fail-closed）

构造：`Evaluate(Record{TB: qa-n1, Layer: effect, Boundary: tool_calls_max=8, StopReason: numeric_ceiling, Closeout: "forged: I claim done"})`

结果：**拒绝**。返回 `stride_closeout_not_allowed: closeout is only allowed with semantic_closeout`（`ErrCloseoutNotAllowed`），不产生 complete。`EXIT=0`（断言通过）。

### N-2 scheduler_wake 被当进展 → 必须不计数、不产生 complete

构造：一个已带 settlement 的 turn，records 全为 `StopReason: scheduler_wake`（delivery + effect 两条）。

结果：`progress=0`、`complete=false`，两条记录 `outcome=incomplete`。唤醒不借 settlement 变成进展。`EXIT=0`。

### N-3 effect 层携带外层 authority / 越权限 / 越接受边界 → 必须 blocked + error

构造 `Guard` 三种请求：`BorrowOuterAuthority=true`、`CrossesAuthority=true`、`CrossesAcceptance=true`。

结果：三者全部返回 error，记录 `outcome=blocked`，`closeout` 为空；`BorrowOuterAuthority` 与 `CrossesAuthority` 落 `authority_boundary`，`CrossesAcceptance` 落 `acceptance_boundary`。内层无法续借外层授权。`EXIT=0`。

### N-4 外层续跑缺 settlement → 必须阻断且不落进展

构造：`SettleTurn(runID, "qa-unsettled", "" /* 无 settlement */, [delivery semantic_closeout with proof])`，随后 `ContinueOuterTurn` 与 `TickBy`。

结果：
- 记录为未结算（`Settled()==false`），`progress=0`；
- `ContinueOuterTurn` 返回错误，`reason_code = settlement_required`；
- `TickBy` 返回 `blocked=true`、`reason_code=settlement_required`、`CurrentStage` 仍为 `plan`，未前进；
- `StrideTurns` 仍只 1 条，未写入新进展。`EXIT=0`。

**附加核验**：`Turn.Complete` 是「存在语义 closeout」的派生事实，未结算 turn 上可读为 `true`，但该字段在 `internal/` 非测试代码中**无任何消费方**（`grep '.Complete'` 仅命中 `stride.recompute` 自身），完成/发布判定仍由 `Engine.Complete` 人类释放路径与 `ContinueOuterTurn` settlement 守卫决定，因此不构成完成判定的绕过。此点记录为观察项，非失效项。

## 2. 是否削弱既有 gate / 既有断言

`git diff --name-status 8303a35..HEAD`（main → 绑定 HEAD）：

```
A  docs/beacon/v2.0.0/features/kernel-stride-control/evidence/go-test-stride-all.txt
A  docs/beacon/v2.0.0/features/kernel-stride-control/evidence/impl-stride-run.md
M  internal/orchestrator/meta.go
A  internal/orchestrator/stride_control.go
A  internal/orchestrator/stride_control_test.go
M  internal/orchestrator/tick.go
A  internal/stride/stride.go
A  internal/stride/stride_test.go
```

- 代码只涉及 `internal/stride`、`internal/orchestrator` 共 6 个文件；包内只多 2 份实跑日志。
- `git diff --name-status 8303a35..HEAD -- '*_test.go'` 只有两个 **新增**（A），没有任何既有测试文件的 M/D —— 既有断言零改动，未为变绿削弱 gate。
- 冻结包相对 `53342bd`：`git diff --name-status` 只有上述 2 个 A；`truth.md/tests.md/tasks.md/evidence.md` blob 逐一相同（`03ed94b`/`f5e6c15`/`a1dfb6e`/`c978868`）。

## 3. AC-009 回归

| 命令 | EXIT |
|---|---|
| `go test ./internal/cli -run TestProgramPlanRequiresProposal -count=1` | 0 |
| `go test ./internal/orchestrator -run TestDeliveryParadigmEndToEnd -count=1` | 0 |

提案门与交付范式端到端回归未受 P1 影响。

## 4. 冻结 8 条 TC 实跑（原样命令 + `-v` 校准）

| TC | 命令选择器 | EXIT | RUN | PASS | FAIL |
|---|---|---|---|---|---|
| TC-001 | `./internal/stride -run TestRecordStrideBoundariesAndStopReason` | 0 | 1 | 1 | 0 |
| TC-002 | `./internal/orchestrator -run TestTurnRecordCarriesStrideRecord` | 0 | 1 | 1 | 0 |
| TC-003 | `./internal/stride -run TestNumericCeilingIsNotCompletion` | 0 | 1 | 1 | 0 |
| TC-004 | `./internal/stride -run TestSemanticCloseoutRequiredForCompletion` | 0 | 1 | 1 | 0 |
| TC-005 | `./internal/stride -run TestInnerLoopCannotCrossAuthorityBoundary` | 0 | 1 | 1 | 0 |
| TC-006 | `./internal/orchestrator -run TestOuterContinuityCannotSkipSettlement` | 0 | 1 | 1 | 0 |
| TC-007 | `./internal/stride -run TestSchedulerWakeIsNotProgress` | 0 | 1 | 1 | 0 |
| TC-008 | `./internal/stride -run TestStateAdvanceCountsAsProgress` | 0 | 1 | 1 | 0 |

`-run` 单独跑与全量跑结论一致（全量中同样 PASS，见下）。

## 5. 全量测试

`go test ./... -count=1` → **`EXIT=0`**

- 17 个包：16 个 `ok`，1 个 `no test files`（`cmd/beacon`）
- `FAIL` 行 0，`panic` 0
- 耗时：`internal/evalgate` 63.0s、`internal/orchestrator` 99.6s、`internal/cli` 27.2s，其余均 < 17s；无异常挂起或劣化

## 6. 结论

四类负向场景全部 fail-closed；未发现为变绿而削弱既有断言或 gate；AC-009 回归通过；冻结 8 条 TC 与全量测试均 `EXIT=0`。**verdict: PASS**。

唯一观察项：`stride.Turn.Complete` 在未结算 turn 上可为 `true`（语义 closeout 的派生事实），当前无生产消费方，不构成完成判定绕过，建议后续若把该字段接入发布路径时补一条 settlement 前置约束。
