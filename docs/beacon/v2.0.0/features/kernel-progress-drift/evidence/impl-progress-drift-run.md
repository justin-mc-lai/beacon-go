# 实现轮实跑记录：kernel-progress-drift

- 分支：`feat/kernel-progress-drift`
- 基线：`main` @ `db36ee0`（含冻结包）
- 实现范围：`internal/progressreview`（回执 schema、状态与模式词表、连续 drift 计数、revision 绑定与 rebind_hint）
- 实跑时间：2026-11-09 08:41 +0800
- 纪律：未改 `truth.md` / `tests.md` / `tasks.md` / `evidence.md` 及任何既有 `evidence/*.md`，未勾 `[x]`，未自签 QA。

## 冻结 TC 逐条实跑

每条先按 tests.md 的 Command 原样跑一次记退出码，再用同一选择器加 `-v` 跑一次，统计 `=== RUN` 与 `--- PASS` 行数，确认不是 0 匹配的假绿。

| TC | Command | EXIT | RUN | PASS |
|----|---------|------|-----|------|
| TC-001 | `go test ./internal/progressreview -run TestProgressReviewReceiptSchemaAndDefaultMode -count=1` | 0 | 1 | 1 |
| TC-002 | `go test ./internal/progressreview -run TestShadowModeRecordsReceiptWithoutReplanObligation -count=1` | 0 | 1 | 1 |
| TC-003 | `go test ./internal/progressreview -run TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation -count=1` | 0 | 1 | 1 |
| TC-004 | `go test ./internal/progressreview -run TestNonCompletedStatusesNeverCountAsHandled -count=1` | 0 | 1 | 1 |
| TC-005 | `go test ./internal/progressreview -run TestReceiptBindsPinnedGoalRevision -count=1` | 0 | 1 | 1 |
| TC-006 | `go test ./internal/progressreview -run TestForeignRevisionIsNotCountedAndReportsRebindHint -count=1` | 0 | 1 | 1 |

## 实现落点

- `internal/progressreview/receipt.go`
  - `Receipt`：`schema_version` 固定 `progress_review_receipt_v0`，字段 `event_id`、`evidence_id`、`status`、`run`、`judgments`、`drift_signal`、`question_version`、`contract_revision` 与冻结契约逐字对齐。
  - 状态词表 `completed` / `abstained` / `failed` / `not_evaluated` / `stale`；`unknown` 与任何未识别值经 `NormalizeStatus` 归为 `unknown`，`Handled()` 只对 `completed` 为真。
  - 模式词表 `off` / `shadow` / `assist`，默认 `off`；`off` 不记录可驱动回执，`shadow` 只追加回执、不累计 streak、不产生义务。
  - `Reviewer.Review`：`assist` 下连续 `completed` 且 `drift_signal=true` 累计，达到阈值（默认 N=2）才写出 `ReplanObligation{Kind: replan}`；第 N-1 条不提升。
  - 非 `completed` 状态、`completed` 但 `drift_signal=false`、重复 `event_id` 均重置连续计数，不产生义务。
  - revision 绑定：未固定 revision（reviewer 无 pin 或回执 `contract_revision` 为空）返回 `assist_blocked_reason=contract_revision_unpinned`；绑定到其它 revision 不计数，并在 `Decision.RebindHint` 与 `Summary()["rebind_hint"]` 报 `newer_receipts_under_unpinned_revision`。
- `internal/progressreview/revision.go`
  - `NormalizeContractRevision`：只接受 64 位 SHA256 hex，大小写归一；`DeriveContractRevision` 由 contract 文本稳定推导，保证同一 goal 固定 revision 可复现。
- `internal/progressreview/receipt_test.go`
  - 六个冻结 TC 方法逐字命名，全部为本轮新增；未改动任何既有测试文件或断言。

## 全量测试

见 `evidence/go-test-progress-drift-all.txt`（`go test ./... -count=1`，含退出码与包数）。

## 未做

- 不写 P3 kernel-change-quality、P5 kernel-effect-journal。
- 未接 orchestrator / CLI 的 replan 调用点：本包只产出 `ReplanObligation` 事实，既有 replan 路径的接线不在本轮范围。
- 未 push、未合 main、未自签 QA。

