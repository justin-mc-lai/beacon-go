# Evidence Index: kernel-truth-change-cancel

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/truth.md | frozen | truth |
| tests | test_truth | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/tests.md | frozen | qa |
| tasks | support_advisory | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/tasks.md | present | truth |
| seed 1 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed1_cancel_identical_returns_frozen.sh | FAIL（unknown flag --cancel） | qa |
| seed 2 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed2_cancel_modified_refuses.sh | FAIL（unknown flag --cancel） | qa |
| seed 3 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-truth-change-cancel/seeds/seed3_cancel_no_pending_idempotent.sh | FAIL（unknown flag --cancel） | qa |

## Reproduction Log

2026-11-11，beacon-go HEAD `199e875`，`go build -o /tmp/beacon-seed ./cmd/beacon` 产物，`BEACON_BIN` 传入，三条 seed 各自 mktemp 隔离 store：

| AC | 命令 | 退出 | 观察 |
|----|------|------|------|
| AC-001 | seed1_cancel_identical_returns_frozen.sh | 1 | `Error: unknown flag: --cancel`，包停在 change_pending |
| AC-002 | seed2_cancel_modified_refuses.sh | 1 | 同上；拒绝文案未指向 refreeze |
| AC-003 | seed3_cancel_no_pending_idempotent.sh | 1 | 同上；无挂起时的幂等路径不存在 |

## Boundary

本包 plan 轮只落文档与 seed，未改生产代码；实现轮只允许改 internal/**，不得改 seed 断言。
