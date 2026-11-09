# 独立 QA Verdict — spec/AC 面：kernel-external-stage-evidence

| 字段 | 值 |
|---|---|
| mode_id | `qa-spec-ac-ee` |
| run_id | none（只读独立验收，未建 goal run） |
| stage | `qa / Implementation-Committed` |
| 分支 | `feat/kernel-external-stage-evidence` @ `16dd05f`（实现 `794433f`） |
| 判定 | **PASS** |
| evidence_refs | 本文件；`truth.md`、`tests.md`、`tasks.md`、`evidence/` 下四份 ac-e 证据与 `go-test-all.txt` |
| next | 等运行时行为面与负向回归面两份 verdict；三方齐后由内核收口 |

plain_summary：AC-001 至 AC-004 判据原文与冻结 truth 一致；8 条 TC 与 AC 一一对应、每条 AC 两条且都是执行层断言；我独立实跑 8/8 退出码 0。实现改动的 4 个既有测试文件只换了 setup，断言没有一处被放宽。

## 一、AC 判据未被扩张

冻结 truth 磁盘 SHA256 = `24003b502aa921c3205eee8b53b79578354ee112d6aee677b98c6e42c9a751e9`，与 `evidence/freeze-log.md` 记录的冻结后 SHA 完全一致（`truth.md` 136 行，工作区 `git status` 干净）。在主仓路径执行 `~/.local/opt/beacon-go/beacon truth drift --project . --feature kernel-external-stage-evidence` 返回 `clean: true` / `body drift: false`。

（说明：在 `/tmp/qa-spec-ee` 内直接跑 drift 会报 `feature_not_found`，因为该临时 worktree 未在本地 beacon 库里注册 feature；主仓注册信息对应同一份 truth，因此以主仓结果为准，文件 SHA 两边一致。）

| AC | 判据原文位置 | 实现落点 | 结论 |
|---|---|---|---|
| AC-001 | truth.md:67 | `external_stage_evidence.go:24-34` + `tick.go:271`（同一事务） | 一致，未扩张 |
| AC-002 | truth.md:68、87-95 | `gates_facts.go:245-254` | 一致，未扩张 |
| AC-003 | truth.md:69、103-107 | `gates_facts.go:255-282`、`lanes.go:219-247` | 一致，取舍关系已写明 |
| AC-004 | truth.md:70、97-101 | `complete.go:58-115`、`cli/goal.go:394-420` | 一致，未扩张 |

## 二、AC 逐条核验

### AC-001 PASS

- `recordManualStageExecuted` 写 `stage_executed`，payload 的 `stage/executor/source/by/placeholder/recording/outcome` 全部由内核生成，只有 `by` 来自 `TickBy` 调用方（`external_stage_evidence.go:24-34`）。事件与 `stage_exit_evaluated`、`stage_advanced`、run 快照在同一个 `WithTx` 内提交（`tick.go:266-296`），不会只留推进记录。
- TC-001 实跑：`TestExternalStageCompletionRecordsRealStageExecuted`，EXIT=0（0.612s）。断言恰好 1 条 `stage_executed`，`stage=plan`、`executor=external`、`source=external`、`by` 等于调用方身份、`placeholder=false`、`recording=false`、`outcome=done`。
- TC-002 实跑：`TestExternalStageEventCannotBeForgedExternally`，EXIT=0（0.386s）。把 `{"executor":"recording","placeholder":true,...}` 整段塞进 `--by`，内核字段仍是 external/false，`by` 原样保留为普通字符串，事件没有多出调用方键。
- 观察（不阻断）：TC-001/TC-002 只显式建了 external run。manual 与 external 共用 `isManualExecutor()`（`delivery_paradigm.go:60-62`）和同一个 `manualCompletion` 分支（`tick.go:173-197`），行为无分叉；但建议后续给 TC-001 补一个 manual 子用例，把 AC 原文里的 manual 也直接钉住。

### AC-002 PASS

