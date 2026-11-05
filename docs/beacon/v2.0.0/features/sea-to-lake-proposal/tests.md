---
slug: sea-to-lake-proposal
version: v2.0.0
materials_status: current
---

# Tests: sea-to-lake-proposal

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator -run TestValidateLakeProposal -count=1` | schema/slug/DAG 校验 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator -run TestValidateLakeProposalSize -count=1` | 少于 3 或多于 7 拒绝 |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator -run TestValidateLakeProposalCommercialFields -count=1` | 缺商用字段/not_proposed 拒绝 |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator -run TestProgramSelectRecordsDecision -count=1` | selected/rejected/by/at 记录；未知 slug 拒绝 |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator -run TestProgramSelectCreatesDraftsOnlyForSelected -count=1` | 只给选中的湖建草稿包 |
| AC-006 | TC-006 | behavior | `go test ./internal/orchestrator -run TestProgramPlanFromProposalRefusesUnselectedDependency -count=1` | 依赖未选中的湖时拒绝 |
| AC-007 | TC-007 | behavior | `go test ./internal/orchestrator -run TestProgramRouteShowsProposalCounts -count=1` | route 暴露 proposal 与选择计数 |
| AC-008 | TC-008 | static | `go test ./skills -run TestPlanSkillMentionsProposalFlow -count=1` | plan skill 写清 propose/select/freeze |
| AC-009 | TC-009 | behavior | `go test ./internal/cli -run TestProgramPlanRequiresProposal -count=1` | 首次直接 plan 被拒；已有 plan 重排放行 |
