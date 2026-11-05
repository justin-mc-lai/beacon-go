---
slug: lifecycle-monotonic
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: lifecycle-monotonic

## Task Ledger

- [x] TASK-001 新增 updateLifecycleMonotonic 守卫 · ac=AC-005 · evidence=internal/orchestrator/lifecycle.go
- [x] TASK-002 CreateRun 复用守卫，不降级已完成 feature · ac=AC-001 · evidence=internal/orchestrator/engine.go
- [x] TASK-003 freeze/refreeze 复用守卫 · ac=AC-002 · evidence=internal/orchestrator/truth.go
- [x] TASK-004 tick/block 复用守卫 · ac=AC-003 · evidence=internal/orchestrator/tick.go
- [x] TASK-005 未完成 feature 行为不变 · ac=AC-004 · evidence=internal/orchestrator/lifecycle_test.go
- [x] TASK-006 补生命周期单调性测试 · ac=AC-005 · evidence=internal/orchestrator/lifecycle_test.go
- [x] TASK-008 全仓 build/test/vet/e2e 通过 · ac=AC-006 · evidence=internal/orchestrator/lifecycle_test.go
- [x] TASK-007 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/lifecycle-monotonic/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
