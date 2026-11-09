# 独立 QA Verdict — spec/AC：kernel-progress-drift

- **mode_id**: `qa-spec-ac-p2`
- **run_id**: 无（只读独立验收，未在 beacon-go 建 goal run）
- **stage**: `qa / Freeze-Completed → Implementation-Committed`
- **target_commit**: `73f947bde683d9f2d7de1c2d7f3aa24e11a37e20`（`feat/kernel-progress-drift`）
- **baseline**: `main` = `db36ee01ab06e311dfc182880ee8d4e78930f594`
- **verdict**: **PASS**

## 1. AC 映射与判据原文

`truth.md` 的「AC ID 映射」与「Acceptance Criteria」两表逐条对照，判据文字与用户原话 / 上游 AC-D 原文一致，未扩张、未放宽。

| 上游编号 | 本包 AC | 判据原文核对 | 结论 |
|----------|---------|--------------|------|
| AC-D1 | AC-001 | `progress_review_receipt_v0 schema + shadow 模式，默认 off。` 逐字一致 | PASS |
| AC-D2 | AC-002 | `连续 N 条 completed 且 drift=true 才提升为既有 replan 义务；unknown/abstained/failed/stale 永不算已处理。` 逐字一致 | PASS（见第 5 节边界判定） |
| AC-D3 | AC-003 | `回执必须绑定 goal 固定 revision，绑定到别处不计数并上报 rebind_hint。` 逐字一致 | PASS |

「Progress Review 回执契约」一节把 AC-001 的字段具体化为 9 个固定字段，属于判据的具体化，没有新增或放宽任何 AC 判据；`contract_revision` 字段服务于 AC-D3，非扩张。

## 2. 冻结包完整性（相对 db36ee0）

四件套 blob 与 baseline 逐一相同：

| 文件 | blob | 结论 |
|------|------|------|
| truth.md | `3c7bf54536dde9dc600c0b23f14f6e800824b947` | SAME |
| tests.md | `969991445f96dbfa869ed8572ecdb2a32cf12b40` | SAME |
| tasks.md | `51363db1f79202294bc2220d5801208615501700` | SAME |
| evidence.md | `88d70266e00e7e1f5c85f26d91b448d4c00a94ae` | SAME |

`db36ee0..HEAD` 的包内变更只有两个新增普通文件：

- `evidence/go-test-progress-drift-all.txt`
- `evidence/impl-progress-drift-run.md`

未新增任何 `[x]`：tasks.md blob 未变，勾选状态仍只有冻结时的 TASK-000。

TASK evidence 链核对（9 条 TASK）：TASK-000→freeze-log.md、TASK-001/002→ac-d1-receipt-schema.md、TASK-003/004→ac-d2-drift-promotion.md、TASK-005/006→ac-d3-revision-binding.md、TASK-007→go-test-all.txt、TASK-008→qa-contract.md；全部指向包内真实普通文件，无自指、无指向未来产物的占位。

## 3. 冻结 TC 实跑（命令原样 + `-v`）

| TC | 包 | EXIT | RUN | PASS | FAIL |
|----|----|------|-----|------|------|
| TC-001 TestProgressReviewReceiptSchemaAndDefaultMode | internal/progressreview | 0 | 1 | 1 | 0 |
| TC-002 TestShadowModeRecordsReceiptWithoutReplanObligation | internal/progressreview | 0 | 1 | 1 | 0 |
| TC-003 TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation | internal/progressreview | 0 | 1 | 1 | 0 |
| TC-004 TestNonCompletedStatusesNeverCountAsHandled | internal/progressreview | 0 | 1 | 1 | 0 |
| TC-005 TestReceiptBindsPinnedGoalRevision | internal/progressreview | 0 | 1 | 1 | 0 |
| TC-006 TestForeignRevisionIsNotCountedAndReportsRebindHint | internal/progressreview | 0 | 1 | 1 | 0 |

每条均以 tests.md 的 Command 原样执行，再用同一选择器加 `-v` 计数；RUN=1 排除了 `-run` 0 匹配的假绿。

## 4. truth / drift 状态

`beacon-go truth show --json --feature kernel-progress-drift`：

| 字段 | 值 |
|------|-----|
| status | `frozen` |
| change_pending | `false` |
| branch_check | `mismatch`（`current_branch=feat/kernel-progress-drift`，`canonical=main`；功能分支上的预期值） |
| baseline_status | `committed`（`baseline_commit=db36ee01ab06e311dfc182880ee8d4e78930f594`） |

`truth drift`：`clean=true`、`body_drift=false`。

## 5. AC 逐条结论与代码锚点

### AC-001 — PASS
- `internal/progressreview/receipt.go:19` 固定 `SchemaVersion = "progress_review_receipt_v0"`；`Receipt` 结构（L126）含契约要求的 9 个字段，`Validate` 校验必填项。
- 模式词表 L46-L58 为 `off/shadow/assist`，`DefaultMode = ModeOff`（L55）；空配置 `ParseMode("")` 归一为 off。
- `Review` 的 off 分支（L329）不记录、不驱动；shadow 分支（L370）记录回执但 `Drivable/Counted/Obligation` 恒为空。TC-001/TC-002 实跑覆盖。

