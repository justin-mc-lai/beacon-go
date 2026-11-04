---
slug: gate-strict-integrity
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: gate-strict-integrity

## Task Ledger

- [x] TASK-001 给 `gate integrity` 增加 `--strict`，强制 block 模式 · ac=AC-001 · evidence=internal/cli/gate.go
- [x] TASK-002 版本 release 检查对候选包使用 block 模式 · ac=AC-002 · evidence=internal/orchestrator/release_check.go
- [x] TASK-003 保持冻结包默认 audit_only（AC-101 不回归） · ac=AC-003 · evidence=internal/cli/gate.go
- [x] TASK-004 复用既有 reason code 与 repair route · ac=AC-004 · evidence=internal/gate/integrity.go
- [x] TASK-005 不改 `EvaluateRelease` 四门前置 · ac=AC-005 · evidence=internal/gate/evaluate.go
- [x] TASK-006 补 CLI 与 orchestrator 行为测试 · ac=AC-001 · evidence=internal/cli/gate_strict_test.go
- [x] TASK-007 补历史包与候选包的分叉测试 · ac=AC-004 · evidence=internal/orchestrator/release_check_test.go
- [x] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/gate-strict-integrity/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
