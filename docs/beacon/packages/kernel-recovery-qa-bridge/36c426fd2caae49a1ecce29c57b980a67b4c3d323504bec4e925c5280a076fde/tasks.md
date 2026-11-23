---
slug: kernel-recovery-qa-bridge
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-recovery-qa-bridge

## Task Ledger

- [ ] TASK-001 `goal recover` 在同一事务内把 `feature_lifecycle` 回写为当前 stage 的 active 快照 · ac=AC-001 · evidence=internal/orchestrator/complete.go
- [ ] TASK-002 补 recover 快照行为测试，seed1 由红转绿 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh
- [ ] TASK-003 `qa autotest run` 聚合成功后补写真实 qa `stage_executed`（placeholder=false, recording=false, outcome=passed） · ac=AC-002 · evidence=internal/orchestrator/autotest.go
- [ ] TASK-004 补 autotest 车道 gate 测试，seed2 由红转绿；失败/空集仍拒绝 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh
- [ ] TASK-005 `featureGateFallback` 排除被 `gate_evaluated` 绑定到其他 run 的 gate 行，qa 出口只采信本 run 事实 · ac=AC-003 · evidence=internal/orchestrator/exitgates.go
- [ ] TASK-006 补跨 run 污染回归，seed3 由红转绿（B 无自有 QA 证据时停在 qa/blocked） · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh
- [ ] TASK-007 负向回归：human gate 仍拒 recover；placeholder/recording/marker 缺失仍不计 qa stage_executed · ac=AC-004 · evidence=internal/orchestrator/complete.go
- [ ] TASK-008 核对 reason code 词表未变、run 内 FORMAL-FIRST 未降级 · ac=AC-005 · evidence=internal/orchestrator/exitgates.go
- [ ] TASK-009 Validate package gates (truth review + freeze) · ac=GATES · evidence=.beacon/evidence/implement/kernel-recovery-qa-bridge/GATES.json

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). 本包停在 plan，全部任务保持未勾选；实现阶段不得改动 seed 的期望失败语义，只允许把红转绿。
