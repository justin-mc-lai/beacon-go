---
slug: program-drive
version: v2.0.0
materials_status: current
---

# Tests: program-drive

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestProgramDriveReturnsAwaitingRelease -count=1` | 单次 drive 把一个就绪湖推到 release gate |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestProgramDriveReusesBoundLakeRun -count=1` | 已绑定的 run 被复用，不重复创建 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestProgramDriveReturnsAwaitingRelease -count=1` | 湖状态为 awaiting_release，不自动 complete |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestProgramDriveBlockedKeepsDependentsPending -count=1` | 失败湖记 blocked + reason；依赖湖不推进 |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator -run TestProgramRouteRollupAndBinding -count=1` | route 暴露 run_id/stage/rollup/program_status |
| AC-006 | TC-006 | behavior | `go test ./internal/orchestrator -run TestProgramDriveReusesBoundLakeRun -count=1` | 可恢复且不重复执行 |
| AC-007 | TC-007 | behavior | `go test ./internal/orchestrator -run TestProgramDriveDeliveredWhenAllComplete -count=1` | 全部完成时 program 为 delivered |
| AC-008 | TC-008 | static | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | 未新增顶层命令，loop 文案不变 |
