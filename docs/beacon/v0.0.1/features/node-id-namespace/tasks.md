---
slug: node-id-namespace
version: v0.0.1
materials_status: current
task_source: acceptance_criteria
package_contract: truth-tasks
---

# Tasks: node-id-namespace

## Task DAG

| TASK | Action | Depends | AC | Command | Verify | Evidence | Auto |
|---|---|---|---|---|---|---|---|
| TASK-001 | 扩展 shortID 回归测试覆盖 5 类身份 id | - | AC-001,AC-003 | edit internal/intenttree/init_test.go：对 nroot_/n_/nfeat_/nmod_/tmig_ 断言跨 versionID 隔离，nroot_ 另断言同输入确定性；chg_/exp_ 只断言参数含 versionID 或父 id（不强求跨调用幂等） | `go test ./internal/intenttree -run TestShortIDNamespacedByVersionID -count=1` | evidence/TC-001.txt | false |
| TASK-002 | 审计生产调用点无 version-only 派生 | TASK-001 | AC-002 | `python3 -c "import glob,pathlib; bad=[f'{p}:{i+1}:{l.rstrip()}' for p in glob.glob('internal/intenttree/*.go') if not p.endswith('_test.go') for i,l in enumerate(open(p)) if 'shortID(' in l and not any(x in l for x in ['func shortID','versionID','VersionID','parentID'])]; print('\n'.join(bad)); raise SystemExit(bool(bad))"` | 退出码 0 且无输出 | evidence/TC-002.txt | false |

## Task Ledger

- TASK-001：绑定 AC-001/AC-003。完成判据=测试覆盖 5 类身份 id 前缀且两 versionID root id 不等断言通过、`go test` 绿，证据落 evidence/TC-001.txt。
- TASK-002：绑定 AC-002。完成判据=白名单审计命令无输出，证据落 evidence/TC-002.txt。
- 任务状态只由执行回执与证据推进，不由声明推进。
