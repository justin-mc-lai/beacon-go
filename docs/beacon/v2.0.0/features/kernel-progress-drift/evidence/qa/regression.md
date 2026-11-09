# QA 回归 · 独立负向/回归验收 — P2 kernel-progress-drift

- **verdict**: PASS
- **current_stage**: `qa / Implementation-Committed`（等待三份 QA 齐备后由 MausBot 裁决合入）
- **target_commit**: `73f947bde683d9f2d7de1c2d7f3aa24e11a37e20`（分支 `feat/kernel-progress-drift`）
- **baseline (canonical main)**: `db36ee0`
- **mode_id**: `qa-regression-p2`
- **scope**: 只判负向/回归，不判 spec 判据与运行时外观。

## 1. 四类必核负向场景（临时测试真实执行，执行后已删除，仓库无残留）

临时文件 `internal/progressreview/zz_negative_tmp_test.go`，用 `go test ./internal/progressreview -run 'TestNeg*' -count=1 -v` 执行，**EXIT=0，7/7 PASS**；执行完立即 `rm`，`git status` 已确认无该文件。

| 场景 | 构造方式 | 实测结果 |
|---|---|---|
| N1 伪造 completed 未绑 pinned revision | `New(Options{Mode: ModeAssist})`，pinned 为空，连续 5 条 `status=completed, drift=true, contract_revision=<合法 hex>` | 每条 `Counted=false`，`AssistBlockedReason=contract_revision_unpinned`，`Consecutive` 恒 0，`Obligation()==nil` — **fail-closed** |
| N2 非 completed 混入 | assist + pinned，先 1 条 completed+drift（count=1），再依次注入 `abstained / failed / not_evaluated / stale / unknown / 表外词 totally_bogus`，最后 1 条 completed+drift | 每种状态 `Counted=false`、`Handled=false`，连续计数被清零；最后那条 completed 只到 1，`Obligation=nil`，第 N-1 条不提升 — **fail-closed** |
| N3 foreign revision | assist + pinned=revA + threshold=2，先 1 条 revA（count=1），再 1 条 revB | `Counted=false`，streak 归零，`AssistBlockedReason=contract_revision_mismatch`，`Decision.RebindHint` 与 `Summary()["rebind_hint"]` 均为 `newer_receipts_under_unpinned_revision`，不产义务 — **fail-closed** |
| N4 unpinned 在 assist 阻断 / shadow 不驱动 | assist + 空 pinned 单条 → 断言 `contract_revision_unpinned`；shadow + pinned + threshold=1 连投 3 条 completed+drift | assist 阻断无义务；shadow 全程 `Drivable=false / Counted=false / Obligation=nil`，streak 恒 0 — **fail-closed** |
| N5 重复 event_id | assist + threshold=2，同一 `Receipt` 投两次 | 第 2 次 `Counted=false`、不产义务、streak 归零、`AssistBlockedReason=progress_review_duplicate_event`、`Receipts()` 仍为 1 条 — **不会被当两条计数** |
| N6 off 默认模式 | 默认 `New(Options{Threshold:1, PinnedRevision:revA})` 投 completed+drift | `Recorded=false / Drivable=false / Counted=false / Obligation=nil`，`Receipts()` 为空 — **不产可驱动回执** |
| N7 坏值 fail-closed | 6 组非法回执（错 schema、空 event_id、空 evidence_id、空 run_id、空 question_version、非 hex revision）逐一 `Validate()` | 全部返回 error — **fail-closed** |

## 2. 未放水：diff 范围（`git diff --name-status db36ee0..HEAD`）

```
A  docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/go-test-progress-drift-all.txt
A  docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/impl-progress-drift-run.md
A  internal/progressreview/receipt.go
A  internal/progressreview/receipt_test.go
A  internal/progressreview/revision.go
```

- 5 个文件全部是 **A（新增）**，零 M / 零 D；范围仅 `internal/progressreview` 三个新文件 + 包内两份新 evidence。
- 既有测试文件与断言零改动：`db36ee0..HEAD` 中唯一 `_test.go` 是新增的 `receipt_test.go`，其余测试文件未出现在 diff 中。
- 冻结四件套 `truth.md / tests.md / tasks.md / evidence.md` 不在 diff 内；未发现新增 `[x]`。
- 未削弱既有 gate：无任何既有包的改动。

## 3. AC-009 回归

| 命令 | EXIT |
|---|---|
| `go test ./internal/cli -run '^TestProgramPlanRequiresProposal$' -count=1` | 0 |
| `go test ./internal/orchestrator -run '^TestDeliveryParadigmEndToEnd$' -count=1` | 0 |

## 4. 冻结 6 条 TC（tests.md 原样命令 + `-v` 计数）

| TC | 测试方法 | EXIT | RUN | PASS |
|---|---|---|---|---|
| TC-001 | TestProgressReviewReceiptSchemaAndDefaultMode | 0 | 1 | 1 |
| TC-002 | TestShadowModeRecordsReceiptWithoutReplanObligation | 0 | 1 | 1 |
| TC-003 | TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation | 0 | 1 | 1 |
| TC-004 | TestNonCompletedStatusesNeverCountAsHandled | 0 | 1 | 1 |
| TC-005 | TestReceiptBindsPinnedGoalRevision | 0 | 1 | 1 |
| TC-006 | TestForeignRevisionIsNotCountedAndReportsRebindHint | 0 | 1 | 1 |

单跑与全量一致：全量 `go test ./... -count=1` 中 `internal/progressreview` 为 ok、0 FAIL，无方法被跳过；不存在 `-run` 匹配 0 个的假绿（每条均 1 RUN/1 PASS）。

## 5. 全量测试

| 命令 | EXIT | ok | no test files | FAIL | panic |
|---|---|---|---|---|---|
| `go test ./... -count=1` | 0 | 17 | 1（`cmd/beacon`） | 0 | 0 |

## 6. 观察项（非失效，不阻断）

1. **重复 event_id 的语义冻结文本未定义**：当前实现是「去重 + 打断计数 + 报 `progress_review_duplicate_event`」，方向 fail-closed，不会把重放当两条进展。若将来希望重复事件「静默忽略、不影响已有连续计数」，需走 truth change 明确口径。
2. **`ReplanObligation` 无既有消费方**：与 spec/AC 验收的边界一致——本包只产出义务事实，未接入 orchestrator。合入回执不得表述为「既有 replan 已接线」。
3. **`rebindHint` 在 Reviewer 生命周期内粘滞**：一旦出现过 foreign revision，`Summary()["rebind_hint"]` 会持续存在直到该 Reviewer 重建。这是上报提示，不参与计数，不影响 fail-closed 结论。

## 7. 工作区状态

除本文件所在未跟踪目录 `docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/qa/` 外无任何改动；未切分支、未 commit、未 push、未合并，未改冻结包与既有证据。
