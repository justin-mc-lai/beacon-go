# QA Verdict: kernel-eval-gate — spec/AC 独立验收

- **verdict**: **PASS**
- **current_stage**: `Freeze/Completed`（truth `status: frozen`、`traceability_required: true`、13 条 TC）
- **复核 HEAD**: `e48bd05064af4dd31335d6455c8df290588d8c00`（分支 `feat/kernel-eval-gate`；main 验收时为 `b277d8c`，分支未合并、未 push）
- **验收方**: 验收员（独立，maker≠checker）
- **验收方式**: 临时 worktree 检出 `e48bd05` 只读复验；未切分支、未 commit/push、未改实现、未动 `kernel-truth-traceability` 包；唯一写入为本文件。

## 结论

冻结 AC-001～005 在 `e48bd05` 上全部成立；13 条 TC 与 AC 一一对应，Command 全部真实可跑；注册表 18 个具名用例四类齐全，实现走真实引擎/CLI，不是空壳；没有任何冻结范围外的代码扩张。P1/P2 为空，P3 四条不影响判定。

## 逐条 AC 结论

| AC | 结论 | 实跑依据 |
|----|------|----------|
| AC-001 | 成立 | `TestEvalRegistryCoverage` 实跑通过：18 用例 / `stage_gate` 2、`conformance` 9、`rejection` 4、`no_degradation` 3；`Validate` 对未知 class、重复 ID、缺期望三种坏注册表均非 0 且错误里含 `case=<ID>`。`TestEvalSuite` 单条命令 `go test ./internal/evalgate -count=1` 跑完 18 个以用例 ID 命名的子测试，`=== RUN` 19 行（1 个 Suite + 18 子测试）、PASS 19、FAIL 0、SKIP 0。 |
| AC-002 | 成立 | 正向 `conformance/all_covered`：exit 0、`ac_tc_evidence_coverage` pass。缺 TC：exit 1、`traceability_missing_tc`、点名 `AC-002`，并同库校验 CLI 文本与 `--json` 同码同 ID。证据三态：文件不存在 / `../` 越界 / 无 Evidence 列，均 exit 1、`traceability_missing_evidence`、点名 `AC-xxx/TC-xxx`。三条 fail-closed 均 exit 1、带 reason code、不出 pass/not_required。历史包 `not_required`：四种既有判定 `truth_frozen / gates_evaluated / verifier_attached / state_exported` 在测试里逐个断言为 pass，退出语义不变。 |
| AC-003 | 成立 | `TestEvalStageGate` 实跑：truth 为 draft 时 truth→implement 出口 blocked、`stage_exit_truth_not_frozen`（与 AC 文本的“implement 门”边界一致）；truth 冻结则 exit 0、状态 pass 且实际到达 `StageImplement`。`TestEvalRejectionPaths`：`gate_type_unknown`、`gate_evaluator_required`、`feature_not_found`、`product_goal_run_not_found` 四例均 exit 1，且 `assertNoGateRows` 证明未写入任何 gate 行。 |
| AC-004 | 成立（见 P3-1） | 注入四类错期望（期望失败却成功、期望成功却失败、错 reason code、输出不可解析）后评测集整体非 0，三条错误均含 `case=`/`expected=`/`actual=`，且显式断言不出现 skip。确定性：两次运行的归一化报告逐字节一致（6273 字节），`HTTP_PROXY/HTTPS_PROXY/ALL_PROXY=127.0.0.1:1` 下 16 个业务用例全通过。 |
| AC-005 | 成立 | 我在 `e48bd05` 自己跑 `go test ./... -count=1`：**EXIT=0**，15 个测试包全 `ok`（含 `internal/evalgate` 53.8s），`cmd/beacon` 无测试文件。TC-013 选择器实跑 4 个 RUN / 4 个 PASS（三个提案门 + `TestDeliveryParadigmEndToEnd`）。`git diff b277d8c..e48bd05 -- '*_test.go'` 只有 2 个 `A`（`internal/evalgate/fixture_test.go`、`suite_test.go`），既有测试零改写。 |

