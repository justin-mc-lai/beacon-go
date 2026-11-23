---
slug: lifecycle-monotonic
version: v2.0.0
materials_status: current
---

# Tests: lifecycle-monotonic

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestCreateRunDoesNotDowngradeCompletedFeature -count=1` | 已完成 feature 建新 run 后仍为 Release/Completed |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestFreezeDoesNotDowngradeCompletedFeature -count=1` | 已完成 feature 再 freeze 仍为 Release/Completed |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestUpdateLifecycleMonotonicBlocksDowngrade -count=1` | 已完成 feature 的 Freeze/InProgress/Blocked 写入被跳过 |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestUpdateLifecycleMonotonicNonCompletedStillUpdates -count=1` | 未完成 feature 的写入照常生效 |
| AC-005 | TC-005 | static | `go test ./internal/orchestrator -run TestLifecycleGuardIsShared -count=1` | CreateRun/freeze/tick 都调用同一守卫 |
| AC-006 | TC-006 | behavior | `go test ./... -count=1` + `bash e2e/run_e2e.sh` | 全仓与端到端通过 |
