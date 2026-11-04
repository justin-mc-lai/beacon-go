# Evidence Index: release-policy-shadow

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
| AC-001 | TestCreateRunRejectsUnsupportedReleasePolicy | PASS — auto and unknown values refused with release_policy_unsupported; human/auto_shadow accepted |
| AC-002 | TestReleaseShadowRecordsVerdictButKeepsHumanGate | PASS — shadow verdict pass recorded; ReleaseHumanConfirmed stays false |
| AC-003 | TestReleaseShadowStillRequiresHumanComplete (same test) | PASS — Complete without human confirmation still rejected after a passing shadow |
| AC-004 | TestGoalReleaseShadowCLI | PASS — goal release-shadow outputs the verdict; status exposes release_policy and release_shadow |
| AC-005 | TestCreateRunRejectsUnsupportedReleasePolicy | PASS — auto is refused with an explicit "not implemented" message |

## Whole-repo verification (2026-11-04)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS — all packages green |
| vet | PASS |
| end-to-end chain | PASS — full chain completed, run_16cfa5e73b550888dc01d235 done |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
