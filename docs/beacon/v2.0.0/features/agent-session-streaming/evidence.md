# Evidence Index: agent-session-streaming

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |

## Verification Log

Every row was executed by the whole Go test suite; the command is the exact
`-run` selector. No row is a hand-written hash.

| AC | Test | Result |
|----|------|--------|
| AC-001 | `TestSessionStore_CRUD` | PASS |
| AC-002 | `TestStreamCompleteChunks` | PASS |
| AC-003 | `TestShellSessionAndStreamWiring` | PASS |
| AC-004 | `TestAppSessionAPIs` | PASS |

Whole-suite state: the full test suite passes, go vet is clean,
gofmt reports nothing.

## 2026-10-29 复核

冻结后复核：证据表引用的测试均在当前仓库存在并通过；材料状态随冻结归位 current。
