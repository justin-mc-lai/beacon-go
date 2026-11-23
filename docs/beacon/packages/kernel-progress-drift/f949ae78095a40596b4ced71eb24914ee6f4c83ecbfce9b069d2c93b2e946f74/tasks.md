---
slug: kernel-progress-drift
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-progress-drift

## Task Ledger

- [x] TASK-000 落 kernel-progress-drift 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/freeze-log.md
- [ ] TASK-001 定义 progress_review_receipt_v0 结构、字段与状态词表 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d1-receipt-schema.md
- [ ] TASK-002 实现 off/shadow/assist 模式，默认 off，shadow 只记录不驱动决策 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d1-receipt-schema.md
- [ ] TASK-003 连续 N 条 completed 且 drift=true 才提升为既有 replan 义务 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d2-drift-promotion.md
- [ ] TASK-004 非 completed 状态与 unknown 永不算已处理，并打断连续计数 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d2-drift-promotion.md
- [ ] TASK-005 回执校验 goal 固定 revision，固定值参与计数 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d3-revision-binding.md
- [ ] TASK-006 未固定或绑定到其它 revision 的回执不计数，分别给 assist_blocked_reason 与 rebind_hint · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/ac-d3-revision-binding.md
- [ ] TASK-007 逐条跑 6 条 TC 并落包内证据，跑全量 go test ./... -count=1 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/go-test-all.txt
- [ ] TASK-008 三份独立 QA verdict 按 evidence/qa-contract.md 契约落盘，maker 不自签 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-progress-drift/evidence/qa-contract.md

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-008 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。
