# Evidence Index: kernel-qa-evidence-hardening

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | authored | truth |
| tests | test_truth | tests.md | executor_checks_run | qa |
| interface | requirement_truth | interfaces.md | authored | truth |
| implementation | acceptance_truth | internal/orchestrator/qa_evidence.go | candidate_pending_independent_qa | implement |
| execution adapters | acceptance_truth | internal/orchestrator/qa_execution.go | executor_checks_run | implement |
| CLI regressions | test_truth | internal/cli/kernel_qa_evidence_cli_test.go | executor_checks_run | qa |
| full chain fixture | test_truth | e2e/run_e2e.sh | executor_checks_run | qa |

用户确认与真实CLI旧内核反例见总盘决策《2026-11-21-kernel-qa-hardening》。反例不代表本修复已实现；产品QA与当前候选证据待实施后登记。

## Implementation verification (not independent acceptance)

Executor observed RED for missing runner directories and parser collision/incomplete/totals cases, then GREEN after the production fixes. The four frozen CLI commands now exercise Go JSON / verbose, JUnit and custom execution; candidate source provenance and event ordering; independent report rejection; formal/exit/manual/external/driver/complete live checks. Existing full-chain fixtures supply actual runner output and structured product reviews.

The immutable delivery and raw command outputs are handed off under `.beacon/candidates/kernel-qa-evidence-hardening/<implementation-commit>/implement-report.md`. The manager independently verifies that exact implementation commit and binary SHA after the executor stops. No project QA/release gate is advanced by this index.

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. Surfaces are named by bare file name until real evidence lands; replace with the full project-relative ref once the artifact exists and is registered.