- `evalQA` 只在 `placeholder` 字段存在且为 `false`、`recording` 缺省或为 `false` 时才计 `qa_stage_executed`（`gates_facts.go:245-254`）。录制执行器写的 `placeholder:true` 一律不计；`placeholder` 缺失也按录制态拒绝。
- TC-003 实跑：`TestQAGateRejectsRecordingExecutorStageEvent`，EXIT=0（0.358s）。仅 recording 事件时 `qa_stage_executed` 为 false、reason 为 `qa_stage_not_executed`、verdict=blocked；再追加一条无 marker 的 `stage_executed` 仍被拒。
- TC-004 实跑：`TestQAGateAcceptsRealExternalStageEvent`，EXIT=0（0.355s）。真实 external 事件让 `qa_stage_executed` 为 true 且 qa 门 pass。
- 关于「字段缺失按录制态拒收」：冻结 truth 的执行事件表（truth.md:72-85）只把 `placeholder` 列为录制标记字段，`recording` 不在表内；`exec_subprocess.go:126-131` 的真实事件本来就不带 `recording`。实现要求 `placeholder` 必须存在且为 false、拒绝 `placeholder` 缺失与 `recording:true`，同时接受 `recording` 缺省，这与冻结判据及真实 subprocess 路径一致；若把缺失 `recording` 也拒收，反而会打断既有真实执行路径。此处属实现已写明读法，不是判据扩张。

### AC-003 PASS

- `evalQA` 按 run 收集 `goal_completed` 且 `source=external`，并只在该 run 存在至少一份 QA 类、hash 仍与磁盘一致的 artifact 时，把 `qa_stage_executed` 置真（`gates_facts.go:255-282`）。等价路径只在没有真实 `stage_executed` 时兜底，不覆盖逐阶段路径。
- `registerArtifacts` 在调用方未给 hash 时从磁盘补算 sha256 与字节数（`lanes.go:219-247`）；空 hash、文件缺失仍判 stale，等价路径 fail-closed。
- TC-005 实跑：`TestExternalReceiveResultIsQAStageEvidence`，EXIT=0（0.352s）。
- TC-006 实跑：`TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts`，EXIT=0（0.422s），三个子用例（无 QA artifact / hash mismatch / 完成记录非 source=external）全部按预期 blocked。
- 与 AC-001 的取舍写在 truth.md:103-107 与 `ac-e3-external-completion.md`：逐阶段优先、整湖回写兜底、两者互不替代。
- 观察（不阻断）：`ReceiveExternalResult` 本身不校验 run 的 ExecutorMode 是否为 external（`delivery_paradigm.go:262-281`），它按命令语义写 `source=external`。这与 AC-003「`receive-result --external` 完成事实」一致，但意味着任何能跑 CLI 的人都能用一份 hash 有效的 QA 文件走等价路径；这是 truth 已授权的信任边界，不是本包扩张。

### AC-004 PASS

- `AttachVerifier` 不再清阻断，事件固定 `cleared_block:false` 并附 `block_pending`（`complete.go:58-78`）。TC-007 实跑 EXIT=0（0.404s）：attach 后仍 blocked、`blocked_streak` 保持 1、第二次 block 累加到 2。
- 新增显式 `Engine.Recover` 要求 `--by`（缺失返回 `goal_recovery_identity_required`），清零 `Blocked/BlockedStreak/LastBlockReason` 并写 `goal_recovered`，同时记录恢复前 streak 与原因（`complete.go:82-115`；CLI `goal recover --run-id --by [--note]`，`cli/goal.go:394-420`）。
- TC-008 实跑 EXIT=0（0.405s）：streak=3 时 `ContinueBy` 返回 `product_goal_blocked_streak` 且阶段不动；`Recover` 后 streak=0，再 tick 推进到 truth。

## 三、4 个既有测试文件是否被削弱（重点核验）

`git diff a3eab40..16dd05f` 对这 4 个文件只有 setup 级改动，**没有一行断言被修改或放宽**：

| 文件 | 改动 | 断言是否变化 |
|---|---|---|
| `internal/cli/cli_test.go:462` | stage_executed payload 字面量补 `placeholder:false, recording:false` | 否；仍是「真实 QA 证据必须 pass + 篡改后必须 stale」 |
| `internal/orchestrator/artifact_upsert_test.go:44` | `env.rec.ExecuteStage` → `appendRealStageExecuted` | 否；三个 upsert/staleness 用例的断言未动 |
| `internal/orchestrator/artifacts_test.go:176` | 同上 | 否；仍断言「有 stage 事件但未注册 artifact 必须 blocked」 |
| `internal/orchestrator/gates_eval_test.go:266,320` | 同上（两处） | 否；`TestQAGateEvidenceFlow` 仍按 blocked→pass→tamper blocked 断言，`TestReleaseGateComposition` 仍要求全链 + 人工确认才 pass |

`appendRealStageExecuted` 定义在 `external_stage_evidence_test.go:56-66`，写的是 `executor=subprocess, source=local, placeholder=false, recording=false` 的真实形状事件；它替代的是原先靠 RecordingExecutor 伪造 `placeholder:true` 才通过的 setup。这正是本次要关闭的路径，不是把断言变松。

