# Beacon Go 两组标准全补 · S3 实现报告（拓扑 AC-171/172/173 + 两组收口）

> 日期：2026-10-22 ｜ 版本：**v2.0.0-go.7**（本片发版）｜ 前片：S1（`docs/stdflow-s1-implementation.md`）、S2（`docs/stdflow-s2-implementation.md`）
> 一句话：项目有几个 git 仓、每个仓归谁管、有没有脏改，这份「地形图」以前只活在 v1 里；本片把它装进 Go 内核——图谱能加载、能投影、能实测探测，进 implement 前必须过这道拓扑门。

## 1. 逐 AC 落地表

| AC | 冻结要求（大白话） | 落点 | 判定 |
|---|---|---|---|
| AC-171 | v1.6.7 的作用域图谱（GitScopeGraph）要能原样加载，投影成 BranchContext，四个字段（拓扑类型/作用域列表/门状态/门引用）一个不能少；图谱缺关键字段直接拒绝 | `internal/orchestrator/topology.go`：`LoadGitScopeGraph`/`ValidateGitScopeGraph`/`ProjectBranchContext`；投影落账在 `topology_record.go`；CLI 在 `internal/cli/goal_topology.go` | ✅ tested |
| AC-172 | 三种拓扑（单仓/monorepo/多仓工作区）都要真判：可写作用域有违规脏改就 blocked，多仓里任一卫星仓 blocked 就整体 blocked | `EvaluateTopologyGate`（纯判定）+ `topologyScopeStates`（实测收集），同一文件 | ✅ tested |
| AC-173 | loop 启动和 implement 准入必须消费拓扑门：没引用拦、状态不是 pass 拦；bundle 的 branch-governance 项必须带 topology 子结论 | run 事实层（`engine.go` CreateRun）、准入第 8 项与 bundle 共用判定 `evalTopologyGateItem`（`topology_record.go`、`admission.go`、`bundle.go`） | ✅ tested |

冻结逐字码全部照原文用：`gitscope_branch_context_parity_failed`、`topology_gate_missing`、`topology_gate_blocked`，加上 v1.6.7 的 `satellite_worktree_dirty`、`monorepo_service_dirty:<key>`、`multi_git_primary_missing`。本片只新登记 1 个码：`topology_scope_unreadable`（作用域目录不是 git 仓、无法检查脏状态时用；v1 没有对应码，登记在 `internal/core/reasons.go` 并在此明示）。

### AC-171 的两个形态决定（先说清）

- **引用形态**：v1 的 `topology_gate_ref` 是一个状态文件路径（`.beacon/state/git-scope-graph/...`）。Go 版按本项目既定约定「状态只存全局 DB 一份、不许自带 JSON 状态文件」，改为内容寻址引用 `bc_<图谱哈希前 16 位>`：同一张图谱永远同一个 ref，图谱改任何一个作用域 ref 就变。投影本身落在锚点 feature 的事件轨迹上（`branch_context_projected` 事件，带完整投影与图谱哈希），没动冻结的九表结构。
- **投影状态是快照、准入永远实测**：投影里存的 `topology_gate_status` 是投影那一刻的实测结果；准入消费时一律拿当前工作区重新判，不信快照——脏改是会变的，存下来的 pass 不能当通行证。

### 图谱字段来源（fixture 实证）

字段形状不是凭冻结文字猜的，是从源仓两处实证对出来的：

- 真实 fixture：`~/workspace/repos/beacon/docs/beacon/archive/v1.6.7/.machine/schemas/git-scope-graph-v167.fixture.json`（schema 名 `git-scope-graph.v1`，含单仓与多仓两个完整案例）；
- 生成器原文：源仓 `beacon/utils/git_scope_graph.py`（423 行）与它的测试 `tests/test_v167_git_scope_graph.py`，scope 字段以生成器实际吐出的为准（`scope_id/git_root/role/branch_resolution_mode/target_branch/worktree_path/admission_artifact_ref/dirty_policy`，外加绑定层的 `writable` 标记）。

校验 fail-closed 的条件：schema 版本不对、拓扑类型不在三值内、scopes 为空、scope 缺 `scope_id`/`git_root`/`role`、scope_id 重复、单仓不是恰好一个 primary、多仓没有 satellite、monorepo 没有 service——全部报 `gitscope_branch_context_parity_failed`。fixture 的多仓案例已写进 Go 测试逐字验证能加载能投影。

### AC-172 的判定口径（两种脏，区别对待）

- **primary 作用域**沿 S2 的 dirty policy：只有需求包目录和受保护路径（P4c 名单，如 `.env`、secrets/、auth/ 等）脏改算违规，普通代码文件在 implement 前脏着不算事——这是 S2 已定的口径，不能在拓扑门里偷偷收紧。
- **satellite / service 作用域**是交付仓，按 v1 原口径：它领地内有任何脏改就 blocked（v1 就是这么判的，码也用 v1 的）。
- 只读作用域（writable=false）不判脏——monorepo 的根 primary 就是只读的，它的脏改不进门。
- 任一作用域 blocked → 汇总 blocked；多仓的 primary 目录根本不是 git 仓 → `multi_git_primary_missing`（v1 同款处理）。