### AC-002 — PASS（附 scope 边界，见下）
- 默认阈值 `DefaultDriftThreshold = 2`；仅 assist 模式下 `completed && drift_signal` 累计；`r.streak >= r.threshold`（L403）时才写出 `ReplanObligation`，第 N-1 条不写出（TC-003 覆盖 N=2 与 N=3 两种阈值）。
- `Status.Handled()`（L109）只对 `completed` 为真；`NormalizeStatus` 把 `unknown`/空/未识别值归为 `unknown`。非 completed 状态、completed 但 `drift_signal=false`、重复 `event_id` 均重置 streak，不计数、不产生义务（TC-004 覆盖 5 个冻结状态 + 2 个表外值 + 重复事件）。

**对「提升为既有 replan 义务」的明确判定：在冻结 scope 下成立，判 PASS，但带一条必须在合入回执中写明的边界。**

理由与证据：
1. 冻结 truth 的 Q2 已把「回执结构、状态词表、shadow/off/assist 模式、连续 drift 计数与 revision 绑定校验」明确落在 `internal/progressreview`；Non-Goals 与 Deferral Ledger（none）都没有把 orchestrator 接线列为本轮交付。
2. 实现确实在策略层「raise」了义务：达到阈值时 `Decision.Obligation` 非空、`Reviewer.Obligation()` 可读回，且义务带 `Kind=replan`、阈值、连续数、绑定 revision 与两条证据 ID。
3. 但「既有」二字在 beacon-go 仓库内没有对应实体：全仓 `rg ReplanObligation` 只命中 `internal/progressreview` 与本包测试，没有任何 orchestrator/CLI 消费方；实现是新建义务类型，不是复用既有义务。因此本包目前证明的是「策略会产出义务事实」，未证明「该义务已进入既有 replan 路径」。
4. 若人的真实意图是端到端接线，那是**冻结 truth 的 scope 缺口**，应通过 truth change 增补一条可验证 AC（例如：assist 模式下触发的义务必须进入 orchestrator 既有 replan 入口并有跨包 TC），而不是在验收阶段口头扩张判据；按当前冻结文本判实现失败会构成对实现的额外要求。
5. 实现证据自述「既有 replan 路径的接线不在本轮范围」，与冻结包 Deferral Ledger 的 `none` 表述不一致；本轮不阻断，但合入回执不得把 AC-002 表述为「既有 replan 已接线」。

### AC-003 — PASS
- `revision.go:16` `NormalizeContractRevision` 只接受 64 位 SHA256 hex，大小写与空白归一；`revision.go:33` `DeriveContractRevision` 由 contract 文本稳定推导，同一 goal 可复现（TC-005）。
- 未固定（reviewer 无 pin 或回执 revision 为空）→ `assist_blocked_reason=contract_revision_unpinned`，不计数；绑定其它 revision → 不计数，`Decision.RebindHint` 与 `Summary()["rebind_hint"]` 均为 `newer_receipts_under_unpinned_revision`（receipt.go:355-360、L310）；随后匹配 revision 的回执从 streak=1 重新开始，未被 foreign 记录污染（TC-006）。

## 6. 非阻断观察项

1. **「既有 replan」无消费方**：见第 5 节 AC-002，已作为 scope 边界记录；后续若做接线，应新开 truth change / 独立包，不在本包内静默扩张。
2. **代码注释与仓库事实不符**：`receipt.go:184` 注释称 “The kernel already has a re-plan path”，但仓库内无该路径；建议后续修正措辞或在接线包中兑现，避免审计误读。
3. **TASK-007 evidence 指针**：其 `evidence=go-test-all.txt` 是冻结轮的全量输出；实现轮全量输出写入了新文件 `go-test-progress-drift-all.txt`，冻结 tasks.md 未回填指针。因该 TASK 未勾 `[x]`，不构成自签；若后续要勾选，需走 truth change 校正指针，不能直接改冻结文本。

## 7. 纪律核对

- 只读验收：未切分支、未 commit、未 push、未合并、未改实现与冻结文本。
- 唯一写入：本文件 `evidence/qa/spec-ac.md`。
- 实现范围未越界：`db36ee0..HEAD` 的代码改动全部在 `internal/progressreview`（3 个新增文件），无既有测试被改写、无既有 gate 被削弱。

## 结论

**verdict: PASS** — AC-D1..D3 映射与判据原文一致，冻结包四件套零字节改动，6 条冻结 TC 全部实跑 EXIT=0 且 `-v` 各 1 RUN/1 PASS，truth 为 frozen / change_pending=false / baseline committed（功能分支 branch_check=mismatch 属预期）。AC-002 在冻结 scope 下成立，但「既有 replan 义务」目前只有义务事实、无仓库内消费方，合入回执必须如实写明该边界。
