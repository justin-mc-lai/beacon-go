# Evidence Index: intent-contract-bridge

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| tasks | support_advisory | tasks.md | present | truth |
| red AC-001 | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac001.txt | FAIL, exit 1 | qa |
| red AC-002 | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac002.txt | FAIL, exit 1 | qa |
| red AC-003 | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac003.txt | FAIL, exit 1 | qa |
| seed AC-001,AC-002,AC-003 | acceptance_truth | internal/cli/intent_contract_seed_test.go | sha256 c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0 (chg_ebbeadee16f3 返工 2 后) | qa |
| seed AC-002,AC-003 | acceptance_truth | internal/orchestrator/intent_contract_seed_test.go | sha256 b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6 (chg_ebbeadee16f3 返工 2 后) | qa |
| red AC-001 (widened seed) | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac001-seed-widen.txt | FAIL, exit 1 | qa |
| red AC-002,AC-003 (widened seed) | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac002-ac003-seed-widen.txt | FAIL, exit 1 | qa |
| red AC-001 (round 2) | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac001-seed-r2.txt | FAIL, exit 1 | qa |
| red AC-002,AC-003 (round 2) | acceptance_truth | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/red-ac002-ac003-seed-r2.txt | FAIL, exit 1 | qa |
| seed sha before/after | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/seed-sha-before-after.txt | recorded | truth |
| seed sha before/after (round 2) | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/seed-sha-before-after-r2.txt | recorded | truth |
| change chg_2735c16724e3 | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/changes/chg_2735c16724e3.md | refreeze cleared it from change_pending; the change doc keeps its original status field (the DB trail is authoritative) | truth |
| change chg_ebbeadee16f3 | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/changes/chg_ebbeadee16f3.md | registered, change_pending | truth |
| change invocation | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/change.txt | exit 0, status change_pending | truth |
| change invocation (round 2) | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/change-r2.txt | exit 0, status change_pending | truth |
| refreeze invocation | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/refreeze.txt | exit 0, status frozen | truth |
| refreeze invocation (round 2) | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/refreeze-r2.txt | see file | truth |
| bootstrap intent (attempt 1) | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/bootstrap-intent.txt | exit 0, one precheck finding quoted | truth |
| bootstrap intent (after AC-002 reword) | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/bootstrap-intent-after-fix.txt | exit 0, no finding | truth |
| bootstrap confirm | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/bootstrap-confirm.txt | exit 0, confirmed=true | truth |
| hash manifest | support_advisory | docs/beacon/v2.0.0/features/intent-contract-bridge/evidence/sha256.txt | recorded | truth |

## 冻结前的两次 precheck

第一次 truth freeze 被 20 条 finding 挡住，全部来自本索引的格式而非需求内容：authority 列用了 truth/test_truth/task_ledger/test_evidence/test_source 这套旧词表，且 red-ac*.txt 写成了包内相对路径，被解析到项目根。原始输出见同目录 freeze-attempt-1.txt。

第二次 precheck 之前做了两处只影响表达、不影响 AC 语义的修正：

1. AC-002 的“A、B、C、D，或携带 Exec 命令配置”改成“A、B、C、D、携带 Exec 命令配置；这一枚举即内核自动执行的完整判定集合”，消除 ac_vague_disjunction 对分支词“或”的误伤，判定集合本身不变。
2. 本索引的 authority 改为冻结包 schema 的合法词表（requirement_truth/test_truth/acceptance_truth/support_advisory），证据一律写项目相对的完整路径，并逐条注册为该 run 的 artifact。

## Reproduction Log

Every red row below was produced on 2026-11-11 against main @ df0a8a5 with the two seed files present in the working tree (untracked). Full output is in the evidence files named in the index above; the tests were not edited.

| AC | Command | Exit | Result |
|----|---------|------|--------|
| AC-001 | go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 | 1 | assertion failure: truth intent record: unknown flag: --briefing |
| AC-002 | go test ./internal/orchestrator -run TestKernelDrivenRunWithoutIntentIsRefused -count=1 | 1 | assertion failure: kernel-driven run created without a confirmed intent |
| AC-003 | go test ./internal/orchestrator -run TestResumeKeepsUtteranceInRunEvent -count=1 | 1 | assertion failure: no goal_resumed event; the utterance was dropped |

补强后的红记录（2026-11-11，main @ df0a8a5 + 新 seed，未跟踪）：

| AC | Command | Exit | Result |
|----|---------|------|--------|
| AC-001 | go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 | 1 | assertion failure: truth intent record: unknown flag: --briefing |
| AC-002,AC-003 | go test ./internal/orchestrator -run 'TestKernelDrivenRunWithoutIntentIsRefused|TestResumeKeepsUtteranceInRunEvent' -count=1 | 1 | 5 类自动执行器与 2 个 intent 负例均未被拒绝；Resume 后 0 条 goal_resumed |

