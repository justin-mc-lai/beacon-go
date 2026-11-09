# 独立 QA Verdict · 负向回归面（第 2 轮）：PASS

- **mode_id**: `qa-regression-rg2`
- **run_id**: none（只读验收，独立临时 worktree）
- **stage**: `qa / negative-regression`
- **bind**: `feat/kernel-recover-guard-scope` @ `6a27a8c9716e923f176c367e9031f3b75ee53505`（修复实现 `15c8133`；上轮判 FAIL 的树 `d1f08c4`）
- **baseline**: `main` @ `ffdfced12656d46a075ee7400a82d9f5fa847a86`（未动）
- **worktree**: `/tmp/qa-rg-rg2`（detached @ `6a27a8c`，独立创建）
- **verdict**: **PASS** — 上轮唯一阻塞项（external 收尾后 status 误报 blocked）已修复并在自写探针上复现；五类 fail-closed 与原因闭集全部成立；10/10 TC、全量、ee 包回归全绿。
- **实测时间**: 2026-11-09 21:47–21:52 +0800

---

## 一、上轮阻塞项复测（本轮最关键）

**路径**：真实 external run → 4 次 tick 到 release → release tick 人类门禁 `release_requires_human_confirmation`（写 `stage_exit_blocked`）→ `ReceiveExternalResult`（带真实 artifact）→ `Engine.Status`。

**实测字段（探针原文）**：

```
external closure final: status="completed" blocked=false reason="" run_status="Succeeded"
```

- 事件：`external_result_received`=1、`goal_completed`=1、`release_confirmed`=0（external 路径按 truth 设计不伪造人类 release）、`stage_exit_blocked`=1。
- 上轮 `d1f08c4` 同一序列的 `status="blocked" blocked=true reason="release_requires_human_confirmation" run_status="Paused"` **已不再出现**。
- 修复点与自报一致：`effectiveBlockView` 在 `LoopStage==done` 时直接返回未阻断；事件流重建时晚于最新人类门禁的 `goal_completed` / `external_result_received` 覆盖该门禁；`ReceiveExternalResult` 语义未动。

**探针**：`go test ./internal/orchestrator -run '^TestQARegRg2' -count=1 -v` → **EXIT=0**（3.975s，11 个用例组全部 PASS，跑毕已删除）。

---

## 二、终止事实优先未开后门（实测 PASS）

| 检查 | 实测 |
|---|---|
| 完成后再补 `stage_exit_blocked` 事件 + attach-verifier + tick + recover | status 仍 `completed / blocked=false / reason=""` |
| **仅伪造 `goal_completed` 事件、未经 external 收尾**（快照仍 blocked） | status 仍 `blocked=true / reason=release_requires_human_confirmation`，loop 仍 `release_gate`（快照阻断优先于事件扫描，不绕过） |
| 反向护栏：release 门活着、无人确认、无 external 收尾 | status `blocked / blocked=true / reason=release_requires_human_confirmation / run_status=Paused`（未早泄成 completed） |

---

## 三、其余绕过路径逐条复测（全部 PASS）

| # | 类别 | 实测结论 |
|---|---|---|
| 1 | 5 个人类门禁 reason 逐一 recover | 全部 `goal_recovery_human_gate_forbidden`；快照 metadata JSON 前后逐字节一致；`goal_recover_rejected`=1、`goal_recovered`=0、`release_confirmed`=0、`human_gate_cleared`=0 |
| 2 | attach-verifier 单独 | status 的 blocked / blocked_streak / last_block_reason / run_status 前后全等；`verifier_attached` payload `cleared_block=false` |
| 3 | ContinueBy（streak≥3） | 返回 `product_goal_blocked_streak`；stage 未推进；`goal_continued`=0 |
| 4 | Resume | 快照被清（既有语义），但 status 仍从最新 `stage_exit_blocked` 还原 `blocked / release_requires_human_confirmation`，不隐藏人类门禁 |
| 5 | receive-result 外部证据四类绕过 | 无外部完成记录（`goal_completed{source=manual}`）/ 非 QA artifact（kind=evidence）/ hash 不符（文件被篡改）/ 文件已删 —— qa 门 `qa_stage_executed` 均不成立，verdict 均 `blocked`；hash 不符时 `qa_artifact_refs_valid` 也不成立 |
| 6 | 原因闭集 | `HumanGateReasonCodes` 恰 5 项；7 个近似码（少字母/全大写/加后缀/`manual_block`/两个新增 reason）均不在集内；近似码阻断按可恢复类正常清除且 status 不复活 blocked |

