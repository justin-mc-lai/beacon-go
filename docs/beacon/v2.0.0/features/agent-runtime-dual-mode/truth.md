---
slug: agent-runtime-dual-mode
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
  truth: docs/beacon/v2.0.0/features/agent-runtime-dual-mode/truth.md
  tests: docs/beacon/v2.0.0/features/agent-runtime-dual-mode/tests.md
  tasks: docs/beacon/v2.0.0/features/agent-runtime-dual-mode/tasks.md
  evidence: docs/beacon/v2.0.0/features/agent-runtime-dual-mode/evidence.md
---

# Requirement Truth: agent-runtime-dual-mode

## 人话

给本机开发者：把需求对齐、角色分工、真实执行器、无头验资和桌面界面接成一个可交付的内核。2026-10-28 的决定撤销了 Web Console（AC-005），界面只保留 Wails 桌面；无头能力仍在 CLI 与 harness。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "让 Beacon 既能被外部 agent 无头调用，也能在本机桌面里对齐需求、推进长程任务"
goal: "内核具备 BYOK 网关、参谋长征询式对齐、角色流水线与工位隔离、无头机器验资和单一桌面界面"
non_goals: "不保留第二套 Web 渲染路径（2026-10-28 撤销 AC-005 的浏览器形态）；不新增厂商私有协议"
constraints: "无证据不得声称通过；maker 不自证；界面只保留桌面一种形态"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 给本机开发者：内核能无头验资、能在桌面推进；完成=真执行器可被驱动、证据哈希可复核、只存在一种界面形态 | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「BYOK 多提供商」 | `TestLLMClientFactory` |
| AC-002 | 目标「参谋长征询式对齐」 | `TestChiefOfStaffIndirectPrompt` |
| AC-003 | 目标「角色流水线与工位隔离」 | `TestMultiAgentPipelineRolesAndGate` |
| AC-004 | 目标「无头机器验资」+ 约束「无证据不得声称通过」 | `TestHarnessVerifyJSON` |
| AC-005 | 2026-10-28 桌面端决定（撤销 Web Console） | `TestSingleSurface` |
| AC-006 | 约束「maker 不自证」+ 证据链 | `TestFSMTransitionEvidenceGuard` |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | BYOK 网关：OpenAI 兼容 / DeepSeek 客户端可按角色配置；缺少 Key 时 fail-closed，不静默降级 |
| AC-002 | 参谋长只做询问式对齐：复述需求、标出隐性边界，绝不直接改代码；结构化尾巴解析失败时标记 degraded 而不是编造 |
| AC-003 | 角色流水线：架构/编码/质检分工，编码在绑定工位内执行，质检与制造者不是同一身份 |
| AC-004 | 无头机器验资：`beacon qa verify-harness --json` 仅在 QA 工件已登记、qa 阶段已执行、每个工件哈希仍匹配时返回 exit 0；否则非零并给出 reason code |
| AC-005 | 单一界面：只保留 Wails 桌面（`beacon ui`）；`/api/invoke`、`EventSource` 与第二份 HTML 不得重新出现（2026-10-28 决定） |
| AC-006 | 状态迁移证据守卫：迁移到 qa/release 前必须有证据且哈希有效；工件被改动即标记 stale 并拦截 |

## FSM 五元组

Not required: domain_required is false.

| State | Event | Guard | To | Action |
|-------|-------|-------|----|--------|

## Parity Matrix

Not required: scope_mode is focused.

| Capability | Covered | Evidence |
|------------|---------|----------|
