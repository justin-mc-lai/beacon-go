---
slug: intent-tree-crud
version: v0.0.1
materials_status: current
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: intent-tree-crud

## Task DAG

| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|
| TASK-001 | 验证子节点提案 schema DAG 校验 | - | AC-001 | `go test ./internal/intenttree -run 'TestValidateChildMatrix$' -count=1` | 退出码 0，合法提案通过，非法形态被拒绝 | evidence/TC-001.txt | false |
| TASK-002 | 验证退化树/非法索引被捕获 | TASK-001 | AC-001 | `go test ./internal/intenttree -run 'TestValidateIndexCatchesDegenerateTrees$' -count=1` | 退出码 0，退化树被拒绝 | evidence/TC-002.txt | false |
| TASK-003 | 验证 feature 生命周期 stage gate | TASK-001 | AC-002 | `go test ./internal/intenttree -run 'TestStageGateFeatureLifecycle$' -count=1` | 退出码 0，状态前置满足放行、不满足拒绝 | evidence/TC-003.txt,TC-004.txt | false |
| TASK-004 | 验证 change request/refreeze 作废链路 | TASK-001 | AC-003 | `go test ./internal/orchestrator -run 'TestTruthChangeAndRefreeze$' -count=1` | 退出码 0，change_pending 不改写 truth body，refreeze 恢复 frozen | evidence/TC-005.txt | false |
| TASK-005 | 验证 change_pending 阻断 frozen fact 与 admission | TASK-004 | AC-003 | `go test ./internal/orchestrator -run 'TestFrozenFactChangePendingVoidsFrozen$' -count=1` | 退出码 0，change_pending 使 frozen fact 失效并阻断 admission | evidence/TC-006.txt | false |

## Task Ledger

- TASK-001：绑定 AC-001。完成判据=TestValidateChildMatrix 绿，证据落 evidence/TC-001.txt。
- TASK-002：绑定 AC-001。完成判据=TestValidateIndexCatchesDegenerateTrees 绿，证据落 evidence/TC-002.txt。
- TASK-003：绑定 AC-002。完成判据=TestStageGateFeatureLifecycle 绿，证据落 evidence/TC-003.txt、TC-004.txt。
- TASK-004：绑定 AC-003。完成判据=TestTruthChangeAndRefreeze 绿，证据落 evidence/TC-005.txt。
- TASK-005：绑定 AC-003。完成判据=TestFrozenFactChangePendingVoidsFrozen 绿，证据落 evidence/TC-006.txt。
- 任务状态只由执行回执与证据推进，不由声明推进。
