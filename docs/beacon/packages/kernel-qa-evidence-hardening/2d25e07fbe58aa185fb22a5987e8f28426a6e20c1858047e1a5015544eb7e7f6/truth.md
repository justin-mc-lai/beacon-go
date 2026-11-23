---
slug: kernel-qa-evidence-hardening
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
  truth: docs/beacon/v2.0.0/features/kernel-qa-evidence-hardening/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-qa-evidence-hardening/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-qa-evidence-hardening/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-qa-evidence-hardening/evidence.md
---

# Requirement Truth: kernel-qa-evidence-hardening

## 人话

让 Beacon Go 只对当前候选的真实测试与独立产品结论放行，空跑和错证据明确拒绝。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "确认下一步， 先修模型路由和内核 QA 共性缺口，再恢复第二湖开发"
goal: "实际执行有效测试、同候选独立QA与统一门禁后恢复第二湖"
non_goals: "不重写业务测试、不另造调度内核、不自动发布安装"
constraints: "原始证据、同run同候选、唯一写者与独立检查者、精确路径本地提交"
scope_mode: "focused"
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 用户已确认修模型路由、修四项内核QA共性缺口，再恢复第二湖；执行有效、同候选绑定、独立结论与统一推进均需真实CLI成功/拒绝证据。 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user_intent: restatement §1、§3 | tests.md#tc-001 |
| AC-002 | user_intent: restatement §2、§3 | tests.md#tc-002 |
| AC-003 | user_intent: restatement §3 | tests.md#tc-003 |
| AC-004 | user_intent: restatement §3、§5 | tests.md#tc-004 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | [cli] qa autotest 对每个 TC 读取实际机器结果，记录执行用例名称与数量；空跑、skip、失败、缺少声明必跑项、缺机器结果均不计通过，不写 passed QA stage。Go JSON、JUnit、自定义机器结果三种适配遵守同一判定；兼容真实脚本不等于接受 exit 0。 (restates=3) |
| AC-002 | [cli] 经现有 artifacts 入口登记当前候选，绑定 run、需求摘要、测试合同摘要、结果提交、maker 和产物路径及 SHA-256。自动测试及 QA 证据携带相同候选身份；当前候选替换、产物变化、错 run、旧需求证据、跨 run 拼接均阻断。 (restates=3) |
| AC-003 | [cli] qa gate 要求结构化独立产品 QA 结论，checker 与 maker 不同，scope=product，verdict=pass，并绑定当前候选和声明 TC。合同review、自报非结构PASS、自验、缺绑定拒绝；同候选最新失败不得被旧成功覆盖。attach-verifier 不能凭空白报告绕过检查。身份是本机授权登记者声明，不冒称平台认证。 (restates=3) |
| AC-004 | [cli] formal QA、QA stage exit、manual/external tick、driver、receive-result 的适用推进路径采用同一当前候选判定；产物数量和非空目录均不得替代QA判定回退放行。complete 对当前证据重验，release 人工批准不变。已发布历史不被回写，新评估缺旧证据则明确阻断。 (restates=3) |

## 用户旅程

登记候选→执行真实测试→登记独立产品QA→同一判定检查→待人工发布。失败修复后重新执行并登记新证据；新候选不继承旧候选结论。

## Domain Model

| Entity | Key | Precedence |
| --- | --- | --- |
| candidate_revision | run_id + manifest_hash | 10 · root |
| execution_record | candidate_revision + TC + invocation | 20 · requires candidate_revision |
| independent_review | candidate_revision + review_id | 30 · requires execution_record |
| eligibility | candidate_revision + evidence_hashes | 40 · requires independent_review |

## Domain FSM

每个候选revision不变；下表为当前观察态。被替代revision保留历史但不再作用于当前run。

| To | From | Guard |
| --- | --- | --- |
| awaiting_tests | - | 候选登记且绑定有效 |
| tested | awaiting_tests | 必跑测试真实执行通过 |
| verified | tested | 独立产品QA通过且对象一致 |
| eligible | verified | 证据新鲜且统一门通过 |
| rejected | awaiting_tests | 测试无效 |
| rejected | tested | QA拒绝 |
| rejected | verified | 证据失效 |
| rejected | eligible | 最新失败或产物变化 |
| awaiting_tests | rejected | 同候选重新验证 |
| superseded | awaiting_tests | 新候选替代 |
| superseded | tested | 新候选替代 |
| superseded | verified | 新候选替代 |
| superseded | eligible | 新候选替代 |
| superseded | rejected | 新候选替代 |

## FSM 五元组

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| awaiting_tests | execute | 必跑测试真实通过 | tested | 留机器证据 |
| tested | review | 独立产品结论匹配 | verified | 登记QA |
| verified | evaluate | 所有证据新鲜 | eligible | 正式门判定 |
| awaiting_tests | reject | 测试无效 | rejected | 保留拒绝原因 |
| tested | reject | QA失败 | rejected | 保留拒绝原因 |
| verified | invalidate | 文件变化 | rejected | 关闭当前资格 |
| eligible | invalidate | 最新失败或产物变化 | rejected | 关闭当前资格 |
| rejected | retry | 候选仍有效 | awaiting_tests | 重新执行验证 |
| awaiting_tests | replace | 新候选登记 | superseded | 保留旧历史 |
| tested | replace | 新候选登记 | superseded | 保留旧历史 |
| verified | replace | 新候选登记 | superseded | 保留旧历史 |
| eligible | replace | 新候选登记 | superseded | 保留旧历史 |
| rejected | replace | 新候选登记 | superseded | 保留旧历史 |

终态：superseded

## Illegal Transitions

- illegal: awaiting_tests -> eligible
- illegal: tested -> eligible

## Guard Failure Cases

测试未实际运行不得awaiting_tests进入tested；合同review不得tested进入verified；缺证据不得verified进入eligible。均留拒绝原因，不伪造通过。每项见对应TC。

## 生产接口

同包 interfaces.md 是本修复机器接口合同。使用现有 artifact/event 存储；不新增第二套数据库。正式CLI用隔离账本实跑，不用同包helper代替入口验收。声明28项通过数不是业务覆盖率；专业反例设计仍由独立QA承担。

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
