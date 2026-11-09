# AC-001 证据：策略默认 off，可选 safe_fix 或 strict_receipt

## 判据（原文）

策略默认 off，可选 `safe_fix` 或 `strict_receipt`。

## 源锚点

loopx @ c9c0644，`loopx/capabilities/change_quality/policy.py` L10-L26 中 `enabled` / `safe_fix` / `strict_receipt` 均为 `raw.get(...) is True`，缺省即 off 且两档独立；`goal_configuration.py` L43-L64 支持 Goal 级独立设置；`README.md` L3-L15 规定默认 off 与两档语义表。摘录见本包 `evidence/source-anchors.md` AC-001 节。

## 验收方式

| TC | Command |
|----|---------|
| TC-001 | `go test ./internal/changequality -run TestChangeQualityPolicyDefaultsToOff -count=1` |
| TC-002 | `go test ./internal/changequality -run TestChangeQualityUnknownModeIsReportedAndNotUpgraded -count=1` |

## 执行记录（实现轮）

- 执行时间：2026-11-09T04:16:38Z（UTC）
- 分支：`feat/kernel-change-quality`
- 实现提交：`1f53752`（feat(changequality): add policy modes, exact-diff receipt and bounded safe fix）
- 全量回归：`evidence/go-test-all.txt`，`go test ./... -count=1`，EXIT=0

| TC | Command | 实测输出 | 退出码 |
|----|---------|----------|--------|
| TC-001 | `go test ./internal/changequality -run TestChangeQualityPolicyDefaultsToOff -count=1` | `ok  	beacon-go/internal/changequality	0.242s` | 0 |
| TC-002 | `go test ./internal/changequality -run TestChangeQualityUnknownModeIsReportedAndNotUpgraded -count=1` | `ok  	beacon-go/internal/changequality	0.235s` | 0 |

原始输出：

```text
$ go test ./internal/changequality -run TestChangeQualityPolicyDefaultsToOff -count=1
ok  	beacon-go/internal/changequality	0.242s
EXIT=0
$ go test ./internal/changequality -run TestChangeQualityUnknownModeIsReportedAndNotUpgraded -count=1
ok  	beacon-go/internal/changequality	0.235s
EXIT=0
```
