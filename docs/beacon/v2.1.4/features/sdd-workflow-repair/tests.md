---
slug: sdd-workflow-repair
version: v2.1.4
materials_status: current
---

# Tests: sdd-workflow-repair

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestSDD -count=1` | scaffold refusal and meaningful stateless coverage |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run 'TestSDDPackageContract|TestSDDTraceability' -count=1` | two-file consumers and runtime traceability default |
| AC-003 | TC-003 | behavior | `go test ./internal/cli ./internal/gate -run 'TestSDDFSM|TestFSM' -count=1` | normalized transitions and guard refusal |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestSDDReview -count=1` | independent current review and stale confirmation refusal |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator ./internal/gate -run TestTaskDAG -count=1` | dependencies execute in order and invalid graphs refuse |
| AC-006 | TC-006 | behavior | `go test ./internal/orchestrator -run TestTaskRepair -count=1` | bounded real repair captures every failed attempt |
