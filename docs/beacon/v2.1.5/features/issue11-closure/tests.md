---
slug: issue11-closure
version: v2.1.5
materials_status: current
---

# Tests: issue11-closure

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test -json ./internal/orchestrator ./internal/cli -run TestSDDDecompose -count=1` | SDD decompose preserves contract and refuses fake readiness |
| AC-002 | TC-002 | behavior | `go test -json ./internal/orchestrator -run TestSDDProposal -count=1` | content proposal validation and SDD selection |
| AC-003 | TC-003 | behavior | `go test -json ./internal/orchestrator ./internal/cli -run 'TestSDDMigr|TestMigrationReadStore' -count=1` | migration preserves commands and SQLite committed snapshot, refuses conflict |
| AC-004 | TC-004 | behavior | `go test -json ./internal/cli ./internal/orchestrator -run 'TestSDDClosureCLI|TestSDDProgram|TestSDDDecomposeFreezeNormalizes' -count=1` | entry success/refusal and legitimate governed refreeze |
