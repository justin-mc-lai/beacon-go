---
slug: kernel-truth-traceability
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-truth-traceability

## Task Ledger

- [x] TASK-000 落 kernel-truth-traceability 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/freeze-log.md
- [x] TASK-001 conformance 新增 ac_tc_evidence_coverage：AC 集合、非软 TC、Evidence 路径三层判定 · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-covered.json
- [x] TASK-002 缺 TC 返回 traceability_missing_tc，缺证据返回 traceability_missing_evidence，detail 点名 AC/TC（缺证据一侧见 traceability-missing-evidence.json） · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-missing-tc.json
- [x] TASK-003 tests.md 缺失、矩阵不可解析、truth 无 AC 时 fail-closed，不降级为 audit · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-failclosed.json
- [x] TASK-004 goal conformance 文本输出、JSON items 与 failed 透出 reason code 与缺失 ID · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-cli-exit.txt
- [x] TASK-005 未声明 traceability_required 的历史包输出 not_required，四项判定与退出码不变 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/traceability-not-required.json
- [x] TASK-006 truth change 文档记录 base requirement_fingerprints · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/change-base-fingerprints.txt
- [x] TASK-007 truth refreeze 计算 AC diff，写 ## AC Diff 与 change_ac_diff 事件，失败保持 change_pending · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/change-ac-diff.md
- [x] TASK-008 truth show --json 返回最近 change_ac_diff · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/truth-show-change-ac-diff.json
- [x] TASK-009 AC-009 三个提案门测试与 delivery-paradigm 端到端测试原样通过 · ac=AC-005 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/ac009-regression.txt
- [x] TASK-010 全量 go test ./... 通过并落 evidence · ac=AC-005 · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/go-test-all.txt
- [x] TASK-011 三份独立 QA verdict 落到 evidence/qa（spec-ac.md 指定，runtime.md / regression.md 同目录），并更新 evidence 索引 · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/qa/spec-ac.md

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-011 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出。
