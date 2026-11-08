# QA 验收 · 回归 — kernel-truth-traceability（#1）

- verdict: **PASS**
- current_stage: `Freeze/Completed`（truth: `frozen`；分支 `feat/kernel-truth-traceability`；HEAD `4db7bd8`；main 仍 `ae95587`）
- 角色: QA 验收 · 回归（grok 独立）；maker≠checker
- 验收对象: `/Users/apple/Developer/Personal/products/beacon-go` `ae95587..4db7bd8`
- 判定口径: 独立跑真实命令；坏包全部**自造**（未使用制造方遗留夹具）；负向必须非 0 且 reason code 正确。

## 0. 结论

- 全量 `go test ./... -count=1`：**退出码 0**（自己跑，不依赖他人 go-test-all.txt）。
- AC-009 三个提案门 + delivery-paradigm 端到端：**原样通过**，未改动测试文件。
- 历史包（未声明 `traceability_required`）：旧 `ae95587` 四项判定全 pass、`pass=true`；新 `4db7bd8` 同样四项全 pass、`pass=true`，额外一项 `not_required`。**判定与退出码一致，无回归**。
- 自造 3 个坏包：缺 TC / 证据文件不存在 / 证据路径越界，**全部 fail 且 reason code 正确、点名 AC**；自造好包 **pass**。
- CLI 真实退出码：好包 `0`；坏包非 0，文本与 `--json` 都带 reason code 与 AC id。
- 代码 diff 审查：仅新增判定/字段，未改动其它门的退出码或既有 JSON 字段（新增字段均 `omitempty`）。

## 1. 全量回归（真实退出码）

命令：
```
cd /Users/apple/Developer/Personal/products/beacon-go
go test ./... -count=1
```
真实结果：`EXIT=0`
```
ok  beacon-go/desktop 0.706s
ok  beacon-go/internal/agentapi 4.920s
ok  beacon-go/internal/benchgate 1.005s
ok  beacon-go/internal/cli 17.531s
ok  beacon-go/internal/core 1.830s
ok  beacon-go/internal/gate 1.638s
ok  beacon-go/internal/harness 12.435s
ok  beacon-go/internal/llm 2.320s
ok  beacon-go/internal/migration 3.676s
ok  beacon-go/internal/orchestrator 93.725s
ok  beacon-go/internal/session 3.496s
ok  beacon-go/internal/store 3.843s
ok  beacon-go/internal/workspace 4.404s
ok  beacon-go/skills 3.692s
?   beacon-go/cmd/beacon [no test files]
```

## 2. AC-009 三个提案门 + delivery-paradigm 端到端

命令与结果（真实 `exit=0`）：
```
== ./internal/cli ==
go test ./internal/cli -run 'TestProgramPlanRequiresProposal|TestValidateLakeProposal$|TestValidateLakeProposalSize|TestValidateLakeProposalCommercialFields|TestProgramPlanFromProposalRefusesUnselectedDependency|TestProgramRouteShowsProposalCounts' -count=1
ok  beacon-go/internal/cli 0.577s   exit=0

== ./internal/orchestrator ==
（同上 -run 过滤）
ok  beacon-go/internal/orchestrator 0.518s   exit=0

go test ./internal/orchestrator -run 'TestDeliveryParadigmEndToEnd' -count=1 -v
--- PASS: TestDeliveryParadigmEndToEnd (0.55s)
ok  beacon-go/internal/orchestrator 0.868s   EXIT=0
```
提案门分布：`TestProgramPlanRequiresProposal` 在 `./internal/cli`；`TestValidateLakeProposal`/`Size`/`CommercialFields`、`TestProgramPlanFromProposalRefusesUnselectedDependency`、`TestProgramRouteShowsProposalCounts` 在 `./internal/orchestrator`。全部实跑通过。

## 3. 历史包兼容：旧 ae95587 对照

在同一最小历史包（single AC、无 `traceability_required`、无 Evidence 列）上，分别在 `ae95587` 与 `4db7bd8` 的导出树用同一交付流程跑 `goal conformance`：

旧 `ae95587`：
```
OLD pass=true failed=[]
OLD item check=truth_frozen status=pass
OLD item check=gates_evaluated status=pass
OLD item check=verifier_attached status=pass
OLD item check=state_exported status=pass
```
新 `4db7bd8`：
```
HISTORICAL status=not_required pass=true failed=[]
NEW item check=truth_frozen status=pass
NEW item check=gates_evaluated status=pass
NEW item check=verifier_attached status=pass
NEW item check=state_exported status=pass
NEW item check=ac_tc_evidence_coverage status=not_required
```
结论：四项判定与整体 `pass` 完全一致；新增项输出 `not_required`，不改变退出码。**无回溯回归**。

## 4. 自造夹具（坏包非 0 / 好包全绿）

在 `4db7bd8` 的导出树中新增独立测试 `internal/orchestrator/zz_qa_reg_independent_test.go`，用自己构造的两 AC 包（未复用制造方 TC 夹具）逐案验证：

