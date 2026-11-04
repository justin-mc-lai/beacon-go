---
slug: program-drive
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: program-drive

## Task Ledger

- [x] TASK-001 新增 ProgramDrive：单次最多一个湖到 release gate · ac=AC-001 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-002 记录/复用湖↔run 绑定 · ac=AC-002 · evidence=internal/orchestrator/meta_p2d.go
- [x] TASK-003 到达 gate 记 awaiting_release，不自动 complete · ac=AC-003 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-004 失败记 blocked + reason，依赖湖 待命 · ac=AC-004 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-005 route 增加 run_id/stage/rollup/program_status · ac=AC-005 · evidence=internal/orchestrator/program_route.go
- [x] TASK-006 已完成湖不重跑，drive 可恢复 · ac=AC-006 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-007 全部完成才 delivered · ac=AC-007 · evidence=internal/orchestrator/program_drive.go
- [x] TASK-008 不新增顶层命令，能力落在 goal program · ac=AC-008 · evidence=internal/cli/goal_program.go
- [x] TASK-009 补 drive/route 测试 · ac=AC-004 · evidence=internal/orchestrator/program_drive_test.go
- [x] TASK-010 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/program-drive/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
