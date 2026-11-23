---
slug: kernel-recovery-qa-bridge
version: v2.0.0
materials_status: current
---

# Tests: kernel-recovery-qa-bridge

## 复跑方式

```sh
cd /Users/apple/Developer/Personal/products/beacon-go
go build -o /tmp/beacon-seed ./cmd/beacon
BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh; echo exit=$?
BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh; echo exit=$?
BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh; echo exit=$?
```

每条 seed 自建独立 store 与临时项目；在 HEAD `afd5cc7` 上三条都必须 `exit=1`。

## Seed 契约（当前必然 FAIL）

| TC ID | AC | 命令 | 当前期望失败现象 | canon sha256（对 seed 打印的 canon 文件取哈希） |
|-------|----|------|------------------|---------------------------------------------------|
| TC-001 | AC-001 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh` | `after_recover=Implement/Blocked`，下一次 tick `reason_code=stage_exit_implement_blocked`，exit=1 | `06828a808db5165abb730701eb98755f0ea0c360b4092e3998b3d4b1e1d9269b` |
| TC-002 | AC-002 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh` | `autotest_passed=1/1` 但 `qa_verdict=blocked`、`qa_reason_codes=qa_stage_not_executed`、`qa_stage_executed_count=0`，exit=1 | `824a353fd733a728720002885817f1e496e0e207745e03097001696ca064b648` |
| TC-003 | AC-003 | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh` | `run_a_qa_verdict=pass` 后，零 QA 证据的 run B `run_b_stage=release, run_b_blocked=False`，exit=1 | `32f60684271289ca9fe9f5c1388f0b105b1a1afed30f070b48d10adbe069aaac` |

## 修复后行为契约

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|--------------------|-----------|
| AC-001 | TC-001 | behavior | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed1_recover_lifecycle.sh` | exit=0；`after_recover` 不再含 `/Blocked`；tick 推进到 qa 且 `reason_code` 不是 `stage_exit_implement_blocked` |
| AC-002 | TC-002 | behavior | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed2_autotest_stage_executed.sh` | exit=0；`qa_verdict=pass`，`qa_stage_executed=True`，真实 qa `stage_executed` 计数 ≥1 |
| AC-003 | TC-003 | behavior | `BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-recovery-qa-bridge/seeds/seed3_cross_run_qa_verdict.sh` | exit=0；run B 无自有 QA 证据时停在 `qa`（或被 qa gate 以 `qa_stage_not_executed` 拦下），不得进 release |
| AC-004 | TC-004 | negative | `go test ./internal/orchestrator -run TestKernelRecoveryNegativeGuards -count=1`（实现阶段新增） | human gate reason 的 recover 仍返回 `recovery_human_gate_forbidden` 且快照不变；executor-marker=true / recording=true / 缺 marker 的 stage_executed 均不计 qa_stage_executed |
| AC-005 | TC-005 | static | `go test ./internal/orchestrator ./internal/gate -run TestKernelRecoveryContractStable -count=1`（实现阶段新增） | reason code 词表未新增/改名；有本 run formal qa verdict 时仍由 verdict 决定，derived 仅兜底 |

## 证据文件 sha256

| 文件 | sha256 |
|------|--------|
| `seeds/lib.sh` | `2c6997bcf2212bb72f87095951a507f4e4544daae3b9384504e5815df71ae606` |
| `seeds/seed1_recover_lifecycle.sh` | `a2476a47b08c24c24d2c6f6210501b535a6d1169b8e949df56812899869e680a` |
| `seeds/seed2_autotest_stage_executed.sh` | `e8285ff425202fc6aeec3fc965510659b0c9f227c5700eedffc946a1b1847c1b` |
| `seeds/seed3_cross_run_qa_verdict.sh` | `744f187e55ad5b914c5158af15bac5a26acf64f92dffe792755b6f9d64eb0ce9` |
| `evidence/seed1-red.txt` | `7b4fc7f303a0b03f37becfc30f49c2844457a8d4bf8ab90e24ee91f6214ef87f` |
| `evidence/seed2-red.txt` | `e5f78907f994d6b9ef7466a140033e41cd70ca67e2c7d1ef80ffe8b51f0f5935` |
| `evidence/seed3-red.txt` | `6051a48656755daad6662f60b785da0037d2535320c6ded199176c1cb7a7dda8` |
