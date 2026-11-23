---
slug: sdd-workflow-repair
surface_strict: true
version: v2.1.4
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
surfaces: [cli]
canonical_refs:
  truth: docs/beacon/v2.1.4/features/sdd-workflow-repair/truth.md
  tests: docs/beacon/v2.1.4/features/sdd-workflow-repair/tests.md
  tasks: docs/beacon/v2.1.4/features/sdd-workflow-repair/tasks.md
  evidence: docs/beacon/v2.1.4/features/sdd-workflow-repair/evidence.md
---

# Requirement Truth: sdd-workflow-repair

## 人话

让使用 Beacon 的 agent 不能以通用模板冒充真实需求，且同一份两文件需求能经过冻结、准入、任务执行和验收，不在下游重新索要已声明可选的文档。

## User Intent Snapshot

```yaml
utterance: "确认按照以上修正顺序，帮我整体修复下。授权继续。"
goal: "修复 SDD 契约断点，补齐独立语义审查、湖内任务驱动与有界修复"
non_goals: "不改原主工作区，不自动合并推送安装发布，不实现通用容器供应与成本治理"
constraints: "内核不调 LLM；兼容旧包；保留人工发布；隔离分支为本次授权的需求编写基线"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 本次是否按审查结论的三阶段顺序整体修复？ | 先补契约再补执行闭环 | answered | 用户确认整体修复，并授权独立分支编写、确认及冻结需求 |

## Grounding

- 源码基线 ac7ff8e5e85baae7863642f6dd9bba61bfe6bbd5；GitHub issue #11、#9 和 PR #12。
- 实测：通用模板录入舍入意图后可冻结；重复 TC 和 true 命令可冻结；缺 evidence.md 的两文件包被 truth map 拒绝；新 FSM 表无法运行时查询。
- 执行者负责实现；独立检查者负责代码审查，不由执行者自签 QA。

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|
| AC-001 | 用户确认：先消除假通过 | conversation:approved-review |
| AC-002 | 用户确认：贯通两文件契约 | conversation:approved-review |
| AC-003 | 用户确认：统一 FSM 和覆盖 | conversation:approved-review |
| AC-004 | 用户确认：独立语义审查绑定确认 | conversation:approved-review |
| AC-005 | 用户确认：湖内 DAG 驱动 | conversation:approved-review |
| AC-006 | 用户确认：有界外部修复及候选证据 | conversation:approved-review |

## Acceptance Criteria

| AC ID | Criterion |
|---|---|
| AC-001 | scaffold 在只录入 intent 后不能冻结；重复 TC 和无验证意义的成功命令不能满足覆盖；小型无状态需求不靠凑数与伪造 FSM 通过。 surface=cli restates=1 |
| AC-002 | 两文件包由统一契约解析必需成员与矩阵；旧注册路径不改变矩阵权威；布局与准入不索要 evidence.md；SDD 缺省启用证据追溯。 surface=cli restates=2 |
| AC-003 | 冻结、FSM check 和 transition 共用规范化模型；支持 From/To/Trigger 与 State/Event 方言；无模型、非法边、缺转移覆盖与未满足 guard 明确拒绝。 surface=cli restates=3 |
| AC-004 | SDD 确认要求 grounding、独立评审与确切内容指纹；自评、过期评审、未关闭阻断项及确认后改需求不能被当作有效确认；内核只核验外部回执。 surface=cli restates=4 |
| AC-005 | 湖内任务图拒绝重复 ID、缺依赖、环及未知 AC；真实执行经过共享准入，依赖失败不跑后继，记录状态证据且拒绝沿用过期候选。 surface=cli restates=5 |
| AC-006 | 自动任务支持显式配置的外部修复命令、次数和超时；每次捕获失败命令与 stdout/stderr，绑定候选并复验；取消与耗尽保留失败；不执行发布安装。 surface=cli restates=5 |

## 执行边界

按 AC-002 契约归一化、AC-001 防假通过、AC-003 FSM、AC-004 确认链、AC-005 DAG、AC-006 修复的依赖顺序实现。复用既有 gate、事件、artifact 与 subprocess，禁止增加平行真源。测试按消费者可见失败设计；以定向 CLI 成功和拒绝链验收，未执行的正式门禁不标通过。
