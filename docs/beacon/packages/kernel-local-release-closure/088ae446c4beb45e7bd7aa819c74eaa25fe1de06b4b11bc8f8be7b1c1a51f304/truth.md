---
slug: kernel-local-release-closure
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
  truth: docs/beacon/v2.0.0/features/kernel-local-release-closure/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-local-release-closure/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-local-release-closure/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-local-release-closure/evidence.md
---

# Requirement Truth: kernel-local-release-closure

## 人话

给本机 Beacon Go 的发布者：把确切源码、被验二进制、证据、一次批准和实际安装连起来；重启能续，漂移可见。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "按照推荐方案执行确保质量稳定交付。确认并授权本地提交。"
goal: "用同一份候选完成独立验收、正式门禁、一次人工批准和可恢复本机安装。"
non_goals: "不推送、不打 tag、不做跨仓发布，不新造通用发布平台，不静默替用户批准。"
constraints: "复用现有 gate 和事件；本地发布工具可用标准库；实现者与 QA 分开；旧 run 的审计保留；不强删工作文件。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 是否按五段复述执行并授权本地需求基线与实现提交？ | 本机单仓，最终产物一次确认 | answered | 确认并授权本地提交。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | user_intent: §1–3 确切候选到独立验收 | tests.md#tc-001 |
| AC-002 | user_intent: §3、§5 安装续跑与拒绝旧构建覆盖 | tests.md#tc-002 |
| AC-003 | user_intent: §2 实际安装对账 | tests.md#tc-003 |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | [cli] Release 在 prepared 状态绑定需求指纹、确切源码 commit、包含的修复、构建条件、二进制 SHA-256、独立 QA 证据哈希及 run 清单。查询给出候选身份。缺证据、证据变动、二进制变动分别拒绝批准和安装，批准必须匹配整份清单摘要。安装使用已验二进制而不重新编译。 (restates=3) |
| AC-002 | [cli] Release 的 approved 候选安装先锁定目标并核对期望旧哈希，保留可验证旧版备份；先持久化安装意图再原子替换，核验目标哈希和功能后登记 installed。替换后记账前中断可从该意图恢复且不重复批准；两个正常安装调用串行；旧调用的前置哈希过期就拒绝覆盖。现有工作文件不得被恢复过程强删。 (restates=5) |
| AC-003 | [cli] Release 在 installed 的历史交付不因机器漂移而抹除。只读查询同时显示期望与实际二进制哈希及 aligned、drifted、missing 三种观察结果，返回确定的非零偏差码；不得自动覆盖未知产物。安装失败可查原因和备份，不能仅因有 QA 文档就显示完成。 (restates=2) |

## 用户旅程

发布者查看源码及修复清单，构建一次候选。独立验收者检查该候选并登记证据。用户核对摘要后批准。工具安装该份产物，核验后记回执；中断后继续同一批准。日后查询发现外部覆盖时显示偏差，保持历史事实。

## Domain Model

| Entity | Key | Precedence |
|--------|-----|------------|
| Release | manifest_digest | root |
| Evidence | content_hash | requires Release |
| Approval | manifest_digest + actor | requires Evidence |
| InstallIntent | manifest_digest + target | requires Approval |
| Observation | target + observed_hash | requires InstallIntent |

## Domain FSM — Release

| To | From | Guard |
|----|------|-------|
| approved | prepared | 用户批准匹配清单和证据 |
| installing | approved | 目标锁与旧哈希前置条件满足 |
| installed | installing | 目标哈希与功能核验通过 |
| failed | installing | 安装核验失败 |
| installing | failed | 原批准有效且明确恢复 |

## FSM 五元组

Only required when frontmatter domain_required: true. Fill the
five-tuple, declare terminal states (终态： ...), and add an
Illegal Transitions section below listing forbidden moves
(- illegal: from -> to). The graph is machine-validated at
freeze: a missing target, a missing event, or a missing illegal
section blocks.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|
| prepared | approve | 人工确认整份清单且证据有效 | approved | 持久化批准 |
| approved | install | 锁定目标且旧哈希匹配 | installing | 先记录意图 |
| installing | verify | 目标哈希及功能符合清单 | installed | 记录完成回执 |
| installing | failure | 操作未成功 | failed | 保留原因及备份 |
| failed | resume | 相同批准和候选仍有效 | installing | 观察实际产物后续跑 |
| installed | observe | 核对实际安装物 | installed | 另记运行偏差，不修改历史 |

终态： installed

## Illegal Transitions

- illegal: prepared -> installing
- illegal: approved -> installed
- illegal: installed -> prepared

## Parity Matrix

Only required when scope_mode: full_parity. List the source capability,
whether Beacon covers it, and the evidence.

| Capability | Covered | Evidence |
|------------|---------|----------|