我重跑了受影响的既有用例：`go test ./internal/orchestrator -run '^(TestQAGateEvidenceFlow|TestReleaseGateComposition|TestRegisterArtifactQAGateFlow|TestQAStalenessUsesLatestPerPath|TestQALegacyDuplicatesNoMigration|TestQAReRegisterClearsStale)$' -count=1` → ok（0.642s）；`go test ./internal/cli -run '^TestHarnessVerifyJSON$' -count=1` → ok（0.574s）。

## 四、TC 与任务追踪

- 8 条 TC 与 AC 映射：AC-001←TC-001/002，AC-002←TC-003/004，AC-003←TC-005/006，AC-004←TC-007/008；每条 AC 两条，均为执行层断言（读真实事件、真实 `EvaluateGate`、真实 `ReceiveExternalResult`、真实 `ContinueBy/Recover`），没有只测辅助函数或常量。
- TASK-000..007 已勾，evidence 全部指向包内真实文件（freeze-log、四份 ac-e、go-test-all），没有自指；TASK-008 仍未勾。
- `go-test-all.txt`：18 个包全 ok、`EXIT=0`、orchestrator 98.905s、commit 字段 `794433f`，是本次新测试覆盖后的全量输出。
- 轻微观察（不阻断）：`evidence.md` 索引里 truth/tests 仍标 `draft`，与 `truth.md` 的 `status: frozen` 不同步；上一批包在冻结提交时也是同样形状（见 `3396558` 的 change-quality evidence.md），属收口时可顺手更新的展示字段，不影响判据、SHA 或证据链路。

## 五、独立实跑清单

```bash
# Cwd: /tmp/qa-spec-ee
shasum -a 256 docs/beacon/v2.0.0/features/kernel-external-stage-evidence/truth.md
# 24003b502aa921c3205eee8b53b79578354ee112d6aee677b98c6e42c9a751e9

~/.local/opt/beacon-go/beacon truth drift --project . --feature kernel-external-stage-evidence
# （主仓路径）clean: true / body drift: false

go test ./internal/orchestrator -run '^TestExternalStageCompletionRecordsRealStageExecuted$' -count=1 -v
# PASS, EXIT=0, 0.612s
go test ./internal/orchestrator -run '^TestExternalStageEventCannotBeForgedExternally$' -count=1 -v
# PASS, EXIT=0, 0.386s
go test ./internal/orchestrator -run '^TestQAGateRejectsRecordingExecutorStageEvent$' -count=1 -v
# PASS, EXIT=0, 0.358s
go test ./internal/orchestrator -run '^TestQAGateAcceptsRealExternalStageEvent$' -count=1 -v
# PASS, EXIT=0, 0.355s
go test ./internal/orchestrator -run '^TestExternalReceiveResultIsQAStageEvidence$' -count=1 -v
# PASS, EXIT=0, 0.352s
go test ./internal/orchestrator -run '^TestExternalEvidenceRequiresRegisteredHashVerifiedQAArtifacts$' -count=1 -v
# PASS（3 子用例）, EXIT=0, 0.422s
go test ./internal/orchestrator -run '^TestAttachVerifierDoesNotClearUnresolvedBlock$' -count=1 -v
# PASS, EXIT=0, 0.404s
go test ./internal/orchestrator -run '^TestHardBlockedContinueResumesAfterCauseCleared$' -count=1 -v
# PASS, EXIT=0, 0.405s
go test ./internal/orchestrator -run '^(TestQAGateEvidenceFlow|TestReleaseGateComposition|TestRegisterArtifactQAGateFlow|TestQAStalenessUsesLatestPerPath|TestQALegacyDuplicatesNoMigration|TestQAReRegisterClearsStale)$' -count=1
# ok, EXIT=0, 0.642s
go test ./internal/cli -run '^TestHarnessVerifyJSON$' -count=1
# ok, EXIT=0, 0.574s
```

## 六、现场与未验证

- 只写本文件（`evidence/qa/spec-ac.md`），未 commit、未 push、未改实现与 truth；`main` ref 仍 `a3eab40`，主工作区停在 feature 分支（HEAD `16dd05f`），工作区干净。
- 为让本 verdict 保持在派单要求的路径，`/tmp/qa-spec-ee` 临时 worktree 暂未移除；取证完成后可由派单方清理。
- 未验证（不在本面）：运行时行为面（@QA 验收 · agy）、负向回归面（@QA 验收 · 回归）、全量 `go test ./... -count=1` 的独立复跑、合入 main、重建安装与发布说明。

**结论：spec/AC 面 PASS。** AC 判据未扩张，8/8 TC 独立实跑通过，4 个既有测试文件的断言未削弱；上述两条 observe 仅作收口建议，不构成返修项。
