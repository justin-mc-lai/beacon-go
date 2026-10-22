# 标准流 S2 实施报告：标准流水线 + 准入捆（AC-138/141/142）

版本保持 v2.0.0-go.6（S3 后统一升版）。冻结原文已逐字核对：
`~/workspace/repos/beacon/docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/truth.md`
的 R10（约 245–250 行）、R11 Standard Execution Flow Contract（约 252–292 行，含
标准失败码表）、AC-138/141/142（673/676/677 行），及 tests.md 的 TC-138/141/142 与
TC-N19/N20/N21。源仓只读，未动冻结 schema，状态只进 store，未新设顶层命令。

## 结论先行

三条 AC 全部落地，行为级真 git 测试覆盖。**冻结码表几乎全覆盖：本片新登记
仅 1 个码**（`git_diff_check_failed`，理由见第 6 节）。全量 **232 个测试全过**
（221 既有 + 11 新增），既有测试只改了 1 处（声明见第 7 节），**E2E 剧本零改动**
原样重跑 exit 0（13 步全 PASS）。

## 1. 逐 AC 落地表

| AC | 冻结口径 | 实现落点 | 测试 |
|---|---|---|---|
| AC-138 标准执行流 | 八步固定顺序、跳步 fail-closed | `orchestrator/stdflow.go`：显式步骤表 `FlowSteps` + `FlowAdvance/FlowStatus`；状态存 run metadata（`FlowState`）+ `flow_step_completed` 事件，不新建表；CLI `goal flow status/advance` | TestS2FlowSkipStepRejected、TestS2FlowFullWalkAndIdempotent、TestS2FlowCommitCriterionBlocks、TestS2FlowGovernanceCriterionBlocks、TestS2GoalFlowCLI |
| AC-141 工位基线检查 | feature/dev 进入 implement 前必须包含 `truth_baseline_commit` 且无 dirty policy 违规，否则 blocked | `orchestrator/baseline_check.go`：单一 binding 解析 + 祖先判定（`git merge-base --is-ancestor`，复用 harness.IsAncestor）；admission 新增第 6/7 项 `baseline_merged`/`dirty_policy` | TestS2BaselineNotMergedBlocksAndMergeFixes、TestS2DirtyPolicyPackageAndOrdinary、TestS2BundleEachLegBlocks(d) |
| AC-142 准入证据捆 | bundle 五类结果缺任一不得进入 implement，缺件码 `standard_flow_verification_incomplete`（TC-142） | `orchestrator/bundle.go`：`evaluateBundle`（纯判定）+ `persistBundle`（落账）；Tick 与 delegate（模拟/真实两路）改走 `admissionBundleForEntry` 同一闸；CLI `goal flow bundle` | TestS2BundlePassesAndPersists、TestS2BundleEachLegBlocks、TestS2TickAndDelegateShareBundleJudgement、TestS2NonGitBundleNotRed |

## 2. AC-138 八步判据表（每步绑真实事实，fail-closed）

状态机定位：这是内核的**第三个小状态机**，与 loop-goal 12 态轴、生命周期轴完全
分开——三轴互不写对方，flow 只读其他面产出的事实。`FlowAdvance` 只能完成「下一
个待办步」，且判据成立才落账；跳步/未知步一律
`standard_flow_order_violation`。

| 步 | 完成判据（事实来源） | 缺事实时的码 |
|---|---|---|
| support_advisory | run 上有 stage plan（咨询/计划记录即计划本身） | `stage_exit_plan_incomplete` |
| canonical_truth_authoring | 包四件套在磁盘上成稿 + 当前分支 = canonical（S1 branchInfo）；非 git 只查四件套 | 缺件 `stage_exit_truth_package_missing`；错分支 `truth_canonical_branch_mismatch` |
| canonical_refreeze | trail 上有 freeze/refreeze 事件 + 共享 FrozenFact 判 frozen | `stage_exit_truth_not_frozen` |
| canonical_commit | FrozenFact `baseline_status=committed`（S1 落账/探针）；非 git 按 S1 口径 frontmatter 即权威、该步由 freeze 满足 | `canonical_refreeze_not_committed` |
| feature_worktree_reconcile | S1 quarantine 新鲜扫描无隔离项（有发现先记录隔离再拒） | `stale_feature_local_refreeze` |
| governance_verify | `truth_integrity` 评估在录且 verdict ∈ {pass, audit}（P3a） | 无记录/被 block 均 `standard_flow_verification_incomplete` |
| workspace_admission | AC-142 bundle 全过（第 4 节；判定同时落 bundle 账） | bundle 的 headline 码 |
| implement_resume | 共享 `decideResume` 给出 `load_next_skill` 结论（P2a） | resume 决策自带码，无则 `standard_flow_order_violation` |

