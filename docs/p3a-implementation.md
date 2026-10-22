# P3a 实施报告：门禁评估写库 + 入口改名变更登记

> 日期：2026-10-22 ｜ 基线：v2.0.0-go.1（P2 出口）｜ 本片不发新版本号（P3 整体出口时发 go.2）
> 一句话：以前 goal 推进时对门禁是「自己算一算」（派生判定，不落库）；这一片把四道门做成了**正式评估**——读事实、出结论、带原因码、写进账本（gate_results + 事件轨迹），谁评的必须留名。顺手把 P2 欠的「入口改名 goal」变更登记账还了。

---

## 0. 先还账：入口改名 goal 的变更登记

- 落盘：[changes/change-goal-entry-rename.md](changes/change-goal-entry-rename.md)，按 gen-change 要素写全：变更内容（用户入口 `loop` → `goal`，`loop` 只留指针）、理由（2026-10-21 拍板第 5 条选 A）、Go 侧 AC 清单调整、状态。
- 受影响的冻结条目逐条列出（冻结包 `beacon-v2-rust-kernel-redesign` truth.md r19）：概述 crate 图、User Intent 锚点、Alignment Surface 引文行、**AC-059 / AC-063 / AC-109**（六能力清单点名 `loop` 的三条）；AC-064 逐条核对后**不受影响**（未点名 loop 子命令）；AC-105–108 里的 loop 是概念名（Loop Engineering 三层 Loop、stop_when 机制），语义不受影响。tests.md 对应 TC-059/TC-063/TC-109 的 `"loop"` id 断言注明 Go 轨读作 `"goal"`。
- **状态如实**：已在 Go 侧落账（本记录 + 口径稿第 6 节第 5 条进展注记 + 代码注释指向）；**上游冻结包在只读镜像中未改，没有对上游执行 gen-change**，待源仓下一次正式变更时把本记录作为输入合并。

## 1. 做了什么（结构）

判定与落库分工（不搞第二套判定）：

| 层 | 文件 | 职责 |
|---|---|---|
| 纯判定 | `internal/gate/gate.go`、`evaluate.go`（254 行） | 五门评估器：事实进、结论（pass/blocked）+ 逐项检查 + 原因码出；无 store、无文件系统，同一事实必出同一结论 |
| 纯判定 | `internal/gate/integrity.go`（488 行） | Truth Integrity Gate：需求包解析 + 互相引用检查（见第 3 节） |
| 事实收集 + 落库 | `internal/orchestrator/gates_eval.go` / `gates_facts.go` / `gates_integrity.go` | 从 store 和需求包文件收事实 → 调纯判定 → 写 gate_results 一行 + lifecycle_events 一条 `gate_evaluated`（integrity 为 `truth_integrity_evaluated`） |
| 命令面 | `internal/cli/gate.go`（257 行） | `beacon gate evaluate / status / integrity`，全部支持 `--json`；blocked 时打印评估并以首位原因码非零退出 |
| 词表 | `internal/core/reasons.go` | 新增 gate 家族 + truth_integrity 家族原因码（逐项见下），旧码一律复用不重拼 |

每次评估必填 `--evaluator`（评估者标识）：缺了在读任何事实之前就拒绝（`gate_evaluator_required`）——没人署名的结论不落账。
重跑评估是追加新行（账本语义），`LatestByType` 读最新；确定性有测试保证（同事实两次评估结论与原因码全等）。

## 2. 四门评估口径表

