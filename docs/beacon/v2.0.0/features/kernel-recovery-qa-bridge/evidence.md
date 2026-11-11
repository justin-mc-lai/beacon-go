# Evidence Index: kernel-recovery-qa-bridge

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| plan | support_advisory | tasks.md | present | truth |
| red evidence | support_advisory | docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh | present | plan |

本包是 plan 包：下面是 2026-11-11 在 HEAD `afd5cc7` 上的真实复现，不是计划，也不含实现 verdict。

## 复现环境

| 项 | 值 |
|----|----|
| 仓库 | `/Users/apple/Developer/Personal/products/beacon-go` |
| 分支 / HEAD | `main` / `afd5cc7` |
| 构建 | `go build -o /tmp/beacon-seed ./cmd/beacon` |
| 二进制版本 | `Beacon CLI v2.0.0-go.15` |
| 隔离 | 每条 seed 用 `mktemp -d` 建项目与 `BEACON_STORE_PATH`，不碰全局 store 与本仓库的 .beacon 目录 |

## Red Verification Log

| AC | TC | Command | Exit | Observed (canon) | canon sha256 |
|----|----|---------|------|------------------|--------------|
| AC-001 | TC-001 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh` | 1 | `after_recover=Implement/Blocked`, `tick_reason=stage_exit_implement_blocked` | `06828a808db5165abb730701eb98755f0ea0c360b4092e3998b3d4b1e1d9269b` |
| AC-002 | TC-002 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh` | 1 | `autotest_passed=1/1`, `qa_verdict=blocked`, `qa_stage_executed_count=0` | `824a353fd733a728720002885817f1e496e0e207745e03097001696ca064b648` |
| AC-003 | TC-003 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh` | 1 | `run_a_qa_verdict=pass`, `run_b_stage=release`, `run_b_blocked=False` | `32f60684271289ca9fe9f5c1388f0b105b1a1afed30f070b48d10adbe069aaac` |

sha256 的取哈希对象是每条 seed 打印的 `canon` 文件（只含确定性的 stage/status/verdict 字段），不是终端全文；三次复跑结果一致。终端全文分别在 docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/evidence/seed1-red.txt 、 docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/evidence/seed2-red.txt 、 docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/evidence/seed3-red.txt 三个文件里。

## 实测输出节选

### seed1

```text
after_block=Implement/Blocked
after_recover=Implement/Blocked
tick_stage=implement
tick_blocked=True
tick_reason=stage_exit_implement_blocked
sha256=06828a808db5165abb730701eb98755f0ea0c360b4092e3998b3d4b1e1d9269b
RESULT: FAIL - recover cleared the run but lifecycle stayed Implement/Blocked
```

### seed2

```text
autotest_total=1
autotest_passed=1
autotest_failed=0
qa_verdict=blocked
qa_reason_codes=qa_stage_not_executed
qa_checks=qa_artifacts_present=True,qa_stage_executed=False,qa_artifact_refs_valid=True
qa_stage_executed_count=0
sha256=824a353fd733a728720002885817f1e496e0e207745e03097001696ca064b648
RESULT: FAIL - autotest passed 1/1 but the qa gate still reports qa_stage_not_executed
```

### seed3

```text
run_a_qa_verdict=pass
run_b_stage=release
run_b_blocked=False
run_b_reason=None
run_b_qa_stage_executed_before_exit_tick=0
sha256=32f60684271289ca9fe9f5c1388f0b105b1a1afed30f070b48d10adbe069aaac
RESULT: FAIL - run B reached release without its own qa evidence, borrowing run A's verdict
```

## 代码定位（只读核对）

| 缺口 | 位置 | 现状 |
|------|------|------|
| A | `internal/orchestrator/tick.go` `applyBlockOn` | 写 `feature_lifecycle=Blocked` |
| A | `internal/orchestrator/complete.go` `Engine.Recover` | 只清 run 字段，不回写 lifecycle |
| A | `internal/orchestrator/exitgates.go` | `Implement+Blocked` 快照直接返回 `ReasonStageExitImplementBlk` |
| B | `internal/orchestrator/autotest.go` | 只写 `autotest_executed` |
| B | `internal/orchestrator/gates_facts.go` | 只收 placeholder=false 且 recording 非 true 的 `stage_executed` |
| B | `internal/gate/evaluate.go` | `EvaluateQA` 用 `QAStageExecuted` 判定 `qa_stage_executed` |
| C | `internal/orchestrator/exitgates.go` `gateFactOf` | run 级优先、feature 级回落 |
| C | `internal/orchestrator/exitgates.go` `featureGateFallback` | 只排除被兄弟 stage-exit 引用的 gate 行 |

## 本包未做什么

- 未修改任何 `*.go`（`git status --short -- '*.go'` 为空）。
- 未运行 `truth freeze`，未写 intent 签名，`intent_confirmed: false`。
- 未 push、未发布、未重建安装 `~/.local/opt/beacon-go/beacon`。
- 未对三条缺口给出 pass/blocked 的最终验收 verdict；修复后的绿由 implement 与独立 QA 判定。
