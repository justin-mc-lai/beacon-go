---
slug: kernel-change-quality
version: v2.0.0
materials_status: current
---

# Tests: kernel-change-quality

每条 AC 至少两条执行层 TC：一条验证正向策略行为，一条验证拒绝或 fail-closed 的负向行为。Command 一律是可直接运行的 `go test` 选择器；实现轮次按此命名落地测试，并把实跑输出追加到对应的包内证据文件。AC-Q1 至 AC-Q3 与 AC-001 至 AC-003 的对应关系见 truth.md 的 AC ID 映射表。

| AC ID | TC ID | Layer | Command | Assertion | Evidence |
|-------|-------|-------|---------|-----------|----------|
| AC-001 | TC-001 | behavior | `go test ./internal/changequality -run TestChangeQualityPolicyDefaultsToOff -count=1` | 未配置时策略为 off；词表只接受 off / safe_fix / strict_receipt | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q1-policy-modes.md |
| AC-001 | TC-002 | negative | `go test ./internal/changequality -run TestChangeQualityUnknownModeIsReportedAndNotUpgraded -count=1` | 未识别取值按 off 处理并返回配置错误，不隐式升级为 safe_fix 或 strict_receipt | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q1-policy-modes.md |
| AC-002 | TC-003 | behavior | `go test ./internal/changequality -run TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA -count=1` | 回执 diff SHA 与当前候选 diff 重新计算的精确 SHA 一致时，premerge 放行 | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q2-diff-sha-receipt.md |
| AC-002 | TC-004 | negative | `go test ./internal/changequality -run TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA -count=1` | SHA 不一致、空回执、格式非法或基线不一致时 premerge 拒绝，且拒绝不降级为 off | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q2-diff-sha-receipt.md |
| AC-003 | TC-005 | behavior | `go test ./internal/changequality -run TestSafeFixAllowsOneBoundedRepairWithinAuthorization -count=1` | 授权范围内的单次修复被放行，修复触碰的文件与 hunk 全部在授权清单内 | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q3-bounded-fix.md |
| AC-003 | TC-006 | negative | `go test ./internal/changequality -run TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed -count=1` | 第二次修复请求被拒绝；任一文件或 hunk 越出授权即 fail-closed，不产生部分修改 | docs/beacon/v2.0.0/features/kernel-change-quality/evidence/ac-q3-bounded-fix.md |

## 回归与门禁

实现轮次除上述 6 条 TC 外，还需跑全量 `go test ./... -count=1` 并落包内证据；三份独立 QA verdict 由三个独立验收方各自写入 evidence/qa，maker 不自签。

