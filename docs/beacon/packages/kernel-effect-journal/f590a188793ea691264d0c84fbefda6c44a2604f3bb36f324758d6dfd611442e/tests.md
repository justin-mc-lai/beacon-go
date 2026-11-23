---
slug: kernel-effect-journal
version: v2.0.0
materials_status: current
---

# Tests: kernel-effect-journal

每条 AC 至少两条执行层 TC：一条验证正向行为，一条验证去重或 fail-closed 的负向行为。Command 一律是可直接运行的 `go test` 选择器；实现轮次按此命名落地测试，并把实跑输出追加到对应的包内证据文件。AC-J1 至 AC-J3 与 AC-001 至 AC-003 的对应关系见 truth.md 的 AC ID 映射表。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | `go test ./internal/effectjournal -run TestImplementEffectCarriesIdempotencyKeyAndWritesJournal -count=1` | implement 阶段每个副作用写入一条 journal 记录，携带非空 idempotency key 且同 run 内唯一 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j1-idempotency-key.md |
| AC-001 | TC-002 | negative | `go test ./internal/effectjournal -run TestEffectWithoutIdempotencyKeyIsRefused -count=1` | 缺少 idempotency key 的副作用被拒绝执行，且不产生 journal 成功记录 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j1-idempotency-key.md |
| AC-002 | TC-003 | behavior | `go test ./internal/effectjournal -run TestReplaySkipsSucceededEffectAndResumeListsProvenStages -count=1` | 同 key 重放命中成功记录时返回既有结果并记 skipped_replay，底层副作用只执行一次；resume 只列 succeeded 阶段 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j2-replay-resume.md |
| AC-002 | TC-004 | negative | `go test ./internal/effectjournal -run TestFailedOrIncompleteRecordsAreNotProvenStages -count=1` | failed 或记录不完整的阶段不进入已证明阶段清单，replay 不为它伪造成功 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j2-replay-resume.md |
| AC-003 | TC-005 | behavior | `go test ./internal/effectjournal -run TestResumeWithValidJournalProceedsFromProof -count=1` | journal 存在且可解析时，resume 按已证明阶段继续，不重跑已证明阶段 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j3-missing-journal.md |
| AC-003 | TC-006 | negative | `go test ./internal/effectjournal -run TestResumeWithMissingOrCorruptJournalFailsClosed -count=1` | journal 缺失、不可解析或版本不识别时 resume fail-closed，返回明确原因且零阶段重跑 | docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j3-missing-journal.md |

## 回归与门禁

实现轮次除上述 6 条 TC 外，还需跑全量 `go test ./... -count=1` 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。

