# 独立 QA Verdict — spec/AC：kernel-change-quality

- **mode_id**: `qa-spec-ac-p3`
- **run_id**: 无（只读独立验收，未在 beacon-go 建 goal run）
- **stage**: `qa / Implementation-Committed`（truth 已冻结，实现分支已提交，QA 未签）
- **target_commit**: `f74881e`（`feat/kernel-change-quality`；实现 `1f53752` + truth/证据 `f74881e`）
- **baseline**: `main` = `3396558`
- **verdict**: **PASS**

## 1. AC 映射与判据原文

`truth.md` 的「AC ID 映射」与「Acceptance Criteria」两表逐条对照，判据与派单原话一致，未扩张、未放宽。

| 上游编号 | 本包 AC | 判据原文 | 结论 |
|----------|---------|----------|------|
| AC-Q1 | AC-001 | 策略默认 off，可选 `safe_fix` `或` `strict_receipt`。 | PASS（见第 5 节呈现说明） |
| AC-Q2 | AC-002 | `strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。 | PASS |
| AC-003 | AC-003 | `safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。 | PASS |

AC-001 的「或」以行内代码标记呈现，是冻结时用于避开 deterministic AC gate 分支词启发式的显示处理，truth.md 已就地披露；连接词语义与判据范围不变，不构成扩张。

## 2. 冻结包完整性

- `truth.md` 磁盘 SHA256 = `97d31d90…c53cab`，与冻结记录、`truth drift` 的 `frozen_truth_sha256`/`current_truth_sha256` 三方一致。
- `truth drift`：`clean: true`、`body_drift: false`、`open_questions: 0`、`status: frozen`。
- 分支相对 `3396558` 的包内改动只有 `internal/changequality/*` 与包内 `truth/tests/tasks/evidence` 文档，无越界文件，无 `cmd/` 接线改动（本轮范围由 truth Q2 钉在 `internal/changequality`）。

TASK evidence 链（9 条）：TASK-000→`evidence/freeze-log.md`、TASK-001/002→`evidence/ac-q1-policy-modes.md`、TASK-003/004→`evidence/ac-q2-diff-sha-receipt.md`、TASK-005/006→`evidence/ac-q3-bounded-fix.md`、TASK-007→`evidence/go-test-all.txt`、TASK-008→`evidence/qa-contract.md`。全部指向包内真实普通文件，无自指、无指向不存在产物的占位；TASK-008 保持未勾，留给三份独立 verdict。

## 3. 6 条 TC 实跑（冻结命令原样 + `-v` 锚定 `^Name$`）

| TC | 测试 | EXIT | RUN | PASS | FAIL |
|----|------|------|-----|------|------|
| TC-001 | TestChangeQualityPolicyDefaultsToOff | 0 | 1 | 1 | 0 |
| TC-002 | TestChangeQualityUnknownModeIsReportedAndNotUpgraded | 0 | 1 | 1 | 0 |
| TC-003 | TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA | 0 | 1 | 1 | 0 |
| TC-004 | TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA | 0 | 1 | 1 | 0 |
| TC-005 | TestSafeFixAllowsOneBoundedRepairWithinAuthorization | 0 | 1 | 1 | 0 |
| TC-006 | TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed | 0 | 1 | 1 | 0 |

6/6 通过，均在 `internal/changequality` 包内实跑，`-run` 锚定全名排除 0 匹配假绿。逐条断言与 AC 判据对应：

- TC-001：默认 off；词表只接受 off / safe_fix / strict_receipt。
- TC-002：未识别值回落 off 且返回 `ConfigError`，不隐式升级。
- TC-003：回执 SHA 与重算的精确候选 diff SHA 一致时放行；SHA 对基线、文件集、内容敏感，与文件顺序无关。
- TC-004：SHA 不符、空回执、格式非法、过期、基线不一致均终态拒绝，不降级为 off。
- TC-005：授权范围内单次修复放行，触碰的文件与 hunk 全在授权清单。
- TC-006：第二次修复 `ErrRepairBudgetExhausted`；越出授权 `ErrOutOfAuthorization` 且零部分修改。

## 4. 逐条 AC 结论

- **AC-001 PASS**：TC-001/002 覆盖正向词表与默认 off、负向未知值回落，均为可执行断言，非软 TC。
- **AC-002 PASS**：TC-003/004 覆盖精确 SHA 放行与全部拒绝分支，拒绝为终态，非软 TC。
- **AC-003 PASS**：TC-005/006 覆盖单次有界修复、授权清单逐文件逐 hunk 校验、越界 fail-closed 零部分修改，非软 TC。

每条 AC 至少两条非软 TC，满足 qa-contract.md 的 spec/AC 必核项。

## 5. 边界与提示（不影响 PASS）

1. AC-001 的「或」代码标记是显示层规避，判据词义未变；若后续修正 gate 启发式，可还原为普通连接词。
2. 验收 worktree 为 `--detach` 检出、不含 `.beacon` store，drift 读自主仓规范 store；两侧 truth SHA256 完全相同，结论等效。
3. 本面只判 spec/AC 一致性；运行时行为与负向回归由另外两份 verdict 覆盖，本 PASS 不代替它们，也不代替合入判定。

