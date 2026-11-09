# Runtime QA Verdict: kernel-effect-journal

- **mode_id**: `qa-runtime-behavior-p5`
- **run_id**: none（只读独立运行时验收，绑定分支 `feat/kernel-effect-journal` @ `f12ca9c`）
- **stage**: `qa / Implementation-Committed`
- **target_commit**: `f12ca9c`（`feat/kernel-effect-journal`；实现 `e7fd1ba` + truth/证据 `f12ca9c`）
- **baseline**: `main` = `3396558`
- **evaluator**: QA 验收 · agy（视觉/端侧 & 独立运行时验收）
- **evaluated_at**: 2026-11-09T12:33:30+08:00
- **verdict**: **PASS**

---

## 1. 冻结 TC 逐条实测（6 条全部独立实跑）

执行目录：`/tmp/qa-rt-ej`（独立 worktree，绑定 `feat/kernel-effect-journal`）
测试命令按 `tests.md` 规范原样执行，追加 `-v` 并锚定 `^Name$` 验证单测真实执行。

| TC ID | AC ID | Package | Command | Exit Code | RUN Count | PASS Count | Duration | Verdict |
|-------|-------|---------|---------|-----------|-----------|------------|----------|---------|
| TC-001 | AC-001 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestImplementEffectCarriesIdempotencyKeyAndWritesJournal$' -count=1` | 0 | 1 | 1 | 0.448s | PASS |
| TC-002 | AC-001 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestEffectWithoutIdempotencyKeyIsRefused$' -count=1` | 0 | 1 | 1 | 0.237s | PASS |
| TC-003 | AC-002 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestReplaySkipsSucceededEffectAndResumeListsProvenStages$' -count=1` | 0 | 1 | 1 | 0.237s | PASS |
| TC-004 | AC-002 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestFailedOrIncompleteRecordsAreNotProvenStages$' -count=1` | 0 | 1 | 1 | 0.220s | PASS |
| TC-005 | AC-003 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestResumeWithValidJournalProceedsFromProof$' -count=1` | 0 | 1 | 1 | 0.247s | PASS |
| TC-006 | AC-003 | internal/effectjournal | `go test ./internal/effectjournal -run '^TestResumeWithMissingOrCorruptJournalFailsClosed$' -count=1` | 0 | 1 | 1 | 0.228s | PASS |

---

## 2. 运行时行为核验细节

### 2.1 幂等键与 Journal 事实写入（AC-001 / TC-001, TC-002）
- **副作用执行严格绑定幂等键**：`Journal.Execute(Effect)` 要求 `IdempotencyKey` 非空，且属于单次 run 内的独立副作用标识。成功执行后生成包含 7 个固定字段的 JSONL 记录（`schema_version: 1`、`run_id`、`stage`、`idempotency_key`、`effect_kind`、`status: "succeeded"`、`recorded_at` RFC3339 时间戳）。
- **缺少幂等键拒绝执行且不落盘**：当 `IdempotencyKey` 为空字符串或纯空白时，直接返回 `ErrMissingIdempotencyKey`；`Apply` 闭包调用次数为 0，磁盘与内存 Journal 记录数为 0，防止无幂等保障的副作用发生。

### 2.2 重放去重与已证明阶段（AC-002 / TC-003, TC-004）
- **同 Key 重放只执行一次**：对已存在且 `status == "succeeded"` 的 `(run_id, idempotency_key)` 发起重放时：
  - 底层 `Apply` 闭包绝不重新调用（调用计数保持 1）；
  - Journal 追加写入一条 `status: "skipped_replay"` 记录；
  - 返回结果 `Executed: false`，`Replayed: true`，`Status: StatusSucceeded`。
- **Failed 与不完整记录不进入已证明阶段**：
  - 发生错误的副作用被记为 `status: "failed"`；
  - 缺少必填字段（如缺少 `idempotency_key`）的不完整记录 `Complete() == false`；
  - `ProvenStages(runID)` 严格过滤：只有 `status == StatusSucceeded` 且 `Complete() == true` 的完整记录才计入已证明阶段；
  - 对 failed 记录的 key 发起重复执行时返回 `ErrDuplicateIdempotencyKey`，绝不为失败记录伪造成功或重新调用。

### 2.3 基于证明的恢复与 Fail-Closed 机制（AC-003 / TC-005, TC-006）
- **基于已证明阶段跳过与续跑**：
  - `Resume(path, runID, stages)` 读取 Journal，将已完整成功的阶段归入 `ProvenStages`，未证明阶段归入 `RerunStages`；
  - 恢复执行时只运行 `RerunStages` 中的阶段，已证明阶段 0 次重跑；
  - 全部阶段完成后再次调用 `Resume`，`RerunStages` 长度为 0。
- **异常 Journal 严格 Fail-Closed**：
  - 缺失文件：返回 `ErrJournalMissing`，`plan.FailClosed = true`，`RerunStages` 为空；
  - 损坏/非 JSON 内容：返回 `ErrJournalCorrupt`，`plan.FailClosed = true`，`RerunStages` 为空；
  - 未知 Schema 版本（如 `schema_version: 99`）：返回 `ErrSchemaUnsupported`，`plan.FailClosed = true`，`RerunStages` 为空；
  - 发生任何异常时零阶段重跑，拒绝盲目恢复。

---

## 3. 全量测试与环境验证

在分支上执行单测与语法检查：
- `go test ./internal/effectjournal -count=1` 退出码 0，耗时 0.351s。
- 6 条 TC 全部独立通过，RUN=1 PASS=1 FAIL=0。
- `go vet ./internal/effectjournal` 无任何报警。

---

## 4. 验收结论与纪律核验

- **Verdict**: **PASS**
- **只读纪律**:
  - 未执行切分支、commit、push、merge 等写分支行为。
  - 冻结包 `truth.md` / `tests.md` / `tasks.md` / `evidence.md` 及既有 evidence 逐字节未变。
  - 仅写入本文件 `evidence/qa/runtime.md`。
