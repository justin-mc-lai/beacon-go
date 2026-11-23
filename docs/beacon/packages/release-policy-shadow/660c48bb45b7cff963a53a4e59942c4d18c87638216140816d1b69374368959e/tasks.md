---
slug: release-policy-shadow
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: release-policy-shadow

## Task Ledger

- [x] TASK-001 run 级 release_policy 字段与取值校验 · ac=AC-001 · evidence=internal/orchestrator/engine.go
- [x] TASK-002 影子判定复用 release facts 并写事件 · ac=AC-002 · evidence=internal/orchestrator/gates_facts.go
- [x] TASK-003 影子不写 ReleaseHumanConfirmed，complete 仍要人 · ac=AC-003 · evidence=internal/orchestrator/complete.go
- [x] TASK-004 status 暴露 release_policy 与影子结果 · ac=AC-004 · evidence=internal/orchestrator/engine.go
- [x] TASK-005 拒绝 auto 并给出明确文案 · ac=AC-005 · evidence=internal/core/reasons.go
- [x] TASK-006 补 orchestrator 与 CLI 测试 · ac=AC-002 · evidence=internal/cli/goal_release_shadow_test.go
- [x] TASK-007 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/release-policy-shadow/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
