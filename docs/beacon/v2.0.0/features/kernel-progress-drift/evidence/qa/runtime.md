# Runtime QA Verdict: kernel-progress-drift

- **verdict**: PASS
- **current_stage**: qa / Freeze-Completed → Implementation-Committed
- **target_commit**: `73f947bde683d9f2d7de1c2d7f3aa24e11a37e20` (branch `feat/kernel-progress-drift`)
- **baseline**: `main` = `db36ee01ab06e311dfc182880ee8d4e78930f594`
- **evaluator**: QA 验收 · agy（视觉/端侧 & 独立运行时验收）
- **evaluated_at**: 2026-11-09T08:52:00+08:00

---

## 1. 冻结 TC 逐条实测（6 条全部独立实跑）

按 `tests.md` Command 原样逐条实跑，并在 `-v` 模式下核对 `=== RUN` 与 `--- PASS` 真实计数，排除 `-run` 匹配 0 个的假绿。

| TC ID | AC ID | Package | Command | Exit Code | RUN Count | PASS Count | Duration | Verdict |
|-------|-------|---------|---------|-----------|-----------|------------|----------|---------|
| TC-001 | AC-001 | internal/progressreview | `go test ./internal/progressreview -run TestProgressReviewReceiptSchemaAndDefaultMode -count=1` | 0 | 1 | 1 | 0.242s | PASS |
| TC-002 | AC-001 | internal/progressreview | `go test ./internal/progressreview -run TestShadowModeRecordsReceiptWithoutReplanObligation -count=1` | 0 | 1 | 1 | 0.251s | PASS |
| TC-003 | AC-002 | internal/progressreview | `go test ./internal/progressreview -run TestConsecutiveDriftCompletedReceiptsRaiseReplanObligation -count=1` | 0 | 1 | 1 | 0.221s | PASS |
| TC-004 | AC-002 | internal/progressreview | `go test ./internal/progressreview -run TestNonCompletedStatusesNeverCountAsHandled -count=1` | 0 | 1 | 1 | 0.245s | PASS |
| TC-005 | AC-003 | internal/progressreview | `go test ./internal/progressreview -run TestReceiptBindsPinnedGoalRevision -count=1` | 0 | 1 | 1 | 0.260s | PASS |
| TC-006 | AC-003 | internal/progressreview | `go test ./internal/progressreview -run TestForeignRevisionIsNotCountedAndReportsRebindHint -count=1` | 0 | 1 | 1 | 0.287s | PASS |

---

## 2. 全量测试结果

执行命令：
```bash
go test ./... -count=1
```

- **Exit Code**: `0`
- **Total Packages**: 18
  - **ok packages**: 17
    - `beacon-go/desktop` (0.684s)
    - `beacon-go/internal/agentapi` (5.797s)
    - `beacon-go/internal/benchgate` (1.925s)
    - `beacon-go/internal/cli` (21.221s)
    - `beacon-go/internal/core` (1.478s)
    - `beacon-go/internal/evalgate` (65.295s)
    - `beacon-go/internal/gate` (2.907s)
    - `beacon-go/internal/harness` (16.029s)
    - `beacon-go/internal/llm` (4.364s)
    - `beacon-go/internal/migration` (4.957s)
    - `beacon-go/internal/orchestrator` (107.323s)
    - `beacon-go/internal/progressreview` (4.446s)
    - `beacon-go/internal/session` (4.496s)
    - `beacon-go/internal/store` (4.159s)
    - `beacon-go/internal/stride` (3.894s)
    - `beacon-go/internal/workspace` (5.514s)
    - `beacon-go/skills` (3.953s)
  - **no-test packages**: 1 (`beacon-go/cmd/beacon`)
  - **FAIL count**: 0
  - **panic count**: 0

### 重点包耗时核验
- `internal/orchestrator`: 107.323s（基线 ~95s-105s，正常范围）
- `internal/evalgate`: 65.295s（基线 ~55s-65s，正常范围）
- `internal/progressreview`: 4.446s（新模块，执行快速无延迟）

---

## 3. 运行时行为抽查与实测判定

针对 `internal/progressreview` 运行时各项关键契约进行行为级实测抽查：

