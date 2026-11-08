---
slug: kernel-eval-gate
version: v2.0.0
materials_status: current
task_source: acceptance_criteria
---

# Tasks: kernel-eval-gate

## Task Ledger

- [ ] TASK-000 落 kernel-eval-gate 四件套并冻结 truth · ac=GATES · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/freeze-log.md
- [ ] TASK-001 新建 internal/evalgate，定义 case 结构（ID、class、fixture 构造、期望退出码、期望 reason code、必须点名的标识） · ac=AC-001
- [ ] TASK-002 实现数据驱动注册表与校验：四类各含具名用例，重复 ID、未知 class、缺失期望 fail-closed · ac=AC-001 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-registry.json
- [ ] TASK-003 实现隔离 fixture 构造：t.TempDir 项目根 + store.EnvStorePath 独立 DB，用例之间不共享状态 · ac=AC-001
- [ ] TASK-004 实现 conformance 正向用例 all_covered，断言 ac_tc_evidence_coverage pass 与退出 0 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-positive.json
- [ ] TASK-005 实现 conformance 缺 TC 用例，断言非 0、traceability_missing_tc 并点名 AC · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-missing-tc.json
- [ ] TASK-006 实现三种证据失败用例（文件不存在、../ 越界、Evidence 列缺失），断言 traceability_missing_evidence 并点名 AC/TC · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-evidence.json
- [ ] TASK-007 实现三条 fail-closed 用例（tests.md 缺失、矩阵不可解析、truth 无 AC），文本与 --json 都校验 reason code · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-failclosed.json
- [ ] TASK-008 实现 not_required 历史包兼容用例，断言四项判定名称与退出码不变 · ac=AC-002 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-not-required.json
- [ ] TASK-009 实现 stage gate 用例：未冻结 truth 阻断并带 stage_exit_truth_not_frozen，事实齐备放行 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-stage-gate.json
- [ ] TASK-010 实现拒绝路径用例：未知 gate type、缺 evaluator、未知 feature、未知 run，断言非 0 且不写 pass 判定 · ac=AC-003 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-rejection-paths.json
- [ ] TASK-011 实现不降级判定：期望不符时评测集非 0、打印 case=<ID> 与 expected/actual，禁止吞错与 skip · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-mismatch-fails.json
- [ ] TASK-012 实现确定性离线校验：归一化随机 ID 与时间戳，两次报告逐字节一致，出网代理指向 127.0.0.1:1 仍通过 · ac=AC-004 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-deterministic-offline.txt
- [ ] TASK-013 逐条跑 13 条 TC 并落包内 evidence，跑 go test ./... 与 AC-009 提案门、delivery-paradigm 端到端 · ac=AC-005 · evidence=docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/go-test-all.txt
- [ ] TASK-014 三份独立 QA verdict 落到 evidence/qa，并更新 evidence 索引 · ac=GATES

## Boundary

Checkboxes are AC-bound ledgers. `[x]` is valid only when the bound evidence file exists (completion ≠ self-tick). TASK-000 covers this truth package only; TASK-001 至 TASK-014 belong to the implementation round on its own branch. Maker 与 checker 必须分离，三份 verdict 由三个独立验收方各自给出，本包不自签。
