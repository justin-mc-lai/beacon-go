# QA Verdict · intent-contract-bridge · 验收员

- run: `run_8d8efdf2c2a00a1f1b8e3778`
- 派工 HEAD: `af6dcd5`；实测 HEAD: `afd5cc7`（+1 纯文档提交，仅新增包内 `intent.json` 27 行，`*.go` 无改动）
- 判定时间: 2026-11-11 16:12 CST
- 边界: 未改源码 / truth / seed / release；未 push；本文件与 `qa-gotest-验收员.txt` 是唯一新增

## 判定：FAIL（AC-001/002/003 全 PASS；硬约束「不 push」已被打破）

代码与验收种子本身经我独立实跑全部通过，**实现不需要按 AC 返工**。判 FAIL 的唯一原因：冻结 truth 的 constraints 与派工 DoD 都写明「不 push」，但 `origin/main` 已被推进到 `af6dcd5`（实现提交 `a9122b2` 已上远端）。qa 门在硬约束被破坏时不能直接放行。

## 一、逐条 AC（我的实测，不采信实现方回执）

| AC | 我的实测 | 结论 |
|---|---|---|
| AC-001 | `go test ./internal/cli/ -run 'Intent' -count=1 -v` → `TestIntentRecordLandsBriefingInPackageAndRun` PASS（3 子例：无 run 只落文件 / --run-id 优先且默认取最近 run / confirmed=false 镜像） | PASS |
| AC-002 | `TestKernelDrivenRunWithoutIntentIsRefused` PASS，10 子例：recording、subprocess、subprocess:<profile>、ralph、Exec 配置均拒建；manual/external 放行；空 restated_intent、confirmed=false、坏 JSON 均拒；`Runs().ListByFeature` 计数不增 | PASS |
| AC-003 | `TestResumeKeepsUtteranceInRunEvent` PASS：非 replay_substep 也落 `goal_resumed`；utterance 逐字一致；空 utterance 时 `utterance_recorded=false` 且无 `utterance` 键；原 `goal_run_created.utterance` 不变 | PASS |

代码落点核对（与 AC 文本一致）：

- AC-001：命令注册 `internal/cli/truth_intent.go:204`，`--briefing` 为必填；落盘 `internal/orchestrator/intent_contract.go:114`；原子写为临时文件 + `os.Rename`（`intent_contract.go:223`）；事件 payload 含 `intent_path` / `intent_sha256` / `briefing_sha256` / `restated_intent` / `risks` / `boundaries` / `proposed_slugs` / `confirmed` / `recorded_by`；seed 断言文件字节 sha256 与事件里的 `intent_sha256` 相等。
- AC-002：门在 `CreateRun` 内、`BuildStageOrder` 与任何 run 行写入之前（`internal/orchestrator/engine.go:479`）；判定集合 `IsKernelDrivenExecutor`（`intent_contract.go:100`）覆盖 recording / subprocess / subprocess:* / ralph / `Exec != nil`；reason code `goal_intent_missing`（`internal/core/reasons.go:360`），错误文案带 `beacon truth intent record` 修复指引。
- AC-003：`Resume`（`internal/orchestrator/resume.go:221`）删除了 `_ = utterance`，每次 resume 在同一事务里追加 `goal_resumed` 并保存 run；payload 含 `resume_action` / `utterance_recorded` / `stage` / `loop_stage`，空 utterance 不写 `utterance` 键。

## 二、派工 DoD

| DoD 项 | 我的实测 | 结论 |
|---|---|---|
| 三条 seed FAIL→PASS | 合并正则实跑，三条全 PASS，EXIT=0 | PASS |
| `go test ./...` 全绿 | 我本机重跑，19 个包 ok，shell exit 0，无 FAIL | PASS |
| seed 断言/helper 未改 | `git diff --stat 2a61a25 -- <两份 seed>` 为空；sha256 与 before 一致 | PASS |
| 不碰 release | `2a61a25..HEAD` 无 release 相关文件改动 | PASS |
| 不 push | `origin/main` 实际已是 `af6dcd5` | **FAIL** |

```
$ shasum -a 256 internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go
c1df802782f9352a15868b96f18b1bd98e246b0ceb588cc4bd9619dec588cbb0  internal/cli/intent_contract_seed_test.go
b1a204f06163851ebf4f6f133a88142b591bdf51d15186c73a0f3d86694450b6  internal/orchestrator/intent_contract_seed_test.go
```

## 三、阻塞项与必改项

### 必改 1：远端 main 已被推送

```
$ git ls-remote origin refs/heads/main
af6dcd594729bddc3b149e25688174eeb41f4176	refs/heads/main

$ git reflog show origin/main --date=iso | head -2
af6dcd5 refs/remotes/origin/main@{2026-11-11 16:02:45 +0800}: update by push
```

即实现提交 `a9122b2`（含 AC 全部业务代码）连同 `cb58087`、`af6dcd5` 已在远端 main。truth.md constraints 与派工 DoD 均为「不 push」。

必改：下一轮任何实现/证据动作前，先停掉会自动 commit + push 的外部进程（该进程以 `kunqi.lai <goodlive.lkq1128@gmail.com>` / `apple(via opencode)` 身份在 16:02:45 推送）；远端是回滚到 `2a61a25`（truth 冻结点）还是接受 `af6dcd5`，需人类裁决。作为 QA 我未执行也不会执行 push / force-push / 回滚。

### 必改 2：回执 HEAD 与 git status 已与实测不一致

- 派工与回执写 `HEAD=af6dcd5`、工作区干净；实测 `HEAD=afd5cc7`（16:03:23 追加 `intent.json`，无 `*.go` 改动）。
- 实测 `git status --short` 非空：`?? docs/changes/ops-2026-11-11-rebuild/`（外部构建重建日志，非本 feature 交付物）。

必改：回执按实测 HEAD/status 重写并说明该未跟踪目录来源；否则「HEAD=af6dcd5、工作区干净」不可复核。

## 四、复现命令（全部由我实跑）

```
cd /Users/apple/Developer/Personal/products/beacon-go
git log --oneline -5
git status --short
git ls-remote origin refs/heads/main
git reflog show origin/main --date=iso | head -3
go test ./internal/cli/ ./internal/orchestrator/ -run 'Intent|ResumeKeepsUtterance' -count=1 -v
go test ./... -count=1                              # 19 包 ok，exit 0
git diff --stat 2a61a25 -- internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go
shasum -a 256 internal/cli/intent_contract_seed_test.go internal/orchestrator/intent_contract_seed_test.go
```

全量测试原始输出见 `qa-gotest-验收员.txt`。

## 五、放行条件

- 若人类接受 `af6dcd5` 已在远端：AC 证据与测试证据足以支撑本轮 qa 的**功能**结论为 PASS，代码无需返工；仍需关闭必改 2 的证据一致性问题。
- 若人类要求遵守「不 push」：必须先处理远端状态并阻断外部自动推送，再重新出具 QA 结论。