幂等：已完成步重复 advance 返回 `already_done`，不写事件、不 bump 任何 revision
（AC-143 的 flow 面）。走通一例的事件断言：每步恰好 1 条 `flow_step_completed`。

## 3. AC-141 落法与 dirty 口径

**binding 解析（单一判定点，admission 与 bundle 共用）**：真实卫星工位记录
（active、非根绑定）→ 判该目录/分支；否则项目根（single/current-branch 绑定）→
判根 HEAD；planned-only 池绑定（只有计划 ref、无真实工位）→ **not_applicable**
（无真实工位/分支可判，不编造结论）。非 git 项目 → **not_applicable**，不明示
拦截、绝不打红（item 明示 `applicable=false` + 原因文本）。

**baseline_merged**：基线取 S1 落账值（FrozenFact 的 `BaselineCommit`，记录优先、
探针兜底）。frozen 但无 committed 基线记录 → `truth_baseline_commit_missing`
（冻结 R11 码表）；HEAD 不含基线 → `truth_baseline_not_merged`（R10/TC-141/
TC-N19 逐字码）；truth 未 frozen → not_applicable（由 truth_frozen 项管辖）；
**no_document 包（种子注册、无磁盘文件）→ not_applicable**，沿 S1 报告 §3 先例
（「只有磁盘上真有 truth 文件的包才探针」）——此口径保住了既有 git 种子型测试
场景（p3c gitEnv 推进流），已在代码注释与本节明示。

**dirty_policy**（`git status --porcelain`，排除内核自有 `.beacon/`、
`.beacon-worktrees/`）：

| 脏改对象 | 口径 | 出处 |
|---|---|---|
| 需求包目录内任何文件 | 一律违规 → `unscoped_dirty_state` | 任务口径 + R11 码表逐字码 |
| P4c denylist 受保护路径 | 一律违规 → 同上（明细标注类别） | P4c 既有 denylist 复用 |
| 普通代码文件 | implement 前时点**不算违规**（明细计数明示） | **DERIVED**：冻结原文未细分。理由：admission 是开工时点，先行阶段产出与调用方 scoped 在途工作是 implement 的画布；R11 的 dirty 语言（「不能混入无关 runtime dirty files」）绑的是 canonical_commit 步，不是本步。放行不等于隐藏——item detail 报出普通脏文件计数 |

## 4. AC-142 bundle 五项事实来源表

| 项 | 判定内容 | 事实来源 | source 标注 |
|---|---|---|---|
| truth_map_verify | 包在 spine/projects 注册且文件布局与注册一致（四件套齐、frontmatter slug 一致、canonical_refs 指向真实文件） | 注册行（store spine/projects）+ 磁盘布局 | **derived**——内核无独立 truth-map 产物，逐项如实标注 derived 并在 detail 说明，不冒充 recorded artifact |
| verify_materials_strict | 四件套 strict 完整性 + 基线指纹不漂移 | `gate.CheckIntegrity` block 模式（与 integrity 门同一 block 模式）+ S1 `pkgsurface-v1` 指纹与 freeze 记录比对 | live |
| git_diff_check | 工作区 diff 无需求正文/受保护路径违规改动 + `git diff --check` 干净（R11 governance_verify 的具名检查） | 第 3 节 dirty 违规类（P4c denylist + S1 指纹面）+ harness.DiffCheck | live |
| branch_governance_verify | canonical 纪律 + 基线 committed + AC-141 祖先腿 + AC-140 隔离扫描干净 | S1 FrozenFact/branchInfo + baseline_check + quarantine 扫描 | recorded |
| workspace_admission | 共享 admission 检查器整体结论（现 7 项） | `checkAdmission` 本人 | recorded |

**headline 规则**：五项中 1–4 任一适用且失败 → headline =
`standard_flow_verification_incomplete`（TC-142 逐字码，逐项码随行）；仅第 5 项
失败 → headline 保持 admission 自己的首败码（如 `steering_unacknowledged`），
使 tick/delegate 的阻断语义不丢精度（测试 (e) 断言此分界）。

