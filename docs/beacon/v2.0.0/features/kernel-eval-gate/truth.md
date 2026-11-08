---
slug: kernel-eval-gate
version: v2.0.0
status: frozen
language: zh
domain_required: false
ux_required: false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: filled
materials_status: current
intent_confirmed: true
traceability_required: true
canonical_refs:
  truth: docs/beacon/v2.0.0/features/kernel-eval-gate/truth.md
  tests: docs/beacon/v2.0.0/features/kernel-eval-gate/tests.md
  tasks: docs/beacon/v2.0.0/features/kernel-eval-gate/tasks.md
  evidence: docs/beacon/v2.0.0/features/kernel-eval-gate/evidence.md
---

# Requirement Truth: kernel-eval-gate

## 人话

给 Beacon 内核维护者与验收方：内核每次演进都靠人工挑几条命令去验，验漏了没人知道，验法还会随人变化。本包把 stage gate、conformance、拒绝路径、失败不降级四类内核行为收成一套确定性回归集，一条 `go test` 命令跑完，好 fixture 全绿，坏 fixture 让调用非零退出并点名用例；集子不联网、可进 CI，并覆盖 #1 `kernel-truth-traceability` 已冻结的覆盖率正负判定、两个 reason code、历史包兼容与三条 fail-closed。

## User Intent Snapshot

```yaml
lake_or_ocean: 湖
language: zh
source: user_intent
utterance: "justin-mc-lai：【内核演进 · #1+#2】truth 可追溯 + 内核回归评测门。#2 kernel-eval-gate：一套确定性内核回归集：stage gate、conformance、拒绝路径、失败不降级。一条命令跑完；好 fixture 全绿、坏 fixture 必须非零退出并指出用例。不依赖网络；可接入 CI（go test 或 CLI 子命令）。验收：坏 fixture → 非零；好 fixture → 全绿。质量门（硬）：至少 3 个 QA 并行独立验收，各出 verdict 文件；maker≠checker。"
goal: "kernel-eval-gate 提供一套确定性、离线、可进 CI 的内核回归集，一条命令跑完 stage gate、conformance、拒绝路径、失败不降级四类用例；好 fixture 全绿，坏 fixture 的底层调用非零退出并点名用例，实际结果与期望不一致时评测集自身非 0 退出并点名 case。评测集覆盖 #1 已落地的 ac_tc_evidence_coverage 正负判定、traceability_missing_tc 与 traceability_missing_evidence、not_required 历史包兼容、三条 fail-closed，且 AC-009 等既有冻结行为不回归。"
non_goals: "不改 #1 kernel-truth-traceability 已冻结包与测试；不重写 delivery-paradigm 四项判定；不引入网络依赖；不做 UI；不改版本号与对外发布说明；本包只写 truth，不写 cmd/internal 代码。"
constraints: "Go 内核；工作区 beacon-go，分支 feat/kernel-truth-traceability 4db7bd8；命令只用 go run ./cmd/beacon 与 go test，不用 ~/.local/bin/beacon-go 与 Python 的 beacon；评测集离线、确定性、可进 CI；maker≠checker；每个交付三份独立 QA verdict；失败一律 fail-closed。"
scope_mode: focused
```

## Open Questions

One question at a time; a package cannot be signed while a row is open.

| Q ID | Question | Recommendation | Status | Answer |
|------|----------|----------------|--------|--------|
| Q1 | 用一句话说清：谁、什么问题、什么算完成？ | 先写目标与非目标，再写一条可验证的完成标准 | answered | justin-mc-lai 本轮原话：Beacon 内核维护者与验收方每次靠人工挑命令验内核，会验漏、验法不一致；完成标准是一套确定性内核回归集，一条命令跑完 stage gate、conformance、拒绝路径、失败不降级，好 fixture 全绿、坏 fixture 非零退出并点名用例，不联网、可进 CI，并覆盖 #1 的覆盖率正负判定、两个 reason code、not_required 兼容与三条 fail-closed。 |
| Q2 | 评测集落在哪里、用哪条命令跑完，才算可进 CI？ | 新建 `internal/evalgate` 测试包，单条 `go test ./internal/evalgate -count=1` | answered | 选择 go test 形态：用例注册表与 fixture 构造放在 `internal/evalgate`，整集由 `go test ./internal/evalgate -count=1` 跑完，每个用例是以用例 ID 命名的子测试；不新增 CLI 子命令，CI 直接跑 go test。 |
| Q3 | 坏 fixture 的「非零退出并指出用例」如何判定，评测集自己怎么证明不会漏判？ | 坏 fixture 的底层调用必须非零并带期望 reason code；实际与期望不符时评测集自身非 0 退出并打印 case ID | answered | 每个用例声明期望退出码、期望 reason code 与必须点名的标识；坏 fixture 的底层调用非 0 退出并命中期望即判用例通过；实际结果与期望不一致（错 reason code、该失败却成功、该成功却失败、输出不可解析）时评测集非 0 退出并打印 `case=<ID>` 与 expected/actual，不得降级为 pass 与 skip。 |

