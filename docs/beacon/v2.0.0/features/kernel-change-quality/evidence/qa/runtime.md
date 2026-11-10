# Runtime QA Verdict: kernel-change-quality

- **mode_id**: `qa-runtime-behavior-p3`
- **run_id**: none（只读独立运行时验收，绑定分支 `feat/kernel-change-quality` @ `f74881e`）
- **stage**: `qa / Implementation-Committed`
- **target_commit**: `f74881e`（`feat/kernel-change-quality`；实现 `1f53752` + truth/证据 `f74881e`）
- **baseline**: `main` = `3396558`
- **evaluator**: QA 验收 · agy（视觉/端侧 & 独立运行时验收）
- **evaluated_at**: 2026-11-09T12:33:00+08:00
- **verdict**: **PASS**

---

## 1. 冻结 TC 逐条实测（6 条全部独立实跑）

执行目录：`/tmp/qa-rt-cq`（独立 worktree，绑定 `feat/kernel-change-quality`）
测试命令按 `tests.md` 规范原样执行，追加 `-v` 并锚定 `^Name$` 验证单测真实执行。

| TC ID | AC ID | Package | Command | Exit Code | RUN Count | PASS Count | Duration | Verdict |
|-------|-------|---------|---------|-----------|-----------|------------|----------|---------|
| TC-001 | AC-001 | internal/changequality | `go test ./internal/changequality -run '^TestChangeQualityPolicyDefaultsToOff$' -count=1` | 0 | 1 | 1 | 0.234s | PASS |
| TC-002 | AC-001 | internal/changequality | `go test ./internal/changequality -run '^TestChangeQualityUnknownModeIsReportedAndNotUpgraded$' -count=1` | 0 | 1 | 1 | 0.235s | PASS |
| TC-003 | AC-002 | internal/changequality | `go test ./internal/changequality -run '^TestStrictReceiptAcceptsReceiptBindingExactCurrentDiffSHA$' -count=1` | 0 | 1 | 1 | 0.236s | PASS |
| TC-004 | AC-002 | internal/changequality | `go test ./internal/changequality -run '^TestStrictReceiptRejectsMismatchedOrMissingReceiptSHA$' -count=1` | 0 | 6 subtests | 6 subtests | 0.236s | PASS |
| TC-005 | AC-003 | internal/changequality | `go test ./internal/changequality -run '^TestSafeFixAllowsOneBoundedRepairWithinAuthorization$' -count=1` | 0 | 1 | 1 | 0.241s | PASS |
| TC-006 | AC-003 | internal/changequality | `go test ./internal/changequality -run '^TestSafeFixSecondAttemptAndOutOfAuthorizationFailClosed$' -count=1` | 0 | 1 | 1 | 0.299s | PASS |

---

## 2. 运行时行为核验细节

### 2.1 策略默认值与词表校验（AC-001 / TC-001, TC-002）
- **默认策略为 off**：`NewPolicy("")` 与空白字符串 `NewPolicy("   ")` 均返回 `ModeOff`，且 `ConfigError() == nil`。
- **合法词表严格受限**：词表严格限制为 `off`、`safe_fix`、`strict_receipt` 3 种枚举值，无其他扩充。
- **未识别模式安全回退且报错**：输入非法值（如 `"turbo"`、`"SAFE_FIX"`、`"safe-fix"`、`"strict"`）时：
  - 强制解析为 `ModeOff`，绝不隐式升级为 `safe_fix` 或 `strict_receipt`；
  - 携带明确的 `*ConfigError`（`errors.Is(err, ErrInvalidMode)`），记录传入的非法原始值。