**no_document 口径**（沿 S1 先例）：包在磁盘上完全不存在（种子注册）时，
truth_map/materials 两项 not_applicable——文件类检查不把无文件包打红；
git 类两项在非 git 时 not_applicable。真实包缺件（如删 evidence.md）照常拦。

## 5. 与既有 admission 的整合说明

- **bundle 是超集出口，只有一套判定**：bundle 第 5 项直接调用 `checkAdmission`
  本人（不复制逻辑）；新 AC-141 两项写进 `checkAdmission` 本身（第 6/7 项），
  bundle 的 branch-governance/diff 两项复用同一 `baseline_check.go` 判定函数。
  不存在第二套 admission。
- **入口收口**：Tick 推进 implement、`DelegateRalph` 与 `DelegateRalphReal`
  打包，三处都改走 `admissionBundleForEntry`（判定+persist）。其余消费方
  （`goal admit` CLI、implement gate 的 evalImplement、PlanImplement、
  ExecuteLaneStage、subprocess executor 的复查）保持调 `checkAdmission` 子集——
  论证：`goal admit` 是只读引擎面（goalReadEngine），物理上不能落 bundle 账，
  它的定位是「预检子集」；真正的 implement 入口（tick/delegate）必经 bundle，
  子集预检与 bundle 结论同源同函数，不会分叉。`goal admit` 的输出结构不变
  （schema/字段兼容，items 由 5 增至 7 是 AC-141 扩项本身）。
- **bundle 落账（gate_results 论证）**：每次入口评估落 1 行
  `gate_results(gate_type=admission_bundle)` + 1 条 `admission_bundle_evaluated`
  事件，走 gate service 的 `persistGateResult` 单事务写路径（AC-013 既有模式：
  裁决与 trail 同生同灭）。gate_type 列是自由账本轴，加新值不改 schema；把
  bundle 放进 gate_results 而非新表，是因为它本质就是一类门裁决，spine/gate
  status 的 LatestByType 读面天然能带出它（E2E trace 已实测可见
  `admission_bundle` 行与事件）。
- 副作用提示：bundle 落账会更新 tickKey 的 gate 指纹——被 bundle 拦下的 tick 在
  事实修复后重跑会自然重新判定（测试 TestS2TickAndDelegateShareBundleJudgement
  的修复段断言此行为），不构成重复记账问题：每次真实入口评估本就该留一条证据。

## 6. 码表出处（逐码标注）

| 码 | 出处 |
|---|---|
| `standard_flow_order_violation` | 冻结 R11 码表（亦 TC-N21） |
| `standard_flow_verification_incomplete` | 冻结 TC-142 / TC-N20 逐字 |
| `truth_baseline_commit_missing` | 冻结 R11 码表逐字 |
| `truth_baseline_not_merged` | 冻结 R10 / TC-141 / TC-N19 逐字 |
| `workspace_admission_missing` | 冻结 R11 码表逐字（已登记词表；本片判定路径以 bundle/ admission 自有码表达，该码保留给 S3/呈现层） |
| `unscoped_dirty_state` | 冻结 R11 码表逐字 |
| `canonical_refreeze_not_committed`、`stale_feature_local_refreeze`、`truth_canonical_branch_mismatch` | S1 已登记（R10/TC-139/TC-140） |
| `git_diff_check_failed` | **本片新登记（唯一）**：R11 governance_verify 具名要求 `git diff --check` 通过，但冻结码表未给它分配码；bundle 的 diff 项需要一个稳定码，故在 core 词表 S2 家族登记并在此明示 |

## 7. 既有测试与 E2E 调整声明

1. **既有测试改 1 处**：`internal/cli/goal_p2c_test.go` 的 TestGoalP2CChain
   断言 admission items = 5 → 改为 7。旧断言：`len(items) != 5` 报错「want 5」；
   新断言：7，并加注释说明 S2/AC-141 扩项（baseline_merged + dirty_policy，
   在该非 git 种子场景下均 not_applicable、结论不变）。理由：AC-141 明确要求
   「扩 admission 新增检查项」，计数断言必然随之变化；除此之外 221 个既有
   测试零改动、零回归。
2. **E2E 零改动**：`e2e/run_e2e.sh` 一字未动，原样重跑 **exit 0**（13 步全
   PASS）。新纪律未触发剧本修正的原因：E2E 的 admit/tick 时点上工作区只有
   untracked 的普通产出（e2e-work/），按第 3 节 DERIVED 口径不算违规；基线
   已在 STEP 4b 提交落账，祖先腿天然成立。

