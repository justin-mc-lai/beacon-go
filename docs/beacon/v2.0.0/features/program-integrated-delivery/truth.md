---
slug: program-integrated-delivery
version: v2.0.0
status: frozen
language: zh
domain_required: true
surfaces: [cli]
surface_strict: true
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/program-integrated-delivery/truth.md
  tests: docs/beacon/v2.0.0/features/program-integrated-delivery/tests.md
  tasks: docs/beacon/v2.0.0/features/program-integrated-delivery/tasks.md
  evidence: docs/beacon/v2.0.0/features/program-integrated-delivery/evidence.md
---

# Requirement Truth: program-integrated-delivery

## 人话

方案 B：单仓内用被验且已合入的成果接力，独立湖并行，最后对同一候选提交总验并一次签字。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确认，就按照方案 b 进行交付。可以通过 /qi-dev-openmausbot 派活到 beacon-go 内核群进行整体的交付验收并重新安装。"
goal: "单仓按需并行、可审计交接、一次海级确认后完成子湖及宿主；独立验收后重装本机 CLI。"
non_goals: "实际部署编排、发布窗口、跨仓发布和回滚、迁移旧 Program。"
constraints: "仅从有效交接提交开工，冲突或未知写范围不得并行，主星串行写入，签字绑定候选提交和独立 QA。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 是否按方案 B 和五段复述交付？ | 单仓按需并行，一次总签 | answered | 确认复述，授权提交本需求的本地冻结基线。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user_intent: §2–3 湖间启动和交接 | tests.md#tc-001 |
| AC-002 | user_intent: §3、§5 并行、冲突和续跑 | tests.md#tc-002 |
| AC-003 | user_intent: §3、§5 候选绑定总签 | tests.md#tc-003 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | [cli] `route`、单步和连续 drive 共用开工判定。上游绑定 run 的独立 QA 通过且被验提交已合入主星时，Program 在 integrating 状态记录合同指纹、QA 证据、源 SHA、集成 SHA。依赖湖从包含全部有效交接的具体 SHA 开工；未集成、提交错误、交接失效分别阻断。 (restates=3) |
| AC-002 | [cli] Program 在 integrating 状态单湖复用主工位；多个就绪且明确不重叠的湖用独立工作树真正并发。未知改动面与共享改动面均串行；合入主星串行，冲突不放行依赖湖。建工位中断、派工中断均恢复原绑定且不重复派工，无关湖仍可继续。 (restates=5) |
| AC-003 | [cli] Program 在全部选中湖到门且同一候选 SHA 总 QA 通过后进入 awaiting_approval。先持久化绑定候选、湖/run 清单及证据的单次人工签字，再进入 finalizing，完成子湖并关闭宿主。候选变更须重验重签；同一候选中断可续；单湖入口不能绕过总签。 (restates=3) |

## 用户旅程

操作者开启冻结的 Program，查询待派湖及等待原因。上游合入后派下游；需要时修复合并冲突。独立验收者对集成候选运行总回归，操作者核对该候选并签字，最后查看湖及宿主完成状态。

## Domain Model

| Entity | Key | Precedence |
|--------|-----|------------|
| Program | host_run_id | root |
| LakeRun | lake_run_id | requires Program |
| Handoff | lake_run_id + integration_sha | requires LakeRun |
| Candidate | host_run_id + candidate_sha | requires Handoff |
| Approval | candidate_sha + evidence_digest | requires Candidate |

## Domain FSM — Program

| To | From | Guard |
|----|------|-------|
| integrating | frozen | 冻结 Program 开始执行 |
| integrating | integrating | 集成回执或冲突结果可查 |
| awaiting_approval | integrating | 全湖有效且总 QA 通过 |
| finalizing | awaiting_approval | 人工签字匹配候选 |
| integrating | awaiting_approval | 候选改变 |
| integrating | finalizing | 候选失效 |
| delivered | finalizing | 审批有效且子湖全部完成 |

## FSM 五元组

实体先后：冻结 Program → 湖/run 绑定 → 工位与派工 → QA 和集成回执 → 海级候选 → 证据绑定签字。

聚合根为 Program；单湖 `awaiting_release` 不代表海可签，湖间交接可先于海级候选。

终态： delivered

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| frozen | start | 冻结范围有效 | integrating | 绑定就绪湖 |
| integrating | handoff | QA 与主星集成 SHA 可核验 | integrating | 记录交接并重算就绪湖 |
| integrating | conflict | 合入失败或工作中断 | integrating | 保留可恢复绑定，不放行依赖湖 |
| integrating | candidate_ready | 全湖集成到门且同一 SHA 总 QA 通过 | awaiting_approval | 绑定候选证据 |
| awaiting_approval | candidate_changed | SHA 或证据失效 | integrating | 撤销候选并重验 |
| awaiting_approval | approve | 人工签字匹配当前候选 | finalizing | 先记录审批再完成子湖 |
| finalizing | resume | 候选和审批一致 | finalizing | 幂等继续落账 |
| finalizing | candidate_changed | 旧审批不能覆盖变更 | integrating | 保留审计并停止旧候选落账 |
| finalizing | seal | 子湖全部完成且候选审批有效 | delivered | 关闭宿主 |

## Illegal Transitions

- illegal: integrating -> finalizing
- illegal: awaiting_approval -> delivered
- illegal: delivered -> integrating

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
