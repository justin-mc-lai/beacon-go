# QA verdict · 运行时行为面（第 2 次复验）· kernel-recover-guard-scope

| 项 | 值 |
|---|---|
| verdict | **PASS** |
| mode_id | qa-runtime-rg2 |
| stage | qa / runtime |
| bind | feat/kernel-recover-guard-scope @ 6a27a8c9716e923f176c367e9031f3b75ee53505 |
| 上一轮 FAIL 树 | 21047a4（初版假阻断）、d1f08c4（external 收尾后被 status 重建误判为 blocked） |
| 返修实现提交 | 15c8133 fix(orchestrator): let terminal facts win over historical human-gate blocks |
| docs/refreeze 提交 | 6a27a8c docs(kernel-recover-guard-scope): record TC-010 terminal-fact fix and refreeze |
| main | ffdfced（本轮未动） |
| 验收时间 | 2026-11-09 21:42 +0800 |
| 验收方 | QA 验收 · agy（视觉/端侧，独立运行时验收，maker≠checker） |
| truth SHA256 | 93adff7b5422ac2915c34c600744df908cb44403ff523dcd274045c2e06292d7 |

---

## 结论

**PASS**。  
在 `6a27a8c` 分支（含修复 `15c8133` + refreeze `6a27a8c`）上完成独立运行时行为复验与自写独立探针实测：

1. **10 条冻结 TC 逐条实跑**：10/10 全部 PASS，退出码均为 0。新增的 TC-010（`TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted`）实跑 0.757s PASS，退出码 0。
2. **全量回归测试独立实跑**：`go test ./... -count=1` 退出码 0（17 包 ok，1 包无测试文件，orchestrator 耗时 96.911s，无任何 FAIL）。
3. **重点复测 External 收尾回归路径（原 d1f08c4 回归项）**：
   - 路径：external run 推进至 release_gate → release tick 触发人类门禁阻断（`release_requires_human_confirmation`）→ `ReceiveExternalResult` 收尾；
   - 最终 `goal status` 实测：**`status="completed"` / `blocked=false` / `last_block_reason=""` / `run_status="Succeeded"`**；
   - 事件流：`external_result_received`=1、`goal_completed`=1、`release_confirmed`=0，验证终止事实优先覆盖既有人类门禁阻断，原回归彻底修复。
4. **反向护栏核验**：
   - release 门活着、未经人类确认且未收到 external 交付时，`goal status` 输出 **`status="blocked"` / `blocked=true` / `last_block_reason="release_requires_human_confirmation"` / `run_status="Paused"`**；
   - 确认终止事实优先规则未将真实等待人类确认的门禁误报为已完成。
5. **六步重复确认路径复验**：
   - 确认 → 挂 verifier → 换身份重新阻断 → recover 拒绝 → 二次确认 Complete 成功，最终 `goal status` 输出 **`completed` / `blocked=false` / `last_block_reason=""` / `run_status="Succeeded"`**；
   - 事件流中 `release_confirmed`=2、`human_gate_cleared`=2，第二次确认事件（索引 21）严格晚于最后一次门禁阻断事件（索引 19）。
6. **既有安全守卫与防御机制全量保持**：
   - 5 个人类门禁 reason 逐一 recover 均被拒绝（`goal_recovery_human_gate_forbidden`），快照 metadata 逐字节不变，仅记录 `goal_recover_rejected`；
   - 非人类门禁 reason 正常恢复并清零 streak；
   - `AttachVerifier` 明确携带 `cleared_block: false` 不清未决阻断；
   - `BlockedStreak >= 3` 时 `ContinueBy` 坚决拒绝（`product_goal_blocked_streak`）；
   - 快照被抹掉时 `goal status` 成功从事件流中最新未被确认覆盖的 `stage_exit_blocked` 真实还原阻断事实。

---

## 一、10 条冻结 TC 逐条实跑结果

执行环境：worktree `/tmp/qa-rt-rg2`（`feat/kernel-recover-guard-scope` @ `6a27a8c`）

