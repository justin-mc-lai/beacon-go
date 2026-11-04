---
slug: skills-distribution
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: skills-distribution

## Task Ledger

- [x] TASK-001 新增 .claude-plugin/plugin.json 与 marketplace.json，列出 1+6 · ac=AC-001 · evidence=.claude-plugin/plugin.json
- [x] TASK-002 实现 skills.VerifyHost / VerifyAll 的缺失与 extra 判定 · ac=AC-002 · evidence=skills/verify.go
- [x] TASK-003 新增 `beacon skills verify`，默认报告、`--strict` 非零 · ac=AC-003 · evidence=internal/cli/skills_verify.go
- [x] TASK-004 只校验 beacon-go*，v1 名称不计 extra · ac=AC-004 · evidence=skills/verify.go
- [x] TASK-005 补 skills 与 CLI 测试 · ac=AC-002 · evidence=skills/verify_test.go
- [x] TASK-006 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/skills-distribution/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
