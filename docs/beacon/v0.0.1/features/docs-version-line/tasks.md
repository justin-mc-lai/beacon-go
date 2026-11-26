---
slug: docs-version-line
version: v0.0.1
materials_status: current
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: docs-version-line

## Task DAG

| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|
| TASK-001 | 验证 docs 版本格式化与 bump 递增规则 | - | AC-001 | `go test ./internal/orchestrator -run 'TestBumpDocsVersion$' -count=1` | 退出码 0，NormalizeDocsVersion 拒绝 runtime 形态，BumpDocsVersion 三级递增正确 | evidence/TC-001.txt | false |
| TASK-002 | 验证项目版本按 project_id 隔离 | TASK-001 | AC-002 | `go test ./internal/store -run 'TestProjectVersionFeatureConstraints$' -count=1` | 退出码 0，ListVersions 不泄漏其他项目版本 | evidence/TC-003.txt | false |
| TASK-003 | 验证 manifest 生命周期合法转移与非法拒绝 | TASK-001 | AC-003 | `go test ./internal/orchestrator -run TestManifestIllegalTransitions$ -count=1` | 退出码 0，draft->frozen->released 合法，draft->released 与 released 上 freeze 被拒绝 | evidence/TC-005.txt | false |

## Task Ledger

- TASK-001：绑定 AC-001。完成判据=TestBumpDocsVersion 与 TestNormalizeDocsVersion 绿，证据落 evidence/TC-001.txt、TC-002.txt。
- TASK-002：绑定 AC-002。完成判据=TestProjectVersionFeatureConstraints 与 TestTwoProjectsSameVersionName 绿，证据落 evidence/TC-003.txt、TC-004.txt。
- TASK-003：绑定 AC-003。完成判据=TestManifestIllegalTransitions 绿，证据落 evidence/TC-005.txt、TC-006.txt。
- 任务状态只由执行回执与证据推进，不由声明推进。
