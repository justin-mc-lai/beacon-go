---
slug: gate-strict-integrity
version: v2.0.0
materials_status: current
---

# Tests: gate-strict-integrity

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/cli -run TestGateIntegrityStrictBlocksFrozenPackage -count=1` | 冻结包 + 缺 ac= 的 tasks.md，`gate integrity --strict` 返回 blocked、退出码非 0，reason code 为既有 `feature_package_validation_failed` |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestReleaseCandidateIntegrityBlocks -count=1` | 有 run 的候选包在 `ReleaseVersionCheck` 中 integrity item 为 fail（`IntegrityBlocked=true`） |
| AC-003 | TC-003 | behavior | `go test ./internal/cli -run TestGateIntegrityFrozenDefaultStaysAudit -count=1` | 同一冻结包不传 `--strict` 时 verdict 为 audit、退出码 0（AC-101 默认不回归） |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestHistoricalFrozenIntegrityAudits -count=1` | 无 run 的历史冻结包仍走 audit_only，integrity item 不因 audit 失败 |
| AC-005 | TC-005 | static | `go test ./internal/gate -run TestEvaluateReleaseHasNoIntegrityPrereq -count=1` | `EvaluateRelease` 的 check 清单不含 integrity 项，四门前置语义未变 |
