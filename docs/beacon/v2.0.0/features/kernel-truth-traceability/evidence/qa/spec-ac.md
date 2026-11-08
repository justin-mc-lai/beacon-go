# QA Verdict — spec/AC 合规（kernel-truth-traceability · #1）

- **verdict: FAIL**
- **mode_id**: `qa-spec-ac`
- **run_id**: 无（独立只读验收，未建 goal run；基于实现分支 `feat/kernel-truth-traceability` @ `4db7bd8`）
- **stage**: `qa` / `Freeze/Completed`
- **reviewer**: 验收员（独立验收，maker≠checker）
- **truth status**: `frozen`，`change_pending=false`；`branch_check=mismatch`（冻结记账记在 main，工作区在实现分支，合并前属预期）
- **判定依据**: 只按冻结 truth.md / tests.md 文本与实跑测试证据判定，不改产出

## 结论摘要

**实现代码层面对 AC-001～AC-005 全部按冻结文本落地，无需求扩张**；12 条 TC 的命令逐条实跑通过，`go test ./... -count=1` 退出码 0，既有测试文件零改动。

**但本包作为「可追溯」交付，自身的冻结测试证据存在一处硬缺陷**：TC-010 的命令只跑了三个 AC-009 提案门测试中的 1 个，无法复现其 Assertion。这正是本 feature 要防的“证据对不上断言”问题，且 #2 评测门会直接复用该命令，若不修会把假绿带进回归集。因此综合 verdict 为 **FAIL**，必改项都在 truth 包文档层，不需要改实现代码。

## 逐条 AC 结论

| AC | 结论 | 核对方式与事实 |
|----|------|----------------|
| AC-001 | **PASS** | `internal/orchestrator/delivery_paradigm.go` 新增 `ac_tc_evidence_coverage`；`conformanceTraceability` 三层判定落地：AC 集合（`gate.DefinedACs`）→ 非软且非 docs-only 的 TC 行（`row.IsSoft()`、`gate.IsDocsOnlyCommand`）→ Evidence 路径在包 `evidence/` 内且 `os.Stat` 为普通文件（`traceabilityEvidencePath`）。`TestConformanceTraceabilityAllCovered` exit 0，detail 为 `2 AC covered by 2 executable TC(s) with in-package evidence`。 |
| AC-002 | **PASS** | 两个 reason code 在 `internal/core/reasons.go` 常量固定；缺 TC → `traceability_missing_tc:missing=AC-002`；证据缺失/越界/不存在 → `traceability_missing_evidence:missing=AC-001/TC-001`；tests.md 缺失、矩阵不可解析、truth 无 AC 三路全部 fail-closed（`TestConformanceTraceabilityFailClosed` 三组均 `pass=false` 且带 reason code）。CLI 文本与 `--json` 的 `items.reason_codes/missing`、`failed` 数组都带 reason code 与缺失 ID（`TestGoalConformanceTraceabilityExitCode` 非 0 退出）。 |
| AC-003 | **PASS** | 未声明 `traceability_required: true` 的包新增项为 `not_required`，`Pass` 仍只由 `truth_frozen`、`gates_evaluated`、`verifier_attached`、`state_exported` 四项决定（`TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict`：四项 pass、`failed` 为空、整体 pass）。CLI 文本对 `not_required` 标 `n/a`，不计失败。 |
| AC-004 | **PASS** | `truth change` 把 base 的 `requirement_fingerprints` 写入 change 文档与 `truth_change_requested` 事件；`truth refreeze` 在状态翻转前计算并写入 `## AC Diff`（added/changed/removed，changed 带 before/after 前 12 位），同一 diff 进 `truth_refrozen` payload；`truth show --json` 透出 `change_ac_diff`。change 文档不可写时 refreeze 报 `truth_change_ac_diff_write_failed`、包保持 `change_pending`、无 `truth_refrozen` 事件（`TestRefreezeACDiffFailClosed`）。 |
| AC-005 | **PASS（断言事实成立，但 TC-010 命令不成立，见必改项 P1-1）** | `git diff --name-status ae95587..4db7bd8` 中测试文件只有 3 个 A（新增），无既有测试文件改动；三个 AC-009 提案门测试与 `TestDeliveryParadigmEndToEnd` 实跑全部 PASS；`go test ./... -count=1` 退出码 0（14 个测试包全 ok）。 |

## 必改项

### P1-1（阻断合并）TC-010 的冻结命令无法复现其 Assertion

tests.md TC-010 写的是：

```
go test ./internal/cli -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision' -count=1
```

但这三个测试分布在两个包：

- `TestProgramPlanRequiresProposal` → `internal/cli/goal_proposal_gate_test.go`
- `TestProgramPlanFreezeChange` → `internal/orchestrator/p2d_program_test.go`
- `TestProgramSelectRecordsDecision` → `internal/orchestrator/lake_proposal_test.go`

