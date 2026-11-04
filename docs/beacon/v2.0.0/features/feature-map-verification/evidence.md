# Evidence Index: feature-map-verification

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
| AC-001 | TestBuildFeatureMapBindsACs | PASS — AC-001 mapped with the entry point from tasks and the command from tests |
| AC-002 | TestBuildFeatureMapBindsACs | PASS — truth.md is byte-for-byte unchanged after build |
| AC-003 | TestBuildFeatureMapBindsACs | PASS — AC-002 has no test command and is marked missing_verification |
| AC-004 | TestBuildFeatureMapBindsACs | PASS — verify reports fresh after build; stale + blocked after tests.md changes |
| AC-005 | TestBuildFeatureMapBindsACs | PASS — AC-003 has no task evidence and is marked missing_entry |

## Whole-repo verification (2026-11-04)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS — all packages green |
| vet | PASS |
| end-to-end chain | PASS — full chain completed, run_88ace42512b20383639ba16f done |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
