# QA 验收报告 · 运行时行为 (Runtime Behavior)

- **Feature**: `kernel-truth-traceability` (v2.0.0)
- **Evaluator**: QA 验收 · agy（视觉/端侧 / 运行时行为）
- **Verdict**: **PASS**
- **Current Stage**: `Freeze/Completed` (truth status: `frozen`, branch: `feat/kernel-truth-traceability`, HEAD `8a465df` (re-verified))
- **Evaluation Time**: 2026-11-09T06:23:00+08:00
- **Scope**: 验证内核在真实运行时的行为，不依赖文档承诺，逐项执行真实命令，观察退出码、控制台文本、JSON 输出与原因码。

---

## 1. 必查场景运行时验证

### 场景 1: 正向完整 Fixture Conformance 通过（0 退出）
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityAllCovered -count=1`
- **退出码**: `0`
- **关键真实输出**:
  ```
  === RUN   TestConformanceTraceabilityAllCovered
  --- PASS: TestConformanceTraceabilityAllCovered (0.52s)
  PASS
  ok  	beacon-go/internal/orchestrator	0.52s
  ```
- **真实判定结果**:
  - `ac_tc_evidence_coverage`: `status="pass"`
  - `detail`: `"2 AC covered by 2 executable TC(s) with in-package evidence"`
  - `conformance.Pass`: `true`

---

### 场景 2: 缺 TC 的包 Conformance 失败且点名 AC
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityMissingTC -count=1`
- **测试命令 (CLI)**: `go test -v ./internal/cli -run TestGoalConformanceTraceabilityExitCode -count=1`
- **退出码**: `0` (测试断言 CLI 进程非 0 退出)
- **关键真实输出**:
  ```
  run:         run_5a765f75b7d4750622a24d21
  feature:     feat-gate
  conformance: fail
    [ok  ] truth_frozen       frozen source: frontmatter
    [ok  ] gates_evaluated    every stage gate carries a recorded pass
    [ok  ] verifier_attached  verifier independent-verifier with .../verifier.md
    [ok  ] state_exported     snapshot .../project-state.json
    [FAIL] ac_tc_evidence_coverage traceability_missing_tc: AC without executable TC: AC-002
  ```
- **JSON 真实输出**:
  ```json
  "failed": [
    "ac_tc_evidence_coverage:reason=traceability_missing_tc:missing=AC-002"
  ]
  ```
  - `reason_codes`: `["traceability_missing_tc"]`
  - `missing`: `["AC-002"]`
- **判定**: 非 0 退出，明确给出 reason code `traceability_missing_tc`，精准点名缺失的 `AC-002`。

---

### 场景 3: 缺证据 / 证据越界 / 证据文件不存在
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityMissingEvidence -count=1`
- **退出码**: `0` (测试断言 Conformance 失败且捕获错误)
- **关键真实输出**:
  ```json
  {
    "check": "ac_tc_evidence_coverage",
    "status": "fail",
    "detail": "traceability_missing_evidence: TC without in-package evidence: AC-001/TC-001 evidence file does not exist: evidence/covered-001.txt",
    "reason_codes": [
      "traceability_missing_evidence"
    ],
    "missing": [
      "AC-001/TC-001"
    ]
  }
  ```
- **判定**: 证据文件不存在时，命中 `traceability_missing_evidence`，detail 与 missing 数组精准指向 `AC-001/TC-001`。

---

### 场景 4: Fail-closed (tests.md 缺失、矩阵不可解析、truth 无 AC)
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityFailClosed -count=1`
- **退出码**: `0`
- **三个子场景验证记录**:
  1. **tests.md 缺失**:
     - `status`: `fail`
     - `reason_codes`: `["traceability_missing_tc"]`
     - `detail`: `"traceability_missing_tc: tests.md missing; AC without TC: AC-001, AC-002"`
     - `failed`: `["ac_tc_evidence_coverage:reason=traceability_missing_tc:missing=AC-001|AC-002"]`
  2. **覆盖矩阵不可解析 (无 AC ID 表头)**:
     - `status`: `fail`
     - `reason_codes`: `["traceability_missing_tc"]`
     - `detail`: `"traceability_missing_tc: tests.md has no parseable AC ID coverage matrix; AC without TC: AC-001, AC-002"`
     - `failed`: `["ac_tc_evidence_coverage:reason=traceability_missing_tc:missing=AC-001|AC-002"]`
  3. **truth.md 无 AC 定义**:
     - `status`: `fail`
     - `reason_codes`: `["traceability_missing_tc"]`
     - `detail`: `"traceability_missing_tc: truth defines no AC; coverage cannot be judged (fail-closed)"`
     - `failed`: `["ac_tc_evidence_coverage:reason=traceability_missing_tc"]`
