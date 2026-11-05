---
slug: truth-version-iteration
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: truth-version-iteration

## Task Ledger

- [x] TASK-001 版本归一化与 patch/minor/major bump helper · ac=AC-003 · evidence=internal/orchestrator/versioning.go
- [x] TASK-002 绿项目首次 truth create 自动注册 v0.0.1 · ac=AC-001 · evidence=internal/orchestrator/truth.go
- [x] TASK-003 truth create 默认 current / auto / latest 解析 · ac=AC-002 · evidence=internal/orchestrator/truth.go
- [x] TASK-004 拒绝 runtime 形态的 docs 版本 · ac=AC-004 · evidence=internal/orchestrator/versioning.go
- [x] TASK-005 projects bump --level 命令 · ac=AC-005 · evidence=internal/cli/projects.go
- [x] TASK-006 truth skill 写清版本流程 · ac=AC-006 · evidence=skills/truth/SKILL.md
- [x] TASK-007 补版本行为测试 · ac=AC-003 · evidence=internal/orchestrator/versioning_test.go
- [x] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/truth-version-iteration/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
