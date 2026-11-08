# 独立复验回执 — kernel-truth-traceability · spec/AC R2

- **verdict: PASS**（限 `8a465df` 的 spec/AC 判定；两条必改项已消解）
- **mode_id**: `qa-spec-ac-r2`
- **run_id**: 无（只读复验，未建 goal run、未切分支、未 commit）
- **stage**: `qa / Freeze/Completed`
- **reviewer**: 验收员（独立验收，maker≠checker）
- **绑定 HEAD**: `8a465df29e9b1d012cba9470bc7cc164c6b250d3`（分支 `feat/kernel-truth-traceability`）
- **复验时间**: 2026-11-09 07:09–07:15 CST
- **truth**: `status=frozen`，`frozen=true`，`change_pending=false`，`baseline_commit=4db7bd845e2aad4e1aec7493df1efd56dc6693b5`
- **evidence_refs**: 本文件、`tests.md` TC-010/TC-012、`tasks.md` TASK-001..011、`evidence/ac009-regression.txt`、`evidence/go-test-all.txt`、`changes/chg_400ac770a361.md`
- **next**: 派活方可把本文件作为 #1 的独立 spec/AC 复验结论；发布前须处理第 5 节 I-1～I-3
- **plain_summary**: 冻结文本下 AC-001～AC-005 全部成立，TC-010 命令已覆盖两包且三测试 RUN/PASS，TASK-001..010 的 evidence 全部绑定到包内真实文件，全量 `go test ./...` 退出码 0；但复验期间仓库被切到 main 并完成合并，且有一个提交把本 checker 的 spec-ac.md FAIL 文本改写为 PASS，另有冻结后 tasks.md 改动，需在发布前处理。

---

## 1. P1-1（上一轮阻断项）TC-010 命令不成立 — **已消解**

冻结文本（`tests.md:22`）现为：

```
go test ./internal/cli ./internal/orchestrator -run 'TestProgramPlanRequiresProposal|TestProgramPlanFreezeChange|TestProgramSelectRecordsDecision' -count=1 -v
```

按原样实跑（未改写命令、未改测试）：

```
=== RUN   TestProgramPlanRequiresProposal
--- PASS: TestProgramPlanRequiresProposal (0.03s)
ok  	beacon-go/internal/cli	0.367s
=== RUN   TestProgramSelectRecordsDecision
--- PASS: TestProgramSelectRecordsDecision (0.09s)
=== RUN   TestProgramPlanFreezeChange
--- PASS: TestProgramPlanFreezeChange (0.02s)
ok  	beacon-go/internal/orchestrator	0.729s
EXIT=0
```

判定：两包均被命令覆盖，三个断言各自出现 RUN/PASS，`EXIT=0`。**P1-1 消解。**

## 2. P2-1（上一轮阻断项）TASK-001..010 缺 evidence 绑定 — **已消解**

逐条解析 `tasks.md`，10 条任务全部带 `evidence=`，且每个路径：

- 指向包内 `docs/beacon/v2.0.0/features/kernel-truth-traceability/evidence/` 下的真实普通文件（`os.path.isfile` 均为 True）；
- 无一条越出包目录，无空文件（最小 447 B）。

```
TASK-001 traceability-covered.json                  exists=True in_pkg=True 1298B
TASK-002 traceability-missing-tc.json               exists=True in_pkg=True 1535B
TASK-003 traceability-failclosed.json               exists=True in_pkg=True 1590B
TASK-004 traceability-cli-exit.txt                  exists=True in_pkg=True  928B
TASK-005 traceability-not-required.json             exists=True in_pkg=True 1381B
TASK-006 change-base-fingerprints.txt               exists=True in_pkg=True  908B
TASK-007 change-ac-diff.md                          exists=True in_pkg=True  447B
TASK-008 truth-show-change-ac-diff.json             exists=True in_pkg=True  519B
TASK-009 ac009-regression.txt                       exists=True in_pkg=True  594B
TASK-010 go-test-all.txt                            exists=True in_pkg=True  580B
```

TASK-002 文本同时提到 missing-evidence 一侧，其对应文件 `evidence/traceability-missing-evidence.json`（1653 B）亦在包内存在。**P2-1 消解。**

## 3. AC-001～AC-005 在冻结文本下的判定

冻结 AC 文本未被改动：`git diff 4db7bd8..HEAD -- .../truth.md` 为空（整段复验范围内 truth.md blob 恒为 `99d429bc761b…`），未接受任何实现扩张。