```
go test ./internal/orchestrator -run 'TestQAGoodPackagePasses|TestQAMissingTCPointsAC|TestQAMissingEvidenceFile|TestQAEvidenceOutOfBounds|TestQAHistoricalNotRequired' -count=1 -v

--- PASS: TestQAGoodPackagePasses
    GOOD pass=true detail="2 AC covered by 2 executable TC(s) with in-package evidence"

--- PASS: TestQAMissingTCPointsAC
    MISSING_TC codes=[traceability_missing_tc] missing=[AC-002]
    failed=[state_exported ac_tc_evidence_coverage:reason=traceability_missing_tc:missing=AC-002]
    → conf.Pass=false（非 0 等价）

--- PASS: TestQAMissingEvidenceFile
    MISSING_EV codes=[traceability_missing_evidence] missing=[AC-002/TC-002]
    → conf.Pass=false

--- PASS: TestQAEvidenceOutOfBounds
    OUT_OF_BOUNDS codes=[traceability_missing_evidence] missing=[AC-002/TC-002]
    （Evidence 写 evidence/../../outside.txt，被拒绝）
    → conf.Pass=false

--- PASS: TestQAHistoricalNotRequired
    HISTORICAL status=not_required pass=true
```

对应验收点：缺 TC → `traceability_missing_tc` 且点名 `AC-002`；证据不存在/越界 → `traceability_missing_evidence` 且点名 `AC-002/TC-002`；好包全绿。

## 5. CLI 真实退出码（自造包）

在 `4db7bd8` 导出树新增 `internal/cli/zz_qa_reg_cli_test.go`，用自造包经 `goal conformance` 实测：

```
--- PASS: TestQACLIExitCodes
    CLI GOOD exit=0        （好包，--json）
    CLI BAD  exit!=0       （缺 TC，--json 含 traceability_missing_tc 与 AC-002）
    CLI TEXT exit!=0       （文本模式出现 [FAIL] ac_tc_evidence_coverage）
```
证据：好包 0 退出；坏包非 0，且 reason code 与 AC 在文本/JSON 两处都可读。

## 6. 真实包自检（12 行解析，含 TC-010 的 `|`）

```
go test ./internal/gate -run 'TestParseCoverageMatrixKeepsPipesInCodeSpan' -count=1 -v
--- PASS: TestParseCoverageMatrixKeepsPipesInCodeSpan (0.00s)
```
说明 code span 内的 `|` 不再被当作列分隔符，真实包 Evidence 列不再错位。

## 7. diff 审查（ae95587..4db7bd8）

非测试改动文件：
- `internal/cli/goal.go`：仅 conformance 文本渲染，新增 `not_required → n/a` 分支，失败仍走同一 `fail(...)` 非 0 路径。
- `internal/core/reasons.go`：新增 `traceability_missing_tc` / `traceability_missing_evidence` 与 `traceability_` 前缀白名单，未改既有码。
- `internal/gate/integrity.go`：`ParseCoverageRows` 保留原语义（内部委托 `ParseCoverageMatrix`），`splitCells` 改为仅在反引号外切列（无 code span 的旧行行为不变）；新增导出 `IsDocsOnlyCommand` 复用既有规则。
- `internal/orchestrator/delivery_paradigm.go`：`ConformanceItem` 新增 `reason_codes`/`missing`（均 `omitempty`），新增 `ac_tc_evidence_coverage` 项；`not_required` 不加入 `Failed`。
- `internal/orchestrator/truth.go` / `branch.go` / `truth_change.go`：新增 `change_ac_diff` / 逐 AC 指纹；refreeze 在状态翻转前计算 diff，失败保持 `change_pending`（fail-closed）。

未发现对其它包判定、退出码或既有 JSON 字段的破坏性改动。全量测试（含 gate/orchestrator 既有用例）0 退出佐证。

## 8. fail-closed 复跑（未只看他人证据）

```
go test ./internal/orchestrator -run 'TestConformanceTraceabilityFailClosed' -count=1 -v
--- PASS: TestConformanceTraceabilityFailClosed (0.59s)
```
tests.md 缺失 / 矩阵不可解析 / truth 无 AC 三种均 fail-closed 且非 0。

## 9. 失败项复现

无失败项。

## 10. 最没把握的一条

真实包 `goal conformance --run-id` 在全局 `~/.beacon/beacon-go.db` 中无该 feature 的正式 run 记录，故第 6 节用的是引擎加载真实包路径 + 导出树夹具 run；第 5 节的 CLI 退出码用的是自造 run（临时 DB）。两者均真跑，但“真实包 + 真实全局 run”的 CLI 组合尚未在同一 DB 上出现，待合并后建正式 run 时可补跑一次。

## 11. 边界遵守

- 只读原仓库代码与 git；未切分支、未 commit、未 push、未改实现、未碰 `kernel-eval-gate`。
- 唯一写入原仓库的文件是本 verdict。
- 自造夹具仅存在于 `/tmp/qa-reg-new`、`/tmp/qa-reg-old` 导出副本，未进入仓库。