| TC ID | 测试函数 | Command | Exit Code | 耗时 | 结果 |
|---|---|---|:---:|:---:|:---:|
| TC-001 | `TestRecoverClearsRecoverableBlockAndResetsStreak` | `go test ./internal/orchestrator -run '^TestRecoverClearsRecoverableBlockAndResetsStreak$' -count=1 -v` | 0 | 0.658s | PASS |
| TC-002 | `TestRecoverRefusesHumanGateReasonsWithoutStateChange` | `go test ./internal/orchestrator -run '^TestRecoverRefusesHumanGateReasonsWithoutStateChange$' -count=1 -v` | 0 | 0.437s | PASS |
| TC-003 | `TestHumanConfirmationClearsHumanGateBlock` | `go test ./internal/orchestrator -run '^TestHumanConfirmationClearsHumanGateBlock$' -count=1 -v` | 0 | 0.774s | PASS |
| TC-004 | `TestRecoverCannotSubstituteForHumanConfirmation` | `go test ./internal/orchestrator -run '^TestRecoverCannotSubstituteForHumanConfirmation$' -count=1 -v` | 0 | 0.347s | PASS |
| TC-005 | `TestRecoverEventCarriesClearReceipt` | `go test ./internal/orchestrator -run '^TestRecoverEventCarriesClearReceipt$' -count=1 -v` | 0 | 0.414s | PASS |
| TC-006 | `TestRejectedRecoverLeavesRejectionTrail` | `go test ./internal/orchestrator -run '^TestRejectedRecoverLeavesRejectionTrail$' -count=1 -v` | 0 | 0.356s | PASS |
| TC-007 | `TestStatusShowsHumanGateBlockTruthfully` | `go test ./internal/orchestrator -run '^TestStatusShowsHumanGateBlockTruthfully$' -count=1 -v` | 0 | 0.764s | PASS |
| TC-008 | `TestRecoverAndAttachVerifierDoNotHideHumanGateBlock` | `go test ./internal/orchestrator -run '^TestRecoverAndAttachVerifierDoNotHideHumanGateBlock$' -count=1 -v` | 0 | 0.797s | PASS |
| TC-009 | `TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked` | `go test ./internal/orchestrator -run '^TestRepeatedHumanConfirmationAfterReblockKeepsStatusUnblocked$' -count=1 -v` | 0 | 0.809s | PASS |
| TC-010 | `TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted` | `go test ./internal/orchestrator -run '^TestExternalReceiveResultAfterHumanGateKeepsStatusCompleted$' -count=1 -v` | 0 | 0.757s | PASS |

---

## 二、分支独立全量回归测试

命令：`go -C /tmp/qa-rt-rg2 test ./... -count=1`  
退出码：**0**  
输出详情：
```text
?   	beacon-go/cmd/beacon	[no test files]
ok  	beacon-go/desktop	0.672s
ok  	beacon-go/internal/agentapi	6.132s
ok  	beacon-go/internal/benchgate	0.976s
ok  	beacon-go/internal/cli	19.551s
ok  	beacon-go/internal/core	1.556s
ok  	beacon-go/internal/evalgate	54.428s
ok  	beacon-go/internal/gate	2.545s
ok  	beacon-go/internal/harness	14.802s
ok  	beacon-go/internal/llm	4.419s
ok  	beacon-go/internal/migration	3.115s
ok  	beacon-go/internal/orchestrator	96.911s
ok  	beacon-go/internal/progressreview	4.718s
ok  	beacon-go/internal/session	4.650s
ok  	beacon-go/internal/store	4.249s
ok  	beacon-go/internal/stride	3.967s
ok  	beacon-go/internal/workspace	4.721s
ok  	beacon-go/skills	3.102s
```
共计 17 个测试包全部通过，1 个包（`cmd/beacon`）无测试文件，无任何 FAIL。

---

## 三、自写独立运行时探针实测细节

探针文件：`internal/orchestrator/agy_probe_test.go`（实跑完毕已彻底清除，工作区干净）。  
探针实跑命令：`go -C /tmp/qa-rt-rg2 test ./internal/orchestrator -run '^TestAgyRuntimeProbe$' -count=1 -v` → **EXIT=0**（耗时 2.561s）。

