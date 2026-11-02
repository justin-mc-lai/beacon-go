---
slug: program-file-graph
version: v2.0.0
materials_status: current
---

# Tests: program-file-graph

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/orchestrator/ -run TestLoadProgramGraphFileMissingKeys -count=1` | 缺顶层/湖字段各产出一条 missing: 诊断 |
| AC-002 | TC-002 | behavior | `go test ./internal/orchestrator/ -run TestLoadProgramGraphFileGraphProblems -count=1` | 自依赖/缺依赖/成环映射到既有 reason code |
| AC-003 | TC-003 | behavior | `go test ./internal/orchestrator/ -run TestLoadProgramGraphFileMissing -count=1` | 文件不存在返回 file_missing 错误，不 panic |
| AC-004 | TC-004 | behavior | `go test ./internal/orchestrator/ -run TestProgramImportCarriesLakeFields -count=1` | 导入后 store 里 parity_subset/status/scope_mode 与文件一致 |
| AC-005 | TC-005 | behavior | `go test ./internal/orchestrator/ -run TestProgramImportRefusesInvalidGraph -count=1` | 校验失败时 store 的 program 状态不变 |
| AC-006 | TC-006 | behavior | `go test ./internal/cli/ -run TestProgramValidateCLIBlockedExit -count=1` | 非法图返回 error（进程非零），合法图返回 nil |
| AC-007 | TC-007 | behavior | `go test ./internal/cli/ -run TestProgramExistingSubcommandsIntact -count=1` | plan/change/freeze/admit/route 仍在且必需 flag 未变 |