### AC-173 的三个落点

- **(a) run 事实层**：`goal run` 在 git 项目上启动时，没投影就如实把 `topology.status=missing` 写进 run 的事实层（RunView 的 `topology` 字段），不硬拦——run 是编排起点，原文管的是「不得进入 implement」。
- **(b) 准入第 8 项**：共享准入检查器新增 `topology_gate` 项，与 S2 的 bundle 共用同一个判定函数，一处判定两处消费。没投影、或只有未确认草案 → `topology_gate_missing` 拦；投影在但实测非 pass → `topology_gate_blocked` 拦。
- **(c) bundle 子结论**：AC-142 bundle 的 `branch_governance_verify` 项新增 `topology_gate` 子结论（status/ref/reason_code），拓扑不过时该项跟着红，失败码保留 AC-173 自己的码（不被 bundle 的通用码吞掉，与第 5 项 workspace_admission 失败时的既有规则一致）。这对应 v1 的 AC-GSG-007/008（bundle 含 topology_gate 项、无有效 ref 的 pass 不成立）。
- 非 git 项目维持 S1/S2 口径：`not_applicable` 明示，不误红。

## 2. 探测判据（没有现成图谱时，`goal topology detect`）

实测探测，不是猜；探测结果是**草案**（来源标 `generated_draft`），人用 `goal topology confirm` 确认后准入才消费，未确认的草案在准入眼里等于没有。判据：

- 项目根下（深度 1）有子目录本身是 git 仓根 → **多仓工作区**：根是仓则根为 primary，每个子仓一个 satellite；根不是仓时 primary 先占位指向根，判定时会如实报 `multi_git_primary_missing`，等人修草案。
- 根是仓、且仓内有独立包根 → **monorepo**：primary 只读，每个包根一个 service 作用域（path=相对目录，与 v1 用 service_key 当仓内路径的口径对应）。「独立包根」的判据标 **DERIVED**：目录内有自己的包清单文件（go.mod / package.json / Cargo.toml / pyproject.toml / setup.py / pom.xml / build.gradle），探测深度 ≤2，跳过 .git/.beacon*/node_modules/vendor。
- 根是仓、其余都没发现 → **单仓**。
- 根不是仓、也没有子仓 → fail-closed 报错 `worktree_not_a_git_repository`，不编图谱。

子仓判定用「该目录是仓根」（rev-parse 的 toplevel 等于自身），根仓的子目录不会被误认成子仓。

## 3. 实跑样例（真二进制 v2.0.0-go.7，非测试替身）

图谱投影（三拓扑各一例，`goal topology project`，输出节选 kind/ref/status）：

```text
multi clean:            multi_git_workspace bc_b3ea37388ad6b5b3 pass
multi 卫星仓改一文件:    multi_git_workspace bc_b3ea37388ad6b5b3 blocked   ← 同 ref、状态实测翻转
monorepo clean:         monorepo bc_67e97fa61f0cce98 pass
monorepo svc-b 改文件:   monorepo bc_67e97fa61f0cce98 blocked
detect 单仓草案:         single_repo bc_de50323731a03acb confirmed=false source=generated_draft
confirm 之后:           single_repo bc_de50323731a03acb confirmed=true
```

准入拦截一例（git 项目、包已冻结提交落基线、`goal admit`）：

```text
无投影:  allow=false headline=topology_gate_missing | topology_gate: ok=false topology_gate_missing
投影后:  allow=true  headline=goal_implement_admission_ok | topology_gate: ok=true (pass over bc_7287af721e7ea089)
该 run 创建于投影之前，goal status 的事实层如实显示: topology.status=missing
```

E2E 全链（`e2e/run_e2e.sh`）exit 0：新增 STEP 5b 投影（ref `bc_e19764fed643b3be`、gate=pass），STEP 7 准入消费同一 ref 通过，全链 13 步 PASS。

## 4. 两组收口表（10 条）

