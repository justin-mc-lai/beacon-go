---
slug: truth-version-iteration
version: v2.0.0
generated_at: 2026-11-05T02:34:11.955909Z
surface_fingerprint: 906c790e660838c99827cddef9d27f1c15e3e9b0bbb5113a1eb1a52f6cf3fd1f
---

# Feature Map: truth-version-iteration

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | internal/orchestrator/truth.go | `go test ./internal/orchestrator -run TestTruthCreateGreenfieldDefaultsToV001 -count=1` | mapped |
| AC-002 | internal/orchestrator/truth.go | `go test ./internal/orchestrator -run TestTruthCreateUsesCurrentVersion -count=1` | mapped |
| AC-003 | internal/orchestrator/versioning.go | `go test ./internal/orchestrator -run TestDocsVersionNormalizeAndUnregistered -count=1` | mapped |
| AC-004 | internal/orchestrator/versioning.go | `go test ./internal/orchestrator -run TestDocsVersionRejectsRuntimeShape -count=1` | mapped |
| AC-005 | internal/cli/projects.go | `go test ./internal/cli -run TestProjectsBumpPatch -count=1` | mapped |
| AC-006 | skills/truth/SKILL.md | `go test ./skills -run TestTruthSkillVersionFlow -count=1` | mapped |
