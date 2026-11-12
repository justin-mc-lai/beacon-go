---
slug: kernel-goal-advise
version: v2.0.0
materials_status: current
---

# Tests: kernel-goal-advise

三条 seed 各自 mktemp 隔离 store，用 `go build ./cmd/beacon` 产物驱动。

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `env BEACON_BIN=/tmp/beacon-advise zsh docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed1_blocked_ranks_first.sh` | 首项是 blocked-feature 且 score 大于等于 100 |
| AC-002 | TC-002 | behavior | `env BEACON_BIN=/tmp/beacon-advise zsh docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed2_deterministic.sh` | 两次输出逐字节相等 |
| AC-003 | TC-003 | negative | `env BEACON_BIN=/tmp/beacon-advise zsh docs/beacon/v2.0.0/features/kernel-goal-advise/seeds/seed3_no_invented_facts.sh` | schema 与版本正确；每个 feature 与 blocker 都能在 spine 里找到 |

## Red 基线

以已安装的旧二进制（无 advise 子命令）为基线，三条 seed 必红，原始输出见 evidence/red-baseline.log。
