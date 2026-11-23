---
slug: sea-to-lake-proposal
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: sea-to-lake-proposal

## Task Ledger

- [x] TASK-001 定义 lake-proposal.v1 与校验（schema/3–7/slug/DAG） · ac=AC-001 · evidence=internal/orchestrator/lake_proposal.go
- [x] TASK-002 湖数量边界 3–7 与稳定 reason code · ac=AC-002 · evidence=internal/orchestrator/lake_proposal.go
- [x] TASK-003 商用五项 + not_proposed 必填 · ac=AC-003 · evidence=internal/orchestrator/lake_proposal.go
- [x] TASK-004 select 记录 selected/rejected/by/at · ac=AC-004 · evidence=internal/orchestrator/lake_proposal.go
- [x] TASK-005 select 只为选中的湖建草稿包 · ac=AC-005 · evidence=internal/orchestrator/lake_proposal.go
- [x] TASK-006 plan --from-proposal 拒绝未选中依赖 · ac=AC-006 · evidence=internal/orchestrator/program.go
- [x] TASK-007 route 暴露 proposal 与选择计数 · ac=AC-007 · evidence=internal/orchestrator/program_route.go
- [x] TASK-008 CLI propose/select/plan --from-proposal · ac=AC-008 · evidence=internal/cli/goal_program.go
- [x] TASK-009 plan skill 写清提案多选流程 · ac=AC-008 · evidence=skills/plan/SKILL.md
- [x] TASK-010 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/sea-to-lake-proposal/GATES.json
- [x] TASK-011 内核拒绝新 program 直接 plan · ac=AC-009 · evidence=internal/cli/goal_program.go

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
