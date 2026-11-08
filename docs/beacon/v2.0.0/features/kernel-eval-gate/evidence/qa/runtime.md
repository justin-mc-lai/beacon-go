# QA 验收报告 · 运行时行为 (Runtime Behavior)

- **Feature**: `kernel-eval-gate` (v2.0.0)
- **Evaluator**: QA 验收 · agy（视觉/端侧 / 运行时行为）
- **Verdict**: **PASS**
- **Current Stage**: `Freeze/Completed` (truth status: `frozen`, target branch: `feat/kernel-eval-gate`, HEAD `e48bd05`)
- **Evaluation Time**: 2026-11-09T07:42:00+08:00
- **Scope**: 验证 #2 `kernel-eval-gate` 评测集在真实环境下的运行时行为，通过隔离临时 worktree 检出 HEAD `e48bd05` 执行真实测试与 CLI 判定，不修改实现代码、不切主仓库分支、不 commit/push。

---

## 1. 评测集整集实跑与用例覆盖 (AC-001)

- **测试命令**: `go test ./internal/evalgate -count=1 -v`
- **退出码**: `0` (PASS, 耗时 49.656s)
- **RUN 行数**: 29 行 `=== RUN`（1 个顶层 `TestEvalSuite` + 18 个以 Case ID 命名的子测试 + 10 个独立 TC 顶层测试）
- **FAIL / SKIP 统计**: `--- FAIL`: 0, `SKIP`: 0
- **18 个 Case ID 四类覆盖核验**:
  - `stage_gate` (2/2):
    - `TestEvalSuite/stage_gate/implement_blocked_unfrozen_truth` (0.03s, PASS)
    - `TestEvalSuite/stage_gate/implement_pass_complete` (0.11s, PASS)
  - `conformance` (9/9):
    - `TestEvalSuite/conformance/all_covered` (0.58s, PASS)
    - `TestEvalSuite/conformance/missing_tc` (0.56s, PASS)
    - `TestEvalSuite/conformance/missing_evidence_file` (0.46s, PASS)
    - `TestEvalSuite/conformance/evidence_escape` (0.47s, PASS)
    - `TestEvalSuite/conformance/evidence_column_missing` (0.47s, PASS)
    - `TestEvalSuite/conformance/failclosed_tests_missing` (0.55s, PASS)
    - `TestEvalSuite/conformance/failclosed_matrix_unparseable` (0.46s, PASS)
    - `TestEvalSuite/conformance/failclosed_no_ac` (0.48s, PASS)
    - `TestEvalSuite/conformance/not_required_compat` (0.47s, PASS)
  - `rejection` (4/4):
    - `TestEvalSuite/rejection/unknown_gate_type` (0.01s, PASS)
    - `TestEvalSuite/rejection/missing_evaluator` (0.01s, PASS)
    - `TestEvalSuite/rejection/unknown_feature` (0.01s, PASS)
    - `TestEvalSuite/rejection/unknown_run_conformance` (0.01s, PASS)
  - `no_degradation` (3/3):
    - `TestEvalSuite/no_degradation/expectation_mismatch_detected` (1.56s, PASS)
    - `TestEvalSuite/no_degradation/deterministic_twice` (12.50s, PASS)
    - `TestEvalSuite/no_degradation/offline_loopback_blocked` (6.02s, PASS)

---

## 2. 确定性与离线断网验证 (AC-004)

- **测试命令**: `go test ./internal/evalgate -run TestEvalDeterministicOffline -count=1 -v`
- **退出码**: `0` (PASS, 耗时 18.59s)
- **真实输出**:
  ```
  === RUN   TestEvalDeterministicOffline
      suite_test.go:845: determinism: two runs over 16 cases normalized to 6273 identical bytes (volatile run ids: 2)
      suite_test.go:846: offline: 16 cases passed with HTTP(S)_PROXY/ALL_PROXY at 127.0.0.1:1 (16 report entries)
  --- PASS: TestEvalDeterministicOffline (18.59s)
  PASS
  ok  	beacon-go/internal/evalgate	18.957s
  ```
