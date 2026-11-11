---
slug: kernel-recovery-qa-bridge
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/evidence.md
---

# Requirement Truth: kernel-recovery-qa-bridge

## 人话

给 Beacon 内核的使用者：现在有三条「明明做完了却走不下去」的死路。第一，`goal block` 会给 feature 的快照记一个 `Implement/Blocked`，但 `goal recover` 只清 run 上的阻塞字段，快照还停在 Blocked，于是同一条 run 永远卡在 implement 出口。第二，`qa autotest run` 把测试跑绿了，却只记 `autotest_executed`，qa 出口闸要的是真实 `stage_executed`，于是「先证明 QA 跑过」和「跑过才给证明」互相等。第三，qa 出口闸在 run 自己没有 verdict 时会回落到同 feature 的旧 verdict，上一条 run 的 qa pass 能让一条毫无 QA 证据的新 run 直接进 release。本包把三条都收成可证伪的契约，先不动生产代码。

## 起点现场（2026-11-11，HEAD afd5cc7）

派工要求把「plan run 未建号」记进 truth，现状如下：

- `docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/` 在派工前不存在，本目录由 plan 手工建立。
- `.beacon/state/` 为空目录，没有该 feature 的 run 状态文件。
- `beacon-go goal status` 不带 `--run-id` 时报 `required flag(s) "run-id" not set`；仓库里也没有该 feature 的 goal run 记录。
- 结论：该 feature 的 plan run **未建号**，本包 truth 由本次派工手工建立，未执行 `truth freeze`，`intent_confirmed: false`。
- 三条 seed 全部使用独立 `BEACON_STORE_PATH` 和临时项目目录，不读也不写全局 `~/.beacon` 与仓库 `.beacon/`。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "修三个已实测的内核缺口（P3）：goal recover 不同步 lifecycle 快照；QA 依赖 executor-marker=false 的 stage_executed 但 autotest 不写；QA exit gate 的 FORMAL-FIRST 让上一次 run 的 blocked/pass verdict 污染新 run"
goal: "把三条缺口写成可在 HEAD afd5cc7 逐条复现失败的 plan 包，并给出修复后的验收契约"
non_goals: "本包不改生产代码；不 freeze；不 push；不自证 QA；不新增顶层命令；不改 reason code 词表"
constraints: "三条 seed 必须可执行、当前必然失败、带 sha256；停在 freeze 前"
scope_mode: focused
```

## 三个缺口（实测）

### 缺口 A：`goal recover` 不同步 lifecycle 快照

- 阻塞写入：`internal/orchestrator/tick.go:50` `applyBlockOn` 把 `feature_lifecycle` 写成 `stage=Implement, status=Blocked, reason_codes=[...]`。
- 恢复写入：`internal/orchestrator/complete.go:82` `Engine.Recover` 只清 run 的 `Blocked/BlockedStreak/LastBlockReason` 和 stage plan 状态，**没有**回写 `feature_lifecycle`。
- 自锁点：`internal/orchestrator/exitgates.go:271` 只要 `LifecycleStage==implement && LifecycleStatus==blocked` 就返回 `ReasonStageExitImplementBlk`。
- 实测：`after_block=Implement/Blocked`，`after_recover=Implement/Blocked`，下一次 tick `blocked=true, reason_code=stage_exit_implement_blocked`。

### 缺口 B：autotest 通过但 qa 闸不认

- `qa autotest run` 只追加 `autotest_executed`（`internal/orchestrator/autotest.go:53`，写入点 `:217`），并注册 `test_result` 产物；它不写 qa 的 `stage_executed`。
- 闸只认真实 `stage_executed`：`internal/orchestrator/gates_facts.go:247` 要求 `Placeholder != nil && !*Placeholder`，且 `Recording` 缺省或 false；`internal/gate/evaluate.go:258` `EvaluateQA` 用 `QAStageExecuted` 决定 `qa_stage_executed` 检查。
- 手动 tick 车道会在 qa 通过时由 `internal/orchestrator/external_stage_evidence.go` 写 executor-marker=false 的 `stage_executed`，所以手动车道能过；断的是 autotest 车道。
- 实测：`autotest total=1 passed=1`，紧接着 `gate evaluate qa` 得到 `verdict=blocked, reason_codes=[qa_stage_not_executed]`，检查明细 `qa_artifacts_present=True, qa_stage_executed=False, qa_artifact_refs_valid=True`，真实 qa `stage_executed` 计数为 0。

### 缺口 C：qa verdict 跨 run 污染

- `internal/orchestrator/exitgates.go:163` `gateFactOf` 先取本 run 的 `LatestGates`，取不到就回落到 `FeatureGates`。
- `internal/orchestrator/exitgates.go:395` `featureGateFallback` 从该 feature 的所有 gate 行构造回落，只排除被兄弟 `stage_exit_evaluated` 事件引用的行。
- `gate evaluate qa --run A` 写入的 gate 行没有对应的 stage-exit 引用，于是会漏进回落。
- 实测：run A 用自己的 QA 证据拿到 `pass`；新建 run B、零 QA 证据，B 的 tick 直接到 `release, blocked=false, reason_code=None`，借用了 A 的 pass。

## Open Questions

One question at a time; a package cannot be signed while a row is open. 以下为 plan 级建议，最终签署留给人。

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | `goal recover` 应该把 lifecycle 快照恢复到什么状态？ | 同 stage、`status=active`、清空 `reason_codes`；已 release/completed 的快照不降级 | answered | 采用：复用 `workflowStageForProductStage`，与 run 当前 stage 对齐 |
| Q2 | autotest 车道怎么补真实 qa 证据？ | 聚合完成后补写一条 `stage_executed`（stage=qa、executor-marker=false、recording=false、outcome=passed），保留 `autotest_executed` 供审计 | answered | 采用：闸的 marker 规则完全不动，只让 autotest 也走真实证据路径 |
| Q3 | autotest 有失败或 0 个 TC 时能不能写 passed？ | 不能；失败/空集不写 outcome=passed 的 stage_executed，qa 闸保持拒绝 | answered | 采用：宁可直接报 `qa_stage_not_executed`，也不伪造通过 |
| Q4 | 跨 run 污染怎么收，是否要改表？ | 不改表：`gate_evaluated` 事件已带 run_id + gate_result_id，feature 回落必须排除「被其他 run 绑定」的 gate 行 | answered | 采用：qa 出口只认本 run 事实；feature 回落只服务未绑定任何 run 的 feature 级 gate |
| Q5 | 是否顺手改 FORMAL-FIRST？ | 不改；本包只保证 formal 事实来自本 run，run 内仍 formal 优先 | answered | 采用：`ReasonStageExit*` 与 formal/derived 优先级保持现状 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 派工缺口①「Implement/Blocked 自锁」 | internal/orchestrator/complete.go:82；internal/orchestrator/tick.go:50；internal/orchestrator/exitgates.go:271 |
| AC-002 | 派工缺口②「autotest 写了 autotest_executed 而闸不认」 | internal/orchestrator/autotest.go:53,217；internal/orchestrator/gates_facts.go:247；internal/gate/evaluate.go:258 |
| AC-003 | 派工缺口③「上一次 run 的 qa verdict 污染后续新 run」 | internal/orchestrator/exitgates.go:163,395；gate_evaluated 事件载荷 |
| AC-004 | 既有安全边界「human gate 不可被 recover 清掉」 | internal/orchestrator/complete.go:97-118；gates_facts.go:247 marker 规则 |
| AC-005 | 派工硬约束「不自证 QA、不新增词表」 | internal/core reason code 常量；exitgates.go:280-300 FORMAL-FIRST 分支 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `goal recover --by <id>` 成功后，`feature_lifecycle` 与本 run 对齐（同 stage、`status=active`、`reason_codes` 为空）；该 run 下一次 implement tick 不再返回 `stage_exit_implement_blocked`，能正常推进到 qa |
| AC-002 | `qa autotest run` 聚合结果为有 TC 且全部通过时，该 run 产生一条真实 qa `stage_executed`（`stage=qa`、`executor-marker=false`、`recording=false`、`outcome=passed`），随后 `gate evaluate qa` 的 `qa_stage_executed` 为 true；失败与 0 个 TC 两种情形均不产生 passed 证据 |
| AC-003 | qa 出口闸只采信本 run 的 gate 事实：同一 feature 的其他 run（无论 pass 还是 blocked）不得让新 run 跳过 qa；feature 级回落只用于未绑定任何 run 的 feature 级 gate |
| AC-004 | 负向回归：human gate reason 仍拒绝 `goal recover` 且快照不变；`executor-marker=true`、`recording=true`、缺失 marker 三种 `stage_executed` 仍不计入 `qa_stage_executed` |
| AC-005 | 不新增、不改名 reason code；run 内 FORMAL-FIRST 保持（有本 run formal qa verdict 时仍由 verdict 决定，derived 只做无 verdict 时的兜底） |

## 建议修复方向（plan 建议，未实现）

- AC-001：在 `Recover` 的同一事务里把 `feature_lifecycle` 回写为当前 stage 的 active 快照；沿用 `updateLifecycleMonotonic` 的 release/completed 保护；human gate 的拒绝路径在任何写之前返回。
- AC-002：`RunAutoTest` 聚合成功（total>0 且 failed==0）后，补写 `stage_executed{stage:qa, outcome:passed, executor-marker:false, recording:false, source:qa_autotest}`；`autotest_executed` 保留为审计事件；闸的 marker 规则不动。
- AC-003：`featureGateFallback` 排除任何被 `gate_evaluated` 事件绑定到其他 run 的 gate 行；等价地，qa 出口只信 `LatestGates`，feature 回落只接受没有任何 run 绑定的行。

## Boundary

- 本包是 plan 包：不改任何 `*.go`，不 freeze，不写 intent 签名，不 push，不发布。
- 不新增 reason code，不改 `ReasonStageExitImplementBlk`、`qa_stage_not_executed` 等既有词。
- 不改 executor/recording/manual 的接口约定，不改 `gate evaluate` 的顶层语义。
- 三条 seed 只证明「当前失败」；修复后的绿由 implement + 独立 QA 验证，本包不自证。