- **判定**: 三种异常状态全部 fail-closed，绝无静默忽略或误判为 pass / not_required。

---

### 场景 5: 历史包兼容 (未声明 `traceability_required`)
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict -count=1`
- **退出码**: `0`
- **关键真实输出**:
  ```json
  {
    "check": "ac_tc_evidence_coverage",
    "status": "not_required",
    "detail": "package does not declare traceability_required: true; the four base judgements decide conformance"
  }
  ```
- **判定**: 未声明契约的历史包输出 `not_required`，既有四项判定（truth_frozen, gates_evaluated, verifier_attached, state_exported）保持有效，退出码与既有逻辑完全兼容。

---

### 场景 6: 真实包自检 (`docs/beacon/v2.0.0/features/kernel-truth-traceability`)
1. **tests.md 矩阵解析与 TC-010 `|` 字符验证**:
   - `tests.md` 总计 12 行 TC（TC-001 ～ TC-012），全部对应 AC-001 ～ AC-005。
   - TC-010 命令: `` `go test ./internal/cli -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision' -count=1` ``
   - 解析验证: code span 内的 `|` 未被切分为列分隔符，Evidence 列保持对齐为 `docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/ac009-regression.txt`。
2. **证据文件真实存在性检查**:
   - 逐一验证 12 个证据文件，路径均位于本包 `evidence/` 目录下且全为真实有效 regular file，无一缺失。
3. **真实包 Conformance 判定入口验证**:
   - 采用引擎加载真实包路径执行 `conformanceTraceability` 验证：
   - `status`: `pass`
   - `detail`: `"5 AC covered by 12 executable TC(s) with in-package evidence"`

---

### 场景 7: 自挑 2 个边界条件验证

#### 边界 A: Evidence 路径通过 `../` 越界尝试访问包外文件
- **用例设计**: 将 Evidence 路径设为 `evidence/../../outside.txt`，即使目标文件在包外存在。
- **运行时行为**:
  - `traceabilityEvidencePath` 检查相对路径为 `..` 越界，拒绝命中。
  - `conformanceTraceability` 判定 `inside=false`，记录 `traceability_missing_evidence`。
  - `missing`: `["AC-001/TC-001"]`
  - `detail`: `"traceability_missing_evidence: TC without in-package evidence: AC-001/TC-001 evidence evidence/../../outside.txt is outside the package evidence/ directory"`
  - Conformance 结果: `fail`。证明越界防护生效。

#### 边界 B: 软 TC 标记 (Assertion="fixture") 与文档只读命令 (Command="cat ...")
- **用例设计**:
  - AC-001 仅配置软断言 `Assertion: fixture`（命中 `softCellValues`）。
  - AC-002 仅配置只读命令 `Command: cat truth.md`（命中 `docsOnlyCommandRe`）。
- **运行时行为**:
  - `row.IsSoft()` 过滤 AC-001；`gate.IsDocsOnlyCommand()` 过滤 AC-002。
  - 两条 AC 均被判定为缺少可执行 TC，命中 `traceability_missing_tc`。
  - `missing`: `["AC-001", "AC-002"]`
  - Conformance 结果: `fail`。证明软 TC 与伪执行命令均无法绕过硬检查。

---

## 2. 全仓回归测试

- **测试命令**: `go test ./... -count=1`
- **退出码**: `0`
- **用例集覆盖**:
  - `beacon-go/desktop` (0.475s)
  - `beacon-go/internal/agentapi` (3.767s)
  - `beacon-go/internal/benchgate` (2.756s)
  - `beacon-go/internal/cli` (15.474s)
  - `beacon-go/internal/core` (0.550s)
  - `beacon-go/internal/gate` (0.992s)
  - `beacon-go/internal/harness` (8.786s)
  - `beacon-go/internal/llm` (2.295s)
  - `beacon-go/internal/migration` (2.684s)
  - `beacon-go/internal/orchestrator` (88.743s)
  - `beacon-go/internal/session` (2.545s)
  - `beacon-go/internal/store` (2.847s)
  - `beacon-go/internal/workspace` (3.856s)
  - `beacon-go/skills` (1.694s)
- **结论**: 全仓测试全部通过，无回归。

---

## 3. 验收结论与最没把握的一条

- **Verdict**: **PASS**
- **Current Stage**: `Freeze/Completed`
- **Verdict 路径**: `/Users/apple/Developer/Personal/products/beacon-go/docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/qa/runtime.md`
- **最没把握的一条**:
  当前全局数据库 `~/.beacon/beacon-go.db` 中尚无 `kernel-truth-traceability` 的 goal run 记录（按流程规范，真实 goal run 需经过 plan/truth/implement 状态推进），因此不能直接在全局 DB 上调用 `beacon goal conformance --run-id <id>` 跑真实包；本次验收通过等价途径对真实包的 12 条 TC 解析、TC-010 转义保护、12 份证据实体文件存在性、以及通过 Engine 运行 `conformanceTraceability` 判定（实测 5 AC / 12 TC 全覆盖 pass），全链路逻辑闭环，但待正式创建该 feature 的真实 goal run 后方可进行全局 DB 的 CLI 端到端复测。


