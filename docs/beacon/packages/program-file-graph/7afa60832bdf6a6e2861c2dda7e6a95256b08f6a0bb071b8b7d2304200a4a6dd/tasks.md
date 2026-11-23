---
slug: program-file-graph
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: program-file-graph

## Task Ledger

- [ ] TASK-001 加载 feature-graph.json 并校验 v1 必填键 · ac=AC-001 · evidence=internal/orchestrator/program_file.go
- [ ] TASK-002 复用 ValidateProgramGraph 覆盖依赖/环问题 · ac=AC-002 · evidence=internal/orchestrator/program.go
- [ ] TASK-003 文件不存在报 file_missing 不 panic · ac=AC-003 · evidence=internal/orchestrator/program_file.go
- [ ] TASK-004 ProgramLake 扩字段并让 import 保留 parity_subset/status/scope_mode/goal · ac=AC-004 · evidence=internal/orchestrator/meta_p2d.go
- [ ] TASK-005 import 先校验，失败不写 store · ac=AC-005 · evidence=internal/orchestrator/program_file.go
- [ ] TASK-006 validate 失败非零退出 · ac=AC-006 · evidence=internal/cli/goal_program.go
- [ ] TASK-007 既有五个子命令语义与 flag 不变 · ac=AC-007 · evidence=internal/cli/goal_program.go
- [ ] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/program-file-graph/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