| 门 | 查什么（逐项） | 事实来源 | block 原因码（逐项） |
|---|---|---|---|
| truth_freeze | truth.md 在且非空、tests.md 在且非空、冻结标记、无挂起变更 | 需求包文件（项目根目录相对路径解析）；**冻结主口径 = truth.md frontmatter `status: frozen`**（package-authoritative：需求真相只认文档自身）；lifecycle 的 Freeze/Completed 快照只作旁证项记录（冻结后生命周期会继续往后走，不能当权威） | 缺文件/空文件 `stage_exit_truth_package_missing`；未冻结 `freeze_ack_required`（填完≠自签，沿旧版 goal 面口径）；有 change 请求未 refreeze `truth_change_pending` |
| design（仅 UX） | 是否需要设计、不需要直接 pass（n/a）；需要时：绑定存在、绑定版本一致 | P2c 的 run 级绑定记录（runs 快照）+ run 轨迹里最近一条 `design_primitive_bound` 事件 | 未绑 `design_primitive_not_bound`；无 run `gate_run_required`；绑定无版本 `design_binding_version_missing`；快照版本 ≠ 事件版本 `design_binding_version_mismatch` |
| implement | 直接复用 P2c 的 `CheckImplementAdmission` 五项（需求冻结/工位绑定/设计绑定/卡点 streak/steering 确认），逐项 1:1 映射 | 同一检查器本体——`goal admit` 是只读预览，本门是正式写库，**两个出口一份判定**，代码层无第二实现 | 五项各自的原因码（与 admit 完全一致，有测试断言两面同判） |
| qa | QA 类产物指针存在、qa 阶段执行证据在、测试结果引用有效（文件在且哈希对） | artifact_refs（kind ∈ qa / qa_* / scorecard / test_result(s)）+ 各 run 的 `stage_executed`(qa) 事件 + P1 的 stale 判定（`store.ArtifactStaleness`：file_missing/file_unreadable/hash_mismatch） | 无产物 `stage_exit_qa_evidence_missing`；无执行证据 `qa_stage_not_executed`；引用失效 `qa_artifact_stale`。**本门只判证据齐不齐、引用活不活，不自己跑测试** |
| release | 前四门最新结论全 pass + 人工确认在录 | gate_results 最新值（design 在无 UX run 时 n/a 不拦）+ 最新 goal run 快照的确认标记或轨迹里的 `release_confirmed` 事件（P2a 口径） | 某门从未评估 `gate_not_evaluated`（逐门点名）；某门最新非 pass `gate_prerequisite_not_passed`；无人确认 `release_requires_human_confirmation`。integrity **不是** release 前置（AC-101 把它绑在 freeze/refreeze 上），报告在此明示 |

## 3. Truth Integrity Gate（AC-101）清单来源

清单定自冻结原文：truth.md 的 **AC-101**（空缺 AC、软 TC、未拍板 deferral、假交付、package validation 失败 → hard block + repair_route；已 frozen 历史包默认 audit_only）+ 任务口径的三条互相引用。v1 的实现 `beacon/utils/truth_integrity.py` 的 `build_truth_integrity_gate` 作为语义对齐源。四件套按 v1.5.2 package-authoritative 的 canonical_refs 定：truth / tests / tasks / evidence（路径优先取 truth frontmatter 的 canonical_refs，其次 feature 行的 truth/tests 路径与同目录兄弟文件）。

逐项检查（任一断裂 → 一条 finding，带文件/行号/修复路线 `beacon-gen-truth`）：

1. 包完整性：四件缺一、truth frontmatter 缺失/未闭合 → `truth_integrity_package_incomplete`
2. tests 引用的 AC 必须在 truth 里有定义（定义形态：`**AC-NNN**` 定义行或 AC 表首列）→ `truth_integrity_tests_unknown_ac`
3. truth 定义的 AC 必须在 tests 覆盖矩阵里有行（空缺 AC）→ `truth_integrity_ac_without_tc`
4. 软 TC：覆盖行只有 `integration/目录存在/fixture/可选` 或缺 Command+Assertion、且无 `BLOCKED:` 逃生口 → `truth_integrity_soft_tc`（AC-101 + AC-164 弱 TC 口径）
5. 未拍板 deferral（truth 中 `| pending |` / `user_decision: pending`）→ `truth_integrity_deferral_pending`
6. 假交付：九类话术行（占位实现/占位验收/假 runner/零断言/占位证据/软测试独过/pending deferral/无证据声称完成/docs-only 闭环）→ `truth_integrity_fake_delivery`
7. evidence 引用的产物：必须在 artifact_refs 有登记（按解析后的绝对路径对）且文件在盘上 → `truth_integrity_evidence_artifact_unregistered` / `truth_integrity_evidence_artifact_missing`
8. tasks 引用的 slug 必须与包 slug 一致 → `truth_integrity_slug_mismatch`

模式规则：`--mode block|audit_only`；不给时按 AC-101 默认——包已 frozen（frontmatter 或 feature 行 status=frozen）→ audit_only（结论记 `audit`、不拦、附 `truth_integrity_audit_only` 标记码）；未 frozen → block。结论同样写 gate_results（gate_type=`truth_integrity`）+ 事件。
入口与四门分开：`beacon gate integrity`；`gate evaluate truth_integrity` 会被拒绝并指路。

