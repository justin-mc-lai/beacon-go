# 独立 QA Verdict — spec/AC：kernel-effect-journal

- **mode_id**: `qa-spec-ac-p5`
- **run_id**: 无（只读独立验收，未在 beacon-go 建 goal run）
- **stage**: `qa / Implementation-Committed`（truth 已冻结，实现分支已提交，QA 未签）
- **target_commit**: `f12ca9c`（`feat/kernel-effect-journal`；实现 `e7fd1ba` + truth/证据 `f12ca9c`）
- **baseline**: `main` = `3396558`
- **verdict**: **PASS**

## 1. AC 映射与判据原文

`truth.md` 的「AC ID 映射」与「Acceptance Criteria」两表逐条对照，判据与派单原话一致，未扩张、未放宽。

| 上游编号 | 本包 AC | 判据原文 | 结论 |
|----------|---------|----------|------|
| AC-J1 | AC-001 | implement 阶段每个副作用带 idempotency key 并写 effect journal。 | PASS（见第 5 节措辞提示） |
| AC-J2 | AC-002 | replay 不重复生效；resume 输出已证明阶段清单。 | PASS |
| AC-J3 | AC-003 | resume 缺 journal 时 fail-closed，不静默重跑。 | PASS |

## 2. 冻结包完整性

- `truth.md` 磁盘 SHA256 = `c0488fac…d280`，与冻结记录、`truth drift` 的 `frozen_truth_sha256`/`current_truth_sha256` 三方一致。
- `truth drift`：`clean: true`、`body_drift: false`、`open_questions: 0`、`status: frozen`。
- 分支相对 `3396558` 的包内改动只有 `internal/effectjournal/*` 与包内 `truth/tests/tasks/evidence` 文档，无越界文件，无 `cmd/` 接线改动（本轮范围由 truth Q2 钉在 `internal/effectjournal`）。

TASK evidence 链（9 条）：TASK-000→`evidence/freeze-log.md`、TASK-001/002→`evidence/ac-j1-idempotency-key.md`、TASK-003/004→`evidence/ac-j2-replay-resume.md`、TASK-005/006→`evidence/ac-j3-missing-journal.md`、TASK-007→`evidence/go-test-all.txt`、TASK-008→`evidence/qa-contract.md`。全部指向包内真实普通文件，无自指、无指向不存在产物的占位；TASK-008 保持未勾，留给三份独立 verdict。

## 3. 6 条 TC 实跑（冻结命令原样 + `-v` 锚定 `^Name$`）

| TC | 测试 | EXIT | RUN | PASS | FAIL |
|----|------|------|-----|------|------|
| TC-001 | TestImplementEffectCarriesIdempotencyKeyAndWritesJournal | 0 | 1 | 1 | 0 |
| TC-002 | TestEffectWithoutIdempotencyKeyIsRefused | 0 | 1 | 1 | 0 |
| TC-003 | TestReplaySkipsSucceededEffectAndResumeListsProvenStages | 0 | 1 | 1 | 0 |
| TC-004 | TestFailedOrIncompleteRecordsAreNotProvenStages | 0 | 1 | 1 | 0 |
| TC-005 | TestResumeWithValidJournalProceedsFromProof | 0 | 1 | 1 | 0 |
| TC-006 | TestResumeWithMissingOrCorruptJournalFailsClosed | 0 | 1 | 1 | 0 |

6/6 通过，均在 `internal/effectjournal` 包内实跑，`-run` 锚定全名排除 0 匹配假绿。逐条断言与 AC 判据对应：

- TC-001：JSONL 每条记录 7 个固定字段，idempotency key 非空且同 run 内唯一，时间戳为 RFC3339。
- TC-002：缺 key 的副作用被拒，且不写任何 journal 记录（不产生伪成功）。
- TC-003：同 key replay 命中 succeeded 返回既有结果并记 `skipped_replay`，底层副作用只执行一次；resume 证明清单只含 succeeded 阶段。
- TC-004：failed 或记录不完整的阶段不进入证明清单，replay 不伪造成功，失败 key 重放返回 `ErrDuplicateIdempotencyKey`。
- TC-005：journal 有效时 resume 从证明继续，只跑未证明阶段。
- TC-006：journal 缺失/坏 JSON/未知版本分别 `ErrJournalMissing`/`ErrJournalCorrupt`/`ErrSchemaUnsupported`，FailClosed 且零阶段重跑。

## 4. 逐条 AC 结论

- **AC-001 PASS**：TC-001/002 覆盖写 journal 与缺 key 拒绝，断言落到字段与账面状态，非软 TC。
- **AC-002 PASS**：TC-003/004 覆盖 replay 去重只执行一次、resume 证明清单只含 succeeded 完整记录，非软 TC。
- **AC-003 PASS**：TC-005/006 覆盖有 journal 从证明续跑与缺失/损坏/版本不识别 fail-closed 零重跑，非软 TC。

每条 AC 至少两条非软 TC，满足 qa-contract.md 的 spec/AC 必核项。

## 5. 边界与提示（不影响 PASS）

1. truth.md Q1 的 answered 文本写成「install 阶段副作用账本」，而上游 utterance 与 AC-J1 均为「implement 阶段」。判据表、TC、实现三处一致用 implement，属 Q1 答复措辞笔误，不改判据范围；建议后续 docs 维护时把 Q1 的「install」改回「implement」。
2. 验收 worktree 为 `--detach` 检出、不含 `.beacon` store，drift 读自主仓规范 store；两侧 truth SHA256 完全相同，结论等效。
3. 本面只判 spec/AC 一致性；运行时行为与负向回归由另外两份 verdict 覆盖，本 PASS 不代替它们，也不代替合入判定。