### 2.2 Strict Receipt 精确 Diff SHA 校验（AC-002 / TC-003, TC-004）
- **规范化 SHA 确定性计算**：`CandidateDiff.Canonical()` 将文件按路径排序、Hunk 按 ID 及内容排序，且字段长度前缀化编码。对于不同文件顺序但内容等价的 Diff，计算出的 SHA256 完全一致；若修改任意 Hunk 内容，SHA256 必然变动。
- **精确匹配放行**：当回执中的 `diff_sha` 与当前候选 Diff 实时重新计算的精确 SHA256 完全一致且 baseline 匹配时，`EvaluatePremerge` 判定 `Allowed: true`，`Mode: strict_receipt`。
- **异常回执终态拒绝且不降级**：
  - `stale sha`（过期/伪造 SHA）：拒绝，原因 `stale_receipt`；
  - `empty receipt`（空回执 / nil）：拒绝，原因 `receipt_missing`；
  - `blank receipt`（纯空白字符串）：拒绝，原因 `receipt_missing`；
  - `missing sha field`（缺少 diff_sha 字段）：拒绝，原因 `receipt_missing`；
  - `malformed json`（非法 JSON）：拒绝，原因 `receipt_malformed`；
  - `baseline mismatch`（基线不一致）：拒绝，原因 `baseline_mismatch`；
  - 所有拒绝判定中，`decision.Allowed == false` 且 `decision.Mode` 始终保持 `strict_receipt`，绝不隐式降级为 `off` 放行。

### 2.3 Safe Fix 单次有界修复与越界防线（AC-003 / TC-005, TC-006）
- **单次修复预算**：`NewSafeFixGate` 设置 `maxAttempts: 1`。首次在授权范围内的合法修复请求被执行（`Applied: true`，`Attempts: 1`），返回修改的文件清单。
- **二次尝试拒绝**：在同一 Gate 实例上发起第二次修复请求时，立即返回 `ErrRepairBudgetExhausted`（`repair_budget_spent`），且内部文件树状态与第一次修复后完全一致，拒绝被再次修改。
- **越界授权 Fail-Closed 且零修改**：当修复请求触碰未在 `Authorization` 清单中的文件或 Hunk 时，返回 `ErrOutOfAuthorization`（`out_of_authorization`），`Applied: false`；不消耗修复次数预算（attempts 保持 0），且原文件树不发生任何部分修改（partial modification = 0）。
- **非 safe_fix 模式禁止创建修复 Gate**：在 `off` 等非 `safe_fix` 策略下调用 `NewSafeFixGate`，立即返回 `ErrModeDoesNotAllow`（`mode_does_not_allow`）。

---

## 3. 全量测试结果

执行命令：
```bash
go test -C /tmp/qa-rt-cq ./... -count=1
```
- **Exit Code**: `0`
- **Total Packages**: 18
  - **ok packages**: 17
    - `beacon-go/desktop` (1.840s)
    - `beacon-go/internal/agentapi` (2.550s)
    - `beacon-go/internal/benchgate` (1.159s)
    - `beacon-go/internal/changequality` (1.922s)
    - `beacon-go/internal/cli` (23.108s)
    - `beacon-go/internal/core` (3.019s)
    - `beacon-go/internal/evalgate` (60.149s)
    - `beacon-go/internal/gate` (3.474s)
    - `beacon-go/internal/harness` (15.378s)
    - `beacon-go/internal/llm` (4.172s)
    - `beacon-go/internal/migration` (4.684s)
    - `beacon-go/internal/orchestrator` (101.387s)
    - `beacon-go/internal/progressreview` (4.472s)
    - `beacon-go/internal/session` (4.620s)
    - `beacon-go/internal/store` (3.899s)
    - `beacon-go/internal/stride` (3.930s)
    - `beacon-go/internal/workspace` (5.514s)
    - `beacon-go/skills` (3.849s)
  - **no-test packages**: 1 (`beacon-go/cmd/beacon`)
  - **FAIL count**: 0
  - **panic count**: 0

---

## 4. 验收结论与纪律核验

- **Verdict**: **PASS**
- **只读纪律**:
  - 未执行切分支、commit、push、merge 等写分支行为。
  - 冻结包 `truth.md` / `tests.md` / `tasks.md` / `evidence.md` 及既有 evidence 逐字节未变。
  - 仅写入本文件 `evidence/qa/runtime.md`。