evidence 引用抽取规则（确定性，明示）：evidence.md 中含 `/`、以文件扩展名或 `/` 结尾的路径型 token（代码段与表格单元格）；跳过 `conversation:` 伪引用、URL、含 `<>` 的模板占位。

## 4. Tick 联动变化（单列）

P2a 的 ExitGates 是全派生判定；本片按「有正式评估认正式的」切换两处，并给 `ExitVerdict` 加两个字段：`derived`（本结论是否派生）、`formal_verdict`（决定性的/最新的正式结论）。

- **qa 出口：正式优先**。有 qa 正式结论时它说了算：pass 放行、blocked/fail 按正式原因码拦——**即使原始产物数 > 0 也拦**（这是行为变化点：旧派生逻辑见产物就放行）。无正式结论才回落派生（产物存在性），且 `derived=true`。
- **release 出口：正式优先但 tick 永不放行**。出 release 就是完成，完成是 `Complete` 的人工动作（源仓原样：release never auto-exits on tick）——所以正式 pass 在 tick 面表现为新码 `release_awaiting_complete`（「门已过、等人工完成」），正式 blocked 用正式结论的首位原因码拦；无正式结论回落旧派生 `release_requires_human_confirmation` + `derived=true`。
- **truth / design / implement 三处维持派生**，逐项理由：
  - truth：派生判定本来就把正式 truth_freeze pass 作为一路输入（P2a 已如此）；且生命周期快照是推进引擎自己写的、与 tick 同步，正式评估是外部时点快照，拿它当唯一权威会让「刚冻结、还没人跑 gate evaluate」的正常流程误拦。正式结论经 `formal_verdict` 透出；当放行证据来自正式 pass 时 `derived=false`。
  - design：同理，派生读的绑定记录就是 P2c 的正式绑定动作产物，正式 design 门的增量信息只有版本一致性，那是评估面而非推进面的职责。
  - implement：tick 的 implement 入口本来就直连正式检查器本体（admission，P2c 已落地），比读一条写库结论更强；出口面的 implement_admission 旧键判定保持不动。
- 幂等不受影响：tick 的指纹（tickKey）本来就含最新 gate 事实，正式结论一变就自动重新武装——「种一条 block，tick 在 qa 停住；改判 pass，tick 放行」有端到端测试。

**测试改动声明：既有 103 个测试一个未改**（含断言），全部原样通过；ExitVerdict 只加字段不改旧字段语义。本片新增 25 个测试（gate 纯判定 11 + orchestrator 行为 10 + CLI 4），总计 128 全过。

## 5. 实跑样例（真二进制，/tmp/p3a-smoke）

项目：frozen 需求包（truth/tests/tasks/evidence 四件）+ 真实 goal run（`run_9bf8c28aa0df9bcee8c6e85f`），评估者 `qa-bot`：

```
$ beacon gate evaluate truth_freeze --feature feat-smoke --evaluator qa-bot --json
{ "gate_type": "truth_freeze", "verdict": "pass", ... 7 项检查全 ok, "gate_result_id": "gate_3d22afce..." }
$ beacon gate evaluate design --run <R> ...     → pass（非 UX run，design_required n/a）
$ beacon gate evaluate implement --run <R> ...  → pass（五项 admission 全过）
$ beacon gate evaluate qa ...                   → blocked [stage_exit_qa_evidence_missing, qa_stage_not_executed]，exit=1
$ beacon gate evaluate release ...              → blocked [gate_prerequisite_not_passed, release_requires_human_confirmation]，exit=1
    （逐项：truth_freeze pass ✓ / design n/a / implement pass ✓ / qa blocked ✗ / 人工确认缺 ✗）
$ beacon gate status
truth_freeze pass / design pass / implement pass / qa blocked / release blocked / truth_integrity not_evaluated
$ beacon spine gate feat-smoke                  → 读回同一批正式结论（只读面，数据来源已在 help 注明）
$ beacon gate integrity ...（frozen 包、evidence 引用未登记产物）
  → verdict=audit（默认 audit_only），findings: evidence_artifact_unregistered + evidence_artifact_missing，exit=0
$ beacon gate integrity --mode block ...        → 同一断裂 verdict=blocked，exit=1
$ beacon gate evaluate qa（不带 --evaluator）   → Error: ... (gate_evaluator_required)，exit=1
```