---

## 四、10 条冻结 TC 逐条实跑（全部 EXIT=0）

`go test ./internal/orchestrator -run '^<TC>$' -count=1 -v`（本 worktree @ `6a27a8c`）

| TC | 测试函数 | Exit |
|---|---|:---:|
| TC-001 | TestRecoverClearsRecoverableBlockAndResetsStreak | 0 |
| TC-002 | TestRecoverRefusesHumanGateReasonsWithoutStateChange | 0 |
| TC-003 | TestHumanConfirmationClearsHumanGateBlock | 0 |
| TC-004 | TestRecoverCannotSubstituteForHumanConfirmation | 0 |
| TC-005 | TestRecoverEventCarriesClearReceipt | 0 |
| TC-006 | TestRejectedRecoverLeavesRejectionTrail | 0 |
| TC-007 | TestStatusShowsHumanGateBlockTruthfully | 0 |
| TC-008 | TestRecoverAndAttachVerifierDoNotHideHumanGateBlock | 0 |
| TC-009 | TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked | 0 |
| TC-010 | TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted | 0 |

TC-010 为执行层断言：真实 tick → `ReceiveExternalResult` → `Engine.Status`，非测辅助函数；`tests.md` 有显式可跑命令。

---

## 五、全量回归与 ee 包回归

- `go test ./... -count=1`（本树独立跑）→ **EXIT=0**；17 包 `ok`、`cmd/beacon` 无测试文件、零 FAIL；`internal/orchestrator` 97.014s。
- `go test ./internal/orchestrator -run 'TestQAGate|TestExternal|TestAttachVerifier|TestHardBlocked' -count=1 -v` → **EXIT=0**（1.359s），14 个用例（含 3 子测）全 PASS。

---

## 六、truth / 范围 / 原因值核对

- `truth.md` SHA256 = `93adff7b5422ac2915c34c600744df908cb44403ff523dcd274045c2e06292d7`，与 freeze-log 记录的 freeze 后值一致（上一版 `fdc99c15…39e27`）。
- `beacon truth drift --project <repo> --feature kernel-recover-guard-scope --version v2.0.0 --json` → EXIT=0：`status=frozen, frozen=true, clean=true, body_drift=false, open_questions=0, base_diverged=false`，frozen 与 current SHA 相同。
- TASK-008 仍未勾；TASK-000..007 的 evidence 指向包内真实文件，无自指。
- `git diff --name-status main..HEAD`：14 个新增 docs + 4 个源码修改（`cli/goal.go`、`core/reasons.go`、`orchestrator/complete.go`、`orchestrator/engine.go`）+ 1 个新增测试文件；**无既有测试被改写**。
- `reasons.go` 既有 reason 字符串值：与 `main` 集合比对 **零删除**，仅新增 2 个常量 + `HumanGateReasonCodes`/`IsHumanGateReason` 与 gofmt 对齐。

---

## 七、非阻塞观察（不改变 PASS 结论）

1. **伪造终止事件 + 快照已被 Resume/Continue 清空**：先 `Resume`（既有语义清快照）再直插一条 `goal_completed` 事件，status 会变为 `running / blocked=false`（不谎报 completed，但不再显示人类门禁）。该路径需要绕过引擎直接写事件库（CLI 无 raw event append），且依赖 AC-001 之外的既有清快照语义，正常业务流程不可达。建议后续包把终止事实的判定收紧为「`external_result_received` + `goal_completed` 成对」或校验事件来源，做纵深防御。
2. `truth.md` 快照块仍有字面 `intent_confirmed: false`（门禁只读 frontmatter，不影响 signed）；两份 `changes/*.md` 仍有 `status: pending`（ledger 已 refrozen）；`truth_sha256_before==after` 是 `truth rebase --accept-current` 痕迹，ledger 有 `truth_rebase` 记录、findings=0，非绕检。均为上轮已记录的非阻塞项。
3. `qa-contract.md` 运行时/负向两行仍写「8 条 TC」（实为 10 条），遗留措辞，建议顺手修订。

---

## 八、边界与未验证

- 只写本 verdict；未 commit、未 push、未 merge、未重装；未改实现与 truth。
- 自写探针 `internal/orchestrator/qa_reg_rg2_probe_test.go` 跑完已删除，worktree 除本 verdict 外干净。
- 未验证：合 main、重装二进制、发布说明；本轮 `main` 仍 `ffdfced`。
