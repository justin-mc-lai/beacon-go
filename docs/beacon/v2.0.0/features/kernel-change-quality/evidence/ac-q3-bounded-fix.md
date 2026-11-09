# AC-003 证据：safe_fix 只允许一次有界修复，越界 fail-closed

## 判据（原文）

`safe_fix` 只允许一次有界修复，且不得越出调用方已有授权；越界 fail-closed。

## 源锚点

loopx @ c9c0644，`loopx/capabilities/change_quality/receipt.py` L202-L205 固定 `max_safe_fix_passes: 1`，L225-L229 未开启策略时禁止改文件；`result.py` L355-L368 在未授权或状态不符时抛 `ValueError`；`README.md` L13-L15、L195-L207 明确 safe_fix 不赋予写权限、受限于调用方已有授权；`change_quality_validation.ts` L15-L85 对越界范围直接 `fail(...)`。摘录见本包 `evidence/source-anchors.md` AC-003 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-005 | `go test ./internal/changequality -run TestSafeFixAllowsOneBoundedRepairWithinAuthorization -count=1` |
| TC-006 | `go test ./internal/changequality -run TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:16:38Z（UTC）
- 分支：`feat/kernel-change-quality`
- 实现提交：`1f53752`（feat(changequality): add policy modes, exact-diff receipt and bounded safe fix）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-005 | `go test ./internal/changequality -run TestSafeFixAllowsOneBoundedRepairWithinAuthorization -count=1` | `ok  	beacon-go/internal/changequality	0.237s` | 0 |
| TC-006 | `go test ./internal/changequality -run TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed -count=1` | `ok  	beacon-go/internal/changequality	0.244s` | 0 |

原始输出：

```text
$ go test ./internal/changequality -run TestSafeFixAllowsOneBoundedRepairWithinAuthorization -count=1
ok  	beacon-go/internal/changequality	0.237s
EXIT=0
$ go test ./internal/changequality -run TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed -count=1
ok  	beacon-go/internal/changequality	0.244s
EXIT=0
```
