---
slug: skills-distribution
version: v2.0.0
generated_at: 2026-11-04T16:41:12.270973Z
surface_fingerprint: ff4bdfdbf6cb7a12a61e6eeae729823a7738c3f97455065371329216f2812c99
---

# Feature Map: skills-distribution

One-way projection of truth + tests + tasks. It never rewrites truth.

| AC | Entry Point | Verification Command | Status |
|----|-------------|----------------------|--------|
| AC-001 | .claude-plugin/plugin.json | `go test ./skills -run TestPluginManifestLists1Plus6 -count=1` | mapped |
| AC-002 | skills/verify.go | `go test ./skills -run TestVerifyHostReportsMissing -count=1` | mapped |
| AC-003 | internal/cli/skills_verify.go | `go test ./internal/cli -run TestSkillsVerifyStrict -count=1` | mapped |
| AC-004 | skills/verify.go | `go test ./skills -run TestVerifyIgnoresV1Names -count=1` | mapped |