## 6. 语义差异 / 简化清单（如实）

1. **假交付 docs-only 模式**：v1 用否定后顾 `(?<!非)(?<!不)` 排除「非 docs-only」类表述；Go 的 RE2 无后顾，改为定位匹配后手工查前一 rune 是否为 非/不——语义等价，写法不同。
2. **TIG 的 determinism 扫描（TIG-DC：TBD/TODO/空话模板）未迁**：它在 v1 文件里是 Phase A 扩展，不在 AC-101 五类与任务三引用之内；本片不冒充全量 TIG，缺口在此明示，后续要补按独立条目立项。
3. **空缺 AC 的方向**：AC-101 只说「空缺 AC」；本片双向都查（tests 引不存在的 AC + truth 有 AC 无 TC 行），后者与 tests.md「每个 AC 至少一行 TC」的冻结口径一致。
4. **design 版本一致性口径是本片定的**：v1/P2c 没有现成的「期望版本」登记处，本片定义为「run 快照绑定版本 == run 轨迹最近一条绑定事件版本」，查的是记录自相矛盾（被改/漂移），不是与外部版本表对齐；绑定无版本直接 block（无版本无从匹配）。
5. **挂起变更的事实源**：store 里尚无 truth 变更队列（truth 命令面属 P4），本片查 feature 轨迹中 `truth_change_requested/change_requested/gen_change` 事件之后有无 `truth_refrozen/truth_frozen/refreeze/gen_refreeze` 事件——事件词表先行，P4 的 truth 面写同名事件即自动接上。
6. **qa 产物 kind 集合**是明示约定（qa / qa_* / scorecard / test_result / test_results）；其他 kind 的产物不算 QA 证据。
7. integrity 的 evidence 引用按「登记 + 在盘」双查（任务字面口径）：包内文档互引（如 evidence 索引 truth.md）若未登记为 artifact 也会报 unregistered——这是硬门的设计，frozen 历史包有 audit_only 兜底。

## 7. 交付物与大小

| 文件 | 行数/大小 | 说明 |
|---|---|---|
| `docs/changes/change-goal-entry-rename.md` | 52 行 | 变更登记（第 0 项） |
| `docs/scope-charter.md` | 改 1 处 | 第 6 节第 5 条落账进展注记 |
| `internal/core/reasons.go` | 增 1 节 | gate + integrity 原因码家族 |
| `internal/gate/gate.go` | 121 行 | 类型扩展（六 gate 类型、CheckItem、Evaluation） |
| `internal/gate/evaluate.go` | 254 行 | 五门纯评估器 |
| `internal/gate/integrity.go` | 488 行 | TIG 纯检查（解析 + 引用一致） |
| `internal/gate/gate_test.go` | 270 行 | 纯判定测试 11 个 |
| `internal/orchestrator/gates_eval.go` | 225 行 | 评估服务（分发/落库/事件） |
| `internal/orchestrator/gates_facts.go` | 228 行 | 五门事实收集（implement 包 admission 本体） |
| `internal/orchestrator/gates_integrity.go` | 125 行 | integrity 服务（包收集/模式默认/落库） |
| `internal/orchestrator/exitgates.go` | 285 行（改） | qa/release 正式优先 + Derived/FormalVerdict |
| `internal/orchestrator/gates_eval_test.go` | 465 行 | 行为测试 10 个 |
| `internal/cli/gate.go` | 257 行 | `beacon gate` 命令组 |
| `internal/cli/gate_test.go` | 新增 | CLI 测试 4 个 |
| `internal/cli/root.go` / `spine.go` | 改 3 处 | 注册 gate 组；loop 注释指向变更记录；spine gate 数据来源注明 |
| `bin/beacon` | 15,191,577 字节 | `go build ./...`、`go vet ./...`、`gofmt` 全净；`go test ./... -count=1` **128 个全过**（103 旧 + 25 新，旧测试零改动） |

边界（未做，留后续片）：v1 数据迁移（P3b）、真执行器/worktree 真集成（P3c）、上游冻结包的正式 gen-change 合并（待源仓侧动作）、TIG-DC determinism 扫描（第 6 节第 2 条）。