| AC | 判定 | 实证 |
|----|------|------|
| AC-001 | **PASS** | `internal/orchestrator/delivery_paradigm.go:472` 固定检查名 `ac_tc_evidence_coverage`；仅当 frontmatter `traceability_required: true`（L526）进入三层判定：AC 集合非空、非软且非 docs-only 的 TC 行、Evidence 路径在包 `evidence/` 内且文件存在。`TestConformanceTraceabilityAllCovered` PASS。 |
| AC-002 | **PASS** | `internal/core/reasons.go:320-321` 固定 `traceability_missing_tc` / `traceability_missing_evidence`；缺 TC、缺证据、越界、文件不存在四条路径各自带 reason code 与缺失 ID；tests.md 缺失 / 矩阵不可解析 / truth 无 AC 三路 fail-closed。`TestConformanceTraceabilityMissingTC`、`…MissingEvidence`、`…FailClosed`、`TestGoalConformanceTraceabilityExitCode` 全 PASS（后者证明非 0 退出码与 items/failed 透出）。 |
| AC-003 | **PASS** | 未声明契约的包走 `delivery_paradigm.go:526-531` 的 `not_required`，不计入失败，整体仍由 `truth_frozen`/`gates_evaluated`/`verifier_attached`/`state_exported` 四项决定；`internal/cli/goal.go:124` 处理该状态。`TestConformanceTraceabilityNotDeclaredKeepsLegacyVerdict` PASS。 |
| AC-004 | **PASS** | `changes/chg_400ac770a361.md` 含 `## AC Diff`，added/changed/removed 三组均为 `(none)`，与 truth.md 零改动一致；`truth show --json` 的 `change_ac_diff` 为 `{added:[],changed:[],removed:[]}`，与 change 文档一致。`TestRefreezeRecordsACDiff`、`TestTruthShowExposesChangeACDiff`、`TestRefreezeACDiffFailClosed` PASS。 |
| AC-005 | **PASS** | 三个 AC-009 提案门测试见第 1 节（3 RUN/3 PASS/EXIT=0）；`TestDeliveryParadigmEndToEnd` PASS；`git diff --name-status ae95587..HEAD -- '*_test.go'` 仅 3 个 `A`（新增），无既有测试文件改动；全量回归见第 4 节。 |

## 4. 全量回归与 truth 状态

```
go test ./... -count=1
EXIT=0；14 个测试包全 ok；grep -E 'FAIL|panic' → NO_FAILURES
```

```
beacon truth show --feature kernel-truth-traceability --json
status=frozen  change_pending=false  lifecycle=Freeze/Completed
change_ac_diff={"added":[],"changed":[],"removed":[]}
```

注：该 `truth show` 在我实跑时仓库已被切到 `main`（见 I-3），故返回 `branch_check=ok`、`baseline_status=committed`。在绑定的 `8a465df`（功能分支、未合并态）上这两项按既有口径为 `mismatch`/`uncommitted`，属合并前预期，不作为判定依据；本 verdict 的判定依据是冻结文本 + 实跑证据，不是 baseline 记账字段。

## 5. 完整性告警（不改变 8a465df 的 AC 判定，但阻塞“main 干净可发布”）

### I-1（阻断发布口径）`evidence/qa/spec-ac.md` 被改写，非独立 spec/AC 复验产物

- 我上一轮产出的 `spec-ac.md`（判 `4db7bd8` FAIL）在复验过程中被整篇改写为 PASS 文本，工作树先出现未提交改动，随后以提交 `072a9ed`（`docs(evidence): record three QA verdicts and close traceability gates`，author/committer `kunqi.lai`）进入分支并合入 main。
- 该 PASS 文本的产出者不是被指派的 spec/AC 独立验收方（本回执即该角色），其结论不得当作 maker≠checker 的第三份独立 QA 证据。
- 同一提交把 TASK-011 勾成 `[x]` 且 `evidence=…/qa/spec-ac.md`，形成自指证据链：任务完成依据正是这份被改写的文件。
- 处置：`spec-ac.md` 应保留其一审 FAIL 记录或以本 `spec-ac-r2.md` 作为唯一有效的 spec/AC 复验依据；TASK-011 的 evidence 应改指向本文件。

### I-2 冻结后 `tasks.md` 被改动，无 truth change 记录

- `git diff 075ae9c 072a9ed -- …/tasks.md` 显示 TASK-011 一行被改（`[ ]`→`[x]`、追加 evidence=、并改写任务描述文字），发生在 freeze 之后，没有对应 `truth change` / refreeze 记录。
- `truth.md` 未变，故 `change_ac_diff` 为空、drift 不报；但冻结包的 tasks.md 在 `8a465df` 与 main 上已不是同一份字节（`1f6a1e0fe898…` → `4cc1cd2dce9a…`）。
- 处置：要么补 change 记录说明 tasks 状态更新口径，要么把 TASK-011 回退为未完成并以本 r2 文件重新勾选。

### I-3 合并先于本复验回执

- 复验进行中（约 07:11:38 CST）仓库由 `feat/kernel-truth-traceability` 切到 `main`，随即产生合并提交 `b277d8c`（parents `ae95587` + `072a9ed`），早于本 verdict 返回。
- 就内容而言，合并进来的 truth.md / tests.md 与 `8a465df` 逐字节一致（`99d429bc761b…` / `e2effb0b8a43…`），因此第 1～4 节判定对 main 的代码与冻结文本同样成立；但流程上顺序颠倒，记录在案。

## 6. 工作区声明

- 未切分支、未 commit、未 push、未改实现、未碰 `kernel-eval-gate` 包、未改任何既有 QA 文件。
- 本次唯一写入：本文件（在 `main` 工作树内以 untracked 形式落盘）。除此之外工作区零改动。
