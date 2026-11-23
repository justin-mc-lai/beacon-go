---
slug: intent-contract-bridge
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: intent-contract-bridge

plan 阶段只产出需求真值、验收 seed 与红证据；实现轮才开始改生产代码。

## Task Ledger

- [ ] TASK-001 AC-001：实现 `truth intent record`，把 AlignIntentStream 的结构化 Briefing 落成 <pkg>/intent.json（临时文件 + rename），并在存在 run 时追加 intent_recorded 事件 · ac=AC-001 · tc=TC-001
- [ ] TASK-002 AC-002：在 Engine.CreateRun 写入 run 行之前加意图门禁，内核自动执行器缺已确认 intent 时返回 goal_intent_missing，manual/external 放行 · ac=AC-002 · tc=TC-002
- [ ] TASK-003 AC-003：Engine.Resume 每次追加 goal_resumed 事件，保留 utterance 与 stage/loop_stage，空 utterance 也记录但标记 utterance_recorded=false · ac=AC-003 · tc=TC-003
- [ ] TASK-004 兼容既有 fixture：为 recording 执行器的旧用例补已确认 intent 种子或改 manual，禁止放宽 seed 断言 · ac=AC-001,AC-002
- [ ] TASK-005 回归：原样重跑三条 seed 命令，逐条与 evidence/red-ac001..003.txt 对照转绿，并把新输出落 evidence/ · ac=AC-001,AC-002,AC-003
- [ ] TASK-006 人审 gate：truth 冻结由人确认后才进入 implement；本包到 freeze 前为止 · ac=GATES

## 完成判据

- 三条 seed 命令退出码 0，seed 文件 sha256 不变。
- `go test ./...` 全绿，旧 fixture 无新增失败。
- 每份 AC 都有对应 evidence 文件；验收由独立 QA 给出，不由实现方自证。

## Boundary

Checkbox 是 AC 绑定的账本；只有对应证据文件存在时才允许 `[x]`（完成不等于自勾）。plan 阶段不勾任何实现项，不改生产代码，不 freeze，不 push。

