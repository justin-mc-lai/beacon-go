# QA 负向回归 verdict：kernel-effect-journal

- **verdict: PASS**
- mode_id: `qa-regression-neg-p5`
- run_id: none（只读独立负向回归验收，未建 goal run）
- stage: `qa / Implementation-Committed`
- 分支 / 目标提交: `feat/kernel-effect-journal` @ `f12ca9cd11c546fc9815538fc511c10611f252df`（实现 `e7fd1ba`）
- 验收方: QA 验收 · 回归（独立，maker 不自签）
- 验收时间: 2026-11-09 12:36–12:40 +08:00
- 独立 worktree: `/tmp/qa-rg-ej`（detach f12ca9c，未 checkout main）

## 判据与结论

| # | 契约必核项 | 结果 | 实际依据 |
|---|-----------|------|----------|
| 1 | 缺 key fail-closed | PASS | 空串 / 空白 / `\t\n` 三种 key 均返回 `ErrMissingIdempotencyKey`，底层 Apply 执行 **0 次**，journal 记录 0 条，且 journal 文件未被创建 |
| 2 | 重复 replay 只执行一次 | PASS | 同 `(run_id, key)` 首次 `Executed=true`，其后 3 次全部 `Replayed=true / Executed=false`；Apply 仅调用 1 次；journal 共 4 条（1 succeeded + 3 skipped_replay）；`ProvenStages` 只含该 stage 一次 |
| 3 | failed / 不完整记录不计为 proven | PASS | Apply 返回 error → 记录 `failed`，`ProvenStages` 为空；同 key 再执行返回 `ErrDuplicateIdempotencyKey` 且 **不再重跑**（calls=0），失败不被翻成成功；手工植入「合法 JSON 但 `effect_kind` 为空、status=succeeded」记录 → `ProvenStages` 仍排除；`Append` 不完整记录被 `ErrIncompleteRecord` 拒绝 |
| 4 | 缺 journal / 损坏 journal fail-closed | PASS | 文件不存在 → `ErrJournalMissing`、`FailClosed=true`、`RerunStages` 空；中间行非 JSON → `ErrJournalCorrupt`；`schema_version:99` → `ErrSchemaUnsupported`；三种情况均零阶段重跑；合法 journal 只跳过 succeeded（s1 证明、s2 重跑）；换 `run_id` 不继承证明（无跨 run 泄漏） |
| 5 | 全量 `go test ./... -count=1` 退出码 0 | PASS | `/tmp/qa-rg-ej` 实跑 **EXIT=0**，18 个包全 `ok`，`real 171.46s`（orchestrator 157.7s、evalgate 96.8s 均真跑） |
| 6 | 既有冻结包测试无改写 | PASS | `git diff --name-only main...feat/kernel-effect-journal -- '*_test.go'` 仅 `internal/effectjournal/effectjournal_test.go`；分支自身改动仅本包实现+本包 docs，无其它包测试文件 |

## 冻结 TC 复跑（排除 `-run` 0 匹配假绿）

6/6 EXIT=0，每条 `--- PASS` 至少 1 行：

```
TestImplementEffectCarriesIdempotencyKeyAndWritesJournal        EXIT=0
TestEffectWithoutIdempotencyKeyIsRefused                        EXIT=0
TestReplaySkipsSucceededEffectAndResumeListsProvenStages        EXIT=0
TestFailedOrIncompleteRecordsAreNotProvenStages                 EXIT=0
TestResumeWithValidJournalProceedsFromProof                     EXIT=0
TestResumeWithMissingOrCorruptJournalFailsClosed                EXIT=0
```

## 我实际跑过的命令

```bash
# 全量回归（worktree /tmp/qa-rg-ej）
/usr/bin/time -p go test ./... -count=1          # EXIT=0, real 171.46s

# 6 条冻结 TC 逐条锚定全名
go test ./internal/effectjournal -run '^TestImplementEffectCarriesIdempotencyKeyAndWritesJournal$' -count=1 -v
go test ./internal/effectjournal -run '^TestEffectWithoutIdempotencyKeyIsRefused$' -count=1 -v
go test ./internal/effectjournal -run '^TestReplaySkipsSucceededEffectAndResumeListsProvenStages$' -count=1 -v
go test ./internal/effectjournal -run '^TestFailedOrIncompleteRecordsAreNotProvenStages$' -count=1 -v
go test ./internal/effectjournal -run '^TestResumeWithValidJournalProceedsFromProof$' -count=1 -v
go test ./internal/effectjournal -run '^TestResumeWithMissingOrCorruptJournalFailsClosed$' -count=1 -v

# 自写独立负向探针（临时文件，跑完已删除，不留在分支）
go test ./internal/effectjournal -run '^TestQANeg' -count=1 -v
#   TestQANegMissingKeyRefusedZeroExecution             PASS
#   TestQANegReplayExecutesOnceThenSkips                PASS
#   TestQANegFailedAndIncompleteNeverProven             PASS
#   TestQANegResumeFailClosedOnBadJournals              PASS
#   ok beacon-go/internal/effectjournal 0.742s

# 范围与合入可行性
git diff --name-only main...feat/kernel-effect-journal -- '*_test.go'
git merge-tree --write-tree --name-only main feat/kernel-effect-journal   # EXIT=0，无冲突
```

## 附注（不影响 PASS）

- `main` 当前 `a3eab40`（冻结 docs 已提交），工作区 0 项；`main` 相对本分支领先 1、本分支领先 2，merge-base `3396558`。`merge-tree` 预演 **无冲突**。
- 两包 truth 在 main 工作树上 `beacon truth drift` 均为 `clean: true / body drift: false`。
- 本 face 只判负向与回归；端到端 CLI/orchestrator 接线不在范围内（truth 已把范围钉在 `internal/effectjournal`）。
- 临时 worktree 保留供取证：`/tmp/qa-rg-ej`。其中只新增本文件；探针测试已删除，`git status` 除 `evidence/qa/regression.md` 外无其它改动，未 commit / 未 push / 未改实现与 truth。