## Intent Coverage

Every AC must trace to the intent or evidence it serves.

| AC ID | Intent Source | Evidence |
|-------|---------------|----------|
| AC-001 | 用户原话：一套确定性内核回归集，一条命令跑完，可接入 CI | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-registry.json |
| AC-002 | 用户原话：评测集必须覆盖 #1 的覆盖率正负判定、两个 reason code、not_required 兼容与三条 fail-closed | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-conformance-positive.json |
| AC-003 | 用户原话：覆盖 stage gate 与拒绝路径，坏 fixture 非零退出并指出用例 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-stage-gate.json |
| AC-004 | 用户原话：好 fixture 全绿、坏 fixture 必须非零退出并指出用例；不依赖网络 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/evalgate-mismatch-fails.json |
| AC-005 | 用户原话：AC-009 等既有冻结行为不得回归；go test ./... 全绿 | docs/beacon/v2.0.0/features/kernel-eval-gate/evidence/go-test-all.txt |

## Acceptance Criteria

| AC ID | Criterion |
|-------|-----------|
| AC-001 | `internal/evalgate` 提供数据驱动的内核回归用例注册表，用例按 class 归入 `stage_gate`、`conformance`、`rejection`、`no_degradation` 四类，每类含一个以上具名用例；每个用例带稳定 ID、class、fixture 构造方式、期望退出码、期望 reason code 与必须点名的标识。整集由单条命令 `go test ./internal/evalgate -count=1` 跑完，每个用例作为以用例 ID 命名的子测试执行，不跳过、不改写期望；注册表出现未知 class、重复 ID、缺失期望时评测集非 0 退出并点名该用例。 |
| AC-002 | 评测集覆盖 #1 的 `ac_tc_evidence_coverage`：完整 fixture 的 conformance 通过且该检查项 pass、退出码 0；缺 TC 的 fixture 退出码非 0、reason code 为 `traceability_missing_tc`、detail 点名缺 TC 的 AC ID；Evidence 文件不存在、路径越出包 `evidence/` 目录、Evidence 列缺失三种 fixture 退出码非 0、reason code 为 `traceability_missing_evidence`、detail 点名 AC ID 与 TC ID；tests.md 缺失、覆盖矩阵不可解析、truth 无 AC 定义三种 fail-closed fixture 退出码非 0 并携带 reason code，不得输出 pass 与 `not_required`；未声明 `traceability_required` 的历史包输出 `not_required`，原有四项判定名称与退出码语义不变。CLI 驱动的用例同时校验文本输出与 `--json` 输出都携带 reason code。 |
| AC-003 | 评测集覆盖 stage gate 与拒绝路径：truth 未冻结时 implement 门判定 blocked 并携带 `stage_exit_truth_not_frozen`，事实齐备时 implement 门 pass；未知 gate type 返回 `gate_type_unknown`，缺 evaluator 返回 `gate_evaluator_required`，未知 feature 返回 `feature_not_found`，未知 run 的 conformance 返回非 0 并带 reason code。上述拒绝用例退出码均非 0，且不写入 pass 判定。 |
| AC-004 | 失败不降级：坏 fixture 的底层调用退出码非 0 并点名用例；实际结果与期望不一致时（错 reason code、期望失败却成功、期望成功却失败、输出不可解析）评测集整体非 0 退出并打印 `case=<ID>` 与 expected/actual，不得记为 pass 与 skip。评测集连续运行两次的归一化报告完全一致；把 `HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY` 指向 `127.0.0.1:1` 后整集仍通过，证明确实不依赖网络。 |
| AC-005 | 既有行为不回归：`go test ./... -count=1` 退出码 0；AC-009 的 `TestProgramPlanRequiresProposal`、`TestProgramPlanFreezeChange`、`TestProgramSelectRecordsDecision` 三个提案门测试原样通过；`TestDeliveryParadigmEndToEnd` 端到端测试原样通过；#1 的 traceability 测试文件不被改写。 |

## 评测集契约 v1

评测集把四类行为写死成数据驱动的用例，实现与 QA 不再另行解释：

| Class | 用途 | 通过条件 | 失败表现 |
|-------|------|----------|----------|
| `stage_gate` | 阶段门事实齐备时放行、事实缺失时阻断 | 每例的实际 verdict、reason code 与期望一致 | 实际与期望不一致时评测集非 0 并点名 case |
| `conformance` | #1 覆盖率检查的正负判定、fail-closed、历史包兼容 | 正向 pass 且退出 0，负向非 0 且 reason code 与点名标识命中 | 漏判（负向通过、正向失败）时评测集非 0 并点名 case |
| `rejection` | 非法输入必须被拒绝，且不留 pass 判定 | 退出码非 0，reason code 命中期望，未写入 pass 记录 | 任何非法输入被接受时评测集非 0 并点名 case |
| `no_degradation` | 评测集自身不吞错、不降级、结果可复现、不联网 | 期望不符必报错，两次归一化报告一致，阻断出网后仍通过 | 吞错、降级、结果漂移、依赖网络时评测集非 0 并点名 case |

