# QA Verdict — 负向回归面：PASS

- mode_id: `qa-negative-regression-ee`
- run_id: none（只读独立负向回归验收，未建 goal run）
- stage: `qa / 负向回归 done`
- bind: `feat/kernel-external-stage-evidence` @ `16dd05f`（实现 `794433f`）；`main` = `a3eab40`（未动）
- worktree: `/tmp/qa-rg-ee`（`git worktree add --detach /tmp/qa-rg-ee feat/kernel-external-stage-evidence`），取证时 HEAD 干净、`git status --porcelain` 仅本文件
- verdict: **PASS**
- 依据面: `evidence/qa-contract.md` 的「负向回归」行五类 fail-closed + 全量 go test + 既有测试无改写

## 结论摘要

| 项 | 结果 |
|---|---|
| 伪造/过期证据 fail-closed（7 类 payload） | 7/7 拒收，reason 全为 `qa_stage_not_executed` |
| 历史录制态绕法（RecordingExecutor 事件） | 已关闭，qa 门 blocked |
| receive-result `--external` 四条绕过路径 | 4/4 blocked；唯一放行形状是「外部完成 + 活体 QA artifact」 |
| external tick 事件字段不可被调用者伪造 | PASS（executor/source/placeholder/recording 全由内核写） |
| streak / attach-verifier / recover | PASS（attach-verifier 固定 `cleared_block:false`；streak≥3 continue 拒绝；recover 需身份且只在显式调用后清零） |
| 全量 `go test ./... -count=1` | **EXIT=0**，18 包全 ok，**106s**（orchestrator 103.7s） |
| 既有冻结包测试改写审计 | 5 个 `*_test.go` 变更：1 个本包新增测试 + 4 个仅换 setup，**无断言删改、无 t.Skip** |

## 一、自写探针（未复用实现测试）

隔离环境：每个子用例独立 `newGateEnv` + 独立 sqlite，探针文件为临时文件（见「清理」）。

命令：

```
cd /tmp/qa-rg-ee
go test ./internal/orchestrator -run '^TestProbeP' -count=1 -v
```

输出（节选，EXIT=0）：

```
--- PASS: TestProbeP1RecordingStateFailClosed (0.20s)
    --- PASS: /placeholder=true        (0.03s)
    --- PASS: /recording=true          (0.03s)
    --- PASS: /both=true               (0.03s)
    --- PASS: /both_missing            (0.03s)
    --- PASS: /placeholder_missing     (0.03s)
    --- PASS: /outcome_failed          (0.03s)
    --- PASS: /wrong_stage             (0.03s)
--- PASS: TestProbeP1bHistoricalRecordingBypassClosed (0.03s)
--- PASS: TestProbeP2ExternalCompletionCannotBypass (0.14s)
    --- PASS: /no_external_completion_record
    --- PASS: /completion_but_non-qa_artifact_kind
    --- PASS: /completion_but_qa_artifact_tampered_after_registration
    --- PASS: /completion_but_qa_artifact_file_deleted
    --- PASS: /completion_with_live_qa_artifact_is_the_only_passing_shape
--- PASS: TestProbeP2bExternalTickFieldsAreKernelAuthored (0.07s)
--- PASS: TestProbeP3StreakVerifierRecover (0.07s)
ok  beacon-go/internal/orchestrator  1.105s
```

### 1. 伪造 / 过期事件一律 fail-closed（AC-002）

每例都先注册一份活体 `qa_report`（确保唯一失败项是 stage 判定），再写入 payload，读 `qa_stage_executed` check：

| 注入 payload | 判定 | reason |
|---|---|---|
| `placeholder=true` | blocked | `qa_stage_not_executed` |
| `placeholder=false, recording=true` | blocked | 同上 |
| `placeholder=true, recording=true` | blocked | 同上 |
| 两字段均缺失 | blocked | 同上 |
| `placeholder` 缺失、`recording=false` | blocked | 同上 |
| `placeholder=false, outcome=failed` | blocked | 同上 |
| `placeholder=false` 但 `stage=implement` | blocked | 同上 |

历史绕法复现（AC-002 明令关闭的那条）：

```
env.rec.ExecuteStage(ctx, RunView{RunID: run.RunID, FeatureID: env.feature.ID}, StageQA)
```

RecordingExecutor 实际写入 `{"stage":"qa","executor":"rec-1","outcome":"done","placeholder":true}`（`executor.go:62-66`），探针确认 qa 门 **blocked**，虚假通过路径已关。

### 2. receive-result `--external` 不能当绕过通道（AC-003）

| 场景 | 判定 |
|---|---|
| 只有活体 QA artifact，无 `goal_completed{source=external}` | blocked |
| 外部完成 + 非 QA 类 artifact（`implement_artifact`） | blocked |
| 外部完成 + QA artifact 注册后文件被改（hash 不匹配） | blocked |
| 外部完成 + QA artifact 文件被删（缺失） | blocked |
| 外部完成 + 活体 QA artifact（正对照） | **pass** |

