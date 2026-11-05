---
slug: program-drive
version: v2.0.0
generated_at: 2026-11-04T23:54:33.068991Z
surface_fingerprint: 1bc52a47b843d919a8b38acfa21624f72a822fef67f49f28f91577d8d1da11aa
---

# Feature Map: program-drive

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveReturnsAwaitingRelease -count=1` | mapped |
| AC-002 | internal/orchestrator/meta_p2d.go | `go test ./internal/orchestrator -run TestProgramDriveReusesBoundLakeRun -count=1` | mapped |
| AC-003 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveReturnsAwaitingRelease -count=1` | mapped |
| AC-004 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveBlockedKeepsDependentsPending -count=1` | mapped |
| AC-005 | internal/orchestrator/program_route.go | `go test ./internal/orchestrator -run TestProgramRouteRollupAndBinding -count=1` | mapped |
| AC-006 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveReusesBoundLakeRun -count=1` | mapped |
| AC-007 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveDeliveredWhenAllComplete -count=1` | mapped |
| AC-008 | internal/cli/goal_program.go | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | mapped |