## AC ↔ TC 对照核对

tests.md 13 行，Command 逐条 grep 到真实测试函数，无空 Command/Assertion/Evidence，无缺席：

| AC | TC | 测试函数（实存） |
|----|----|------------------|
| AC-001 | TC-001 / TC-002 | `TestEvalSuite` / `TestEvalRegistryCoverage` |
| AC-002 | TC-003～007 | `TestEvalConformancePositive` / `…MissingTC` / `…Evidence` / `…FailClosed` / `…NotRequiredCompat` |
| AC-003 | TC-008 / TC-009 | `TestEvalStageGate` / `TestEvalRejectionPaths` |
| AC-004 | TC-010 / TC-011 | `TestEvalMismatchFailsLoudly` / `TestEvalDeterministicOffline` |
| AC-005 | TC-012 / TC-013 | `go test ./... -count=1` / 四测试两包选择器 |

18 个注册用例与 truth.md「用例目录」逐行一致（ID、Fixture 描述、期望），无「集合跑通」冒充单条 AC：TC-003～011 每条都有自己的选择器与针对性子测试，且各自独立跑过一次 exit 0。

## 任务与索引核对

- `tasks.md`：TASK-000～013 全部 `[x]` 且 `evidence=` 指向的文件真实存在（14 个路径逐一 `-f` 校验，全部 OK）；TASK-014（三份独立 QA verdict + 索引更新）保持未勾，正确——maker 不自签。
- `evidence.md`：20 行 artifact 与包内真实文件逐一对应，状态 `recorded` 与文件存在一致；`qa/spec-ac.md`、`qa/runtime.md`、`qa/regression.md` 三行写法**正确**——索引 artifact 一律相对 `evidence/` 目录，解析后即 `evidence/qa/spec-ac.md` 等真实路径，与其余行（如 `evalgate-suite.txt`）同一约定；三行 `planned` 也正确，verdict 落库后才转 `recorded`。

## 需求扩张检查

`git diff b277d8c..e48bd05`：18 个文件，新增仅 `internal/evalgate/{registry.go, fixture_test.go, suite_test.go}`，其余为 #2 包内 evidence/tasks。`-- cmd` 为 0 行，未新增 CLI 子命令，未改 gauge/gate 语义，未改 `truth.md` / `tests.md`（0 字节差异），未改任何既有测试文件。

## 问题分级

- **P1**：无。
- **P2**：无。
- **P3-1（不阻断）**：AC-004 的「连续两次归一化报告一致」比较的是 18 个用例中的 16 个，排除了 `deterministic_twice` 与 `offline_loopback_blocked` 两个 meta 用例自身。这是避免自引用的必要处理，16 个用例已覆盖全部业务与错误路径，判定成立；建议下次改 tests.md 时把该口径写进 Assertion，或让报告显式列出被排除项。
- **P3-2（不阻断）**：TC-006 的 Assertion 写「文本与 --json 不出现 pass 与 not_required」，实现里只有 `failclosed_tests_missing` 一例走了 CLI 文本/`--json` 断言，另两例是引擎级断言。AC-002 只要求 CLI 驱动的用例校验两路输出，AC 级结论不受影响；建议收窄 Assertion 措辞或补齐另两例的 CLI 校验。
- **P3-3（不阻断）**：TASK-013 的 `evidence=` 只绑定 `go-test-all.txt`，而该任务还覆盖 13 条 TC 与 AC-009 回归产物。tests.md 侧 TC-013 已正确绑定 `ac009-regression.txt`，是台账单值绑定的表达限制。
- **P3-4（不阻断）**：`Case.Fixture` 是中文描述串而非可执行构造器指针，「fixture 构造方式」由 `executorFor` 映射承担，语义可追溯，非缺陷。

## 未验证 / 保留

- 本次只读复验绑定 `e48bd05`；分支尚未合并，合并后建议在 main 上再跑一次全量与整集（预期无差异，代码路径同字节）。
- 未验证 release/版本号与对外说明——按规范归人类确认，不属本 verdict 范围。
