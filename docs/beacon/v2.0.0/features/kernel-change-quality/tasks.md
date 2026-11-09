---
slug: kernel-change-quality
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-change-quality

## Task Ledger

- [x] TASK-000 落 kernel-change-quality 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/freeze-log.md
- [x] TASK-001 定义 off / safe_fix / strict_receipt 策略词表与默认 off · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q1-policy-modes.md
- [x] TASK-002 未识别策略取值按 off 处理并报告配置错误，不隐式升级 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q1-policy-modes.md
- [x] TASK-003 strict_receipt 在 premerge 计算当前候选 diff 的精确 SHA 并与回执比对 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q2-diff-sha-receipt.md
- [x] TASK-004 回执缺失、格式非法、SHA 或基线不一致时拒绝合入，拒绝为终态不降级 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q2-diff-sha-receipt.md
- [x] TASK-005 safe_fix 每次调用至多一次修复尝试，第二次请求直接拒绝 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q3-bounded-fix.md
- [x] TASK-006 修复触碰的文件与 hunk 必须落在调用方授权清单内，越界 fail-closed 且不产生部分修改 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q3-bounded-fix.md
- [x] TASK-007 逐条跑 6 条 TC 并落包内证据，跑全量 go test ./... -count=1 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/go-test-all.txt
- [ ] TASK-008 三份独立 QA verdict 按 evidence/qa-contract.md 契约落盘，maker 不自签 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-change-quality/evidence/qa-contract.md

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-008 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。

