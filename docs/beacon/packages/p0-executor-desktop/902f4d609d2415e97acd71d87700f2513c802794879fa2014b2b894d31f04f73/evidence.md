# Evidence Index: p0-executor-desktop

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | frozen | truth |
| tests | test_truth | tests.md | frozen | qa |

## Verification Log — 2026-10-28

Machine-checked ACs. Every row was executed; none was skipped.

| AC | TC | Test | Result |
|----|----|---------|--------|
| AC-001 | TC-001 | `TestBuiltinProfileFlagsExistInRealCLI` | PASS — the real `codex exec --help` and `claude -p --help` are consulted for every switch; `--context-file` no longer appears |
| AC-004 | TC-004 | `TestProfilePromptCarriesContextPath` | PASS — the resolved argv carries the stage-context absolute path |
| AC-005 | TC-005 | `TestProfileBackendFailureSurfaces` | PASS — missing binary, exit 3 and timeout each surface a reason code; none fabricates success |
| AC-006 | TC-006 | `TestAPIRunAdvanceAndExecutorConfig` | PASS — Executors / UseExecutor / AdvanceRun are exposed and reach the engine |
| AC-008 | TC-008 | `TestAPIParksWithoutExecutor` | PASS — parks blocked with `stage_executor_not_registered`, never as completion |
| AC-007 | TC-007 | `TestDesktopRunControlsPresent` | PASS — the backend picker and advance control are bound to the real backend |

Whole-suite state after the change: all packages ok, 0 failures; gofmt reports nothing; go vet is clean.

## Real-binary runs — AC-002 (codex) and AC-003 (claude)

Both were actually executed (`-tags=realcli`, one run each, minimal prompt).

- **codex-style**: the real `codex exec` accepted the argv and echoed a prompt that carried the stage-context path — direct proof that the template fix and the context handoff work against the real binary. It then failed with `Provider error 400 … Upstream request failed: Model is unavailable` (grok-4.5 on the local gateway).
- **claude-style**: the real `claude -p` accepted the argv and started; it failed with `API Error: 410 Model deepseek-v4-flash has been deprecated`.

Neither is a Beacon defect: the argv was accepted and the CLI started in both cases. Because a skip is never a pass, **AC-002 and AC-003 stay open** — their Beacon-side shape is evidenced above, and end-to-end success waits on an available model. Re-run when the provider is healthy:

```
go test harness RealIteration with the realcli tag
```

## Incidental finding (fixed here)

The docs beacon directory was shadowed by an unanchored gitignore rule named after the binary, so frozen truth packages could never be committed and therefore never became a consumable baseline. The rule is now anchored to the repository root.
