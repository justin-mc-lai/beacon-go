---
slug: sea-to-lake-proposal
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
  truth: docs/beacon/v2.0.0/features/sea-to-lake-proposal/truth.md
  tests: docs/beacon/v2.0.0/features/sea-to-lake-proposal/tests.md
  tasks: docs/beacon/v2.0.0/features/sea-to-lake-proposal/tasks.md
  evidence: docs/beacon/v2.0.0/features/sea-to-lake-proposal/evidence.md
---

# Requirement Truth: sea-to-lake-proposal

## 人话

给 Beacon 内核的使用者：一句话要做一个项目时，AI 先给 3–7 个“可选的湖”，每个湖说清用户能得到什么、商业价值、怎么衡量、怎么运营、依赖谁、多大、验收起点；人看着表格多选确认后，内核才建草稿包并冻结湖图。未选中的湖不建、不跑；拒绝过的湖留痕，防止 AI 反复瞎发挥。技能层写清流程之外，内核也会拒绝“新海直接排期”；只有已经有 plan 的重排才放行。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "我想要一句话实现一个项目，AI 自动拆湖但具体有哪些湖要跟我互动确定，从完整商用、有价值、可运营、实际用户使用角度给简洁选择，让我多选，确保高质量、意图清晰、不漂移"
goal: "AI 提案有界拆湖 + 人多选确认 + 内核只冻结并执行选中的湖"
non_goals: "不自动替人选湖；不做通用项目管理器；不做并行跑湖；不做自动发布"
constraints: "3–7 个候选湖；每湖有商用五项和验收起点；未知变问题；拒绝留痕；人确认才冻结"
scope_mode: focused
```

## Open Questions

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 最多几个候选湖？ | 3–7；超过先分版本 | answered | 采用 3–7，超限拒绝并提示分版本 |
| Q2 | 多选后要不要自动建草稿包？ | 要，只建骨架，不冻结 | answered | 采用；未选中的湖不建 |
| Q3 | 选中的湖依赖了没选的湖怎么办？ | 拒绝并说明，让人补选 | answered | 采用拒绝，避免偷偷补湖 |

## Intent Coverage

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 目标「有界提案」 | internal/orchestrator/lake_proposal.go |
| AC-002 | Q1 的答案 | internal/orchestrator/lake_proposal.go |
| AC-003 | 目标「商用五项」 | internal/orchestrator/lake_proposal.go |
| AC-004 | Q2 的答案 | internal/orchestrator/lake_proposal.go |
| AC-005 | Q3 的答案 | internal/orchestrator/program.go |
| AC-006 | 目标「只跑选中的湖」 | internal/orchestrator/program_route.go |
| AC-007 | 目标「留痕可追溯」 | internal/orchestrator/meta.go |
| AC-008 | 目标「skill 可执行」 | skills/plan/SKILL.md |
| AC-009 | 目标「内核层也强制」 | internal/cli/goal_program.go |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `goal program propose --file` 接受 `lake-proposal.v1`，校验 schema、slug 唯一、DAG 合法 |
| AC-002 | 候选湖数量必须在 3–7；越界拒绝并给稳定 reason code |
| AC-003 | 每个候选湖必须有 outcome/value/metric/operable/size/acceptance_seed；`not_proposed` 必须非空且带原因 |
| AC-004 | `goal program select` 记录 selected/rejected/by/at；未知 slug 拒绝；选中数必须大于 0；不自动 freeze |
| AC-005 | 选中的湖自动建草稿包（绿项目 v0.0.1）；未选中的湖不建、不跑 |
| AC-006 | `goal program plan --from-proposal --freeze` 只记录选中的湖；依赖未选中的湖时拒绝 |
| AC-007 | 事件可追溯：`lake_proposal_created`、`lake_proposal_selected`；route 显示 proposal 与 selected/rejected 计数 |
| AC-008 | `beacon-go-plan` 的 pln-program 写清 propose → 表格 → 多选 → confirm → freeze；全仓 test/vet/e2e 通过 |
| AC-009 | 首次为 run 建立 program 时，若该 run 尚无 plan 且没有已选提案，`goal program plan` 必须拒绝并返回 `lake_proposal_required`；已有 plan 的重排/change 不受影响；`plan --from-proposal` 仍可用 |

## Boundary

- 内核不生成提案内容；AI/宿主负责内容，内核负责校验、记录、冻结、执行。
- 不自动 release、不自动写代码、不做并行跑湖。
