---
slug: package-task-binding
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: package-task-binding

## Task Ledger

- [ ] TASK-001 在 pkgvalidate 解析 checkbox 行的 ac=/evidence= 绑定并产出 task_binding_missing · ac=AC-001 · evidence=internal/gate/pkgvalidate.go
- [ ] TASK-002 已勾选行缺 evidence= 同样报 task_binding_missing · ac=AC-002 · evidence=internal/gate/pkgvalidate.go
- [ ] TASK-003 路径形态 evidence 查存在性且 size>=2，产出 checkbox_checked_without_evidence_file · ac=AC-003 · evidence=internal/gate/pkgvalidate.go
- [ ] TASK-004 未勾选行只要求 ac=，不要求 evidence= · ac=AC-004 · evidence=internal/gate/pkgvalidate.go
- [ ] TASK-005 保持既有 AC/TC/evidence 判定不变（只追加 findings） · ac=AC-005 · evidence=internal/gate/pkgvalidate_test.go
- [ ] TASK-006 迁移本仓既有冻结包的 ac=/evidence= 行，规则启用后无新增 finding · ac=AC-006 · evidence=docs/beacon/v2.0.0/features/package-task-binding/evidence.md
- [ ] TASK-007 不新增 truth validate-package 子命令 · ac=AC-007 · evidence=internal/cli/truth.go
- [ ] TASK-008 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/package-task-binding/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick).