| AC | 内容 | 落点 | 测试 |
|---|---|---|---|
| AC-136 | truth 写入只许 canonical 分支 | S1 `branch.go` 五写动作守卫 | TestS1BranchGuardRejectsNonCanonical 等 4 个 |
| AC-137 | refreeze 拒空转/拒漂移 | S1 三路 fail-closed + 表面指纹 | TestS1RefreezeEmptyRefused/RevisionDrift/ChainDrift |
| AC-138 | 八步标准流状态机 | S2 `stdflow.go` | TestS2FlowFullWalkAndIdempotent 等 4 个 + CLI 1 个 |
| AC-139 | 冻结须提交才成基线 | S1 FrozenFact 基线维度 + `truth baseline` | TestS1BaselineLifecycle 等 2 个 + E2E 4/4b |
| AC-140 | 卫星工位 stale refreeze 隔离 | S1 标记式隔离 + merge 前重扫 | TestS1WorktreeQuarantineBlocksMerge |
| AC-141 | 基线须被包含 + dirty policy | S2 准入第 6/7 项 | TestS2BaselineNotMergedBlocksAndMergeFixes 等 2 个 |
| AC-142 | 准入证据捆五项 | S2 `bundle.go`（S3 起第 4 项含 topology 子结论） | TestS2Bundle* 4 个 + CLI 1 个 |
| AC-171 | 图谱加载与投影 parity | S3 `topology.go` | TestS3FixtureGraphProjectsWithFrozenFields / TestS3GraphParityFailures + CLI |
| AC-172 | 三拓扑门行为 | S3 `EvaluateTopologyGate` | 纯判定 1 个 + 真 git 实测 3 个（单仓/monorepo/多仓） |
| AC-173 | 拓扑门强制消费 | S3 事实层 + 准入第 8 项 + bundle 子结论 | TestS3AdmissionTopologyLegs / TestS3DetectDraftConfirmFlow / TestS3RunFactLayer + E2E 5b/7 |

`docs/ac-matrix.md` 已同步：10 行全部改为 tested 并逐行写明测试名；统计 tested 71→81、implemented 67→64、deferred-external 21→17、dropped-by-charter 12→9，合计仍 171（逐行自查一致）。

## 5. 测试与改动声明

全量：`go build/vet/gofmt/test ./... -count=1` 全过，**243 个顶层测试**（基线 232 + 新增 11，零回归）。新增测试：orchestrator 10 个（`topology_s3_test.go`、`topology_s3_live_test.go`）+ CLI 1 个（`goal_topology_test.go`）。

既有测试改动（逐条声明，全部是「补投影种子」或计数跟随判定扩展，没有一处放宽判定）：

1. `stdflow_s2_test.go` 的 `s2Setup`：基线落账后补一行单仓投影种子——git 项目自 AC-173 起 implement 入口必须有投影，这个公共夹具代表「已投影的项目」；S2 各场景继续判自己的腿。
2. `stdflow_s1_test.go` TestS1BaselineLifecycle：在「准入转绿」断言前补同一颗种子（理由同上）。
3. `p3c_test.go` TestDelegateRalphRealLoopCollects、`cli/goal_p3c_test.go` TestGoalP3CRealChain、`cli/stdflow_s2_test.go` TestS2GoalFlowCLI：三个真集成场景在走 bundle 前补投影种子（后两者经真 CLI `goal topology project` 播种）。
4. `cli/goal_p2c_test.go`：准入项数断言 7→8（新增 topology_gate 项；该夹具是非 git 项目，新项为 not_applicable，行为断言未动）。
5. `cli/goal_p2d_test.go`：版本串断言 go.6→go.7（发版跟随）。

E2E 改动声明：新增 STEP 5b（投影）与 STEP 7 的 topology_gate 项断言；头部链路说明与版本守卫同步 go.7。改完重跑 exit 0，与 S1 加 STEP 4b 的先例同款处理。

一处测试中发现并记录的事实：git porcelain 对未跟踪目录只报目录名（如 `auth/`），报不到文件级条目，所以受保护路径的脏改实测要用文件级条目（如根下 `.env`）——S3 的 blocked 实测即按此构造，判定代码未因此改动（S2 的 dirtyViolations 对文件级条目工作正常）。

## 6. 交付清单（文件名 + 大小）

| 文件 | 大小 |
|---|---|
| `internal/orchestrator/topology.go`（图谱/投影/纯判定/实测收集，440 行） | 16,416 B |
| `internal/orchestrator/topology_record.go`（落账/解析/确认/事实层/共享判定，271 行） | 10,405 B |
| `internal/orchestrator/topology_detect.go`（实测探测，150 行） | 5,699 B |
| `internal/orchestrator/topology_s3_test.go`（408 行） | 16,678 B |
| `internal/orchestrator/topology_s3_live_test.go`（122 行） | 5,089 B |
| `internal/cli/goal_topology.go`（show/project/detect/confirm，218 行） | 7,358 B |
| `internal/cli/goal_topology_test.go`（75 行） | 2,808 B |
| 改动：`internal/core/reasons.go`（S3 码家族 + 前缀登记）、`admission.go`（第 8 项）、`bundle.go`（子结论）、`meta.go`/`engine.go`（事实层）、`cli/goal.go`（注册子命令组）、`cli/root.go`（版本 go.7） | — |
| `docs/ac-matrix.md`（10 行终局 + 统计） | 61,344 B |
| `e2e/run_e2e.sh`（STEP 5b + 版本） | 18,056 B |
| **二进制 `bin/beacon`（实跑确认 Beacon CLI v2.0.0-go.7）** | **16,422,657 B** |

边界自查：源仓只读未动；冻结 schema 未动（投影走既有事件轨迹，无新表）；未新设顶层命令（topology 折叠进 goal 组）；所有新文件 <500 行。