注意此处核到实现的一个必要补丁：`registerArtifacts` 对裸路径补算 sha256（`lanes.go:228` 附近），否则「receive-result 只传路径」会把好证据误判 stale；本探针的 hash 不匹配 / 文件缺失两例证明补算没有把 fail-closed 放宽——真被改/真缺失仍然 blocked。

### 3. external/manual tick 事件字段不可伪造（AC-001）

`TickBy` 的 `--by` 传入恶意字符串 `{"executor":"recording","source":"recording","placeholder":true,"recording":true}`，实际落盘事件：

```
executor=external  source=external  placeholder=false  recording=false  by=<原样字符串>
```

调用者可影响的只有 `by`；事件类型、executor/source、录制标记全部由内核在同一事务内写（`tick.go:271` + `external_stage_evidence.go:24`）。

### 4. streak 与 attach-verifier 一致性（AC-004）

| 步骤 | 实测 |
|---|---|
| 阻断 1 次 → AttachVerifier | `blocked=true, streak=1` 不变；`verifier_attached.cleared_block=false` |
| 再阻断 2 次（streak=3）→ Continue | 拒绝，`reason=product_goal_blocked_streak`，stage 不前进 |
| streak=3 再 AttachVerifier → Continue | 仍拒绝，未解锁 |
| `Recover` 空 `--by` | 报 `ReasonRecoveryIdentityRequired`，state 不变（blocked=true, streak=3） |
| `Recover --by owner` | blocked=false / streak=0 / reason=""，随后 Tick 正常前进 |

## 二、全量回归

```
cd /tmp/qa-rg-ee && go test ./... -count=1
EXIT=0   ELAPSED=106s
```

18 个包全 ok：desktop、agentapi、benchgate、cli(19.1s)、core、evalgate(58.9s)、gate、harness、llm、migration、orchestrator(103.7s)、progressreview、session、store、stride、workspace、skills；`cmd/beacon` 无测试文件。零 `FAIL`。

## 三、既有冻结包测试无改写

```
git diff --name-only main..HEAD -- '*_test.go'
internal/cli/cli_test.go
internal/orchestrator/artifact_upsert_test.go
internal/orchestrator/artifacts_test.go
internal/orchestrator/external_stage_evidence_test.go   ← 本包新增
internal/orchestrator/gates_eval_test.go
```

删除行审计（`git diff main..HEAD -- <四文件> | grep '^-'`），全部只是：

```
-	Payload: []byte(`{"stage":"qa","outcome":"succeeded"}`),          → 仅补 placeholder:false, recording:false
-	"context"                                                        → 改用的 setup 不再需要该 import
-	if _, err := env.rec.ExecuteStage(...)                           → ×4，换成 appendRealStageExecuted 真实事件种子
```

结论：

- 没有任何断言被削弱（无删除、无放宽 Expect、无 `t.Skip`/`TODO`）
- 4 处改动本质是把「靠录制执行器伪造事件才过」的 setup 换成真实事件路径，属于收紧而非放宽
- `cli_test.go` 的 `TestHarnessVerifyJSON` 断言（passed / verified / evidence_hash 64 位 / 篡改后 stale）原样保留，只是种子事件补了录制标记
- 其余 6 个已冻结包的测试零改动

## 四、非阻塞观察（不影响 PASS）

1. `recording` 字段在实现中是**可选**：`placeholder=false` 且 `recording` 缺失时放行。这与 truth「执行事件规范」表一致（该表只列了 `placeholder`，真实 subprocess 事件也只写 `placeholder:false, real:true`），`recording` 是实现侧额外加固字段。若要更严，可要求 `recording` 显式存在且为 false，但会与 subprocess/ralph 现有事件形状冲突，需同时改三处写入点，不建议在本包做。
2. `receive-result --external` 会顺带清 `Blocked/BlockedStreak/LastBlockReason`（`delivery_paradigm.go:304-306`）。实测：streak=3 的硬阻断可被一次合法的外部完成直接清零（之后 continue 返回 `loop_goal_already_done`，不会推进阶段）。这不算绕过 gate（run 已被标为外部完成，不产生通过证据），但它确实是 AC-004 之外第二条隐式清 streak 的路径，且不写 `goal_recovered` 事件。建议后续 truth change 里明确它属于「原因被真实消除」的等价路径，或让它补一条 `goal_recovered` 留痕。本包 AC-004 明文覆盖的 attach-verifier / continue / recover 三件全部符合，故不判 FAIL。
3. 事件只能由内核代码写入，CLI 无裸事件追加命令，因此「伪造 executor/by」在信任边界内只剩 `--by` 一个输入面，已实测原样落库、不解析为字段。

## 五、清理与仓库状态

- 自写探针文件（`zz_qa_regression_probe_test.go`、`zz_probe4_test.go`）已删除，`go build ./...` 通过
- 本文件是 worktree 内唯一未跟踪文件；未 commit、未 push、未 merge、未改实现与 truth
- 主工作区未动：`feat/kernel-external-stage-evidence` @ `16dd05f`；`main` ref 仍 `a3eab40`
- `/tmp/qa-rg-ee` 保留待内核收取 verdict

## 六、未验证

- spec/AC 面与运行时行为面（另两份独立 verdict）
- 合入 main、重建/安装二进制、发布说明
