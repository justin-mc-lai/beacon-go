# Evidence Index: kernel-lake-freeze-closure

## Evidence Index

| Surface | Authority | Canonical Artifact | Status | Route |
|---------|-----------|--------------------|--------|-------|
| truth | requirement_truth | truth.md | draft | truth |
| tests | test_truth | tests.md | draft | qa |

## Boundary

`evidence.md` is an index/read model. It must not contain final verdict, gate truth, or release readiness authority fields. Surfaces are named by bare file name until real evidence lands; replace with the full project-relative ref once the artifact exists and is registered.

## Implement Evidence — 2026-11-19 · ExecEngineer · run_18d7031fba3b78cb7249b13b

Base commit: `07d40c91d5a7dee9f05d69099ff20a4c50264947` (asserted before any edit).
Landing commit: `2f5efef` (merged to local main as `e2c3ee3`, not pushed).

### Change landing points

| AC | Change | File:line |
|----|--------|-----------|
| AC-001 | Contract fingerprint (truth/tests/tasks/restatement) + legacy-tolerant drift check | `internal/core/fingerprint.go:122` `PackageContractFingerprint`, `internal/core/fingerprint.go:141` `PackageSurfaceDrifted`; callers use `internal/orchestrator/gates_integrity.go:68` `surfaceDrifted` |
| AC-002 | Feature-scoped artifact registration (run_id sentinel, no schema change) | `internal/orchestrator/artifacts.go:143` `RegisterFeatureArtifact` (`:163` shared write half, `:268` list); `internal/store/dao_rest.go:183` `FeatureScopeRunID`, `:187` `ListByFeatureScope`; evidence check gathers scope rows at `internal/orchestrator/gates_integrity.go:104` and `internal/orchestrator/bundle.go:196` |
| AC-003 | `truth_precheck_failed` feature event (best-effort, stderr warning) | `internal/orchestrator/truth.go:147` event const, `:153` writer, `:735` freeze call site; `internal/orchestrator/truth_change.go:356` tx-guard call site, `:368` precheck call site; stderr print `internal/cli/truth.go:51`; read face `internal/orchestrator/resume.go:309` `TraceFeature` + `beacon goal trace --feature` |
| AC-002 | CLI scope flag | `internal/cli/goal_artifacts.go:59` register `--feature` (list at `:125`) |

### TC runs (verbatim)

```
$ go test ./internal/gate -run TestEvidenceAppendDoesNotDriftPackageSurface -count=1
ok  	beacon-go/internal/gate	0.539s

$ go test ./internal/gate -run TestContractEditStillDriftsPackageSurface -count=1
ok  	beacon-go/internal/gate	0.234s

$ go test ./internal/orchestrator -run TestFeatureScopedArtifactSatisfiesEvidence -count=1
ok  	beacon-go/internal/orchestrator	0.319s

$ go test ./internal/orchestrator -run TestTruthPrecheckFailureRecordsFeatureEvent -count=1
ok  	beacon-go/internal/orchestrator	0.369s

$ go test ./internal/... -count=1
ok  	beacon-go/internal/agentapi	4.111s
ok  	beacon-go/internal/benchgate	0.716s
ok  	beacon-go/internal/changequality	1.126s
ok  	beacon-go/internal/cli	17.002s
ok  	beacon-go/internal/core	2.251s
ok  	beacon-go/internal/effectjournal	1.847s
ok  	beacon-go/internal/evalgate	47.926s
ok  	beacon-go/internal/gate	3.133s
ok  	beacon-go/internal/harness	12.013s
?   	beacon-go/internal/httpapi	[no test files]
ok  	beacon-go/internal/llm	2.915s
ok  	beacon-go/internal/migration	3.688s
ok  	beacon-go/internal/orchestrator	88.694s
ok  	beacon-go/internal/progressreview	2.672s
ok  	beacon-go/internal/session	2.529s
ok  	beacon-go/internal/store	2.699s
ok  	beacon-go/internal/stride	2.914s
ok  	beacon-go/internal/workspace	3.904s
exit=0
```

Full-suite run (`go test ./... -count=1`): every package ok, exit=0 — raw log
`.beacon/evidence/kernel-lake-freeze-closure/implement/go-test-all.log`.

### Artifacts (feature scope, no run)

- `art_3476efdb1ddbfa00499b536d` test_results `.beacon/evidence/kernel-lake-freeze-closure/implement/go-test-all.log`
- `art_cbdc4c4e93dc30bfe08879af` test_results `.beacon/evidence/kernel-lake-freeze-closure/implement/TC-005.log`
- raw TC logs: `.beacon/evidence/kernel-lake-freeze-closure/implement/TC-00{1..5}.log`, `conformance.log`, `goal-status.log`

### Run state (honest)

`beacon goal status` → stage `plan`, run `run_18d7031fba3b78cb7249b13b` (external executor, not auto-driven).
`beacon goal conformance` → **fail**: `gates_evaluated` (no stage exit verdicts recorded yet),
`verifier_attached` (no independent verifier). `truth_frozen` and `state_exported` pass.

### Not done / risks

- Not committed to remote, not released, no release notes (human gate pending).
- Independent QA has not judged this work (maker ≠ checker).
- Legacy `pkgsurface-v1` freeze records keep their evidence-inclusive hash until
  their next refreeze; only newly recorded freezes get `pkgcontract-v1`.
