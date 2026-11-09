# Independent QA Verdict · spec/AC 面（第 2 次复验）

- verdict: **PASS**
- mode_id: `qa-spec-ac-rg3`
- stage: `qa / spec-AC`
- bind: `feat/kernel-recover-guard-scope` @ `6a27a8c`（修复 `15c8133`，证据 `6a27a8c`；上一轮 FAIL 树 `d1f08c4`，更早 `21047a4`）
- main: `ffdfced`（未动）
- verifier: 验收员（独立验收，非 maker）
- checked_at: 2026-11-09 21:31 +0800（Asia/Shanghai）
- worktree: `/tmp/qa-spec-rg3`（`git worktree add --detach /tmp/qa-spec-rg3 feat/kernel-recover-guard-scope`，HEAD `6a27a8c`）

## 1. truth 变更是否越界

**结论：未越界。只新增一段 status 规则，AC 判据原文与 fingerprint 均未变。**

- truth SHA256 实测 `93adff7b5422ac2915c34c600744df908cb44403ff523dcd274045c2e06292d7`，与内核回执及 freeze-log 一致；上一版 `fdc99c155b0516539657ac30fda0a95c65c0d4762a0f6df37c2eafa1c6939e27`。
- `git diff d1f08c4..6a27a8c -- truth.md` 只有一处新增（status 呈现规则「终止事实优先」一段），无删除、无改写。
- Acceptance Criteria 小节与 `d1f08c4` 逐字节相同（脚本比对 `AC section identical: True`）。
- AC-001..004 判据原文与四份 `evidence/ac-r*.md` 逐字一致，长度 218 / 240 / 367 / 283 字。
- `beacon truth show --json` 的 `change_ac_diff` 为 `added/changed/removed` 全空；最近一次 `truth_refrozen` 事件的 `requirement_fingerprints` 为 AC-001 `9752f2d1…`、AC-002 `7e9f35f5…`、AC-003 `dcd242a8…`、AC-004 `810186c6…`，与原冻结值完全相同。
- 新增段落的语义边界核过：只在 LoopStage=done 或存在晚于最新人类门禁的 `goal_completed` / `external_result_received` 时解除重建；仍存活在 release 门上的 run 不受影响（独立探针 `LIVE_GATE` 复验仍为 blocked/paused）。AC-004 的「release 等待人类」主场景未被放宽。
- `beacon truth drift --project . --feature kernel-recover-guard-scope`：`clean: true`、`body drift: false`，EXIT=0（主仓实跑）。
- truth change 过程：`truth_change_requested` 13:17:03 → `truth_change_rebased` 13:17:54.138（`--accept-current`，base 为冻结的 `d1f08c4` 正文）→ `truth_integrity_evaluated` findings=0 → `truth_refrozen` 13:17:54.487；ledger 有据，非绕检。

## 2. TC-010 是否为执行层断言

**结论：是。**

- `internal/orchestrator/recover_guard_scope_test.go:439` `TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted`：真实 `TickBy` 推进 4 次 → release tick 断言人类门禁 → 真实 `ReceiveExternalResult` → 真实 `Engine.Status` 断言 `completed / blocked=false / last_block_reason="" / run_status=Succeeded`，并核对 `external_result_received`、`goal_completed` 各 1 条。不是只测辅助函数或常量。
- `tests.md` 已显式新增 TC-010 行（AC-004 负向映射），Command 为可直接运行的 `go test ./internal/orchestrator -run '^TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted$' -count=1`，本 verdict 实跑 EXIT=0。
- TC-009 同为执行层：真实 6 步 Engine 路径，断言最终四字段与 `release_confirmed`/`human_gate_cleared` 各 2 条。

## 3. 冻结 TC 实跑（逐条 `-count=1 -v`，worktree `/tmp/qa-spec-rg3`）

