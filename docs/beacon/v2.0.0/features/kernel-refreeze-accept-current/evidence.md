# Evidence Index: kernel-refreeze-accept-current

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/truth.md | frozen | truth |
| tests | test_truth | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/tests.md | frozen | qa |
| tasks | support_advisory | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/tasks.md | present | truth |
| seed 1 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed1_accept_current_heals_drift.sh | red on cb7805f, green on HEAD | qa |
| seed 2 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed2_accept_current_requires_pending.sh | red on cb7805f, green on HEAD | qa |
| seed 3 | acceptance_truth | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed3_strict_refreeze_unchanged.sh | green guard, no regression | qa |
| red log | support_advisory | docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/evidence/red-baseline.log | recorded | qa |

## Reproduction Log

2026-11-11，基线二进制由 `git worktree` 检出 `cb7805f` 后 `go build -o /tmp/beacon-baseline ./cmd/beacon`；实现对 `a05ca30` 起的 HEAD 构建 `/tmp/beacon-seed`。

| AC | 基线（cb7805f） | 当前 HEAD |
|----|-----------------|-----------|
| AC-001 | FAIL：`unknown flag: --accept-current`，包停在 change_pending | PASS：先拒后收口到 frozen |
| AC-002 | FAIL：`unknown flag: --accept-current` | PASS：无挂起时拒绝且不写事件 |
| AC-003 | PASS（guard） | PASS（guard） |
