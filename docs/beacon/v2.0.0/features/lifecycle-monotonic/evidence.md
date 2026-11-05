# Evidence Index: lifecycle-monotonic

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |
| plan | support_advisory | tasks.md | present | truth |

## Verification Log

Every row is a real command run on 2026-11-05; nothing below is a plan.

| AC | Test selector | Result |
|----|---------------|--------|
| AC-001 | TestCreateRunDoesNotDowngradeCompletedFeature | PASS — a new run leaves a released feature at Release/Completed |
| AC-002 | TestFreezeDoesNotDowngradeCompletedFeature | PASS — freeze on a released feature keeps Release/Completed |
| AC-003 | TestUpdateLifecycleMonotonicBlocksDowngrade | PASS — Freeze/InProgress/Blocked writes are skipped for a released feature |
| AC-004 | TestUpdateLifecycleMonotonicNonCompletedStillUpdates | PASS — non-completed features still advance normally |
| AC-005 | TestLifecycleGuardIsShared | PASS — engine.go / truth.go / tick.go all call updateLifecycleMonotonic |
| AC-006 | full suite + e2e | PASS — all packages green; e2e run_7f73837888b5bd1d5f6cbddf done |

## Whole-repo verification (2026-11-05)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS |
| vet | PASS |
| end-to-end chain | PASS |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
