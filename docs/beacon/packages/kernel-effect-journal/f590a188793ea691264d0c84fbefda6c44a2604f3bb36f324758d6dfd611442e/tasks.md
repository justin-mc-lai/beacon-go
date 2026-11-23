---
slug: kernel-effect-journal
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-effect-journal

## Task Ledger

- [x] TASK-000 落 kernel-effect-journal 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/freeze-log.md
- [x] TASK-001 定义 effect journal 记录结构与 idempotency key 唯一性规则 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j1-idempotency-key.md
- [x] TASK-002 implement 阶段每个副作用写 journal，缺 key 的副作用拒绝执行 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j1-idempotency-key.md
- [x] TASK-003 replay 以 (run_id, idempotency_key) 去重，命中成功记录不再执行底层副作用 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j2-replay-resume.md
- [x] TASK-004 resume 输出已证明阶段清单，仅包含 succeeded 且记录完整的阶段 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j2-replay-resume.md
- [x] TASK-005 resume 有有效 journal 时从证明继续，不重跑已证明阶段 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j3-missing-journal.md
- [x] TASK-006 journal 缺失、不可解析或版本不识别时 resume fail-closed，不静默重跑 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/ac-j3-missing-journal.md
- [x] TASK-007 逐条跑 6 条 TC 并落包内证据，跑全量 go test ./... -count=1 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/go-test-all.txt
- [x] TASK-008 三份独立 QA verdict 按 evidence/qa-contract.md 契约落盘，maker 不自签 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/qa/spec-ac.md · verdicts=docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/qa/spec-ac.md docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/qa/runtime.md docs/beacon/v2.0.0/features/kernel-effect-journal/evidence/qa/regression.md

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-008 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。

