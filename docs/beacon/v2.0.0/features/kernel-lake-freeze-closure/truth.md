---
slug: kernel-lake-freeze-closure
surface_strict: true
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft
materials_status: current
surfaces: [cli]
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-lake-freeze-closure/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-lake-freeze-closure/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-lake-freeze-closure/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-lake-freeze-closure/evidence.md
---

# Requirement Truth: kernel-lake-freeze-closure

## 人话

program 拆解一个意图时，湖必须在流程内**全部冻结**；但现在冻结要求 evidence 引用实现产物，产物又只能登记到 run，run 又要求 truth 已冻 —— 循环逼迫人在实现期反复 refreeze。本湖拆开这个环：合同指纹与 evidence 分离、artifact 支持 feature 作用域、预检失败进入事件流。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "issue #10：一个意图要能在流程内拆解到全部湖冻结，而不是实现期反复人工冻结"
goal: "evidence 追加不再使冻结失效；artifact 可 feature 作用域登记；truth 预检失败写 feature 维度事件"
non_goals: "不取消 restatement/intent 签名；不降低合同面 drift 拦截；不改 release 人工门；不动产品仓"
constraints: "合同面（truth/tests/tasks/restatement）任一变化仍按现行 drift 流程；事件写失败不阻断 freeze 主流程但 stderr 告警；只改 beacon-go 内核"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | evidence 与合同指纹如何分界？ | 以文件路径集合为界：evidence-only 变更不重算合同指纹 | answered | 只锁 truth/tests/tasks/restatement |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | issue #10 R2：evidence 追加触发反复 refreeze | evidence.md#AC-001 |
| AC-002 | issue #10 R1：freeze↔run↔artifact 循环 | evidence.md#AC-002 |
| AC-003 | issue #10 事件流核对：pre-run 失败不在流里 | evidence.md#AC-003 |
| AC-004 | 约束：不得降低 gate 严格度 | evidence.md#AC-004 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 修改 evidence.md（新增/改写内容）后，冻结包不报 `revision_authority_drift`；修改 truth.md/tests.md/tasks.md/restatement.md 任一文件仍报 drift (restates=1) |
| AC-002 | `goal artifacts register --feature <slug>` 在无 run 时登记成功，且该 feature 的 evidence 引用校验通过；run 作用域登记同时保留 (restates=3) |
| AC-003 | `truth freeze/refreeze` 预检失败时写一条带 `feature_id` 的生命周期事件（含 reason codes），可在 `goal trace`/spine 按 feature 查到 (restates=5) |
| AC-004 | restatement/intent 签名、release 人工门、gate 严格度不变；合同面变更仍被拦 (restates=4) |

## FSM 五元组

Only required when frontmatter domain_required: true. Fill the
five-tuple, declare terminal states (终态： ...), and add an
Illegal Transitions section below listing forbidden moves
(- illegal: from -> to). The graph is machine-validated at
freeze: a missing target, a missing event, or a missing illegal
section blocks.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
