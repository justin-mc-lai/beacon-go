---
slug: program-sea-complete
version: v2.0.0
materials_status: current
---

# Tests: program-sea-complete

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneSkipsAwaitingAndContinues -count=1` | 已到 gate 的湖被跳过，循环继续推进下一个湖 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneSkipsAwaitingAndContinues -count=1` | 全部湖到 gate 后返回 awaiting_release，不要求每湖确认 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneStopsOnBlocked -count=1` | 任一湖 blocked 时循环立即停，依赖湖不推进 |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | 一次 program complete 完成所有在 gate 的湖 |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | program_release_confirmed 事件含确认人与各湖 run id；湖的归因为 program:<name> |
| AC-006 | TC-006 | behavior | `go test ./internal/orchestrator -run TestProgramCompleteRefusesLakeNotAtGate -count=1` | 不在 gate 的湖导致 complete 拒绝，reason code 稳定 |
| AC-007 | TC-007 | behavior | `go test ./internal/orchestrator -run TestProgramCompleteSingleSignature -count=1` | 确认后 program_status=delivered，rollup 全部 completed |
| AC-008 | TC-008 | static | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | 未新增顶层命令，能力在 goal program 下 |
| AC-009 | TC-009 | behavior | `go test ./internal/orchestrator -run TestProgramDriveUntilDoneStartsDependentWhenDependencyIsAtGate -count=1` | 上游湖到 gate 未确认时，下游湖仍可启动 |
