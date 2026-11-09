---
slug: kernel-external-stage-evidence
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
traceability_required: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-external-stage-evidence/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-external-stage-evidence/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-external-stage-evidence/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence.md
---

# Requirement Truth: kernel-external-stage-evidence

## 人话

给 Beacon 内核维护者与外部房间验收方：由外部房间执行的 goal run 目前永远拿不到 qa 门，因为 manual 与 external run 完成阶段时只记录阶段退出，不写 stage_executed 事件；而 qa 门只认 stage_executed。现行绕法是写入一条录制执行器产生的录制态事件，qa 门照收，形成一条虚假通过路径。本包要求 external 与 manual run 在阶段完成时落下内核写入的真实 stage_executed，qa 门拒收录制态事件，允许 receive-result 的外部完成事实配合已注册且 hash 校验通过的 QA artifacts 作为等价证据，并把 blocked_streak 达到 3 之后的 continue 拒绝语义与 attach-verifier 解除阻断的条件对齐。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "【内核缺陷 · external/manual goal run 无法满足 qa 门 · 请修复】external 运行的 goal run 永远无法让 qa 门通过，qa 门 check qa_stage_executed 恒为 false；现行绕法是塞一条 placeholder 录制事件就翻 pass。要求外部完成留下真实 stage_executed，qa 门拒收录制态事件，receive-result 外部完成加已注册 QA artifacts 可作为等价证据，并让 blocked_streak 与 attach-verifier 行为一致。"
goal: "external 与 manual run 的阶段完成留下内核写入的真实 stage_executed 证据；qa 门只认真实事件与合规的外部完成证据，录制态事件一律 fail-closed；blocked_streak 达到 3 之后的 continue 语义与 attach-verifier 解除阻断条件保持一致。"
non_goals: "不改 kernel-change-quality、kernel-effect-journal、kernel-progress-drift、kernel-stride-control、kernel-eval-gate、kernel-truth-traceability 六个已冻结包；不改版本号与对外发布说明；不清理历史库中既有事件；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核，工作区 beacon-go，canonical 分支 main；truth 与控制面写入 canonical-only；每条 AC 配两条执行层 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级；不削弱既有 gate 与已冻结 AC。"
scope_mode: focused
intent_confirmed: true
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 派单原话：外部房间真正执行后再回写的湖无法通过 qa 门与 conformance；完成标准是 external 与 manual run 留下真实 stage_executed、qa 门拒收录制态事件、receive-result 外部完成加已注册 QA artifacts 可作为等价证据、blocked_streak 与 attach-verifier 行为一致。 |
| Q2 | 真实证据与外部完成证据的取舍关系是什么？ | external 进程内阶段完成走真实 stage_executed；整湖由外部房间做完后回写时走 receive-result 外部完成事实加已注册 QA artifacts；两条路径都要求 source=external 且不可由调用者自证 | answered | AC-001 覆盖外部房间逐阶段 tick 的完成路径，AC-003 覆盖整湖外部完成后回写的路径；两者都要求事件由内核写入并标 source=external，外部调用者不能自定义事件类型绕过门禁。 |
| Q3 | 录制执行器的事件如何处理？ | qa 门判定时读取 payload 的录制标记字段，值为真即拒收；录制执行器本身保留，用于本地演示与非门禁场景 | answered | qa 门的 qa_stage_executed 判定必须拒收录制标记字段为真的事件；录制执行器继续存在，但它的输出不再能当作 qa 阶段已执行的门禁证据。 |
| Q4 | blocked_streak 达到 3 之后如何恢复？ | continue 在 streak 达到 3 时拒绝并给出原因码；attach-verifier 只有在阻塞原因被真实消除后才解除阻断；原因消除后由显式恢复动作清零 streak | answered | streak 达到 3 后 continue 返回 product_goal_blocked_streak 且不推进阶段；attach-verifier 不得单独清除未消除原因的阻断；阻塞原因被新证据真实消除后，显式恢复动作清空阻断与 streak，随后 continue 与 tick 正常推进。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：external 与 manual run 的阶段完成必须留下真实 stage_executed | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e1-external-stage-event.md |
| AC-002 | 用户原话：qa 门拒收录制态事件，现存虚假通过路径 fail-closed | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e2-recording-rejected.md |
| AC-003 | 用户原话：receive-result 外部完成加已注册 QA artifacts 作为等价证据 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e3-external-completion.md |
| AC-004 | 用户原话：blocked_streak 与 attach-verifier 行为一致 | docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e4-streak-verifier.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | external 与 manual run 记录阶段完成时，内核写入一条 stage_executed 事件，payload 含 source=external、executor=external、by 身份与录制标记字段 false，外部调用者无法自行伪造该事件。 |
| AC-002 | qa 门只接受真实 stage_executed 事件作为 qa_stage_executed 依据；录制执行器产生的录制态事件不得让 qa 门通过，依靠该事件通过的历史路径一律 fail-closed。 |
| AC-003 | beacon goal receive-result --external 记录的完成事实，连同已注册且内容 hash 校验通过的 QA artifacts，可作为 qa_stage_executed 的等价证据，事件 payload 明确 source=external，并与 AC-001 的逐阶段路径写明取舍关系。 |
| AC-004 | unresolved 阻断不得仅靠 attach-verifier 清除；blocked_streak 达到 3 后 continue 的拒绝语义与阻塞原因被真实消除后的恢复路径保持一致，attach-verifier 只在恢复条件同时满足时解除阻断。 |

