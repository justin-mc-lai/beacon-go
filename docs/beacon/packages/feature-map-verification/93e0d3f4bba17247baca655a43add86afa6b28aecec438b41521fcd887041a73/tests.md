---
slug: feature-map-verification
version: v2.0.0
materials_status: current
---

# Tests: feature-map-verification

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | 每个 AC 一行；AC-001 为 mapped，入口点来自 tasks，命令来自 tests |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | 生成前后 truth.md 字节一致 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | 无命令的 AC-002 标 `missing_verification` |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | 构建后 verify 为 fresh；改 tests.md 后为 stale 且 blocked |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | 无任务的 AC-003 标 `missing_entry` |
