---
slug: kernel-goal-advise
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
  truth: docs/beacon/v2.0.0/features/kernel-goal-advise/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-goal-advise/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-goal-advise/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-goal-advise/evidence.md
---

# Requirement Truth: kernel-goal-advise

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "内核只排事实，AI 只改措辞，这样每次都可复跑"
goal: "提供 goal advise：按确定性事实给一个项目的 feature 排序，输出建议阶段与证据字段；不写任何自然语言建议，不含时间戳，保证同一份事实输入得到逐字节一致的结果"
non_goals: "不调用模型、不生成散文、不读需求正文、不改变任何 run 状态、不新增 reason code"
constraints: "只读账本；规则版本化 advise-v1；排序必须可复跑；输出字段必须全部来自 spine 快照"
scope_mode: focused
```

## 人话

进度已经能从 spine 看到，但没人替你排"先做哪个"。本需求给内核一个只读的排序器：把阻塞、进行中、阶段、暂停等事实折算成分数，给出建议的下一阶段；措辞交给内核之外的一层，内核不写句子。

## Acceptance Criteria

| AC ID | 说明 | 级别 |
|-------|------|------|
| AC-001 | 同一项目内，带硬阻塞码的 feature 排在健康 feature 之前，且其分数不低于 100；迁移类阻塞码（migration_ 前缀）不参与加分 | Must |
| AC-002 | 同一份事实连续执行两次，输出逐字节一致（输出不含时间戳与随机量） | Must |
| AC-003 | 输出中的每个 feature 都必须存在于该项目的 spine 快照，每个 blocker 都必须出现在该 feature 的快照阻塞码里；schema 与版本号固定为 goal-advise.v1 与 advise-v1 | Must |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | goal：按确定性事实排序 | seeds/seed1_blocked_ranks_first.sh |
| AC-002 | utterance：每次都可复跑 | seeds/seed2_deterministic.sh |
| AC-003 | non_goals：不生成散文、不虚构事实 | seeds/seed3_no_invented_facts.sh |

## 设计约定

1. 只读存储；排序纯函数，输入是 spine 快照切片。
2. 分数规则：硬阻塞 +100，进行中 +40，实施或验收阶段 +20，暂停 +10；同分按 feature 名升序。
3. 输出字段只做拷贝，不派生任何原文里没有的字符串。
