---
slug: beacon-plan-pln-fsm-integration
version: v2.0.0
materials_status: current
---

# Tests: beacon-plan-pln-fsm-integration

Every command runs from the repository root.

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./skills/... -v` | 验证技能测试覆盖 pln-fsm 模式 |
| AC-002 | TC-002 | behavior | `go test ./skills/... -run TestSkillsContract -v` | 验证 SKILL.md 硬门禁契约约束 |
| AC-003 | TC-003 | behavior | `go test ./skills/... -run TestExamples -v` | 验证新增跑腿拆解范例合规且可解析 |
| AC-004 | TC-004 | behavior | `beacon-go skills verify` | 验证各客户端环境与技能集对齐 |

## 回归

`go test ./skills/...`
`beacon-go skills verify`
