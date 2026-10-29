---
slug: p0-executor-desktop
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: p0-executor-desktop

## Task Ledger

- [x] TASK-001 修正 codex-style/claude-style 内置模板为真实 CLI 参数（去掉不存在的 `--context-file`，改为单个 prompt 位置参数） · ac=AC-001
- [x] TASK-002 让阶段上下文经 prompt 文本传递到真实执行器（路径写入 argv 的 prompt 元素） · ac=AC-004
- [ ] TASK-003 真跑 codex 完成 1 次成功 iterate 并入库 TC-002 证据（限最小代价 1 次） · ac=AC-002
  - BLOCKED 外部依赖：真跑已执行，argv 被真 codex 接受且 prompt 携带上下文路径，但上游返回 `Model is unavailable`（grok-4.5）。非 Beacon 缺陷，待 provider 恢复后重跑 `-tags=realcli`
- [ ] TASK-004 真跑 claude 完成 1 次成功 iterate 并入库 TC-003 证据（限最小代价 1 次） · ac=AC-003
  - BLOCKED 外部依赖：真跑已执行，argv 被真 claude 接受并启动，但返回 `410 Model deepseek-v4-flash has been deprecated`。非 Beacon 缺陷，待模型可用后重跑 `-tags=realcli`
- [x] TASK-005 失败面机检：命令缺失/非零退出/超时均返回明确 reason_code 且不伪造 success · ac=AC-005
- [x] TASK-006 agentapi 暴露推进（Tick/单步driver）与执行器配置接口 · ac=AC-006
- [x] TASK-007 桌面端前端加执行器选择 + 推进按钮，绑定 TASK-006 接口 · ac=AC-007
- [x] TASK-008 无执行器时桌面端如实 park（blocked + stage_executor_not_registered），不显示完成 · ac=AC-008
- [x] TASK-009 Validate package gates (truth review + freeze) · ac=GATES

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
