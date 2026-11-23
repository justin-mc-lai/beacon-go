---
slug: kernel-refreeze-accept-current
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks

实现阶段只在 truth 冻结后开始；勾选一项必须附可复跑证据。

- [ ] TASK-01 冻结本包：确认三处 fail-closed 出口范围 · ac=AC-001
- [ ] TASK-02 `truth refreeze --accept-current` 在 change base、事务守卫、AC diff 基线缺失三处放行 · ac=AC-001
- [ ] TASK-03 收口走 applyFreeze，重建包指纹与 truth_refrozen 事件 · ac=AC-001
- [ ] TASK-04 无挂起变更时拒绝并保持事件不变 · ac=AC-002
- [ ] TASK-05 严格路径不回退的无回归测试 · ac=AC-003
- [ ] TASK-06 三条 seed 转绿（TC-001..003） · ac=AC-001
- [ ] TASK-07 `go test ./...` 全绿，既有 refreeze 语义不变 · ac=AC-003
