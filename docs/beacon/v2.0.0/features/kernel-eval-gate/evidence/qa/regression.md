# QA Regression Verdict — kernel-eval-gate

- **mode_id**: `qa-regression`
- **verdict**: **PASS**
- **current_stage**: `qa / Freeze/Completed`
- **HEAD**: `e48bd05064af4dd31335d6455c8df290588d8c00` (`feat/kernel-eval-gate`)
- **base**: `b277d8c` (main at dispatch time)
- **run**: 只读独立验收，临时 worktree `/tmp/qa-reg-evalgate` 检出 `e48bd05`；未切分支、未 commit/push、未改实现
- **reviewed_at**: 2026-11-09 07:50 CST

## 1. 全量测试（自己实跑，不引用包内 go-test-all.txt）

命令：`go test ./... -count=1`（在 `e48bd05` 的干净检出上）

- **真实退出码：`EXIT=0`**
- 15 个测试包全部 `ok`，0 个 `FAIL`、0 个 `panic`；`cmd/beacon` 无测试文件
- 关键包耗时：`internal/evalgate 59.0s`、`internal/orchestrator 97.2s`、`internal/cli 22.7s`

包级结果：

```
ok  beacon-go/desktop                    2.303s
ok  beacon-go/internal/agentapi          7.923s
ok  beacon-go/internal/benchgate         2.471s
ok  beacon-go/internal/cli              22.744s
ok  beacon-go/internal/core              3.099s
ok  beacon-go/internal/evalgate         59.032s
ok  beacon-go/internal/gate              4.056s
ok  beacon-go/internal/harness          13.453s
ok  beacon-go/internal/llm               4.950s
ok  beacon-go/internal/migration         5.520s
ok  beacon-go/internal/orchestrator     97.219s
ok  beacon-go/internal/session           4.785s
ok  beacon-go/internal/store             4.555s
ok  beacon-go/internal/workspace         5.488s
ok  beacon-go/skills                     3.757s
```

## 2. 测试文件 diff（既有测试零改写）

`git diff b277d8c..e48bd05 --stat -- '*_test.go'`：

```
 internal/evalgate/fixture_test.go | 284 +++++++++++++
 internal/evalgate/suite_test.go   | 847 +++++++++++++++++++++++++++++++++++++++
 2 files changed, 1131 insertions(+)
```

结论：只有两个**新增**文件，0 删除、0 修改；既有包（orchestrator / cli / gate / store 等）的测试文件字节未变。

## 3. TC-013 复跑（四测试两包选择器）

命令：

```
go test ./internal/cli ./internal/orchestrator \
  -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision|TestDeliveryParadigmEndToEnd' \
  -count=1 -v
```

- **EXIT=0**，`=== RUN` 4 行，`--- PASS` 4 行，`--- FAIL` 0、`SKIP` 0
- `ok internal/cli 0.648s`、`ok internal/orchestrator 0.985s`

## 4. #1 能力未回退抽查（同一 HEAD 实跑）

```
go test ./internal/orchestrator ./internal/cli \
  -run 'TestConformanceTraceability|TestGoalConformanceTraceabilityExitCode' -count=1 -v
```

EXIT=0，6/6 PASS：

| 测试 | 覆盖 |
|---|---|
| `TestConformanceTraceabilityAllCovered` | 好包 `ac_tc_evidence_coverage` pass |
| `TestConformanceTraceabilityMissingTC` | 缺 TC 非 0 + `traceability_missing_tc` |
| `TestConformanceTraceabilityMissingEvidence` | 证据不存在/越界 + `traceability_missing_evidence` |
| `TestConformanceTraceabilityFailClosed` | fail-closed 非 0 |
| `TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict` | 历史包 `not_required` 兼容 |
| `TestGoalConformanceTraceabilityExitCode` | CLI 好包 0 / 坏包非 0 |

## 5. 无新增 CLI 子命令 / 既有输出不变

`git diff b277d8c..e48bd05 --stat -- cmd internal/cli` → **空**。

整段改动 18 文件、+1755/−121，落在 `internal/evalgate/`（新增实现与测试）与 `kernel-eval-gate` docs 包；`cmd/` 零改动，因此没有新增子命令，既有命令输出路径未变。

## 6. truth 状态

- `beacon truth show --feature kernel-eval-gate`：`status: frozen`、`lifecycle: Freeze/Completed`
- `beacon truth drift --feature kernel-eval-gate`：`clean: true`、`body drift: false`
- `branch check=mismatch`（冻结记账在 main、实现在分支）为预期状态，合并后对齐

## 7. 残留与未验证

- 与首次验收相同的最没把握项：真实性依赖 `internal/evalgate` 用例本身，尚未在「已合并 main 的真实包 + 真实全局 store run」组合下跑过一次；建议合并、重装二进制后补一次真实 `beacon truth show` + 整集联跑。
- 验收期间主工作区有他人进行中的 `internal/stride/`、`internal/orchestrator/stride_control*.go` 与 `tick.go` 修改；本 verdict 一律在 `e48bd05` 的隔离 worktree 中判定，未触碰这些文件。
- 临时 worktree `/tmp/qa-reg-evalgate` 已清理。
