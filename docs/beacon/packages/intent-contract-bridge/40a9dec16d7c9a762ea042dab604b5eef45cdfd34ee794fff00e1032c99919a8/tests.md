---
slug: intent-contract-bridge
version: v2.0.0
materials_status: current
test_source: acceptance_criteria
seed_policy: assert-only-red-then-green
---

# Tests: intent-contract-bridge

## 基线

- 基线提交：main @ df0a8a5（v2.0.0），2026-11-11 实测。
- 三条 seed 在基线上全部 exit 1，失败原因均为断言失败，不是编译错误。
- seed 只走已经存在的公开入口（CLI、Engine.CreateRun、Engine.Resume、Engine.Trace），不引用尚未存在的生产符号，因此可以先红后绿。
- 红证据原文见 evidence/ 目录，命令与退出码一并保存。

| Seed 文件 | sha256 |
|-----------|--------|
| internal/cli/intent_contract_seed_test.go | c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0 |
| internal/orchestrator/intent_contract_seed_test.go | b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6 |

上表是 chg_2735c16724e3 首轮补强、chg_ebbeadee16f3 返工 2 补齐 AC-001 后经 refreeze 的 seed 指纹。实现轮必须保持这两个 sha256 不变；任何进一步的断言变更都必须再走一次 change/refreeze 并同步本表。

## AC ↔ TC ↔ Command

| AC ID | TC ID | Layer | Command | 基线上必须出现的断言失败 |
|-------|-------|-------|---------|--------------------------|
| AC-001 | TC-001 | behavior / CLI 端到端 | `go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1` | `truth intent record: unknown flag: --briefing`，说明结构化简报还没有落盘通道 |
| AC-002 | TC-002 | behavior / 运行时门禁 | `go test ./internal/orchestrator -run TestKernelDrivenRunWithoutIntentIsRefused -count=1` | `kernel-driven run created without a confirmed intent; want refusal goal_intent_missing before any run row exists` |
| AC-003 | TC-003 | behavior / resume 持久化 | `go test ./internal/orchestrator -run TestResumeKeepsUtteranceInRunEvent -count=1` | `no goal_resumed event on run ... after Resume(...); the utterance was dropped` |

## 红记录

| AC | 证据文件 | 退出码 | 失败摘要 |
|----|----------|--------|----------|
| AC-001 | evidence/red-ac001.txt | 1 | truth intent record: unknown flag: --briefing (stdout "")；want the aligned briefing landed in the package and the run |
| AC-002 | evidence/red-ac002.txt | 1 | kernel-driven run created without a confirmed intent; want refusal goal_intent_missing before any run row exists |
| AC-003 | evidence/red-ac003.txt | 1 | no goal_resumed event on run run_20b93447c7794ff48b028590 after Resume(...); the utterance was dropped |
| AC-001（补强后） | evidence/red-ac001-seed-widen.txt | 1 | truth intent record: unknown flag: --briefing |
| AC-002,AC-003（补强后） | evidence/red-ac002-ac003-seed-widen.txt | 1 | 5 类自动执行器均未被 goal_intent_missing 拒绝；空/未确认 intent 未被拒绝；Resume 后 0 条 goal_resumed |

## 每条 TC 断言的合同点

| TC ID | 具体断言 |
|-------|----------|
| TC-001 | 落盘位置与 truth.md 同目录；JSON 顶层 schema=beacon-intent-v1、feature_slug、version、truth_path、recorded_at（RFC3339 UTC）、recorded_by、confirmed（镜像 briefing.confirmed，false 也保留）；briefing 保留 original_prompt/restated_intent/identified_risks/boundaries/proposed_slugs/confirmed/degraded；有多个 run 时 --run-id 优先、省略时取该 feature 最近 run，事件只落在选中的 run；intent_recorded 事件的 intent_sha256 与文件字节 sha256 一致；trace 能读到事件；输出侧只要求 run_id（无 run 时为空），不要求 intent_path |
| TC-002 | recording/subprocess/subprocess:<profile>/ralph 或带 Exec 配置的 run 在没有已确认 intent 时必须返回 goal_intent_missing，且 Runs().ListByFeature 不新增 run；manual/external 不被挡；补上 confirmed=true 且 restated_intent 非空的 intent.json 后同一参数可以创建并停在 plan |
| TC-003 | 每次 resume 都追加 goal_resumed，payload 含 resume_action、utterance（逐字一致）、utterance_recorded=true、stage、loop_stage；空 utterance 时 utterance_recorded=false 且无 utterance 键；goal_run_created.utterance 不被改写 |