| TC | 测试函数 | EXIT |
|---|---|:---:|
| TC-001 | `TestRecoverClearsRecoverableBlockAndResetsStreak` | 0 |
| TC-002 | `TestRecoverRefusesHumanGateReasonsWithoutStateChange` | 0 |
| TC-003 | `TestHumanConfirmationClearsHumanGateBlock` | 0 |
| TC-004 | `TestRecoverCannotSubstituteForHumanConfirmation` | 0 |
| TC-005 | `TestRecoverEventCarriesClearReceipt` | 0 |
| TC-006 | `TestRejectedRecoverLeavesRejectionTrail` | 0 |
| TC-007 | `TestStatusShowsHumanGateBlockTruthfully` | 0 |
| TC-008 | `TestRecoverAndAttachVerifierDoNotHideHumanGateBlock` | 0 |
| TC-009 | `TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked` | 0 |
| TC-010 | `TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted` | 0 |

每条 1 个 `--- PASS`、0 个 `--- FAIL`；逐条日志在 `/tmp/tc-<TestName>.log`。

全量：`go test ./... -count=1` → **EXIT=0**（本分支独立跑，17 包 ok + `cmd/beacon` 无测试文件；orchestrator 97.6s）。

## 4. 验收员自写探针（临时文件，已删）

命令：`go test ./internal/orchestrator -run '^TestQAProbeRg3$' -count=1 -v` → **EXIT=0**。

- 六步重复确认路径（外部 run → release tick 阻断 → 首次确认无 verifier 失败但写事实 → 挂 verifier → 换身份 tick 重新阻断 → 被拒 recover → 二次确认 Complete）：最终 **`status=completed` / `blocked=false` / `last_block_reason=""` / `run_status=Succeeded`**；事件 24 条，`release_confirmed`=2、`human_gate_cleared`=2，最后确认索引 21 晚于最后门禁阻断索引 19。
- external 收尾路径（release tick 阻断 → `ReceiveExternalResult`）：最终 **`completed / blocked=false / "" / Succeeded`**，本次 FAIL 的回归已消除。
- 反向护栏：release 门仍存活、无人确认、无终止事件时，status 仍为 **`blocked / paused / release_requires_human_confirmation`**，新增规则没有把真实等待人类误报成完成。
- 探针断言 `HumanGateReasonCodes` 恰 5 项，且近似码 `release_requires_human_confirmation_typo` 不在集合内。

## 5. 其余形式项

- TASK-000..007 已勾，指向的 evidence 文件全部存在且无自指；TASK-008 保持未勾（`[ ]`，文案已更新为 10 条 TC）。
- `qa-contract.md` 运行时面已从「8 条」改为「10 条」，遗留措辞已收口。
- `reasons.go`：既有 301 个 reason 常量值零改动、零删除，只新增 `ReasonRecoveryHumanGateForbidden`、`ReasonReleaseConfirmIDRequired` 与封闭 `HumanGateReasonCodes`（5 项）。
- ee 包既有测试：`go test ./internal/orchestrator -run 'TestQAGate|TestExternal|TestAttachVerifier|TestHardBlocked' -count=1` → **EXIT=0**（1.443s）。
- 范围：`git diff --stat main..HEAD` 19 文件 = 5 个 Go（cli/goal.go、core/reasons.go、orchestrator/complete.go、engine.go、recover_guard_scope_test.go）+ 14 个本包 docs；`cmd/` 无关代码、版本号、ee 包、另两个分支均未动。
- worktree 除本 verdict 外干净；未 commit、未 push、未改实现与 truth。

## 非阻塞 nit

1. `truth.md` 的 User Intent Snapshot 块里仍有字面 `intent_confirmed: false`，而 frontmatter 为 `true`、ledger 有 `truth_intent_confirmed` 事件。门禁只读 frontmatter，不影响 signed 状态；属快照历史字段，建议后续清理措辞。
2. 两份 `changes/*.md` 的 frontmatter 仍写 `status: pending`，但 ledger 已 `truth_refrozen`、`truth show` 为 `frozen/change_pending=false`。属变更文档展示口径，不影响判定。
3. `truth_sha256_before == after == 93adff7b…` 是因为 rebase `--accept-current` 把当前正文确认为基线；过程已在 freeze-log 与 ledger 留痕，integrity findings=0。

## 判定

**PASS（spec/AC 面）。** 上一轮阻塞项（external 收尾后 status 误报 blocked）已修复，AC 判据未被扩张或放宽，10 条冻结 TC 与独立探针均实跑通过。

未验证：运行时面与负向回归面在 `6a27a8c` 上的复验、合 main、重装、发布说明。
