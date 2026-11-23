---
slug: issue11-closure
surface_strict: true
version: v2.1.5
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
  truth: docs/beacon/v2.1.5/features/issue11-closure/truth.md
  tests: docs/beacon/v2.1.5/features/issue11-closure/tests.md
  tasks: docs/beacon/v2.1.5/features/issue11-closure/tasks.md
  evidence: docs/beacon/v2.1.5/features/issue11-closure/evidence.md
---

# Requirement Truth: issue11-closure

## 人话

让已有两文件 SDD 能安全使用拆解、提案与迁移入口，并用真实 CLI 验证所有入口不把候选建议当作已确认需求。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "先安全收口原工作区，再把缺口逐项绑定到 #11 完成清单，真实入口验收后才关闭 issue"
goal: "补齐拆解和迁移入口与提案内容校验，给出可运行样例并独立验收"
non_goals: "不改已发布记录，不自动删除旧文档，不把本地工作目录冒充安全沙箱"
constraints: "内核不调用 LLM；旧包兼容；用户确认和独立评审不能绕过；owner 选择选项2，安全沙箱由 #1 承接，不作 #11 关闭前置"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|---|---|---|---|---|
| Q1 | 是否继续修复上轮明确列出的入口缺口？ | 按真实入口逐项验收 | answered | 用户要求继续，并要求验收后才关闭 issue |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|---|---|---|
| AC-001 | 用户要求：拆解入口统一 | conversation:issue11-continuation |
| AC-002 | 用户要求：湖提案内容校验 | conversation:issue11-continuation |
| AC-003 | 用户要求：旧包迁移和样例 | conversation:issue11-continuation |
| AC-004 | 用户要求：真实入口验收后关闭 | conversation:issue11-continuation |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | decompose 帮助与 JSON 明示确定性非 LLM 候选；SDD 分支不覆写 truth 为测试文档、不生成 tests.md、不伪造测试断言；保留开放问题与人工来源旅程边界，生成的草稿仍不可冻结。 surface=cli restates=1 |
| AC-002 | 新 SDD 提案合同显式绑定 AC 与状态适用性；校验缺项、未知引用和非法图；选择只生成 SDD 草稿，冻结沿既有单包门禁；旧提案明确标注 legacy 而非新合同合格。 surface=cli restates=2 |
| AC-003 | truth migrate 支持 dry-run，保留原文件与备份，冲突拒绝，冻结包必须先 change；迁移后旧确认失效且需独立评审，命令不得丢失真实测试命令断言与任务依赖。 surface=cli restates=3 |
| AC-004 | stateless、workflow、decompose、legacy migration 样例通过真实 CLI 成功和拒绝链，独立验收绑定源码与命令输出；正式门禁通过后按 owner 选项2关闭 #11，容器安全隔离由 #1 承接且不宣称完成。 surface=cli restates=4 |

## Grounding

基线 main 64a8ecd、beacon-go#11 原文、PR #13；decompose 当前会把 TestsPath 指向的 truth.md 覆写为测试文档，是本轮必须消除的真实路径。

## 边界

有界外部修复已在 go.16 发布。owner 于本轮明确选择选项2：容器安全隔离由 #1 承接，不属于 #11 关闭前置；本轮仍必须完成真实入口验收和正式 QA/release。dry-run 不改人工需求和账本数据；SQLite lock/index sidecar 可能创建，已明确披露。
