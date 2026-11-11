# QA Verdict: kernel-recovery-qa-bridge

- **verdict: PASS**（AC-001..AC-005 独立复跑全过；无代码必改项）
- maker: BeaconGo 内核
- checker: 验收员（独立，maker≠checker）
- 验收时间: 2026-11-11 17:02–17:09 CST
- HEAD: `0e78397227629216e1cc7026ced4bb27230d1e05`（验收开始时为 `3f50e09`；期间新增 `943512e`/`292e090`/`0e78397` 三个证据类提交。`git diff --stat 3f50e09..HEAD -- '*.go'` 为空，seeds/truth/tests 也逐字未变，故 3f50e09 上的测试结论对 0e78397 成立）
- truth: `frozen=true`、`change_pending=false`、baseline `b0c9165`、truth.md sha256 `bfb60aeef632c4c4227741c0a80488746c76f55b0dd9a682501d210d1fe022b2`（与冻结记录一致）
- 构建: `go build -o /tmp/beacon-qa-accept ./cmd/beacon` → OK（go1.27.3 darwin/arm64）

## 总判

AC-001..AC-005 全部独立复跑通过：三条 seed 均 exit 0 且输出 `RESULT: PASS`；四条 seed sha256 与 tests.md 逐字一致；AC-004 五个子用例、AC-005 均 PASS；`go test ./...` exit 0（19 包 ok、0 FAIL）。AC-001 另在**真实 feature**（kernel-recovery-qa-bridge 冻结 truth）+ 隔离 store 副本上完整复现 Blocked→recover→qa 不自锁。

没有需要返工的代码项。**有一条进程类必读项**：派单指定的真实 run 在我验收期间被外部身份推进到 release，并写入了一条不是我签发的 qa pass（见 §6.1、§7）。

## AC 结论

| AC | 结论 | 依据 |
|----|------|------|
| AC-001 | PASS | seed1（Blocked→InProgress，tick→qa 不 block）+ 真实 feature 隔离副本复现 + Release/Completed 非降级探针 + 代码复核 `updateLifecycleMonotonic` |
| AC-002 | PASS | seed2（autotest 1/1 全过 → qa stage_executed count=1、qa gate pass）；代码仅 `Total>0 && Failed==0 && Blocked==0` 才写 |
| AC-003 | PASS | seed3（run A pass 后，零证据 run B 停在 qa，reason=stage_exit_qa_evidence_missing）；`featureGateFallback` 排除任何被 run 绑定的 gate 行 |
| AC-004 | PASS | 5 个子用例全 PASS（human gate 拒绝且快照不变；marker=true/recording=true/缺 marker 均不计；真实 stage_executed 仍计） |
| AC-005 | PASS | 词表常量未改名；run 内 FORMAL-FIRST 三态（formal pass / formal blocked / 无 verdict 时 derived）全过 |

## 1. 三条 seed（我自己的复跑，HEAD 3f50e09）

```text
$ BEACON_BIN=/tmp/beacon-qa-accept zsh .../seeds/seed1_recover_lifecycle.sh
after_block=Implement/Blocked
after_recover=Implement/InProgress
tick_stage=qa
tick_blocked=False
tick_reason=None
sha256=488c496ebc2335ff223c8cfbdbb977c4005f07c392d7a2d1347b9e30ce2474fe
RESULT: PASS - recover restored the lifecycle snapshot and implement advanced to qa
exit=0

$ .../seed2_autotest_stage_executed.sh
autotest_total=1
autotest_passed=1
autotest_failed=0
qa_verdict=pass
qa_reason_codes=
qa_checks=qa_artifacts_present=True,qa_stage_executed=True,qa_artifact_refs_valid=True
qa_stage_executed_count=1
sha256=7eacb12dd0b3ef5fc3ca248395b5ec869ae8a36011b870098d523f879e903833
RESULT: PASS - autotest produced qa stage evidence and the qa gate passed
exit=0

$ .../seed3_cross_run_qa_verdict.sh
run_a_qa_verdict=pass
run_b_stage=qa
run_b_blocked=True
run_b_reason=stage_exit_qa_evidence_missing
run_b_qa_stage_executed_before_exit_tick=0
sha256=92cdec723e6c5d6ca3c6e7dc7a3012f2f7e77c257b31178541c5294ab6a93670
RESULT: PASS - run B stayed at qa (blocked=True, reason=stage_exit_qa_evidence_missing) without its own qa evidence
exit=0
```

