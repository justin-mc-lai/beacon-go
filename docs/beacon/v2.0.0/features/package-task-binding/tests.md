---
slug: package-task-binding
version: v2.0.0
materials_status: current
---

# Tests: package-task-binding

| AC ID | TC ID | Layer | Command | Assertion |
|-------|-------|-------|---------|-----------|
| AC-001 | TC-001 | behavior | `go test ./internal/gate/ -run TestValidateTaskBindingMissingAC -count=1` | unchecked 行缺 ac= 产生 task_binding_missing，Detail 含 TASK-002 |
| AC-002 | TC-002 | behavior | `go test ./internal/gate/ -run TestValidateTaskBindingMissingEvidence -count=1` | checked 行缺 evidence= 产生 task_binding_missing，Detail 含 TASK-001 |
| AC-003 | TC-003 | behavior | `go test ./internal/gate/ -run TestValidateTaskBindingEvidenceFile -count=1` | 路径形态 evidence 不存在/空文件时报 checkbox_checked_without_evidence_file；存在且非空不报 |
| AC-004 | TC-004 | behavior | `go test ./internal/gate/ -run TestValidateTaskBindingUncheckedNoEvidenceOK -count=1` | 未勾选行缺 evidence= 不报 task_binding_missing |
| AC-005 | TC-005 | behavior | `go test ./internal/gate/ -run TestValidateTaskBindingKeepsExistingVerdicts -count=1` | 已绑定的包 AC/TC/evidence 检查 verdict 与 findings 不变 |
| AC-006 | TC-006 | behavior | `go test ./internal/gate/ -run TestHostPackagesTaskBindingsClean -count=1` | 从仓库读四个既有包，断言无 task_binding_missing / checkbox_checked_without_evidence_file |
| AC-007 | TC-007 | static | `go test ./internal/cli/ -run TestNoTruthValidatePackageCmd -count=1` | truth 子命令集中不存在 validate-package |
