---
slug: gate-strict-integrity
version: v2.0.0
generated_at: 2026-11-04T16:41:12.226388Z
surface_fingerprint: 6bd4ff694ff925f1964a9a59c9302008c5a18838251c8970caa220ec4f7521cf
---

# Feature Map: gate-strict-integrity

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/cli/gate.go | `go test ./internal/cli -run TestGateIntegrityStrictBlocksFrozenPackage -count=1` | mapped |
| AC-002 | internal/orchestrator/release_check.go | `go test ./internal/orchestrator -run TestReleaseCandidateIntegrityBlocks -count=1` | mapped |
| AC-003 | internal/cli/gate.go | `go test ./internal/cli -run TestGateIntegrityFrozenDefaultStaysAudit -count=1` | mapped |
| AC-004 | internal/gate/integrity.go | `go test ./internal/orchestrator -run TestHistoricalFrozenIntegrityAudits -count=1` | mapped |
| AC-005 | internal/gate/evaluate.go | `go test ./internal/gate -run TestEvaluateReleaseHasNoIntegrityPrereq -count=1` | mapped |
