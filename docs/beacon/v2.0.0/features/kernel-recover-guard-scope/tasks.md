---
slug: kernel-recover-guard-scope
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-recover-guard-scope

## Task Ledger

- [x] TASK-000 落 kernel-recover-guard-scope 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/freeze-log.md
- [x] TASK-001 recover 对五种人类门禁原因返回 goal_recovery_human_gate_forbidden，运行快照零改动 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r1-recover-scope.md
- [x] TASK-002 recover 对非人类门禁原因保留清空 blocked、streak、reason 的现有语义 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r1-recover-scope.md
- [x] TASK-003 complete --human-confirmed --confirmed-by 独占清除人类门禁阻断并写 release_confirmed 与 human_gate_cleared · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md
- [x] TASK-004 recover 不写 release_confirmed、不修改 release_human_confirmed · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r2-human-confirmation.md
- [x] TASK-005 goal_recovered 补齐 before/after 字段与 decision，保留 by、note、stage、prior_reason_code、prior_blocked_streak · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r3-recover-trail.md
- [x] TASK-006 被拒绝的 recover 追加 goal_recover_rejected，含 attempted_reason_code、refusal_reason_code、before/after 与 human_confirmation_required · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r3-recover-trail.md
- [x] TASK-007 status 输出 last_block_reason，并在快照被擦除时从最新人类门禁 stage_exit_blocked 还原 blocked、paused、reason、streak · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/ac-r4-status-truth.md
- [ ] TASK-008 逐条跑 9 条 TC 并落包内证据，跑全量 go test ./... -count=1；三份独立 QA verdict 由三个独立验收方落盘 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-recover-guard-scope/evidence/go-test-all.txt

## Boundary

Checkboxes are AC-bound ledgers. [x] is valid only when the bound evidence file exists (completion is not self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-008 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。
