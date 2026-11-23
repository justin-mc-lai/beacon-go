# Evidence Index: program-sea-complete

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
| AC-001 | TestProgramDriveUntilDoneStartsDependentWhenDependencyIsAtGate | PASS — a dependency at its gate unlocks the dependent lake |
| AC-002 | TestProgramDriveUntilDoneSkipsAwaitingAndContinues | PASS — all lakes at gate returns awaiting_release without advancing |
| AC-003 | TestProgramDriveUntilDoneStopsOnBlocked | PASS — a blocked lake stops the sea with a reason |
| AC-004 | TestProgramCompleteSingleSignature | PASS — one program complete closes every lake at its gate |
| AC-005 | TestProgramCompleteSingleSignature | PASS — program_release_confirmed written; lake confirmations attributed to program:<name> |
| AC-006 | TestProgramCompleteRefusesLakeNotAtGate | PASS — a lake not at its gate refuses the whole-sea confirmation |
| AC-007 | TestProgramCompleteSingleSignature | PASS — delivered with rollup completed=2 |
| AC-008 | TestProgramExistingSubcommandsIntact | PASS — no new top-level command |
| AC-009 | TestProgramDriveUntilDoneStartsDependentWhenDependencyIsAtGate | PASS — release-ready closure is enforced |

## Whole-repo verification (2026-11-05)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS — all packages green |
| vet | PASS |
| end-to-end chain | PASS — full chain completed, run_4b0f2d90c682b754d58285c6 done |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