- **关于 16 个 Case 对比范围是否满足 AC-004 的明确结论**:
  - **满足并符合设计要求**。
  - 注册表全部 18 个 Case 中，被排除在 `comparisonCases()` 之外的仅有 `no_degradation/deterministic_twice` 与 `no_degradation/offline_loopback_blocked` 两个 meta-evaluator 用例自身。这是为了防止 meta 自身递归调用陷入死循环。
  - 其余 16 个用例完整覆盖了 `stage_gate` (2)、`conformance` (9)、`rejection` (4) 以及不降级探测 `expectation_mismatch_detected` (1) 的全部内核行为。
  - 两次运行产出的完整报告在剥离可变 run ID 和时间戳归一化后，6273 字节完全一致；在代理强制指向 `127.0.0.1:1` 下 16 个用例全部离线通过，证明内核评测零外部网络依赖。AC-004 判定通过。

---

## 3. 全量测试回归验证 (AC-005)

- **测试命令**: `go test ./... -count=1`
- **退出码**: `0`
- **被测包通过情况**: 15 个包全部 `ok`，0 失败、0 panic：
  - `beacon-go/desktop` (1.283s)
  - `beacon-go/internal/agentapi` (5.111s)
  - `beacon-go/internal/benchgate` (1.566s)
  - `beacon-go/internal/cli` (19.317s)
  - `beacon-go/internal/core` (2.191s)
  - `beacon-go/internal/evalgate` (53.207s)
  - `beacon-go/internal/gate` (2.635s)
  - `beacon-go/internal/harness` (13.616s)
  - `beacon-go/internal/llm` (3.826s)
  - `beacon-go/internal/migration` (4.050s)
  - `beacon-go/internal/orchestrator` (92.861s)
  - `beacon-go/internal/session` (4.285s)
  - `beacon-go/internal/store` (4.074s)
  - `beacon-go/internal/workspace` (5.572s)
  - `beacon-go/skills` (3.672s)

---

## 4. 抽查非空壳实现实证

- **真实引擎 API 驱动**:
  - `newFixtureWithStatus` 在 `t.TempDir()` 下为每个用例启动独立的 SQLite 数据库（`store.EnvStorePath` 绑定），初始化真实 `store.Store` 与 `orchestrator.NewEngine`。
  - 真实步进 run 状态机：`CreateRun` -> `Tick` -> `RegisterArtifact` -> `AttachVerifier` -> `Complete` -> `EvaluateGate` -> `ExportProjectState` -> `Conformance`，非 mock 内存假数据。
- **真实 CLI 进程调用**:
  - `runBeaconCLI` 在同一进程空间内调用 `cli.NewRootCmd().Execute()`，通过 `store.EnvStorePath` 共享同一 DB，实测 `goal conformance --run-id <runID>` 与 `--json` 参数。
- **负向拦截精准点名与原因码**:
  - `conformance/missing_tc`: 文本与 `--json` 同步非 0 退出，精准携带 `reason_codes: ["traceability_missing_tc"]` 并点名 `missing: ["AC-002"]`。
  - `conformance/missing_evidence_file` / `evidence_escape` / `evidence_column_missing`: 精准非 0 退出并携带 `traceability_missing_evidence`。
  - `conformance/failclosed_*`: tests.md 缺失、矩阵不可解析、无 AC 定义时全部 fail-closed 退出非 0，绝不输出 pass 或 not_required。
  - `stage_gate/implement_blocked_unfrozen_truth`: truth 为 draft 时精准拦截 implement 阶段准入，原因码为 `stage_exit_truth_not_frozen`。
  - `rejection/*`: 未知 gate 类型报 `gate_type_unknown`、缺 evaluator 报 `gate_evaluator_required`、未知 feature 报 `feature_not_found`、未知 run 报非 0，且均未记录 pass 判定。
  - `no_degradation/expectation_mismatch_detected`: 篡改期望时立即通过 `Check` 报出 `case=<ID> expected=... actual=...` 并使评测非 0 退出，绝不吞错。

---

## 5. 验收结论

- **HEAD**: `e48bd05` (`e48bd05064af4dd31335d6455c8df290588d8c00`)
- **current_stage**: `Freeze/Completed`
- **Verdict**: **PASS**