## 2. seed sha256（与 tests.md 逐字一致）

```text
2c6997bcf2212bb72f87095951a507f4e4544daae3b9384504e5815df71ae606  seeds/lib.sh
a2476a47b08c24c24d2c6f6210501b535a6d1169b8e949df56812899869e680a  seeds/seed1_recover_lifecycle.sh
e8285ff425202fc6aeec3fc965510659b0c9f227c5700eedffc946a1b1847c1b  seeds/seed2_autotest_stage_executed.sh
744f187e55ad5b914c5158af15bac5a26acf64f92dffe792755b6f9d64eb0ce9  seeds/seed3_cross_run_qa_verdict.sh
```

## 3. AC-004

```text
$ go test ./internal/orchestrator -run TestKernelRecoveryNegativeGuards -count=1 -v
--- PASS: TestKernelRecoveryNegativeGuards (0.14s)
    --- PASS: .../human_gate_recover_refused_without_state_change (0.03s)
    --- PASS: .../stage_executed_rejected:_executor_marker_true (0.03s)
    --- PASS: .../stage_executed_rejected:_recording_true (0.03s)
    --- PASS: .../stage_executed_rejected:_marker_missing (0.03s)
    --- PASS: .../real_stage_executed_still_counts (0.03s)
PASS
ok  beacon-go/internal/orchestrator  0.451s
exit=0
```

## 4. AC-005

```text
$ go test ./internal/orchestrator ./internal/gate -run TestKernelRecoveryContractStable -count=1 -v
--- PASS: TestKernelRecoveryContractStable (0.00s)
PASS
ok  beacon-go/internal/orchestrator  0.328s
testing: warning: no tests to run
PASS
ok  beacon-go/internal/gate  0.523s [no tests to run]
exit=0
```

`internal/gate` 包里该测试不存在（Go 以 `no tests to run` + exit 0 返回），派单方已说明这是预期口径；契约断言实际落在 orchestrator 包。

## 5. go test ./...

```text
ok  beacon-go/internal/cli          16.633s
ok  beacon-go/internal/evalgate     58.799s
ok  beacon-go/internal/orchestrator 105.532s
...（共 19 个包 ok）
EXIT=0
FAIL 行数: 0
```

## 6. AC-001 真实 run 验收

### 6.1 派单指定的 run 已在我检查前被外部推进

```text
$ truth show --feature kernel-recovery-qa-bridge --version v2.0.0 --json
lifecycle_stage=Qa  lifecycle_status=Completed

$ goal status --run-id run_0e931985c7e20c51de10f590 --json
current_stage=release  blocked=False  release_human_confirmed=False
```

真实 store 事件（UTC，CST=+8）：

| 时间 (UTC) | 事件 | 要点 |
|---|---|---|
| 09:02:57 | goal_recovered | stage=implement，by=apple(via opencode) |
| 09:03:00 | implement_admission_blocked | blocked_streak=3，product_goal_blocked_streak |
| 09:03:33 | goal_recovered | stage=implement，prior_blocked=true |
| 09:03:37 | stage_advanced | implement→qa（derived exit pass） |
| 09:03:46 | gate_evaluated | qa verdict=pass，evaluator=`QA 复核 · opencode` |
| 09:03:57 | stage_exit_evaluated / stage_advanced | qa formal pass；qa→release |
| 09:04:05 | verifier_attached | source=beacon-eval-qa |

即：派单预期的 `Implement/Blocked` 起始态在 09:02:57–09:03:33 被外部 recover 消耗，随后被推进到 release。该 run 上已不存在可复现的 Blocked 自锁场景。

我**没有**在这个真实 run 上跑 recover/tick/block：前置场景已消失，此时 recover 只会把 `Qa/Completed` 改写成 `Release/InProgress`，无法验证 Blocked 路径，反而会改动一个 release 待确认的状态。

### 6.2 真实 feature + 隔离 store 副本复现（我自己的写与读）