## 8. 实跑样例（真 git 仓库，bin/beacon 文本输出摘录）

```
--- freeze + commit + baseline (S1 facts the flow consumes)
freeze baseline_status: uncommitted
baseline: committed 0bb9cbd90dee recorded: True
--- skip attempt: goal flow advance --step canonical_commit (from 0/8)
{ "run_id": "run_9f0c66cee43d2beb995a3ad4", "steps": [ ... pending ... ] }
  -> error: standard_flow_order_violation（next pending: support_advisory）
--- eight steps in order
advance support_advisory            advanced: True count: 1
advance canonical_truth_authoring   advanced: True count: 2
advance canonical_refreeze          advanced: True count: 3
advance canonical_commit            advanced: True count: 4
advance feature_worktree_reconcile  advanced: True count: 5
advance governance_verify           advanced: True count: 6
advance workspace_admission         advanced: True count: 7
advance implement_resume            advanced: True count: 8
--- repeat (idempotent)
already_done: True complete: True
--- goal flow bundle
status: pass allow: True baseline: 0bb9cbd90dee
  truth_map_verify           ok: True applicable: True [derived]
  verify_materials_strict    ok: True applicable: True [live]
  git_diff_check             ok: True applicable: True [live]
  branch_governance_verify   ok: True applicable: True [recorded]
  workspace_admission        ok: True applicable: True [recorded]
```

AC-141 的未合入拦截→合并转绿、包目录脏改拦截、bundle 五项逐项缺失拦截、
tick/delegate 同判、非 git 不误红，均有自动化测试逐步断言
（stdflow_s2_test.go / bundle_s2_test.go），不在此重复贴日志。

## 9. 验证结果与交付物

- `go build ./...`、`go vet ./...`、`gofmt -l` 全清；
  `go test ./... -count=1` **232 个测试全过**（221 既有零回归 + 新增 11：
  orchestrator 10 + cli 1），含真 git 临时仓库行为测试。
- E2E：`bash e2e/run_e2e.sh` **exit 0**，全链 PASS（13 步，剧本零改动）。
- 二进制：`bin/beacon` 16,327,717 B（v2.0.0-go.6 不变）。
- 新增文件：`internal/orchestrator/stdflow.go`（374 行，八步状态机）、
  `internal/orchestrator/bundle.go`（485 行，AC-142 捆）、
  `internal/orchestrator/baseline_check.go`（233 行，AC-141 判定）、
  `internal/cli/goal_flow.go`（148 行，`goal flow` 子命令组）、
  `internal/orchestrator/stdflow_s2_test.go`（303 行）、
  `internal/orchestrator/bundle_s2_test.go`（285 行）、
  `internal/cli/stdflow_s2_test.go`（81 行）；改动面：core/reasons.go（S2 码
  家族）、harness/git.go（+StatusEntries/DiffCheck 两个只读帮手）、
  orchestrator/meta.go（+FlowState）、admission.go（+第 6/7 项）、tick.go、
  delegate.go、exec_delegate.go（入口改走 bundle）、cli/goal.go（注册 flow
  子组）、cli/goal_p2c_test.go（声明见第 7 节）。全部源文件 < 500 行。

## 10. 已知边界（留给 S3 的衔接点）

- AC-143 的其余面（refreeze summary 的 idempotent resume 全语义）与 AC-144
  （六 skill progressive examples 呈现）不在本片：本片只落了 flow 的
  already_done 幂等面与 CLI 薄面，未动 skill 包。
- AC-173（S3 拓扑）将要求 bundle 增 `topology_gate` 项——bundle 的五项表是
  显式切片，S3 追加一项即可，headline 规则不变。
- dirty 判定以项目根 = 仓库顶层为前提（与 S1 基线探针同一前提）；项目根是
  仓库子目录时，卫星工位内的包相对路径仍按项目布局镜像判定，根绑定则按
  子目录 status 判定，porcelain 路径以判定目录为基准——子目录型项目的
  包目录腿可能漏判父级折叠的 untracked 条目，属已知边界，留待真实场景出现
  再收紧。
- flow 的 implement_resume 判据只读 `decideResume` 的结论，不执行 resume 的
  副作用（软阻断清除/中断消费仍归 `goal resume` 本人）——flow 是程序状态机，
  不是第二个 resume 入口。
