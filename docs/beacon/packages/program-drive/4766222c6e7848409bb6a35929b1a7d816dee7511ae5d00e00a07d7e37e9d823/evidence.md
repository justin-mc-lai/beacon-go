# Evidence Index: program-drive

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| plan | support_advisory | tasks.md | present | truth |

## Verification Log

Every row is a real command run on 2026-11-04; nothing below is a plan.

| AC | Test selector | Result |
|----|---------------|--------|
| AC-001 | TestProgramDriveReturnsAwaitingRelease | PASS — one call advances the ready lake to its release gate |
| AC-002 | TestProgramDriveReusesBoundLakeRun | PASS — the bound run is reused; no duplicate run is created |
| AC-003 | TestProgramDriveReturnsAwaitingRelease | PASS — lake status is awaiting_release; no auto-complete |
| AC-004 | TestProgramDriveBlockedKeepsDependentsPending | PASS — failed lake records blocked + reason; dependent does not advance |
| AC-005 | TestProgramRouteRollupAndBinding | PASS — route exposes run_id, stage, rollup and program_status |
| AC-006 | TestProgramDriveReusesBoundLakeRun | PASS — completed/bound work is not re-executed |
| AC-007 | TestProgramDriveDeliveredWhenAllComplete | PASS — all complete yields program_status delivered |
| AC-008 | TestProgramExistingSubcommandsIntact | PASS — no new top-level command; loop pointer unchanged |

## Whole-repo verification (2026-11-04)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS — all packages green |
| vet | PASS |
| end-to-end chain | PASS — full chain completed, run_363e52111752dbd41a749585 done |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
