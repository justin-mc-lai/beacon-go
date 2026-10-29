---
slug: p1-skill-router-acp
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: p1-skill-router-acp

## Task Ledger

- [x] TASK-001 落地路由表文件与 schema（beacon-skill-router.v1）· ac=AC-001 · evidence=TestRouterTableSchema
- [x] TASK-002 实现路由匹配，例句唯一命中 · ac=AC-002 · evidence=TestRouterMatchesEverySample
- [x] TASK-003 未命中与多义命中返回候选，不猜测 · ac=AC-003 · evidence=TestRouterRefusesToGuess
- [x] TASK-004 渐进式参考加载顺序声明 · ac=AC-004 · evidence=TestProgressiveMapOrder
- [x] TASK-005 必读参考路径存在性校验 · ac=AC-005 · evidence=TestProgressiveMapPathsExist
- [x] TASK-006 实现 ACP session/new 与 session/prompt 会话迭代 · ac=AC-006 · evidence=TestAcpSessionDrivesTheProtocolCorrectly
- [x] TASK-007 无真实 ACP 目标时带 reason code 拒绝，不伪造 · ac=AC-007 · evidence=TestAcpRefusesWithoutTarget
- [x] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=truth_refreeze

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