每个用例在 `t.TempDir()` 隔离项目根与隔离 store（`store.EnvStorePath`）中构造，用例之间不共享状态；比较前归一化随机 run ID 与时间戳，归一化后两次运行的报告逐字节一致。

## 用例目录

| Class | Case ID | Fixture | 期望 |
|-------|---------|---------|------|
| `stage_gate` | `stage_gate/implement_blocked_unfrozen_truth` | truth 未冻结，其余事实齐备 | blocked，`stage_exit_truth_not_frozen` |
| `stage_gate` | `stage_gate/implement_pass_complete` | truth 冻结，门事实齐备 | pass，退出 0 |
| `conformance` | `conformance/all_covered` | 每条 AC 有非软 TC，Evidence 文件存在 | `ac_tc_evidence_coverage` pass，退出 0 |
| `conformance` | `conformance/missing_tc` | 删除一条 AC 的 TC 行 | 非 0，`traceability_missing_tc`，点名该 AC |
| `conformance` | `conformance/missing_evidence_file` | Evidence 指向不存在的文件 | 非 0，`traceability_missing_evidence`，点名 AC/TC |
| `conformance` | `conformance/evidence_escape` | Evidence 路径为 `../` 越界 | 非 0，`traceability_missing_evidence`，点名 AC/TC |
| `conformance` | `conformance/evidence_column_missing` | 覆盖矩阵没有 Evidence 列 | 非 0，`traceability_missing_evidence`，点名 AC/TC |
| `conformance` | `conformance/failclosed_tests_missing` | 包内没有 tests.md | 非 0，携带 reason code |
| `conformance` | `conformance/failclosed_matrix_unparseable` | 覆盖矩阵无 AC ID 表头 | 非 0，携带 reason code |
| `conformance` | `conformance/failclosed_no_ac` | truth 没有定义 AC | 非 0，携带 reason code |
| `conformance` | `conformance/not_required_compat` | 未声明 `traceability_required` 的历史包 | `not_required`，四项判定与退出码不变 |
| `rejection` | `rejection/unknown_gate_type` | `gate evaluate` 传入未知 gate type | 非 0，`gate_type_unknown` |
| `rejection` | `rejection/missing_evaluator` | `gate evaluate` 缺 `--evaluator` | 非 0，`gate_evaluator_required` |
| `rejection` | `rejection/unknown_feature` | 查询不存在的 feature | 非 0，`feature_not_found` |
| `rejection` | `rejection/unknown_run_conformance` | conformance 查询不存在的 run | 非 0，携带 reason code |
| `no_degradation` | `no_degradation/expectation_mismatch_detected` | 注入与真实结果不符的期望 | 评测集非 0，打印 `case=<ID>` 与 expected/actual |
| `no_degradation` | `no_degradation/deterministic_twice` | 同一注册表连续跑两次 | 归一化报告逐字节一致 |
| `no_degradation` | `no_degradation/offline_loopback_blocked` | 出网代理指向 `127.0.0.1:1` | 整集仍通过 |

## Evidence Map

| 观察 | 证明方式 | 归属 |
|------|----------|------|
| 一条命令跑完整集 | `go test ./internal/evalgate -count=1` 退出 0，子测试覆盖四类用例 | AC-001 |
| 覆盖率正负判定被覆盖 | 正向 case pass，缺 TC case 报 `traceability_missing_tc` 并点名 AC | AC-002 |
| 证据三类失败被覆盖 | 不存在、越界、缺列三种 case 报 `traceability_missing_evidence` 并点名 AC/TC | AC-002 |
| fail-closed 三态被覆盖 | tests.md 缺失、矩阵不可解析、truth 无 AC 三种 case 均非 0 且带 reason code | AC-002 |
| 历史包不被回溯判死 | 未声明契约的包输出 `not_required`，四项判定不变 | AC-002 |
| 阶段门与拒绝路径被覆盖 | 未冻结阻断、事实齐备放行、四类非法输入非 0 且 reason code 命中 | AC-003 |
| 评测集自身不降级 | 注入不符期望时评测集非 0 并点名 case；两次报告一致；断网仍通过 | AC-004 |
| 既有行为不回归 | 全量 go test 退出 0，AC-009 三提案门与 delivery-paradigm 端到端原样通过 | AC-005 |

## Non-Goals

- 不改 #1 kernel-truth-traceability 已冻结包与已落地测试。
- 不重写 delivery-paradigm 的四项 conformance 判定。
- 不引入网络依赖，不把评测集做成在线回归。
- 不改版本号与对外发布说明；这两项由人确认。
- 本包只写 truth，不写 cmd/internal 代码。

## Deferral Ledger

| Capability | Item | Reason | Status | Approved By |
|------------|------|--------|--------|-------------|
| none | none | none | dropped | n/a |
