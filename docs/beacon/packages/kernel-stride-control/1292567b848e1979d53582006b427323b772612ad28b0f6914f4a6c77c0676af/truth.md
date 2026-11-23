---
slug: kernel-stride-control
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
traceability_required: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-stride-control/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-stride-control/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-stride-control/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-stride-control/evidence.md
---

# Requirement Truth: kernel-stride-control

## 人话

给 Beacon 内核维护者与验收方：内核跑一个 turn 时，什么时候该停、为什么停、停在哪个边界，现在没有统一记录，只能从日志里回猜。本包要求每个 turn 留下 effect、delivery、authority 三层 stride 的边界与停止原因；数量上限只当安全上限，触顶不算完成，完成必须给出语义 closeout；内层工具循环不越权限与接受边界，外层续跑不跳过 settlement；调度唤醒只表示可以检查状态，不算语义进展。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：【内核硬化程序】P1 kernel-stride-control：AC-S1 每次 turn 记录 stride{effect,delivery,authority} 的边界与停止原因；AC-S2 数值上限只作安全上限，触顶不算完成，完成必须有语义 closeout；AC-S3 内层工具循环不得跨越权限/接受边界，外层连续不得跳过 settlement；AC-S4 调度唤醒不算语义进展。参考 loopx docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md。"
goal: "内核每个 turn 都留下可审计的三层 stride 记录：记录 effect、delivery、authority 边界与停止原因；数值上限只当安全上限，触顶标记为未完成；完成由语义 closeout 证明；内层工具循环在权限与接受边界前停下，外层续跑前必须结算；调度唤醒只触发状态检查，不计为语义进展。"
non_goals: "不实现 P2 kernel-progress-drift、P3 kernel-change-quality、P5 kernel-effect-journal；不改 kernel-truth-traceability 与 kernel-eval-gate 两个已冻结包；不改版本号与对外发布说明；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核；工作区 beacon-go，canonical 分支 main；truth 写入 canonical-only；AC 原文照抄，不扩张；每条 AC 至少一条可执行 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 本轮原话：内核维护者与验收方无法从 turn 记录判断为什么停、停在哪个边界；完成标准是每个 turn 记录 effect、delivery、authority 三层边界与停止原因，数值触顶不算完成，完成必须有语义 closeout，内层不越权、外层不跳 settlement，调度唤醒不算进展。 |
| Q2 | stride 记录落在运行时哪一层，TC 用什么命令验证？ | 新建 internal/stride 承载记录与判定，turn 级集成测试放 internal/orchestrator | answered | 三层 stride 的记录结构与停止原因判定放 internal/stride，turn 结算路径的集成断言放 internal/orchestrator；所有 TC 用 go test 选择器，一条命令可直接跑。 |

## AC ID 映射

上游派单使用 AC-S1 至 AC-S4 的编号；本仓 parser 只识别三位数字的 AC 编号，因此冻结文本用 AC-001 至 AC-004 建链，判据文字照抄上游原文。

| 上游编号 | 本包 AC ID | 判据原文 |
|----------|------------|----------|
| AC-S1 | AC-001 | 每次 turn 记录 `stride{effect,delivery,authority}` 的边界与停止原因。 |
| AC-S2 | AC-002 | 数值上限只作安全上限，触顶不算完成；完成必须有语义 closeout。 |
| AC-S3 | AC-003 | 内层工具循环不得跨越权限/接受边界；外层连续不得跳过 settlement。 |
| AC-S4 | AC-004 | 调度唤醒不算语义进展。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话 AC-S1：每次 turn 记录三层 stride 的边界与停止原因 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-001-stride-record.md |
| AC-002 | 用户原话 AC-S2：数值上限只作安全上限，触顶不算完成，完成必须有语义 closeout | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-002-semantic-ceiling.md |
| AC-003 | 用户原话 AC-S3：内层工具循环不得跨越权限/接受边界，外层连续不得跳过 settlement | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-003-layer-boundaries.md |
| AC-004 | 用户原话 AC-S4：调度唤醒不算语义进展 | docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-004-scheduler-wake.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 每次 turn 记录 `stride{effect,delivery,authority}` 的边界与停止原因。 |
| AC-002 | 数值上限只作安全上限，触顶不算完成；完成必须有语义 closeout。 |
| AC-003 | 内层工具循环不得跨越权限/接受边界；外层连续不得跳过 settlement。 |
| AC-004 | 调度唤醒不算语义进展。 |

## Stride 记录契约

一个 turn 的 stride 记录是只读事实，不是完成声明。字段固定如下，实现轮次按此命名：

| 字段 | 取值 | 说明 |
|------|------|------|
| `turn_id` | 非空字符串 | 该 turn 的稳定标识，同一 turn 重放得到同一值 |
| `layer` | `effect` / `delivery` / `authority` | 本次记录所属的 stride 层 |
| `boundary` | 受控词表 | 本次 turn 触达的边界，如工具调用上限、接受边界、权限门、settlement 边界 |
| `stop_reason` | 受控词表 | 停止原因：`semantic_closeout`、`numeric_ceiling`、`authority_boundary`、`acceptance_boundary`、`settlement_required`、`scheduler_wake`、`blocked` |
| `closeout` | 非空证明，或空 | 语义完成证明；`stop_reason` 不是 `semantic_closeout` 时必须为空 |
| `outcome` | `complete` / `incomplete` / `blocked` | 由停止原因与 closeout 共同决定，不由触顶次数决定 |

判定规则：`stop_reason=numeric_ceiling` 一律得到 `outcome=incomplete`；只有 `stop_reason=semantic_closeout` 且带非空 `closeout` 才得到 `outcome=complete`；`stop_reason=scheduler_wake` 只允许触发状态检查，不产生语义进展计数；内层 `layer=effect` 的记录不得携带外层 `layer=authority` 的授权，外层 `layer=delivery` 续跑前必须已有 settlement 记录。

## 源锚点

上游参考仓 loopx，commit `c9c0644be428bbb0ab10726637f5b36602972912`。本包只引用文件与小节，行号与摘录留在包内证据文件。

| AC ID | 上游文件 | 小节 |
|-------|----------|------|
| AC-001 | `docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md` | Decision Summary；The Three-Layer Model |
| AC-002 | `docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md` | 6.1 Semantic boundaries outrank numeric budgets |
| AC-003 | `docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md` | 6.2 Inner continuation cannot borrow outer authority；6.3 Outer continuity cannot excuse missing settlement |
| AC-004 | `docs/architecture/rfcs/hierarchical-agent-stride-control-v0.md` | 6.4 Scheduler cadence is not semantic cadence |

## Non-Goals

- 不实现 P2 kernel-progress-drift、P3 kernel-change-quality、P5 kernel-effect-journal。
- 不改 kernel-truth-traceability 与 kernel-eval-gate 两个已冻结包。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
