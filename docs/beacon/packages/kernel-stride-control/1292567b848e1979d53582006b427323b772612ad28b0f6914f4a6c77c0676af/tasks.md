---
slug: kernel-stride-control
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-stride-control

## Task Ledger

- [x] TASK-000 落 kernel-stride-control 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/freeze-log.md
- [ ] TASK-001 定义 stride 记录结构与受控词表：turn_id、layer、boundary、stop_reason、closeout、outcome · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-001-stride-record.md
- [ ] TASK-002 在 turn 结算路径写入 stride 记录并提供只读查询 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-001-stride-record.md
- [ ] TASK-003 把数值上限降级为安全上限：触顶记录 numeric_ceiling 且 outcome=incomplete · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-002-semantic-ceiling.md
- [ ] TASK-004 完成判定绑定语义 closeout，缺 closeout 一律未完成 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-002-semantic-ceiling.md
- [ ] TASK-005 内层工具循环在权限与接受边界前停下，不得续借外层授权 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-003-layer-boundaries.md
- [ ] TASK-006 外层连续续跑前强制 settlement，缺 settlement 阻断继续 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-003-layer-boundaries.md
- [ ] TASK-007 调度唤醒只触发状态检查，不增加语义进展计数 · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-004-scheduler-wake.md
- [ ] TASK-008 状态推进加 settlement 才算语义进展，并与调度唤醒可区分 · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/ac-004-scheduler-wake.md
- [ ] TASK-009 逐条跑 8 条 TC 并落包内证据，跑全量 go test ./... -count=1 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/go-test-all.txt
- [ ] TASK-010 三份独立 QA verdict 按 evidence/qa-contract.md 契约落盘，maker 不自签 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-stride-control/evidence/qa-contract.md

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-010 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。
