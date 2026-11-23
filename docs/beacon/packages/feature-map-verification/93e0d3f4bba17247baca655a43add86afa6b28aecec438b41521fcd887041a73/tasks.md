---
slug: feature-map-verification
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: feature-map-verification

## Task Ledger

- [x] TASK-001 实现 BuildFeatureMap 生成 feature-map.md · ac=AC-001 · evidence=internal/orchestrator/feature_map.go
- [x] TASK-002 生成器只读四件套，不改 truth · ac=AC-002 · evidence=internal/orchestrator/feature_map.go
- [x] TASK-003 验证命令只取 tests.md Command 列 · ac=AC-003 · evidence=internal/gate/integrity.go
- [x] TASK-004 truth map verify 增加 fresh/stale/missing 判定 · ac=AC-004 · evidence=internal/orchestrator/truthmap.go
- [x] TASK-005 入口点只取 tasks.md 的 evidence= 文件路径 · ac=AC-005 · evidence=internal/orchestrator/feature_map.go
- [x] TASK-006 新增 `truth map build` CLI · ac=AC-001 · evidence=internal/cli/truth_map.go
- [x] TASK-007 补生成/过期/单向投影测试 · ac=AC-004 · evidence=internal/orchestrator/feature_map_test.go
- [x] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/feature-map-verification/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
