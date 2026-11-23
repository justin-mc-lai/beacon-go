---
slug: truth-version-iteration
version: v2.0.0
materials_status: current
---

# Tests: truth-version-iteration

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestTruthCreateGreenfieldDefaultsToV001 -count=1` | 绿项目首次建包自动用 v0.0.1 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestTruthCreateUsesCurrentVersion -count=1` | 有版本时默认 current；latest 用最大 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestDocsVersionNormalizeAndUnregistered -count=1` | 归一化/非法/未注册三种情况 |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestDocsVersionRejectsRuntimeShape -count=1` | 含 -go. 的版本被拒绝 |
| AC-005 | TC-005 | behavior | `go test ./internal/cli -run TestProjectsBumpPatch -count=1` | bump patch 从 v0.0.1 到 v0.0.2 并设为 current |
| AC-006 | TC-006 | static | `go test ./skills -run TestTruthSkillVersionFlow -count=1` | skill 写清 v0.0.1、projects bump、双轴 |
