# AC-002 证据：strict_receipt 在 premerge 校验回执与精确当前 diff SHA 一致

## 判据（原文）

`strict_receipt` 在 premerge 校验回执与**精确当前 diff SHA** 一致，否则拒绝。

## 源锚点

loopx @ c9c0644，`loopx/capabilities/change_quality/scope.py` L190-L215 计算 `scope_fingerprint`，把 base、committed/staged/unstaged 二进制 diff 与 untracked 全部纳入 SHA-256；`receipt.py` L42-L73、L505-L520 定义 `stale_receipt` / `receipt_missing`，启用 strict 时 `ok = receipt_valid or not strict`；`README.md` L87-L91 要求 premerge 回执与真实当前 diff SHA 严格吻合。摘录见本包 `evidence/source-anchors.md` AC-002 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-003 | `go test ./internal/changequality -run TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA -count=1` |
| TC-004 | `go test ./internal/changequality -run TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:16:38Z（UTC）
- 分支：`feat/kernel-change-quality`
- 实现提交：`1f53752`（feat(changequality): add policy modes, exact-diff receipt and bounded safe fix）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-003 | `go test ./internal/changequality -run TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA -count=1` | `ok  	beacon-go/internal/changequality	0.240s` | 0 |
| TC-004 | `go test ./internal/changequality -run TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA -count=1` | `ok  	beacon-go/internal/changequality	0.233s` | 0 |

原始输出：

```text
$ go test ./internal/changequality -run TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA -count=1
ok  	beacon-go/internal/changequality	0.240s
EXIT=0
$ go test ./internal/changequality -run TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA -count=1
ok  	beacon-go/internal/changequality	0.233s
EXIT=0
```
