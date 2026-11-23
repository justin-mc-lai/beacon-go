# Evidence Index: agent-desktop-workspace

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
| AC-001 | `TestShellScriptParses` | PASS |
| AC-002 | `TestUnregisteredProjectIsEmptyNotInvented` | PASS |
| AC-003 | `TestWailsConfigValid` | PASS |

Whole-suite state: the full test suite passes, go vet is clean,
gofmt reports nothing.

## 2026-10-29 复核

冻结后复核：证据表引用的测试均在当前仓库存在并通过；材料状态随冻结归位 current。
