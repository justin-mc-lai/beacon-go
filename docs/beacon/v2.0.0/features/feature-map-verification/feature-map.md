---
slug: feature-map-verification
version: v2.0.0
generated_at: 2026-11-04T16:41:12.302093Z
surface_fingerprint: 8d28f4f9c51f2d059a8770b6d4fc2aff8128a019bd413c4a25efbafdd045341c
---

# Feature Map: feature-map-verification

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/feature_map.go | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | mapped |
| AC-002 | internal/orchestrator/feature_map.go | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | mapped |
| AC-003 | internal/gate/integrity.go | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | mapped |
| AC-004 | internal/orchestrator/truthmap.go | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | mapped |
| AC-005 | internal/orchestrator/feature_map.go | `go test ./internal/orchestrator -run TestBuildFeatureMapBindsACs -count=1` | mapped |
