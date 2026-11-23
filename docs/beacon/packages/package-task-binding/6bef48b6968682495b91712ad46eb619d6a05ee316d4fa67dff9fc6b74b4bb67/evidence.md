# Evidence Index: package-task-binding

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |

## Verification Log

Filled after implementation; every row is a real command run, never a plan.

## Verification Log — 2026-11-02

All rows are real command runs on this machine.

| AC | TC | Test | Result |
|----|----|------|--------|
| AC-001 | TC-001 | `go test ./internal/gate -run TestValidateTaskBindingMissingAC -count=1` | PASS — unchecked row without ac= yields `task_binding_missing: TASK-001 missing ac= binding` |
| AC-002 | TC-002 | `go test ./internal/gate -run TestValidateTaskBindingMissingEvidence -count=1` | PASS — checked row without evidence= is flagged; unchecked row is not |
| AC-003 | TC-003 | `go test ./internal/gate -run TestValidateTaskBindingEvidenceFile -count=1` | PASS — path-shaped evidence missing yields `checkbox_checked_without_evidence_file`; identifier-shaped is accepted |
| AC-004 | TC-004 | `go test ./internal/gate -run TestValidateTaskBindingUncheckedNoEvidenceOK -count=1` | PASS — unchecked row with ac= stays clean |
| AC-005 | TC-005 | `go test ./internal/gate -run TestValidateTaskBindingKeepsExistingVerdicts -count=1` | PASS — fully bound ledger adds no findings |
| AC-006 | TC-006 | `go test ./internal/gate -run TestHostPackagesTaskBindingsClean -count=1` | PASS — the four migrated frozen packages report no task-binding findings |
| AC-007 | TC-007 | `go test ./internal/cli -run TestProgramExistingSubcommandsIntact -count=1` | PASS — no validate-package subcommand exists; diagnostics stay on `gate integrity` |

Migration actually applied (frozen packages, via change then refreeze):

```
agent-desktop-workspace   3 rows + ac=      (AC-001..003 already named in the title)
agent-runtime-dual-mode   6 rows + ac=      (AC-001..006)
agent-session-streaming   4 rows + ac=      (AC-001..004)
p0-executor-desktop       7 rows + evidence= (test-function identifiers / gate:truth_freeze)
```

Post-migration check: `beacon-go gate integrity` on each of the four → verdict pass, 0 findings, 0 task-binding findings. Full suite: every package ok.