## 断言补强（chg_2735c16724e3）

冻结后的只读预检指出「合同宽、seed 窄」：旧 seed 只覆盖 executioner 枚举里的 manual 放行与 ralph+Exec，TC-003 只跑非空 utterance，TC-001 只看字段长度。按旧 seed 实现可以合法转绿却漏做合同点。本次 change 只新增断言，没有删除或放宽任何既有断言。

| 合同点 | 新增断言（调用点见 seed 文件） |
|--------|-------------------------------|
| TC-001 briefing 保留 original_prompt | 逐字比对 fixture 原值 |
| TC-001 briefing 保留 proposed_slugs | 逐值比对 ["intent-contract-bridge"]，不再只看 fixture |
| TC-001 briefing 保留 degraded | 字段必须存在且等于 fixture 的 true |
| TC-001 briefing_sha256 | 事件 payload.briefing_sha256 等于 briefing 文件字节的 sha256 |
| TC-001 recorded_at | 必须是 RFC3339 且带 Z（time.Parse 后 Location()==time.UTC），+08:00 不接受 |
| TC-001 无 run 分支 | 同一命令退出码 0、run_id 为空、intent.json 落盘、Runs().ListByFeature 不新增 run |
| TC-002 recording | 无确认 intent 时 goal_intent_missing 且不新增 run；补 intent 后放行并停在 plan |
| TC-002 subprocess | 同上 |
| TC-002 subprocess:<profile> | 同上 |
| TC-002 ralph | 同上（不再依赖 Exec 才触发门禁） |
| TC-002 携带 Exec 配置 | 同上（空 executor + Exec 配置也必须被判为自动执行） |
| TC-002 manual、external 放行 | 无 intent 也能创建并停在 plan |
| TC-002 intent 判定负例 | restated_intent 为空、confirmed=false 都必须拒绝且不新增 run |
| TC-003 每次 resume 追加 | 连续两次 Resume 后 goal_resumed 恰好 2 条 |
| TC-003 空 utterance | 事件仍追加、utterance_recorded=false、payload 无 utterance 键 |
| TC-003 stage、loop_stage | 两个字段都必须非空且等于 ResumeView 的 CurrentStage/LoopStage |
| TC-003 原 utterance 不被改写 | 两次 resume 后 goal_run_created.utterance 仍是创建时的文本 |

命令与失败输出见 Red 记录里的“补强后”两行；命令本身没有改名。

## 返工 2（chg_ebbeadee16f3）

二轮独立复检指出 AC-001 仍有「run 选择分支未覆盖」「confirmed 只测 true」两处缺口，并指出两段输出侧 `intent_path` 断言属于合同外要求。本次 change 补合同、删过头断言，不碰生产代码。

| 合同点 | 动作 |
|--------|------|
| TC-001 `--run-id` 优先、否则最近 run | 本行补入 TC-001 合同；seed 新建 run A、B，指定 A 时事件只落 A（B 为 0），省略 `--run-id` 时事件落 B（A 仍为 1） |
| TC-001 `confirmed` 镜像 `briefing.confirmed` | 新增 `confirmed=false` 的 briefing 分支：命令退出 0，intent.json 顶层与 `briefing.confirmed` 都必须 false，事件 payload 的 confirmed 也 false |
| TC-001 输出侧只要求 run_id | 删除两段把 `intent_path` 当输出字段的断言；落盘位置继续由直接读 `<pkg>/intent.json` 验证 |
| AC-002 intent 必须可解析 | 新增写坏 JSON 的 recording 负例：必须 `goal_intent_missing` 且不新增 run 行 |
| AC-003 即使不在 replay_substep 也追加事件 | 首个 Resume 增加 `rv.ResumeAction != replay_substep` 断言 |

首轮补强的全部断言保持不变，没有任何一条被删除或放宽（仅删除上述合同外的输出侧 `intent_path` 两段）。红输出见 evidence/red-ac001-seed-r2.txt、evidence/red-ac002-ac003-seed-r2.txt。

## 转绿规则

- 三条命令必须用同一条命令行原样跑通，退出码 0；不许换测试名、不许缩小断言。
- seed 文件的 sha256 必须保持不变；实现轮只改生产代码与既有 fixture 种子。
- 旧 fixture 可以新增 intent.json 种子，或显式改用 manual 执行器；不得为此放宽任何 seed 断言。
- 转绿后的输出也要落 evidence/，与红记录一一对应。

## 边界

本包不验证 AlignIntentStream 的提示词质量与模型选择，也不验证 release 人审流程；这两项分别由原有测试与人审 gate 覆盖。
