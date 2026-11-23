---
slug: program-sea-complete
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: program-sea-complete

## Task Ledger

- [x] TASK-001 ProgramDrive 增加 UntilDone 循环，跳过已完成/已到 gate 的湖 · ac=AC-001 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-002 到达 gate 记 awaiting_release 后继续下一个湖 · ac=AC-002 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-003 blocked 湖立即停，依赖湖保持待命 · ac=AC-003 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-004 新增 ProgramComplete：一次程序级确认完成所有在 gate 的湖 · ac=AC-004 · evidence=internal/orchestrator/program_complete.go
- [x] TASK-005 写 program_release_confirmed 事件并归因 program:<name> · ac=AC-005 · evidence=internal/orchestrator/program_complete.go
- [x] TASK-006 不在 gate / 无 verifier / 无 qa pass 时拒绝 · ac=AC-006 · evidence=internal/orchestrator/program_complete.go
- [x] TASK-007 route 暴露 awaiting_release / delivered · ac=AC-007 · evidence=internal/orchestrator/program_route.go
- [x] TASK-008 CLI：drive --until-done + program complete · ac=AC-008 · evidence=internal/cli/goal_program.go
- [x] TASK-009 补循环与单次确认测试 · ac=AC-004 · evidence=internal/orchestrator/program_sea_complete_test.go
- [x] TASK-011 依赖湖到 gate 视为 release-ready，下游可启动 · ac=AC-009 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-010 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/program-sea-complete/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
