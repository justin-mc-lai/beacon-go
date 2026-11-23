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
| AC-003 | TC-003 | behavior | `go test -json ./internal/orchestrator ./internal/cli -run TestSDDMigrate -count=1` | migration preserves original commands and refuses conflict |
| AC-004 | TC-004 | behavior | `go test -json ./internal/cli -run TestSDDClosureCLI -count=1` | real command success and refusal without issue false closure |
