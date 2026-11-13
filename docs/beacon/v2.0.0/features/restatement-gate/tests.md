---
slug: restatement-gate
version: v2.0.0
materials_status: current
---

# Tests: restatement-gate

所有 TC 用 `go build ./cmd/beacon` 产物驱动；e2e seed 各自 mktemp 隔离 store，不触碰 `~/.beacon` 与本仓冻结包。

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | unit | `go test ./internal/gate -run '^TestParseRestatement$' -v` | 解析四键；正文 hash 与独立 sha256 一致；`## 1`…`## 5` 全识别；`## 3.2` 计为 3 的子节并可被节号命中 |
| AC-002 | TC-002 | negative | `go test ./internal/orchestrator -run '^TestFreezeRequiresRestatement$' -v` | 夹具包无 restatement.md：freeze 非 0，block_reasons 含 `restatement_required`，包仍 draft |
| AC-002 | TC-003 | negative | `go test ./internal/orchestrator -run '^TestFreezeRejectsUnconfirmedRestatement$' -v` | 文件在但 `confirmed_by` 空：非 0，`restatement_unconfirmed` |
| AC-002 | TC-004 | negative | `go test ./internal/orchestrator -run '^TestFreezeRejectsStaleRestatement$' -v` | 确认后正文改一个字：非 0，`restatement_stale` |
| AC-002 | TC-005 | negative | `go test ./internal/orchestrator -run '^TestFreezeRejectsMissingSections$' -v` | 删掉 `## 4 边界与不做`：非 0，`restatement_sections_missing` |
| AC-002 | TC-006 | negative | `go test ./internal/orchestrator -run '^TestFreezeOrderSurfaceBeforeRestatement$' -v` | surface 与 restatement 同时非法：先报 `truth_surface_required`（插入顺序断言） |
| AC-004 | TC-007 | negative | `go test ./internal/orchestrator -run '^TestFreezeRejectsUntraceableAC$' -v` | AC 写 `restates=9`：非 0，`ac_not_traceable_to_restatement` |
| AC-003 | TC-008 | behavior | `go test ./internal/orchestrator -run '^TestFreezeAcceptsConfirmedRestatement$' -v` | 五段齐全 + confirmed + hash 一致：freeze 退出 0，status=frozen |
| AC-003 | TC-009 | behavior | `go test ./internal/orchestrator -run '^TestRefreezeLegacyWaiverWithoutRestatement$' -v` | 已冻结、无 restatement 的老包 refreeze 退出 0，只出 warning，不新增 blocked |
| AC-005 | TC-010 | behavior | `go test ./internal/orchestrator -run '^TestTruthShowRestatementBlock$' -v` | `truth show --json` 的 `restatement` 块含 path/confirmed_by/confirmed_at/hash/matches；老包无该块 |
| AC-006 | TC-011 | unit | `go test ./internal/core -run '^TestRestatementReasonCodes$' -v` | 五个常量值与契约字符串逐字一致 |
| AC-007 | TC-012 | full | `go test ./... 2>&1 | tee .beacon/evidence/restatement-gate/impl/go-test-all.log` | 退出 0；尾部 `ok` 覆盖全部包；既有测试零回归 |
| AC-008 | TC-013 | review | `git status --short && git log --oneline -1 && git rev-parse --abbrev-ref @{u} 2>/dev/null || true` | 仅新增文件与最小插入点；commit 在本地；无 push、未重装 |
| AC-007 | TC-014 | e2e | `env BEACON_BIN=/tmp/beacon-restatement zsh docs/beacon/v2.0.0/features/restatement-gate/seeds/seed_restatement_freeze.sh` | 用真实二进制跑 8 个场景；落 `qa/freeze-blocked.log`、`qa/freeze-pass.log` 完整 freeze 输出 |

## Red 基线

以 `4dc8c57` 构建的二进制为基线：TC-002..TC-009 中的 refused 用例必红（当前没有这道门，freeze 直接通过），TC-011 必红（常量不存在）；TC-001/TC-010 为新增面。
