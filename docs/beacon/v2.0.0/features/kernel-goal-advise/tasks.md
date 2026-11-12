---
slug: kernel-goal-advise
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks

- [ ] TASK-01 冻结本包：确认排序规则与输出 schema · ac=AC-001
- [ ] TASK-02 实现只读排序器（硬阻塞 100 / 进行中 40 / 阶段 20 / 暂停 10） · ac=AC-001
- [ ] TASK-03 输出不含时钟与随机量，同分按 feature 升序 · ac=AC-002
- [ ] TASK-04 输出字段全部来自 spine 快照，schema 固定 · ac=AC-003
- [ ] TASK-05 三条 seed 转绿，go test ./... 全绿 · ac=AC-001