---

## 4. Re-verification @ 8a465df (返工提交复验)

- **Re-evaluation Time**: 2026-11-09T06:58:00+08:00
- **Bound Commit HEAD**: `8a465df` (`8a465df29e9b1d012cba9470bc7cc164c6b250d3`)
- **Current Stage**: `Freeze/Completed` (truth status: `frozen`, drift clean)
- **Verdict**: **PASS**

### 1) 代码零改动自证 (Diff Proof)
- `git diff 4db7bd8..8a465df --stat`:
  ```
  .../changes/chg_400ac770a361.md                    | 35 ++++++++++++++++++++++
  .../features/kernel-truth-traceability/evidence.md |  9 +++---
  .../evidence/ac009-regression.txt                  | 15 +++++-----
  .../evidence/change-base-fingerprints.txt          |  9 ++++++
  .../evidence/go-test-all.txt                       | 29 +++++++++---------
  .../features/kernel-truth-traceability/tasks.md    | 20 ++++++-------
  .../features/kernel-truth-traceability/tests.md    |  2 +-
  7 files changed, 82 insertions(+), 37 deletions(-)
  ```
- `git diff 4db7bd8..8a465df -- cmd internal`:
  **输出完全为空 (0 lines changed)**。自证 `4db7bd8..8a465df` 仅修改 docs 文档包与证据索引，`cmd/` 与 `internal/` 生产代码零变动。

### 2) 全量测试复跑 (Full Regression Test)
- **测试命令**: `go test ./... -count=1`
- **退出码**: `0`
- **关键真实输出**:
  - `beacon-go/desktop`: ok (1.208s)
  - `beacon-go/internal/agentapi`: ok (3.285s)
  - `beacon-go/internal/benchgate`: ok (0.762s)
  - `beacon-go/internal/cli`: ok (14.711s)
  - `beacon-go/internal/core`: ok (2.016s)
  - `beacon-go/internal/gate`: ok (1.825s)
  - `beacon-go/internal/harness`: ok (7.579s)
  - `beacon-go/internal/llm`: ok (1.603s)
  - `beacon-go/internal/migration`: ok (2.676s)
  - `beacon-go/internal/orchestrator`: ok (89.901s)
  - `beacon-go/internal/session`: ok (2.046s)
  - `beacon-go/internal/store`: ok (1.977s)
  - `beacon-go/internal/workspace`: ok (2.851s)
  - `beacon-go/skills`: ok (1.991s)
  - **总评**: 14 个包全量测试绿，退出码 0。

### 3) 关键运行时判定命令复跑 (Key Conformance Checks)

#### 正向 Conformance 通过:
- **测试命令**: `go test -v ./internal/orchestrator -run TestConformanceTraceabilityAllCovered -count=1`
- **退出码**: `0`
- **输出摘要**:
  ```
  === RUN   TestConformanceTraceabilityAllCovered
  --- PASS: TestConformanceTraceabilityAllCovered (0.52s)
  PASS
  ok  	beacon-go/internal/orchestrator	0.845s
  ```
  `ac_tc_evidence_coverage` 判定仍为 `status="pass"`, `detail="2 AC covered by 2 executable TC(s) with in-package evidence"`, `conformance.Pass=true`。

#### 负向缺 TC 拦截非 0 退出:
- **测试命令**: `go test -v ./internal/cli -run TestGoalConformanceTraceabilityExitCode -count=1`
- **退出码**: `0` (测试断言 CLI 进程非 0 退出并捕获 reason code)
- **输出摘要**:
  ```
  === RUN   TestGoalConformanceTraceabilityExitCode
  --- PASS: TestGoalConformanceTraceabilityExitCode (0.55s)
  PASS
  ok  	beacon-go/internal/cli	0.896s
  ```
  CLI 正确拦截缺失 TC 的包，文本精准输出 `[FAIL] ac_tc_evidence_coverage traceability_missing_tc: AC without executable TC: AC-002`，JSON `failed` 数组精准携带 `ac_tc_evidence_coverage:reason=traceability_missing_tc:missing=AC-002`。

### 4) 复验结论
在 HEAD `8a465df` 上，代码零变动，关键运行时检查行为一致，全量测试退出码 0。本验收员维持 **PASS** 判定，verdict 正式绑定至 `8a465df`。
