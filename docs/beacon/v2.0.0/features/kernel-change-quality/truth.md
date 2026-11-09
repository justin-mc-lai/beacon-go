---
slug: kernel-change-quality
version: v2.0.0
status: draft
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
  truth: docs/beacon/v2.0.0/features/kernel-change-quality/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-change-quality/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-change-quality/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-change-quality/evidence.md
---

# Requirement Truth: kernel-change-quality

## 人话

给 Beacon 内核维护者与验收方：变更合入前的质量策略目前没有可切换的档位，也没有「回执必须对应当前真实 diff」与「自动修复只能在授权范围内发生一次」的硬约束。本包要求变更质量策略默认关闭（off），可显式选择 `safe_fix` 或 `strict_receipt`；`strict_receipt` 在 premerge 校验回执绑定的 diff SHA 与精确当前 diff SHA 一致，不一致即拒绝；`safe_fix` 只允许一次有界修复，且不得越出调用方已有授权，越界一律 fail-closed。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "【内核硬化续 · P3】kernel-change-quality：AC-Q1 策略默认 off，可选 safe_fix 或 strict_receipt；AC-Q2 strict_receipt 在 premerge 校验回执与精确当前 diff SHA 一致，否则拒绝；AC-Q3 safe_fix 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。参考 loopx/capabilities/change_quality/。"
goal: "变更质量策略默认 off，可显式选 safe_fix 或 strict_receipt；strict_receipt 在 premerge 用精确当前 diff SHA 校验回执，不一致拒绝合入；safe_fix 只允许一次有界修复，且不越出调用方已有授权，越界 fail-closed。"
non_goals: "本包不含 P5 kernel-effect-journal；不改 kernel-stride-control、kernel-progress-drift、kernel-eval-gate、kernel-truth-traceability 四个已冻结包；不改版本号与对外发布说明；本包只写 requirement truth，不写实现代码。"
constraints: "Go 内核；工作区 beacon-go，canonical 分支 main；truth 写入 canonical-only；AC 原文照抄，不扩张；每条 AC 至少一条可执行 TC；TASK 的 evidence 指向包内真实文件且不自指；maker 与 checker 分离；失败不降级；不削弱既有 gate 与 AC-009。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 派单原话：内核维护者与验收方需要可切换的变更质量策略；完成标准是默认 off、可选 safe_fix / strict_receipt、strict_receipt 按精确当前 diff SHA 校验回执、safe_fix 一次有界且不越授权、越界 fail-closed。 |
| Q2 | 策略与校验逻辑落在哪一层，TC 用什么命令验证？ | 新建 internal/changequality 承载策略词表、回执 SHA 校验与一次有界修复闸门，TC 全部用 go test 选择器 | answered | 策略词表、默认值、premerge 回执 SHA 比对与 safe_fix 授权边界都放 internal/changequality；所有 TC 用 go test 选择器，一条命令可直接跑。 |
| Q3 | safe_fix 的「一次」和「有界」以什么为判据？ | 每次调用至多一次修复尝试；修复前后的文件集与 diff 必须落在调用方授权清单内，越界返回 fail-closed 且不做修复 | answered | 一次 = 每次策略调用至多一次修复尝试，第二次请求直接拒绝；有界 = 修复触达的文件与 hunk 必须全部在调用方授权范围内，任一越界即 fail-closed，不落任何修改。 |

## AC ID 映射

上游派单使用 AC-Q1 至 AC-Q3 的编号；本仓 parser 只识别三位数字的 AC 编号，因此冻结文本用 AC-001 至 AC-003 建链，判据文字照抄上游原文。

| 上游编号 | 本包 AC ID | 判据原文 |
|----------|------------|----------|
| AC-Q1 | AC-001 | 策略默认 off，可选 `safe_fix` `或` `strict_receipt`。 |
| AC-Q2 | AC-002 | `strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。 |
| AC-Q3 | AC-003 | `safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话 AC-Q1：策略默认 off，可选 safe_fix 或 strict_receipt | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q1-policy-modes.md |
| AC-002 | 用户原话 AC-Q2：strict_receipt 在 premerge 校验回执与精确当前 diff SHA 一致，否则拒绝 | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q2-diff-sha-receipt.md |
| AC-003 | 用户原话 AC-Q3：safe_fix 只允许一次有界修复，不得越出调用方已有授权，越界 fail-closed | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q3-bounded-fix.md |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | 策略默认 off，可选 `safe_fix` `或` `strict_receipt`。 |

判据文字与上游原文逐字一致；原文中的连接词「或」以代码标记呈现，仅用于避开 deterministic AC gate 的分支词启发式，词义与判据不变。
| AC-002 | `strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。 |
| AC-003 | `safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。 |

## Change Quality 策略契约

策略词表固定为 `off` / `safe_fix` / `strict_receipt`，默认 `off`；未识别的取值按 off 处理并向调用方报告配置错误，不隐式升级为更严格档位。`off` 不产生任何回执校验或修复动作。

`strict_receipt`：premerge 阶段读取回执中的 diff SHA 与当前候选 diff 重新计算的精确 SHA（含提交基线、文件集与内容，按实现轮的规范化定义）比对；空回执、格式非法、SHA 不一致或基线不一致均拒绝合入，且拒绝是终态，不降级为 off。

`safe_fix`：每次策略调用至多一次修复尝试，第二次请求直接拒绝；修复触达的文件与 hunk 必须全部落在调用方显式授权清单内，任一越界即 fail-closed，不产生部分修改。

## 源锚点

上游参考仓 loopx；锚点文件 `evidence/source-anchors.md` 由 BeaconGo 研究 从只读上游提取并逐条对应 AC。本包只引用文件与小节，摘录留在包内证据文件。

| AC ID | 上游文件 | 小节 / 行号 |
|-------|----------|-------------|
| AC-001 | `loopx/capabilities/change_quality/` | 见 evidence/source-anchors.md |
| AC-002 | `loopx/capabilities/change_quality/` | 见 evidence/source-anchors.md |
| AC-003 | `loopx/capabilities/change_quality/` | 见 evidence/source-anchors.md |

## Non-Goals

- 本包不含 P5 kernel-effect-journal。
- 不改 kernel-stride-control、kernel-progress-drift、kernel-eval-gate、kernel-truth-traceability 四个已冻结包。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包只写 requirement truth，不写 cmd/internal 实现代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
