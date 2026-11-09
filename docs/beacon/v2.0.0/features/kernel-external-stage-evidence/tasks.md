---
slug: kernel-external-stage-evidence
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-external-stage-evidence

## Task Ledger

- [x] TASK-000 落 kernel-external-stage-evidence 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/freeze-log.md
- [x] TASK-001 external 与 manual run 记录阶段完成时，内核写入真实 stage_executed，payload 含 source=external、executor=external、by 身份与录制标记字段 false · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e1-external-stage-event.md
- [x] TASK-002 外部调用者无法伪造真实事件字段，命令层只传 by 身份 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e1-external-stage-event.md
- [x] TASK-003 qa 门读取 payload 录制标记字段，值为 true 时拒收，字段缺失按录制态处理 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e2-recording-rejected.md
- [x] TASK-004 qa 门接受真实 external stage_executed，既有虚假通过路径 fail-closed · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e2-recording-rejected.md
- [x] TASK-005 receive-result 外部完成事实加已注册且 hash 校验通过的 QA artifacts 作为 qa_stage_executed 等价证据，payload 标 source=external · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e3-external-completion.md
- [x] TASK-006 unresolved 阻断不得仅靠 attach-verifier 清除，streak 达到 3 后 continue 拒绝，原因消除后显式恢复清零 streak · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/ac-e4-streak-verifier.md
- [x] TASK-007 逐条跑 8 条 TC 并落包内证据，跑全量 go test ./... -count=1 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/go-test-all.txt
- [ ] TASK-008 三份独立 QA verdict 按 evidence/qa-contract.md 契约落盘，maker 不自签 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-external-stage-evidence/evidence/qa-contract.md

## Boundary

Checkboxes are AC-bound ledgers. [x] is valid only when the bound evidence file exists (completion is not self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-008 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。
