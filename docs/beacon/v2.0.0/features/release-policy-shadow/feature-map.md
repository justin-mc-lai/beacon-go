---
slug: release-policy-shadow
version: v2.0.0
generated_at: 2026-11-04T16:41:12.243449Z
surface_fingerprint: 43ffe419622a8b97cd6dd9a3e9dc4b4381cb749985cde9050c02bfb12babdd72
---

# Feature Map: release-policy-shadow

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/engine.go | `go test ./internal/orchestrator -run TestCreateRunRejectsUnsupportedReleasePolicy -count=1` | mapped |
| AC-002 | internal/orchestrator/gates_facts.go | `go test ./internal/orchestrator -run TestReleaseShadowRecordsVerdictButKeepsHumanGate -count=1` | mapped |
| AC-003 | internal/orchestrator/complete.go | `go test ./internal/orchestrator -run TestReleaseShadowStillRequiresHumanComplete -count=1` | mapped |
| AC-004 | internal/orchestrator/engine.go | `go test ./internal/cli -run TestGoalReleaseShadowCLI -count=1` | mapped |
| AC-005 | internal/core/reasons.go | `go test ./internal/orchestrator -run TestReleasePolicyAutoNotImplemented -count=1` | mapped |
