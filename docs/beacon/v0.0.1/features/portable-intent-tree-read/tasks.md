---
slug: portable-intent-tree-read
version: v0.0.1
materials_status: current
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: portable-intent-tree-read

## Task DAG

| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|
| TASK-001 | 让两个读入口无需创建或迁移本机账本即可读取意图树 | - | AC-001 | `go test ./internal/cli -run '^TestPortableIntentTreeRead$/^local_' -count=1` | 无账本可读；旧账本不迁移；无树时拒绝；各场景不改写权威树 | evidence/TC-001.txt,evidence/TC-002.txt,evidence/TC-005.txt | false |
| TASK-002 | 保留登记的当前版，并处理未登记、歧义与指定版本错误 | TASK-001 | AC-002 | `go test ./internal/cli -run '^TestPortableIntentTreeRead$/^version_' -count=1` | 保留当前版；未登记且单版可读；歧义列出候选；指定版缺失或损坏时拒绝 | evidence/TC-003.txt,evidence/TC-004.txt,evidence/TC-006.txt | false |

## Task Ledger

- TASK-001 对应 AC-001。完成判据是 TC-001、TC-002 和 TC-005 通过。
- TASK-002 对应 AC-002。完成判据是 TC-003、TC-004 和 TC-006 通过。
- 任务状态由执行回执和证据推进。