## 执行事件规范

真实事件与录制态事件共用事件类型 stage_executed，判定依靠 payload 字段。

| 字段 | 真实事件取值 | 录制态事件取值 |
|------|--------------|----------------|
| stage | 阶段名 | 阶段名 |
| executor | external | 录制执行器 ID |
| source | external | 录制执行器 ID |
| by | 记录完成的身份 | 空 |
| placeholder | false | true |
| outcome | done | done |

真实事件由内核在 manual 与 external 路径记录阶段完成时写入，与 stage_advanced 同事务提交。外部调用者提交的 payload 不进入该路径：命令层只传递 by 身份，事件字段由内核填写。

## qa 门判定规则

qa_stage_executed 为真需要满足以下任一条件。

第一条路径：存在事件类型为 stage_executed、stage 为 qa、outcome 不是 failed 且 placeholder 为 false 的事件。

第二条路径：存在 receive-result 外部完成记录，source 为 external，同时存在至少一份已注册的 QA 类 artifact，且其内容 hash 校验通过。

录制标记字段为 true 的事件不满足第一条路径。字段缺失按录制态处理，门禁拒绝。

## blocked_streak 与 attach-verifier

blocked_streak 由每次阻断累加，达到 3 时原因码固化为 product_goal_blocked_streak。continue 在 streak 达到 3 时返回该原因码，并保持当前阶段不变。

attach-verifier 记录独立检查方，不改变阻塞原因是否被消除。原因未消除时 attach-verifier 返回阻断未解除的结果，不写 cleared_block=true。原因被新证据真实消除后，显式恢复动作清空阻断标记与 streak，continue 与 tick 随后正常推进。

## AC-003 与 AC-001 的取舍

AC-001 是逐阶段路径：外部房间按阶段推进时，每次阶段完成都由内核写下真实事件，qa 门直接读到 qa 阶段的完成事实。

AC-003 是整湖路径：外部房间把活干完后一次性回写，此时没有逐阶段事件，qa 门的等价证据是外部完成记录加已注册且 hash 校验通过的 QA artifacts。两条路径互不替代：逐阶段推进时不得只依赖回写记录，整湖回写时不得伪造逐阶段事件。

## 源锚点

本仓 main 分支 commit a3eab40 的代码与 sqlite 事件流。逐条文件、行号与实跑摘录写在包内 evidence/source-anchors.md。

| AC ID | 本仓文件 | 行号 |
|-------|----------|------|
| AC-001 | internal/orchestrator/tick.go | L113-L200（manual 与 external 分支只写 recordStageExit 与 stage_advanced） |
| AC-001 | internal/orchestrator/executor.go | L49-L72（录制执行器写录制态 stage_executed，录制标记字段为 true） |
| AC-002 | internal/orchestrator/gates_facts.go | L227-L241（evalQA 只判 stage 与 outcome） |
| AC-002 | internal/gate/evaluate.go | L258-L282（qa 门三项 check） |
| AC-003 | internal/orchestrator/delivery_paradigm.go | L258-L333（ReceiveExternalResult 注册 artifacts 并写 external_result_received 与 goal_completed） |
| AC-003 | internal/orchestrator/artifacts.go | L40-L70（QA 类 artifact 判定） |
| AC-004 | internal/orchestrator/tick.go | L40-L70、L348-L388（applyBlock 累加 streak，Continue 在 streak 达到 3 时拒绝） |
| AC-004 | internal/orchestrator/complete.go | L32-L56（AttachVerifier 无条件写 cleared_block） |
| AC-004 | internal/orchestrator/admission.go | L166-L173（streak 达到 3 且无修复循环时拒绝 implement 入场） |

## Non-Goals

- 不改六个已冻结包：kernel-change-quality、kernel-effect-journal、kernel-progress-drift、kernel-stride-control、kernel-eval-gate、kernel-truth-traceability。
- 不改版本号与对外发布说明；这两项由人确认。
- 不清理历史库中的既有事件与既有虚假通过记录；修复只影响判定与后续写入。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
