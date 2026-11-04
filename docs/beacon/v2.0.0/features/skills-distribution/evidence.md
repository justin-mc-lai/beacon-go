# Evidence Index: skills-distribution

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
| AC-001 | TestPluginManifestLists1Plus6 | PASS — plugin.json and marketplace.json exist; the skills list is exactly the 1+6 paths |
| AC-002 | TestVerifyHostReportsMissing | PASS — a partial install reports each missing beacon-go* name |
| AC-003 | TestSkillsVerifyReportOnlyAndStrict | PASS — default exits 0; --strict exits non-zero with skill_install_drift |
| AC-004 | TestVerifyIgnoresV1Names | PASS — v1 beacon-goal is not counted as installed or extra |

## Whole-repo verification (2026-11-04)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS — all packages green |
| vet | PASS |
| end-to-end chain | PASS — full chain completed, run_778ccdaee61d4135b31c64be done |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
