# Runtime QA Verdict: kernel-external-stage-evidence

- **mode_id**: `qa-runtime-behavior-ee`
- **run_id**: none（只读独立运行时行为验收，绑定分支 `feat/kernel-external-stage-evidence` @ `16dd05f`）
- **stage**: `qa / runtime done`
- **target_commit**: `16dd05f`（`feat/kernel-external-stage-evidence`；实现 `794433f` + 证据/truth `16dd05f`）
- **baseline**: `main` = `a3eab40`
- **evaluator**: QA 验收 · agy（视觉/端侧 & 独立运行时验收）
- **evaluated_at**: 2026-11-09T19:15:00+08:00
- **verdict**: **PASS**

---

## 1. 冻结 TC 逐条实测（8 条全部独立实跑）

执行工作区：`/tmp/qa-rt-ee`（独立临时 worktree，detached HEAD `16dd05f`）  
测试命令严格按 `tests.md` 规范执行，追加 `-v` 并正则锚定全名 `^TestName$`，杜绝 0 匹配假绿。

| TC ID | AC ID | Layer | Command | Exit Code | RUN Count | PASS Count | Duration | Verdict |
|-------|-------|-------|---------|-----------|-----------|------------|----------|---------|
| TC-001 | AC-001 | behavior | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1 -v` | 0 | 1 | 1 | 0.625s | **PASS** |
| TC-002 | AC-001 | negative | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1 -v` | 0 | 1 | 1 | 0.408s | **PASS** |
| TC-003 | AC-002 | negative | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1 -v` | 0 | 1 | 1 | 0.346s | **PASS** |
| TC-004 | AC-002 | behavior | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1 -v` | 0 | 1 | 1 | 0.349s | **PASS** |
| TC-005 | AC-003 | behavior | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1 -v` | 0 | 1 | 1 | 0.352s | **PASS** |
| TC-006 | AC-003 | negative | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1 -v` | 0 | 3 subtests | 3 subtests | 0.416s | **PASS** |
| TC-007 | AC-004 | negative | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1 -v` | 0 | 1 | 1 | 0.364s | **PASS** |
| TC-008 | AC-004 | behavior | `go test -C /tmp/qa-rt-ee ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1 -v` | 0 | 1 | 1 | 0.395s | **PASS** |

TC-006 包含的 3 个独立子测试全量通过：
- `no qa artifact`: PASS (0.04s)
- `hash mismatch`: PASS (0.03s)
- `missing source external`: PASS (0.03s)

---

## 2. 自写探针深度核验（三项实跑，跑后已清理探针文件）

编写独立探针集（涵盖边界与拒绝分支），实跑后立即清除，未污染代码库。

### 探针 1：录制态拒收覆盖面（Fail-Closed 闭环）
核验目标：`placeholder` 缺失、`recording=true`、`placeholder=true` 三种 payload 必须全部使 qa 门 `blocked`。
- **缺失 `placeholder`**（旧版录制态事件 payload 缺省）：`qaCheck.OK == false`，门禁判定 `blocked`，原因包含 `qa_stage_not_executed`。
- **`placeholder=true, recording=false`**（羽球营地缺陷复现绕法）：`qaCheck.OK == false`，门禁判定 `blocked`，拒绝虚假通过。
- **`placeholder=false, recording=true`**（录制标记开启）：`qaCheck.OK == false`，门禁判定 `blocked`。
- **`placeholder=true, recording=true`**：`qaCheck.OK == false`，门禁判定 `blocked`。
- **对照正向用例 `placeholder=false, recording=false`**：`qaCheck.OK == true`，门禁判定 `pass`。
- **结论**：三类录制态 payload 均被严格拒收，fail-closed 闭环成立。

### 探针 2：external 事件不可伪造性核验
核验目标：CLI 层只能传 `--by`，调用方无法控制或篡改内核关键证据字段。
- **内核权威写入**：调用 `TickBy(ctx, runID, by)` 推进 manual/external run，内核在同一事务内落盘 `stage_executed` 事件：
  - `executor`: 固定为 `"external"`（非调用方输入）
  - `source`: 固定为 `"external"`
  - `placeholder`: 固定为 `false`
  - `recording`: 固定为 `false`
  - `outcome`: 固定为 `"done"`
  - `by`: 取自调用方并自动 `TrimSpace`
- **CLI 接口核验**：`beacon goal tick` 参数仅暴露 `--by string`，无 `--executor`、`--source`、`--placeholder`、`--recording` 等后门参数。
- **防伪造阻断**：即便外部攻击者试图直接向事件表注入带 `placeholder=true` 但伪造了 `executor=external, source=external` 的事件，`evalQA` 仍准确识别并判为 `blocked`。
- **结论**：外部调用方无法伪造真实证据，防篡改成立。

### 探针 3：`beacon goal recover` 与 `attach-verifier` 行为一致性
核验目标：`attach-verifier` 绝不解除未解决阻断；`recover` 必须提供 `--by` 且能正确重置 streak 与阻断态。
- **`attach-verifier` 行为固定**：
  - 当 run 因阻断累加至 `BlockedStreak=3` 时调用 `AttachVerifier`；
  - `verifier_attached` 事件内 `cleared_block` 固定为 `false`，新增记录 `block_pending=true`；
  - run 的 `Blocked` 仍为 `true`，`BlockedStreak` 保持 `3` 不变；
  - 阻断状态下调用 `ContinueBy` / `TickBy` 仍坚决拒绝（返回 `ReasonBlockedStreak`）。
- **`recover` 命令行为**：
  - 空 `--by`（如未提供或全空白字符）调用 `Recover` 立即失败，返回 `ReasonRecoveryIdentityRequired`（`goal_recovery_identity_required`）；
  - 传入有效 `--by`（如 `qa-operator`）和 `note` 后成功恢复：
    - `Blocked` 重置为 `false`；
    - `BlockedStreak` 严格清零为 `0`；
    - `LastBlockReason` 置空；
    - 记录 `goal_recovered` 事件（包含 `by`, `note`, `prior_blocked: true`, `prior_blocked_streak: 3`）；
  - 恢复后后续 `TickBy` 正常前进至下个阶段（`truth`），无滞留阻断。
- **CLI 接口核验**：`beacon goal recover` 包含 `--run-id`（required）、`--by`（required）、`--note` 参数，参数要求与内核语义完全对齐。
- **结论**：阻断不可被挂接验收员单方旁路，显式恢复机制与身份追溯完整生效。

---

## 3. CLI 测试套件回归

执行命令：
```bash
go test -C /tmp/qa-rt-ee ./internal/cli -count=1 -run 'TestGoal'
```
- **Exit Code**: `0`
- **Duration**: 5.583s
- **Status**: PASS

---

## 4. 验收结论与只读纪律

- **Verdict**: **PASS**
- **只读纪律验证**：
  - 未执行 `git commit`、`git push`、`git merge`。
  - 主仓 `main` 保持 `a3eab40` 未动。
  - 临时探针文件已全量清除，分支工作区未留下未跟踪代码改动。
  - 本次仅在临时 worktree 内写入本份 `evidence/qa/runtime.md` 供后续合入归档。