1. **默认模式为 off 且不产生可驱动回执**：
   - `DefaultMode == ModeOff` ("off")，`New(Options{})` 构造出的 Reviewer 模式恒为 `off`。
   - `ModeOff` 下调用 `Review(receipt)`：返回 `Recorded=false`、`Drivable=false`、`Obligation=nil`，内部 `Receipts()` 长度恒为 0，完全不产生可驱动回执。
2. **shadow 模式仅记录回执、绝不产生 replan 义务**：
   - `ModeShadow` 下调用 `Review(receipt)`：返回 `Recorded=true`、`Drivable=false`、`Counted=false`、`Obligation=nil`。
   - 回执完整追加至内部 `Receipts()` 列表，连续连续计数 `ConsecutiveDrift()` 恒为 0，`Obligation()` 恒为 `nil`。
3. **assist 模式阈值控制（第 1 条不提升，连续第 2 条才提升）**：
   - 阈值设为 2（默认值），注入第 1 条 `completed` 且 `drift_signal=true` 回执：`Counted=true`、`Consecutive=1`、`Obligation=nil`。
   - 注入第 2 条 `completed` 且 `drift_signal=true` 回执：`Counted=true`、`Consecutive=2`、`Obligation!=nil`，生成义务 `Kind="replan"`, `Consecutive=2`, `Threshold=2`，带全量证据链 `EvidenceIDs`。
4. **非 completed 状态与表外未知值永不算已处理，并强制打断连续计数**：
   - 状态词表 `abstained`, `failed`, `not_evaluated`, `stale`, `unknown` 以及表外未知值（如 `"garbage"`, `""` 均经 `NormalizeStatus` 归一为 `StatusUnknown`），其 `Handled()` 方法恒返回 `false`。
   - 处于连续 drift=1 的 Reviewer 收到非 completed 回执：`Handled=false`、`Counted=false`、`Consecutive=0`，连续计数 `ConsecutiveDrift()` 被立即归零清空。后续再来 completed 回执时重新从 1 起计，不发生泄露。
5. **重复 event_id 阻断并打断计数**：
   - 相同 `event_id` 重复提交时，`seenEvents` 拦截去重，重置连续计数为 0（`Consecutive=0`），`Counted=false`，assist 模式下明确置 `AssistBlockedReason=progress_review_duplicate_event`。
6. **contract_revision SHA256 hex 强约束**：
   - `DeriveContractRevision(contract)` 输出确定性 64 字符 SHA256 hex 摘要。
   - `NormalizeContractRevision` 统一转小写，严格校验 64 位长度与十六进制合法性，非法长度或非法字符一律返回错误拒绝。
7. **目标 revision 未固定时阻断计数并报 contract_revision_unpinned**：
   - Reviewer 未固定 revision（`PinnedRevision: ""`）或回执未绑定 revision（`ContractRevision: ""`）时，无法计入连续 drift，明确输出 `AssistBlockedReason="contract_revision_unpinned"`。
8. **foreign revision 不计数且输出 rebind_hint**：
   - 当回执携带的 `ContractRevision` 与目标 pinned revision 不匹配时，`Counted=false`，连续计数被打断归零，输出 `AssistBlockedReason="contract_revision_mismatch"`，决策与 Summary 均携带 `rebind_hint="newer_receipts_under_unpinned_revision"`。
9. **固定 Schema 字段齐全与 Validate 坏值拒止**：
   - 回执结构包含 9 个固定字段（`schema_version`, `event_id`, `evidence_id`, `status`, `run`, `judgments`, `drift_signal`, `question_version`, `contract_revision`）。
   - `Validate()` 对错误 `schema_version`、空 `event_id`、空 `evidence_id`、空 `question_version`、空 `run.run_id` 及非法 `contract_revision` 均实现 fail-closed 拦截并返回具体错误。

---

## 4. 纪律与工作区状态核实

- **唯一新增产物**：`docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/qa/runtime.md`
- 冻结包四件套（`truth.md`, `tests.md`, `tasks.md`, `evidence.md`）相对 `db36ee0` 零字节修改。
- 未切换分支、未 commit、未 push、未合并。
- 当前 HEAD 保持为 `73f947bde683d9f2d7de1c2d7f3aa24e11a37e20`。
