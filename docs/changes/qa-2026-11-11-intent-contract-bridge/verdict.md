# QA Verdict · intent-contract-bridge · 验收员（独立验收）

- current_stage: `qa`
- run: `run_8d8efdf2c2a00a1f1b8e3778`
- HEAD: `afd5cc7`（16:15:55 CST 实测；af6dcd5 + 1 个纯 docs 提交）
- 评估者: `验收员 · 独立验收`（maker != checker；实现方为 `apple(via opencode)` / `kunqi.lai`）
- 判定时间: 2026-11-11 16:18 CST

## 判定：PASS

三条 AC 全部由我在当前 HEAD 上用真实命令实跑通过；truth 已冻结且 intent.json 与 truth 一致；run 内已注册证据哈希复核无 stale；qa 门已用独立身份重新评估并持久化通过。

边界：未改源码 / truth / tests / seed；未 push、未打 tag、未写发布说明。本目录文件与 store 中的 artifact 注册是仅有的新增。

## 1. truth 与 intent 核验

| 项 | 实测 | 结论 |
|---|---|---|
| truth 状态 | `beacon truth list` → `intent-contract-bridge v2.0.0 frozen/Qa`；frontmatter `status: frozen`、`intent_confirmed: true` | PASS |
| intent.json 位置/形状 | 与 truth.md 同目录；schema/feature_slug/version/truth_path 与包一致；`confirmed=true`；restated_intent 与 truth 的 goal 同义 | PASS |
| intent 与 run 事件 | `intent_recorded` 落在记录时刻的 feature 最新 run `run_c707f2797d34f417dafc210c`；payload `intent_sha256=9405596bb65d2c099aeb8f1a9c6...` 与文件 sha256 `9405596bb65d2c099aeb8f1a9c6a6e8795ca36574c5daea27ba6343cf49094d7` 一致 | PASS |
| Go 版本 | 模块内 `go version` = go1.27.3（go.mod 声明 1.27.3，系统 1.25.6 由 toolchain 自动切换） | PASS |

备注（不构成冲突）：intent.json 的 `original_prompt` 是 truth utterance 的同义缩写，不是逐字副本；AC-001 要求 briefing 原样保留对齐器返回值，语义与 truth 一致，不判失败。

## 2. AC 覆盖与实测

全部命令在 `HEAD=afd5cc7` 上 `-count=1` 重跑（不复用 cached）。原始输出：`ac-seed-rerun.txt`。

| AC | TC | 实测命令 | 结果 | 支撑 |
|---|---|---|---|---|
| AC-001 | TC-001 | `go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 -v` | PASS（3 子例：无 run 只落文件 / `--run-id` 优先且默认最近 run / confirmed=false 镜像） | 测试实跑 + TC-001/result.json |
| AC-002 | TC-002 | `go test ./internal/orchestrator -run TestKernelDrivenRunWithoutIntentIsRefused -count=1 -v` | PASS（10 子例：recording / subprocess / subprocess:<profile> / ralph / Exec 配置均拒；manual、external 放行；空 restated_intent、confirmed=false、坏 JSON 均拒且不新增 run） | 测试实跑 + TC-002/result.json |
| AC-003 | TC-003 | `go test ./internal/orchestrator -run TestResumeKeepsUtteranceInRunEvent -count=1 -v` | PASS（每次 resume 追加 goal_resumed；非 replay_substep 也落；空 utterance 标记 false 且无 utterance 键；原 utterance 不改写） | 测试实跑 + TC-003/result.json |

代码落点复核（与 AC 文本一致）：

- AC-001：`internal/cli/truth_intent.go:204`；落盘 `internal/orchestrator/intent_contract.go:114`；原子写 `intent_contract.go:223`（临时文件 + `os.Rename`，见 :249）。
- AC-002：门在 `CreateRun` 内、`BuildStageOrder` 与 run 行写入之前（`internal/orchestrator/engine.go:479`）；判定集合 `IsKernelDrivenExecutor`（`intent_contract.go:100`）覆盖 recording / subprocess / subprocess:* / ralph / `Exec != nil`；reason code `goal_intent_missing`。
- AC-003：`internal/orchestrator/resume.go:221` 每次 resume 追加 `goal_resumed`；`_ = utterance` 已删除。

seed 指纹未变（与 tests.md 表一致）：

```
c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0  internal/cli/intent_contract_seed_test.go
b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6  internal/orchestrator/intent_contract_seed_test.go
```

## 3. 证据真伪与完整性

run 内已注册的 QA 族 artifact（`goal artifacts list`）及我复算的 sha256：

