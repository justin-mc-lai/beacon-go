---
slug: program-sea-complete
version: v2.0.0
generated_at: 2026-11-04T23:54:53.683303Z
surface_fingerprint: ed9b9a9efe5ee94356963cf2473e7be0a12994b02708a8017f89edb3e2540e39
---

# Feature Map: program-sea-complete

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneSkipsAwaitingAndContinues -count=1` | mapped |
| AC-002 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneSkipsAwaitingAndContinues -count=1` | mapped |
| AC-003 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneStopsOnBlocked -count=1` | mapped |
| AC-004 | internal/orchestrator/program_complete.go | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | mapped |
| AC-005 | internal/orchestrator/program_complete.go | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | mapped |
| AC-006 | internal/orchestrator/program_complete.go | `go test ./internal/orchestrator -run TestProgramCompleteRefusesLakeNotAtGate -count=1` | mapped |
| AC-007 | internal/orchestrator/program_route.go | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | mapped |
| AC-008 | internal/cli/goal_program.go | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | mapped |
| AC-009 | internal/orchestrator/program_drive.go | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneStartsDependentWhenDependencyIsAtGate -count=1` | mapped |