实跑冻结命令的 `-v` 输出只有 `=== RUN TestProgramPlanRequiresProposal`，另外两个被静默跳过（Go 对 `-run` 未匹配到测试的包不报错）。因此该 TC 的 Command 证明不了 Assertion（“三个 AC-009 提案门测试原样通过”）。`evidence/ac009-regression.txt` 里人工补跑了 orchestrator 两条，说明事实成立，但矩阵命令本身不可复现，且 #2 评测门会直接执行这些命令。

**已实测可行的修法**（二选一，推荐前者，一条命令）：

```
go test ./internal/cli ./internal/orchestrator -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision' -count=1
```

实测：cli 跑 `TestProgramPlanRequiresProposal`，orchestrator 跑 `TestProgramSelectRecordsDecision` + `TestProgramPlanFreezeChange`，全部 PASS。或拆成 TC-010/TC-010b 两行。

**修复方式**：走 `truth change` → 改 tests.md（只改 Command，不改 AC 文本）→ 重跑证据 → `truth refreeze`。tests.md 属 surface 变更，refreeze 不会被 AC-137 空差异规则拒绝。

### P2-1（合并前应修）tasks.md 勾选未绑定 evidence

tasks.md 的 Boundary 自己写明：“`[x]` is valid only when the bound evidence file exists”。但 TASK-001～010 全部打勾却没有任何 `evidence=` 绑定（全文件仅 TASK-000 有）。AC 级证据确实存在，但按本包自己的规则，这些勾选不可核验。应在同一次 truth change 里为 TASK-001～010 补 `evidence=` 指向对应的 TC Evidence 文件。

## 非阻断发现

- **P3** `evidence.md` 索引与实际 verdict 文件名不一致：索引写 `qa-spec.md` / `qa-runtime.md` / `qa-regression.md`，实际落盘为 `spec-ac.md` / `runtime.md` / `regression.md`；且 runtime、regression 已落盘但索引仍标 `planned`。三份 verdict 齐后更新索引即可。
- **P3** `internal/cli/goal.go` 的 `goal conformance` 帮助文本仍写 “Exit code 0 only when all four pass”，实际已是四项 + `ac_tc_evidence_coverage`（或 `not_required`）。行为正确，文案陈旧。

## 实跑命令与结果（本次验收真实执行）

| # | 命令 | 退出码 | 结果 |
|---|------|--------|------|
| 1 | `go test ./internal/orchestrator -run TestConformanceTraceabilityAllCovered -count=1` | 0 | ok |
| 2 | `go test ./internal/orchestrator -run TestConformanceTraceabilityMissingTC -count=1` | 0 | ok |
| 3 | `go test ./internal/orchestrator -run TestConformanceTraceabilityMissingEvidence -count=1` | 0 | ok |
| 4 | `go test ./internal/cli -run TestGoalConformanceTraceabilityExitCode -count=1` | 0 | ok |
| 5 | `go test ./internal/orchestrator -run TestConformanceTraceabilityFailClosed -count=1` | 0 | ok |
| 6 | `go test ./internal/orchestrator -run TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict -count=1` | 0 | ok |
| 7 | `go test ./internal/orchestrator -run TestRefreezeRecordsACDiff -count=1` | 0 | ok |
| 8 | `go test ./internal/cli -run TestTruthShowExposesChangeACDiff -count=1` | 0 | ok |
| 9 | `go test ./internal/orchestrator -run TestRefreezeACDiffFailClosed -count=1` | 0 | ok |
| 10 | `go test ./internal/cli -run 'TestProgramPlanRequiresProposal\|TestProgramPlanFreezeChange\|TestProgramSelectRecordsDecision' -count=1 -v` | 0 | **只跑 1/3（复现 P1-1）** |
| 11 | `go test ./internal/orchestrator -run 'TestProgramPlanFreezeChange\|TestProgramSelectRecordsDecision' -count=1 -v` | 0 | 2 个 PASS（人工补跑） |
| 12 | `go test ./internal/cli ./internal/orchestrator -run '三个测试名' -count=1 -v` | 0 | 3 个全 PASS（建议修法） |
| 13 | `go test ./internal/orchestrator -run TestDeliveryParadigmEndToEnd -count=1` | 0 | ok |
| 14 | `go test ./... -count=1` | 0 | 14 个测试包全 ok |

## 证据路径（绝对路径）

- truth: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/truth.md`
- tests: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/tests.md`
- tasks: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/tasks.md`
- AC-009 证据: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/ac009-regression.txt`
- 全量测试证据: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/go-test-all.txt`
- 覆盖矩阵解析缺陷证据: `/Users/apple/Developer/Personal/products/beacon-go/internal/gate/traceability_matrix_test.go`（`TestParseCoverageMatrixKeepsPipesInCodeSpan` PASS）

## 边界声明

只读代码与 git；未切分支、未 commit、未 push、未改实现、未动 `kernel-eval-gate` 包。本次唯一写入是本 verdict 文件。

## next

1. 制造方按 P1-1、P2-1 走 `truth change` 修 tests.md / tasks.md 并 refreeze（不改 AC 文本、不改实现代码）。
2. 修复后重跑 TC-010 与全量测试，更新 `ac009-regression.txt`。
3. 本验收员复验 TC-010 可复现性与 tasks 绑定后，再出 PASS/FAIL 终判。