用 `sqlite3 ~/.beacon/beacon-go.db ".backup /tmp/qa-real-copy-1.db"` 复制真实库，在副本上新建 run `run_a43d7480f9c34f68f5f79e22`（feature=kernel-recovery-qa-bridge，真实冻结 truth），不触碰真实库。

```text
tick 1 → current_stage=truth
tick 2 → current_stage=implement
goal block --reason-code evidence_gap → blocked=true reason=evidence_gap

BEFORE_RECOVER: Implement/Blocked reasons=["evidence_gap"]
$ goal recover --run-id run_a43d7480f9c34f68f5f79e22 --by acceptance
exit=0  blocked=False  blocked_streak=0  last_block_reason=None
AFTER_RECOVER:  Implement/InProgress reasons=[]

$ goal tick --run-id run_a43d7480f9c34f68f5f79e22 --by acceptance
exit=0  current_stage=qa  blocked=False  reason_code=None
AFTER_TICK:     Implement/Completed reasons=[]   # tick 离开 implement 时把该 stage 记为 Completed
```

没有出现 `stage_exit_implement_blocked`，run 正常进入 qa。

非降级探针：把副本 lifecycle 置为 `Release/Completed` 后对同一 run 跑 recover——

```text
SET_TERMINAL:              Release/Completed reasons=[]
recover exit=0
AFTER_RECOVER_ON_TERMINAL: Release/Completed reasons=[]   # 未被降级
```

human gate 探针：block `release_requires_human_confirmation` 后 recover——

```text
recover exit=1
Error: goal.Recover: human gate blocker release_requires_human_confirmation requires human
confirmation; recover cannot clear it (goal_recovery_human_gate_forbidden)
AFTER_HUMAN_RECOVER: Release/Completed reasons=[]   # 快照不变
```

### 6.3 定性

**真实 run 上 AC-001 通过，且不是“只覆盖 seed 场景”。** 真实 feature 的冻结 truth 下复现了完整的 `Implement/Blocked → recover → Implement/InProgress → tick 到 qa（不 block）`，并额外证明 Release/Completed 不被降级、human gate 仍拒绝 recover。真实 run 本身已由外部推进到 release（`release_human_confirmed=false`，未 release），不存在“真实库仍卡死”。

补充：本机安装的 CLI `~/.local/opt/beacon-go/beacon` 是 16:05 从 `afd5cc7+dirty` 构建的旧二进制（`go version -m`），不含 16:56 的修复。因此 17:02–17:04 真实库上外部产生的推进，我不作为修复证据，只作为状态漂移记录；上述结论来自我用当前 HEAD 新建的二进制。

## 7. 负向与边界

- 未 push：`origin/main=af6dcd5`，`git rev-list --left-right --count HEAD...origin/main` = `8  0`；HEAD 无 tag。
- 未 release：真实 run `release_human_confirmed=false`。
- 未在真实 store 写 qa verdict；真实 store 里那条 qa pass 的 evaluator 是 `QA 复核 · opencode`，不是本验收员。
- 未改 seed（四条 sha256 一致）、未改 reason code 词表、未动 intent-contract-bridge 的 `??` 文件。
- 我未在真实 run 上 tick / recover / block；唯一新增的仓库文件是本文件。
- 进程类必读：派单后、我验收期间，身份 `apple(via opencode)` 把 `run_0e931985` 从 implement 推进到 release，并写入 qa pass（§6.1）。这条 pass 不是独立验收签发；release 仍由 human gate 挡住。把该 run 当作发布候选前，应由 human 决定 reset/reopen，或明确接受这次外部 qa pass。

## 8. 复现命令

```sh
cd /Users/apple/Developer/Personal/products/beacon-go
go build -o /tmp/beacon-qa-accept ./cmd/beacon
BEACON_BIN=/tmp/beacon-qa-accept zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh
BEACON_BIN=/tmp/beacon-qa-accept zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh
BEACON_BIN=/tmp/beacon-qa-accept zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh
go test ./internal/orchestrator -run TestKernelRecoveryNegativeGuards -count=1
go test ./internal/orchestrator ./internal/gate -run TestKernelRecoveryContractStable -count=1
go test ./...
```
