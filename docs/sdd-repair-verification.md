# SDD repair verification — 2026-11-23

## Object

- Reviewed upstream baseline: ac7ff8e5e85baae7863642f6dd9bba61bfe6bbd5.
- Authorized scope commit: 09463dd66a6a656654fd89ffbc17d1f54458a746.
- Workspace: beacon-go-worktrees/sdd-workflow-repair; branch fix/sdd-workflow-repair.
- Delivery is a local code candidate, not an installation or release.

## Observed checks

- `go test ./internal/gate ./internal/orchestrator ./internal/cli -run 'TestSDD|TestFSM|TestTask|TestImplementTasksCLI|TestCoverageMatrix|TestPackageContract|TestIntentFSMRequiredWhenDomain' -count=1`: all three packages passed on the final candidate sources.
- `go vet ./internal/gate ./internal/orchestrator ./internal/cli`: exit 0.
- `go build -o /tmp/beacon-sdd-repair-candidate ./cmd/beacon`: exit 0.
- Candidate binary SHA256: afe9cf98c6b13984170b2c196888356c6ceb437b6fa0821e7d6a8ef36ebad216.
- Complete gate and CLI package tests passed in the broad three-package run. The initial orchestrator run failed the missing-domain-FSM reason contract; the missing/empty graph compatibility was corrected. The complete orchestrator rerun passed in 220.631 seconds. Subsequent scoped review fixes were verified by the final targeted run above, not another unchanged broad suite.

## Real CLI smoke

Fixture root `/tmp/beacon-sdd-repair-smoke`, separate store `/tmp/beacon-sdd-repair-smoke.db`:

1. Generated scaffold freeze refused with nonzero exit and scaffold/risk/review findings.
2. Completed stateless numerical-formatting specification with one AC; confirmation refused without an external review receipt.
3. Independent SmokeSpecReviewer read truth/tasks, checked the exact review fingerprint and supplied its own passing receipt. Recording, owner confirmation and freeze succeeded.
4. `truth map verify` returned `status: pass`, `layout_ok: true` with no tests/evidence/restatement authored files beyond truth/tasks. Missing optional feature map remained informational.
5. `truth fsm check` on declared stateless work returned `applicable: false`; no invented model required.
6. `implement tasks run` executed Command, Verify and final Verify; report and subsequent status returned passed. Every attempt and task evidence snapshot was registered.
7. Changing implementation inputs caused the prior task status to refuse with `task_candidate_stale`.
8. A fresh run exercised command exit 1, explicit external repair once, command retry, Verify and final Verify. It returned passed with repairs=1; failed attempt remained in the immutable report.

The fixture verifies wiring and observable behavior, not a full product delivery. It was not used to grant this repair run a QA pass.

## Independent reviews and fixes

Go review found and drove fixes for freeze-written strict metadata drifting semantic fingerprints, optional malformed FSM bypasses, stale intent read views, aggregate-ambiguous transition coverage, trivial executable spellings and missing independent integrity DAG checks.

Execution review found and drove fixes for earlier-task regression on the final candidate, shared evidence overwrites, incomplete source-candidate coverage, package-manager install aliases and Go verification argument normalization.

Behavior-preserving simplification consolidated signing normalization to one pass. Other suggested changes were left out when they added churn without changing the tested contracts.

## Limits

- External review and guard identities/claims are caller-attested, not authenticated. The kernel does not call an LLM or prove prose semantics.
- Commands are trusted project code. Workdir isolation is not a security sandbox; arbitrary scripts can perform side effects outside the command-name denylist.
- Direct Go test verification checks actual non-skipped cases. Custom verification commands remain trusted programs, not a general-purpose assertion proof engine.
- No desktop/browser, container isolation, cost-governance or production multi-platform acceptance was performed.
- Formal product QA/release gates, human release approval and installation were not performed. The original workspace and installed binary were not replaced.
