# Evidence Index: truth-version-iteration

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
| AC-001 | TestTruthCreateGreenfieldDefaultsToV001 | PASS — greenfield auto-opens v0.0.1 and creates the package under it |
| AC-002 | TestTruthCreateUsesCurrentVersion | PASS — an existing project uses its current docs line |
| AC-003 | TestDocsVersionNormalizeAndUnregistered | PASS — X.Y.Z normalized; invalid and unregistered explicit versions refused |
| AC-004 | TestDocsVersionNormalizeAndUnregistered | PASS — runtime-shaped (x.y.z-go.N) versions are refused as docs versions |
| AC-005 | TestProjectsBumpPatch | PASS — v2.0.0 → v2.0.1, marked current |
| AC-006 | TestTruthSkillVersionFlow + full suite + e2e | PASS — skill documents v0.0.1/bump/dual-axis; e2e run_a023679bbc934e7e50fd3ee9 |

## Whole-repo verification (2026-11-05)

| Surface | Result |
|---------|--------|
| build | PASS |
| unit + integration suite | PASS |
| vet | PASS |
| end-to-end chain | PASS |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
