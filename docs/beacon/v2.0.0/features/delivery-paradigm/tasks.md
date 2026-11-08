---
slug: delivery-paradigm
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: delivery-paradigm

## Task Ledger

- [x] TASK-000 落 delivery-paradigm 四件套并冻结 truth（TASK-1） · ac=GATES · evidence=docs/beacon/v2.0.0/features/delivery-paradigm/evidence/freeze-log
- [x] TASK-001 goal run 接受 manual 与 external 执行器，创建输出写明需 goal tick --by 记录完成 · ac=AC-001 · evidence=internal/cli/goal.go
- [x] TASK-002 拒绝未知执行器并返回 stage_executor_unsupported · ac=AC-002 · evidence=internal/orchestrator/tick.go
- [x] TASK-003 goal tick 离开阶段时持久化 evaluated=true 的出口判定，未评估则 park · ac=AC-003 · evidence=internal/orchestrator/tick.go
- [x] TASK-004 goal receive-result 增加 --external，免除 delegation-id 并写回 artifact 指针 · ac=AC-004 · evidence=internal/cli/goal_p2c.go
- [x] TASK-005 program route 输出 next actions 拓扑顺序与 truth freeze 建议 · ac=AC-005 · evidence=internal/orchestrator/program_route.go
- [x] TASK-006 gate integrity 检查 ux_required 与正文 UI 章节的一致性 · ac=AC-006 · evidence=internal/gate/integrity.go
- [x] TASK-007 新增 goal conformance 自检并给可判定退出码 · ac=AC-007 · evidence=internal/cli/goal.go
- [x] TASK-008 新增六阶段端到端测试，五门加工 plan 出口全部 evaluated=true · ac=AC-008 · evidence=docs/beacon/v2.0.0/features/delivery-paradigm/evidence/task2-e2e
- [x] TASK-009 AC-009 提案门回归测试原样通过 · ac=AC-009 · evidence=docs/beacon/v2.0.0/features/delivery-paradigm/evidence/task2-ac009
- [x] TASK-010 全量 go test ./... 通过并落 evidence · ac=AC-008 · evidence=docs/beacon/v2.0.0/features/delivery-paradigm/evidence/task2-go-test

## Boundary

Checkboxes are AC-bound ledgers. `[x]» is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-010 belong to the implementation round on the same branch.

