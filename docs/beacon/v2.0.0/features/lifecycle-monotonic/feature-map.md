---
slug: lifecycle-monotonic
version: v2.0.0
generated_at: 2026-11-05T00:37:36.411861Z
surface_fingerprint: 76efb3e0ad719044cb5c3cf1e35f518a0da98fbcd2c43c7f4e10adf12674d929
---

# Feature Map: lifecycle-monotonic

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/engine.go | `go test ./internal/orchestrator -run TestCreateRunDoesNotDowngradeCompletedFeature -count=1` | mapped |
| AC-002 | internal/orchestrator/truth.go | `go test ./internal/orchestrator -run TestFreezeDoesNotDowngradeCompletedFeature -count=1` | mapped |
| AC-003 | internal/orchestrator/tick.go | `go test ./internal/orchestrator -run TestUpdateLifecycleMonotonicBlocksDowngrade -count=1` | mapped |
| AC-004 | internal/orchestrator/lifecycle_test.go | `go test ./internal/orchestrator -run TestUpdateLifecycleMonotonicNonCompletedStillUpdates -count=1` | mapped |
| AC-005 | internal/orchestrator/lifecycle.go | `go test ./internal/orchestrator -run TestLifecycleGuardIsShared -count=1` | mapped |
| AC-006 | internal/orchestrator/lifecycle_test.go | `go test ./... -count=1` + `bash e2e/run_e2e.sh` | mapped |
