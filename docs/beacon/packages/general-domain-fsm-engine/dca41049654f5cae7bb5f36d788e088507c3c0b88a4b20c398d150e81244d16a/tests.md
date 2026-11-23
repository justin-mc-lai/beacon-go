---
slug: general-domain-fsm-engine
version: v2.0.0
materials_status: current
---

# Tests: general-domain-fsm-engine

Every command runs from the repository root.

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/gate/... -run TestGeneralDomainKindExemption -v` | domain_kind 豁免非电商 User Journey 阻断 |
| AC-002 | TC-002 | behavior | `go test ./cmd/... -run TestTruthFSMCheckCLI -v` | beacon truth fsm check 正确输出状态机投影 |
| AC-003 | TC-003 | behavior | `go test ./cmd/... -run TestFSMTransitionCLI -v` | beacon fsm transition 正确判定合法与非法跃迁 |
| AC-004 | TC-004 | behavior | `go test ./internal/gate/... ./cmd/... -v` | 全量单元测试全绿 |

## 回归

`go test ./internal/gate/... -run TestGeneralDomainKindExemption`
`go test ./cmd/... -run TestFSM`
