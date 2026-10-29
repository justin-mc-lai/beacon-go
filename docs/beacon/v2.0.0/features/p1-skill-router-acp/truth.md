---
slug: p1-skill-router-acp
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/p1-skill-router-acp/truth.md
  tests: docs/beacon/v2.0.0/features/p1-skill-router-acp/tests.md
  tasks: docs/beacon/v2.0.0/features/p1-skill-router-acp/tasks.md
  evidence: docs/beacon/v2.0.0/features/p1-skill-router-acp/evidence.md
---

# Requirement Truth: p1-skill-router-acp

## 人话

给把 beacon 技能装进自己工具链的宿主 agent（以及本机使用者）：现在 7 个技能是平铺的菜单，宿主拿到一句话只能靠模型自己挑，容易选最省事的那条；也不知道跑某个技能前该先读哪份参考。本包补两样东西——路由表（一句话到技能）与渐进式参考加载顺序——并把 ACP 协议从握手探针推进到真会话迭代。做到「宿主能自动选对技能并按序读参考、Beacon 能经标准协议驱动另一个 agent、没有真目标时如实拒绝」算完。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "为 beacon 技能集建立路由表与渐进式参考加载顺序，并实现 ACP 协议的会话迭代（session/new 加 session/prompt），让宿主 agent 自动选对技能并按序加载参考"
goal: "宿主 agent 拿到一句话能自动选中正确的 beacon 技能并按正确顺序读取参考；Beacon 能经标准 ACP 协议驱动另一个 agent 完成一次真迭代"
non_goals: "不重写现有 7 个技能的合同约束；不改 SKILL.md 正文；不新增 LLM 依赖"
constraints: "路由与加载顺序必须可机检，不依赖模型自由发挥；ACP 迭代需真实 ACP agent 目标方为证据，无目标时如实拒绝而不是伪造结果"
scope_mode: "focused"
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 给装了 beacon 技能的宿主 agent：7 个技能平铺无路由导致选错技能、也不知道先读哪份参考；本包补路由表与渐进式加载顺序并推进 ACP 真迭代；完成=宿主自动选对技能并按序读参考、有真 ACP 目标时能驱动、无目标时如实拒绝 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「宿主能自动选对技能」+ 约束「机检不靠模型自由发挥」 | 现状：7 技能平铺于 skills/ 与 ~/.workbuddy-ai/skills，无路由文件；ChiefOfStaff 只做意图对齐（internal/orchestrator/chief_of_staff.go）无 SelectSkill |
| AC-002 | 目标「自动选对技能」 | 路由需对每条例句给出唯一命中，由 TC-002 逐条断言 |
| AC-003 | 约束「机检」+ no-fake-delivery | 未命中不得猜测，避免宿主拿错技能还以为对 |
| AC-004 | 约束「按正确顺序读取参考」 | 现状：skills/<id>/references/ 存在但无加载顺序声明，宿主可能跳过必读件 |
| AC-005 | 目标「按序读参考」 | 路径存在性由 TC-005 断言，防止声明了不存在的文件 |
| AC-006 | 目标「经标准协议驱动另一个 agent」+ 全局冻结 AC-046 | 实现已到 session/prompt 会话迭代（internal/harness/acp_session.go）；缺的只是真实 ACP agent 目标作为外部证据 |
| AC-007 | 约束「无目标时如实拒绝」+ no-fake-delivery | 变更 chg_4f1f6fab3ea5：实现补全后不再无条件拒绝；本条改为要求缺目标时带 reason_code 且绝不伪造 IterationResult（internal/harness/acp_session.go） |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 路由表以机器可读文件落地（schema `beacon-skill-router.v1`），为每个技能声明触发信号、样例语句与排除条件 |
| AC-002 | 给定一句用户原话，路由返回唯一命中的技能 id；对路由表自带的每条例句断言命中结果正确 |
| AC-003 | 无命中与多义命中如实返回「未命中」并给出候选，不猜一个技能顶上 |
| AC-004 | 每个技能声明渐进式参考加载顺序（首批必读 references 路径），顺序可机检 |
| AC-005 | 路由结果附带该技能首批必读参考的绝对路径，且这些路径在磁盘上真实存在 |
| AC-006 | ACP 后端能对真实 ACP agent 目标完成 1 次会话迭代（session/new 与 session/prompt），输出符合 AC-042 schema |
| AC-007 | 无真实 ACP agent 目标时，Iterate 返回明确 reason_code 且不返回伪造的 IterationResult |

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
