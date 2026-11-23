---
slug: skills-distribution
version: v2.0.0
materials_status: current
---

# Tests: skills-distribution

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | static | `go test ./skills -run TestPluginManifestLists1Plus6 -count=1` | 两个插件清单存在，skills 列表正好是 1+6 |
| AC-002 | TC-002 | behavior | `go test ./skills -run TestVerifyHostReportsMissing -count=1` | 安装部分技能后，缺失项被逐条列出 |
| AC-003 | TC-003 | behavior | `go test ./internal/cli -run TestSkillsVerifyStrict -count=1` | 默认退出 0；`--strict` 在缺失时非零退出 |
| AC-004 | TC-004 | behavior | `go test ./skills -run TestVerifyIgnoresV1Names -count=1` | `beacon-goal`（v1）不被计为 extra，也不被删除 |
