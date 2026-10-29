---
slug: p0-executor-desktop
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
  truth: docs/beacon/v2.0.0/features/p0-executor-desktop/truth.md
  tests: docs/beacon/v2.0.0/features/p0-executor-desktop/tests.md
  tasks: docs/beacon/v2.0.0/features/p0-executor-desktop/tasks.md
  evidence: docs/beacon/v2.0.0/features/p0-executor-desktop/evidence.md
---

# Requirement Truth: p0-executor-desktop

## 人话

给 Beacon 内核的使用者（本机开发者）：内置执行器模板写的 `--context-file` 参数在真实 `codex exec` / `claude -p` 里根本不存在，导致执行器永远停在 `stage_executor_not_registered`、Beacon 只会 park 不会跑；桌面端也没有任何地方能配置执行器或点一下推进。本包把模板改成真实 CLI 认可的参数、用真二进制跑通集成证据（对齐已冻结的 AC-043/AC-045），并把"选执行器 + 推进"接到桌面端。做到"真实 codex/claude 能被驱动、界面能推进、没有执行器时如实 park 而不是假装完成"算完。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "修正内置执行器 profile 使其匹配真实 codex/claude CLI 参数，并让桌面端能配置执行器、点击推进长程任务"
goal: "Beacon 能真实驱动 codex/claude 执行 stage，用户能在桌面端选择执行器并推进，无执行器时如实 park"
non_goals: "不新增厂商私有协议；不改 SubprocessBackend 核心语义；ACP 协议属 P1 不在本包"
constraints: "参数以真实 CLI --help 为唯一依据；上下文经 stdin 或 prompt 传递；真跑验证限定最小代价一次"
scope_mode: "focused"
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.
Use beacon truth ask / beacon truth answer to drive this ledger.

| Q ID | Question | Recommendation | Status | Answer | Source |
|------|----------|----------------|--------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | 给 Beacon 内核使用者：内置执行器模板参数与真实 CLI 不符导致永远 park，本包修正模板并让桌面端可配置/推进；完成=真实 codex/claude 能被驱动且桌面端能推进，无执行器时如实 park | human |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 约束「参数以真实 CLI --help 为唯一依据」 | 实测 `codex exec --help` / `claude -p --help` 均无 `--context-file`（grep 无匹配），当前模板违反；见 internal/harness/profile.go:46,51 |
| AC-002 | 目标「真实驱动 codex」+ 全局冻结 AC-043 | docs/ac-matrix.md:52 deferred-external（前置=真 codex）；本机 `codex --version`=0.156.1 已装，前置满足 |
| AC-003 | 目标「真实驱动 claude」+ 全局冻结 AC-045 | docs/ac-matrix.md:54（复测 claude 2.1.281 已装，"缺的只是真多轮证据"）；本机已装 |
| AC-004 | 约束「上下文经 prompt 传递」 | codex --help 明载 stdin 追加语义；claude 的 stdin 未证实，故统一采用 prompt 文本传递（两 CLI 均接受单个位置参数） |
| AC-005 | 非目标「不改 SubprocessBackend 语义」+ no-fake-delivery | internal/harness/backend.go：fabricates IterationResult 即为 fixture |
| AC-006 | 目标「桌面端可选执行器并推进」 | 现状 internal/agentapi/api.go 仅 StartGoal/GetGoalStatus/ReleaseGoal，无 Tick/Driver |
| AC-007 | 目标「桌面端能推进」 | 现状 desktop/frontend/index.html 仅"创建目标运行/刷新状态/评估发布"三按钮 |
| AC-008 | 目标「无执行器时如实 park」 | internal/orchestrator/tick.go：`!ok` → applyBlockBatched(ReasonExecutorNotReg) |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 内置 codex-style / claude-style 模板的每个 flag 必须真实存在于对应 CLI 的 `--help` 输出；不得出现 `--context-file` 这类真实 CLI 不认识的 flag（当前模板即违反此条） |
| AC-002 | codex-style profile 能驱动**真实 codex 二进制**完成 1 次成功 iterate：exit_code=0、success=true、上下文路径可达（对齐全局 AC-043 的形状，本包取最小 1 次） |
| AC-003 | claude-style profile 能驱动**真实 claude 二进制**完成 1 次成功 iterate：exit_code=0、success=true、上下文路径可达（对齐全局 AC-045） |
| AC-004 | 阶段上下文经 prompt 文本传递：解析后 argv 的 prompt 元素含 stage-context.json 绝对路径，执行器据此读取上下文；由 TC-004 机检该路径确实出现于 argv（不依赖真跑） |
| AC-005 | 执行器缺失 / 不存在 / 非零退出 / 超时时，返回明确 reason_code 且 success=false，绝不伪造 success（失败面机检，不依赖真跑） |
| AC-006 | `agentapi` 暴露推进能力（Tick / 单步 driver）与执行器配置（选 profile、可选覆盖命令），供桌面端调用 |
| AC-007 | 桌面端前端能选择执行器并点击推进，实际调用 AC-006 暴露的接口，状态随之刷新 |
| AC-008 | 无执行器注册时，桌面端如实显示 blocked/parked 与 reason_code（`stage_executor_not_registered`），不显示"完成/通过" |

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
