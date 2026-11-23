# Evidence Index: p1-skill-router-acp

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |

## Verification Log

Machine-checked, executed by the whole Go test suite (selector shown):

| AC | Test | Result |
|----|------|--------|
| AC-001 | `TestRouterTableSchema` | PASS |
| AC-002 | `TestRouterMatchesEverySample` | PASS |
| AC-003 | `TestRouterRefusesToGuess` | PASS |
| AC-004 | `TestProgressiveMapOrder` | PASS |
| AC-005 | `TestProgressiveMapPathsExist` | PASS |
| AC-006 | `TestAcpSessionDrivesTheProtocolCorrectly` + `TestAcpIterateMapsSessionOntoResult` | PASS (protocol-level with a scripted ACP agent; a real ACP target stays external) |
| AC-007 | `TestAcpRefusesWithoutTarget` | PASS |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields.
