# Evidence Index: program-file-graph

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |

## Verification Log

Filled after implementation; every row is a real command run, never a plan.

## Verification Log — 2026-11-02

| AC | TC | Test | Result |
|----|----|------|--------|
| AC-001 | TC-001 | `go test ./internal/orchestrator -run TestLoadProgramGraphFileMissingKeys -count=1` | PASS — each missing key yields `missing:<key>` or `lakes[i]:missing:<key>` |
| AC-002 | TC-002 | `go test ./internal/orchestrator -run TestLoadProgramGraphFileGraphProblems -count=1` | PASS — self-dependency maps to `program_dependency_self_reference` |
| AC-003 | TC-003 | `go test ./internal/orchestrator -run TestLoadProgramGraphFileMissing -count=1` | PASS — nonexistent file yields `file_missing`, no panic |
| AC-004 | TC-004 | `go test ./internal/orchestrator -run TestProgramImportCarriesLakeFields -count=1` | PASS — parity_subset, status, scope_mode and lake goal survive the import |
| AC-005 | TC-005 | `go test ./internal/orchestrator -run TestProgramImportRefusesInvalidGraph -count=1` | PASS — invalid import leaves `program_plan_missing` (nothing written) |
| AC-006 | TC-006 | `go test ./internal/cli -run TestProgramValidateCLIBlockedExit -count=1` | PASS — valid graph returns nil, blocked graph returns an error and the binary exits 1 |
| AC-007 | TC-007 | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | PASS — plan, change, freeze, admit, route unchanged |

Real graph (agent-delivery-bus, superagent-router v0.1.5, 13 lakes):

```
valid
→ program: superagent-router (lakes=13)   exit 0

copy with scope_mode deleted
→ feature_graph_invalid: missing:scope_mode   exit 1

nonexistent path
→ file_missing   exit 1
```