返工 2 的红记录（2026-11-11，main @ df0a8a5 + 新 seed，未跟踪）：

| AC | Command | Exit | Result |
|----|---------|------|--------|
| AC-001 | go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 | 1 | assertion failure: truth intent record: unknown flag: --briefing |
| AC-002,AC-003 | go test ./internal/orchestrator -run 'TestKernelDrivenRunWithoutIntentIsRefused|TestResumeKeepsUtteranceInRunEvent' -count=1 | 1 | 5 类自动执行器、空/未确认/坏 JSON 三个负例均未被拒绝；Resume 后 0 条 goal_resumed |

## Seed File

| File | sha256 |
|------|--------|
| internal/cli/intent_contract_seed_test.go | c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0 |
| internal/orchestrator/intent_contract_seed_test.go | b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6 |

这两个 sha256 与 tests.md 的返工 2 记录逐位一致；首轮补强的 7c8ef35e… / 3ab6cb35… 与冻结前的 6ca7edfa… / 8555ee1e… 保留在 seed-sha-before-after*.txt 与 sha256.txt 里。红基线仍是 main @ df0a8a5 上的这两份 test 文件。

## 冻结证据

| Artifact | What it shows |
|----------|---------------|
| bootstrap-intent.txt | 第一次 truth intent 快照的完整 JSON，含当时唯一的阻塞项 ac_vague_disjunction |
| bootstrap-intent-after-fix.txt | 改词后的第二次快照，block_reasons 为空 |
| bootstrap-confirm.txt | truth confirm 原文与退出码 0，confirmed=true、ready=true |
| sha256.txt | 四件套与两份 seed 的 sha256 复核 |
| freeze-attempt-1.txt | 第一次 truth freeze 的 20 条 finding 原文（索引格式问题） |
| freeze.txt | 最终 truth freeze 原文与退出码（同目录） |

## 断言补强与 refreeze（chg_2735c16724e3）

只读预检在冻结后指出合同宽、seed 窄。本次 change 只补断言，不删除、不放宽旧断言：TC-002 覆盖 recording / subprocess / subprocess:<profile> / ralph / 携带 Exec 五类自动执行器以及 manual/external 两个放行对照和 2 个 intent 判定负例；TC-003 覆盖每次 resume 追加、空 utterance 的 utterance_recorded=false 与无 utterance 键、stage/loop_stage、原 utterance 不被改写；TC-001 覆盖 original_prompt、proposed_slugs 逐值、degraded、briefing_sha256、UTC 时间与无 run 分支。逐条合同点见 tests.md 的“断言补强”表。

变更前后 seed sha256、change 与 refreeze 的命令原文和退出码分别在 seed-sha-before-after.txt、change.txt、refreeze.txt。

## 返工 2 与 refreeze（chg_ebbeadee16f3）

二轮只读复检在 AC-001 上仍指出两处缺口（run 选择分支、confirmed 只测 true）和一处过头断言（输出侧 intent_path）。本次 change 只改 seed 与 Truth 包：

| 合同点 | 落点 |
|--------|------|
| TC-001 run 选择 | 建 run A、B；`--run-id A` 时 A 有 1 条、B 为 0；省略 `--run-id` 时 B 得到 1 条、A 仍为 1 |
| TC-001 confirmed 镜像 | `confirmed=false` 的 briefing：退出 0，intent.json 顶层与 briefing.confirmed 均为 false，事件 payload 也为 false |
| TC-001 输出侧合同 | 删除两段输出 intent_path 断言，只保留 run_id 与直接读文件验证 |
| AC-002 可解析 | 坏 JSON 的 recording 负例：goal_intent_missing 且不新增 run 行 |
| AC-003 非 replay | 首个 Resume 断言 ResumeAction != replay_substep |

红输出在 red-ac001-seed-r2.txt 与 red-ac002-ac003-seed-r2.txt；sha 对比在 seed-sha-before-after-r2.txt；change 与 refreeze 命令原文在 change-r2.txt、refreeze-r2.txt。

关于 change 文档状态：refreeze 关闭的是 DB/trail 里的未关闭变更（`truth show` 的 `change_pending` 回到 false），changes 目录里的变更文档文件头仍保留登记时的 status 字段，refreeze 不改写它。本索引按实测描述，不把该文件写成 closed。该文件状态与 change_pending 的一致性作为待查项交人类，不在本轮改生产代码。

## Boundary

evidence.md 是索引/读模型，不含终审结论、gate truth 或 release readiness 权威字段。状态列只反映落盘时的事实；红转绿后由实现轮按实测追加，不得预填。