| 文件 | sha256 | 说明 |
|---|---|---|
| `.beacon/autotest/run_8d8.../TC-001/result.json` | `7b3ace73b57f2efeaf6377b0ddde81addcb54dea1831e5feb7dd76d11959821e` | 15:58 生成，对应 AC-001 |
| `.beacon/autotest/run_8d8.../TC-002/result.json` | `d35bd1fa5416ddefadf7177ea083a224d9b6be61e90f8696d3fa31456c1484c5` | 15:59 生成，对应 AC-002 |
| `.beacon/autotest/run_8d8.../TC-003/result.json` | `dc72f59199cbb33510909412697bde6be2dc13969500fd7ccf1b454d6fecbb13` | 15:59 生成，对应 AC-003 |
| `.beacon/autotest/run_8d8.../report.json` | `6869a23cb2a338af776036ec6ed893861c01bf02d0854d2396e0aa35135948b9` | 3/3 passed, failed 0 |
| `.beacon/artifacts/run_8d8.../qa-007.stdout.txt` | `d4afba2c2c4cf9b0d52c6f41830ff31d12d25c644f2789680c26d9f59de1085e` | 16:01 本 run 的 qa stage stdout |
| `.beacon/artifacts/run_8d8.../qa-007.stderr.txt` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | 空 |

时间线核对：autotest 15:58–15:59、qa stage 16:01，均早于 intent.json 的 16:03 提交，但对应代码在 af6dcd5 已落地；我在 afd5cc7（仅多该 json）上重跑三条 seed 全绿，说明证据不是旧跑残留、也不依赖未提交改动。

证据弱点（已用我的实跑补强）：`qa-007.stdout.txt` 只有 16 个包的 ok 行（缺 desktop/cmd/skills），stage 事件 payload 也只有一行 summary。因此我另跑全量：

```
go test ./... -count=1   # 2026-11-11 16:15:55 CST, HEAD=afd5cc7
19 个包 ok（含 desktop、skills）+ cmd/beacon [no test files]
EXIT_STATUS=0
```

原始输出：`full-gotest.txt`。

## 4. qa 门评估（正式、持久化）

```
$ beacon-go qa evaluate --project . --feature intent-contract-bridge \
    --run run_8d8efdf2c2a00a1f1b8e3778 --evaluator '验收员 · 独立验收'
gate:      qa
feature:   intent-contract-bridge
verdict:   pass
run:       run_8d8efdf2c2a00a1f1b8e3778
evaluator: 验收员 · 独立验收
  [ok   ] qa_artifacts_present     QA artifact pointers recorded
  [ok   ] qa_stage_executed        a qa stage execution is on record (stage_executed)
  [ok   ] qa_artifact_refs_valid   every QA artifact file exists and its hash matches
recorded:  gate_dd7af7d0943bdb6c23be9ed8 (2026-11-11T08:17:56.589166000Z)
EXIT=0
```

## 5. 门开后 goal status（实测）

```
run:    run_8d8efdf2c2a00a1f1b8e3778
status: blocked (loop_stage qa_routing)
stage:  qa
blocked: true
run_status: Paused
blocked_streak: 1
last_block_reason: qa_stage_not_executed
reason: qa_stage_not_executed
gate:   qa evaluated=true verdict=pass
next:   beacon-qa
```

门本身已 pass，但 run 的 stage-exit 阻塞快照没有被 `qa evaluate` 重算，`blocked=true` 仍是 16:01 的旧记录。这不是验收失败，也不是代码缺陷：下一步应由 goal driver 执行 `goal continue`/推进一档清掉这个软阻塞（streak=1，可解）。我没有替 driver 推进阶段。

## 6. 过程备注（不改变本判定）

- 实现提交已于 16:02 由运维车道推到 `origin/main=af6dcd5`；该 push 属于 later 的仓库维护派工（人类告警驱动），不是实现轮自行推送。QA 判定针对功能与证据；发布说明仍须人类确认。`afd5cc7`（docs intent.json）仍是本地未推送提交，推不推由人类定。
- 16:04 并行车道的 `run_4b94...`（executor=subprocess）已把 qa 门标过一次 pass，评估者为 `apple(via opencode)`（maker 侧）；我已用独立身份重新评估覆盖为可归属的 verdict。

## 7. 复现命令

```
cd /Users/apple/Developer/Personal/products/beacon-go
git rev-parse HEAD && git status --porcelain -uall
beacon-go truth list --project .
go test ./internal/cli -run TestIntentRecordLandsBriefingInPackageAndRun -count=1 -v
go test ./internal/orchestrator -run 'TestKernelDrivenRunWithoutIntentIsRefused|TestResumeKeepsUtteranceInRunEvent' -count=1 -v
go test ./... -count=1
beacon-go goal artifacts list --run-id run_8d8efdf2c2a00a1f1b8e3778
beacon-go qa evaluate --project . --feature intent-contract-bridge --run run_8d8efdf2c2a00a1f1b8e3778 --evaluator '验收员 · 独立验收'
beacon-go goal status --run-id run_8d8efdf2c2a00a1f1b8e3778
```

## 8. 本目录文件 sha256（自证）

```
2b0aa8c14ad61481ffe4a6d62be5709c2fb24f33f4c094f0789f18c2355daf1f  ac-seed-rerun.txt
aed00df2b62640394c123c563ce59f49908440c8b6346a20aea0beef1f98723e  full-gotest.txt
```

verdict.md 自身 sha256 随文件定稿后在回执中给出。
