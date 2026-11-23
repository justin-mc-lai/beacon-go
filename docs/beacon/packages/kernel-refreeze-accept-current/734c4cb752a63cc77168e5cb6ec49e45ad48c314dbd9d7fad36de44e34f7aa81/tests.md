---
slug: kernel-refreeze-accept-current
version: v2.0.0
materials_status: current
---

# Tests: kernel-refreeze-accept-current

三条 seed 全部由 `go build ./cmd/beacon` 产物驱动，各自 mktemp 隔离 store，不触碰 `~/.beacon` 与本仓 `.beacon`。

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed1_accept_current_heals_drift.sh` | plain refreeze 非 0 且含 revision_authority_drift、包仍 change_pending；--accept-current 后退出 0、status=frozen、pending 为空、truth_refrozen 事件 ≥1 |
| AC-002 | TC-002 | negative | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed2_accept_current_requires_pending.sh` | 无挂起时非 0 退出、提示 no pending change、status 仍 frozen、freeze 事件计数不变 |
| AC-003 | TC-003 | behavior | `env BEACON_BIN=/tmp/beacon-seed zsh docs/beacon/v2.0.0/features/kernel-refreeze-accept-current/seeds/seed3_strict_refreeze_unchanged.sh` | 正文未移动时 plain refreeze 退出 0、status=frozen、pending 为空（不回退） |

## Red 基线

以 `cb7805f`（未含 --accept-current）构建的二进制为基线：TC-001/TC-002 必红（unknown flag / 无法收口），TC-003 为绿色守卫。原始输出见 evidence/red-baseline.log。
