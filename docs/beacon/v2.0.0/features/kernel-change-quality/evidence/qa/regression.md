# QA 负向回归 verdict：kernel-change-quality

- **verdict: PASS**
- mode_id: `qa-regression-neg-p3`
- run_id: none（只读独立负向回归验收，未建 goal run）
- stage: `qa / Implementation-Committed`
- 分支 / 目标提交: `feat/kernel-change-quality` @ `f74881e5adacb2ef18879266b83cb6003897e3e8`（实现 `1f53752`）
- 验收方: QA 验收 · 回归（独立，maker 不自签）
- 验收时间: 2026-11-09 12:36–12:40 +08:00
- 独立 worktree: `/tmp/qa-rg-cq`（detach f74881e，未 checkout main）

## 判据与结论

| # | 契约必核项 | 结果 | 实际依据 |
|---|-----------|------|----------|
| 1 | 未识别模式 fail-closed 且绝不升级 | PASS | 自写负向探针：`STRICT_RECEIPT` / `Safe_Fix` / `safe-fix` / `strictreceipt` / `1` / `on` 全部解析为 `off` 且带 `*ConfigError`(`ErrInvalidMode`)；goal 级垃圾值不会让 package 级 `strict_receipt` 生效 |
| 2 | 伪造/过期 diff SHA fail-closed | PASS | 篡改候选 diff（多一文件）→ `stale_receipt`；全 0 SHA → `stale_receipt`；基线不符 → `receipt_baseline_mismatch`；nil/空白/非 JSON/空 diff_sha/短 SHA 全拒；拒绝时 `Mode` 仍为 `strict_receipt`（无隐式降级）；长度前缀编码不存在边界伪造碰撞（`1:xx` vs `1`+`xxo` 的 SHA 不同），输入顺序不影响 SHA |
| 3 | 第二次修复 fail-closed | PASS | 首次授权内修复生效（`hello`→`HELLO`），第二次 `Request` 返回 `ErrRepairBudgetExhausted`，树保持第一次结果未被改动 |
| 4 | 越授权 hunk fail-closed 且零部分修改 | PASS | 多文件修复中 file1 已授权、file2 未授权 → `ErrOutOfAuthorization`，两文件内容与调用前完全一致，`Attempts()==0`（不消耗预算）；已授权文件上的未授权 hunk 同样拒绝且零改动；`off` / `strict_receipt` 下无法打开 safe-fix 门（`ErrModeDoesNotAllow`） |
| 5 | 全量 `go test ./... -count=1` 退出码 0 | PASS | `/tmp/qa-rg-cq` 实跑 **EXIT=0**，18 个包全 `ok`，`real 171.89s`（orchestrator 158.3s、evalgate 94.8s 均真跑） |
| 6 | 既有冻结包测试无改写 | PASS | `git diff --name-only main...feat/kernel-change-quality -- '*_test.go'` 仅 `internal/changequality/changequality_test.go`；分支自身改动仅本包实现+本包 docs，无其它包测试文件 |

## 冻结 TC 复跑（排除 `-run` 0 匹配假绿）

6/6 EXIT=0，每条 `--- PASS` 至少 1 行：

```
TestChangeQualityPolicyDefaultsToOff                                   EXIT=0
TestChangeQualityUnknownModeIsReportedAndNotUpgraded                   EXIT=0
TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA              EXIT=0
TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA                  EXIT=0 (7 PASS 行)
TestSafeFixAllowsOneBoundedRepairWithinAuthorization                   EXIT=0
TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed                EXIT=0
```

## 我实际跑过的命令

```bash
# 全量回归（worktree /tmp/qa-rg-cq）
/usr/bin/time -p go test ./... -count=1          # EXIT=0, real 171.89s

# 6 条冻结 TC 逐条锚定全名
go test ./internal/changequality -run '^TestChangeQualityPolicyDefaultsToOff$' -count=1 -v
go test ./internal/changequality -run '^TestChangeQualityUnknownModeIsReportedAndNotUpgraded$' -count=1 -v
go test ./internal/changequality -run '^TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA$' -count=1 -v
go test ./internal/changequality -run '^TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA$' -count=1 -v
go test ./internal/changequality -run '^TestSafeFixAllowsOneBoundedRepairWithinAuthorization$' -count=1 -v
go test ./internal/changequality -run '^TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed$' -count=1 -v

# 自写独立负向探针（临时文件，跑完已删除，不留在分支）
go test ./internal/changequality -run '^TestQANeg' -count=1 -v
#   TestQANegUnknownModesNeverUpgrade                    PASS
#   TestQANegForgedOrStaleReceiptAlwaysRefused           PASS
#   TestQANegCanonicalBoundaryForgery                    PASS
#   TestQANegSecondRepairAndOutOfAuthLeaveZeroChange     PASS
#   ok beacon-go/internal/changequality 0.455s

# 范围与合入可行性
git diff --name-only main...feat/kernel-change-quality -- '*_test.go'
git merge-tree --write-tree --name-only main feat/kernel-change-quality   # EXIT=0，无冲突
```

## 附注（不影响 PASS）

- `main` 当前 `a3eab40`（冻结 docs 已提交），工作区 0 项；`main` 相对本分支领先 1（冻结提交）、本分支领先 2（实现+证据），merge-base `3396558`。`merge-tree` 预演 **无冲突**，此前担心的「main 未提交 docs 与分支 docs 重叠」在冻结提交落库后已不成立。
- 本 face 只判负向与回归；端到端 CLI/orchestrator 接线不在范围内（truth 已把范围钉在 `internal/changequality`）。
- 临时 worktree 保留供取证：`/tmp/qa-rg-cq`。其中只新增本文件；探针测试已删除，`git status` 除 `evidence/qa/regression.md` 外无其它改动，未 commit / 未 push / 未改实现与 truth。
