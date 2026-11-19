---
slug: kernel-lake-freeze-closure
version: v2.0.0
materials_status: current
---

# Tests: kernel-lake-freeze-closure

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/gate -run TestEvidenceAppendDoesNotDriftPackageSurface -count=1` | evidence-only edit keeps the frozen surface |
| AC-001 | TC-002 | behavior | `go test ./internal/gate -run TestContractEditStillDriftsPackageSurface -count=1` | truth/tests/tasks/restatement edit still drifts |
| AC-002 | TC-003 | behavior | `go test ./internal/orchestrator -run TestFeatureScopedArtifactSatisfiesEvidence -count=1` | feature-scoped registration satisfies evidence with no run |
| AC-003 | TC-004 | behavior | `go test ./internal/orchestrator -run TestTruthPrecheckFailureRecordsFeatureEvent -count=1` | failed precheck writes a feature event with reason codes |
| AC-004 | TC-005 | regression | `go test ./internal/... -count=1` | gate strictness, restatement and intent gates unchanged |