### 1. External 收尾与反向护栏实测（重点复测项）
- **反向护栏核验**（门禁活着、未确认、未收到外部交付）：
  - `status`: **`blocked`**
  - `blocked`: **`true`**
  - `last_block_reason`: **`release_requires_human_confirmation`**
  - `run_status`: **`Paused`**
  - 确认引擎保持 fail-closed，未发生状态早泄。
- **External 交付收尾核验**：
  - 调用 `ReceiveExternalResult` 输入交付报告；
  - 返回视图：`v.Status == "completed"`, `v.Blocked == false`；
  - 最终调用 `eng.Status(run.RunID)` 读取实际状态：
    - `status`: **`completed`**
    - `blocked`: **`false`**
    - `last_block_reason`: **`""`**（空）
    - `run_status`: **`Succeeded`**
  - 对应事件验证：`external_result_received` 写入 1 条，`goal_completed(source=external)` 写入 1 条，`release_confirmed` 为 0 条。终止事实优先逻辑生效，阻断未被误重建。

### 2. 重复人类确认全路径复测
- 路径时序（基于 `createExternalRun` 真实 Engine 路径）：
  1. 推进 4 个 stage 至 `release_gate`；
  2. Release 门 tick 阻断（`ReasonReleaseRequiresHuman`）；
  3. 首次 Complete（无 verifier）：按设计拒绝（`ReasonVerifierRequired`），写入第 1 条 `release_confirmed` 与 `human_gate_cleared`；
  4. 挂载 verifier（`AttachVerifier`）；
  5. 换身份 `room-second` tick：重新触发门禁阻断；
  6. 第三方调用 Recover：被拒绝（`ReasonRecoveryHumanGateForbidden`）；
  7. 二次 Complete：成功收尾；
  8. `Status` 最终四个字段：
     - `status`: **`completed`**
     - `blocked`: **`false`**
     - `last_block_reason`: **`""`**
     - `run_status`: **`Succeeded`**
  9. 事件流核对：`release_confirmed`=2，`human_gate_cleared`=2，最后一条确认事件（索引 21）晚于最后一条门禁阻断事件（索引 19）。

### 3. 五类人类门禁 Recover 拒绝与快照不可篡改核验
- 对 `release_requires_human_confirmation`、`release_awaiting_complete`、`driver_awaiting_human`、`self_heal_requires_human_confirmation`、`planner_review_human_gate_p1` 逐一测试：
  - 返回错误码：`core.ReasonRecoveryHumanGateForbidden`；
  - 快照比较：`string(rowBefore.Metadata) == string(rowAfter.Metadata)`，逐字节完全一致；
  - 事件留痕：`goal_recover_rejected`=1，`goal_recovered`=0。

### 4. 非人类门禁原因 Recover 核验
- 构造 `manual_reason` 阻断，调用 `Recover`：
  - 返回 `v.Blocked == false`, `v.BlockedStreak == 0`, `v.LastBlockReason == ""`；
  - 成功写入 `goal_recovered` 1 条。

### 5. 防御性机制与状态重建核验
- `AttachVerifier`：视图维持 `Blocked: true`, `LastBlockReason: release_requires_human_confirmation`；
- `BlockedStreak >= 3`：`ContinueBy` 坚决拒绝返回 `ReasonBlockedStreak`（`product_goal_blocked_streak`）；
- 快照历史擦除模拟：人为将快照中的 `Blocked`, `BlockedStreak`, `LastBlockReason` 置空后调用 `Status`，引擎成功从事件流中最新的人类门禁 `stage_exit_blocked` 准确重建：
  - `st.Blocked`: `true`
  - `st.LastBlockReason`: `release_requires_human_confirmation`
  - `st.RunStatus`: `Paused`

---

## 四、约束与工作区边界

1. 自写探针文件 `internal/orchestrator/agy_probe_test.go` 已在实跑结束后完全删除。
2. `/tmp/qa-rt-rg2` 工作区除本文件 `evidence/qa/runtime.md` 外，无任何未跟踪改动（工作区干净）。
3. 本轮复验未 commit、未 push、未 merge、未重装二进制，未篡改任何代码与 truth 正文。
