# Beacon Go 重构 — Phase 0 盘点报告（Inventory + Baseline + Skeleton）

> 日期：2026-10-21 ｜ 执行决定：按 Go 重构继续，不管 Rust 轨道——即用 Go 实现已冻结的 v2 内核设计，不追 Rust 代码。
> 源仓（只读，未改动）：`~/workspace/repos/beacon/`，HEAD `43c48a8a`，Python 842 文件、`beacon/commands/` 78 个 .py、`tests/test_*.py` 全仓 500 个。
> 冻结真相（只读）：`~/workspace/repos/beacon/docs/beacon/archive/v2.0.0/features/beacon-v2-rust-kernel-redesign/{truth,tests,tasks,evidence}.md`（truth 842 行，`status: frozen`，revision `r19-scenario-completeness-s01-s19`，`scope_mode: full_parity`）。
> 新仓：`~/workspace/beacon-go/`（本报告所在仓）。

---

## 1. 工具链

| 项 | 值 |
|---|---|
| Go 版本 | **go1.27.3 linux/amd64**（go.dev/dl `?mode=json` 当前最新 stable，2026-10-21 实测） |
| 安装包 | `go1.27.3.linux-amd64.tar.gz`，70,588,678 字节，sha256 `65efbc80fdc1fa2cd2631cad5b481777a68e8725f34e89f50fc6ac714e379b40`（与官方 JSON 一致，`sha256sum -c` OK） |
| 下载方式 | 经 `$HTTPS_PROXY` 从 `https://go.dev/dl/` 直下，一次成功，未用镜像、未用 apt |
| 安装位置 | `~/workspace/toolchains/go/`（持久目录，276M）；`~/workspace/toolchains/go/bin/go version` → `go version go1.27.3 linux/amd64` |
| 后续使用 | 每次 shell 需 `export PATH=~/workspace/toolchains/go/bin:$PATH`（未写全局 profile） |

---

## 2. CLI 全量盘点

### 2.1 来源与口径

- `beacon/command_catalog.py`（227 行）：分层目录（layers）是命令分类权威。
- `beacon/cli_factory.py`（262 行）：Typer 实际注册面。工厂注册了目录里没有的命令（goal/truth/eval 等），盘点必须以**两者并集**为准。
- `SUPPORTED_SUBCOMMANDS`：62 条原始 / **61 条唯一**（`qa` 重复出现 1 次，是目录自身的小瑕疵）；它比 layers 并集少 `change`、`prd-bootstrap`、`labs`、`bridge`，多 `planner-review` 1 条。

### 2.2 分组统计（command_catalog.py layers）

| 分组 | Catalog layer | 数量 | 命令 |
|---|---|---:|---|
| core 7 | current | 7 | prd, user-story, test-case, prototype, implement, qa, release |
| support | support | 30 | board, code2truth, help, status, doctor, archive, ask, skill, profile, orchestrate, report, review, real-project-validation, friction-intake, feedback, freeze, plan, program, reopen, remember, recall, resume, version, evolution-summary, init, test-progress, debug, deep, design, hooks |
| compat | compat | 5 | think, workflow, gate, verify, auto-test |
| labs | labs | 12 | analyze, api-design, brainstorm, data-dictionary, jtbd, kano, lighthouse, memory-friction-evolution, moscow, persona, roadmap, ui-spec |
| bridge | advanced（BRIDGE） | 3 | run, team, ralph-bridge |
| deprecated | deprecated | 3 | prompt, save, work |
| internal | internal | 4 | change, prd-bootstrap, labs, bridge |
| **目录并集** | | **64** | 七组互不重叠 |

工厂额外注册（不在 catalog layers 中，共 13 条顶层名 + 2 个容器）：

`goal`、`loop-goal`、`truth`、`truth-map`、`truth-cutover`、`eval`、`manifest`、`task`、`workspace`、`planner-review`、`gen-truth`（truth gen 别名）、`materialize-feature-package`、`validate-feature-package`；容器 `labs`、`bridge`（已在 internal 组）；嵌套子命令 `bridge ralph`（= ralph-bridge 的桥内名，不单列）。

**全量口径：可调用顶层命令名 = 64 + 13 = 77 个**（不含根命令 `beacon` 本身、不含 `bridge ralph` 嵌套别名）。`beacon/commands/` 下 78 个 .py 文件（72 顶层 + `implement/` 子包 6 个）与此基本对应；另有 `automation.py`、`feature_package.py` 等只经工厂以函数/子包形式接入。

默认用户面（`DEFAULT_USER_SURFACE`）= core 7 + support 30 = 37；高级面（`ADVANCED_SURFACE`）= labs 12 + bridge 3 + deprecated 3 + compat 5 = 23（工厂里默认 hidden，`BEACON_LABS=1` / `BEACON_ADVANCED=1` 才显示容器）。

### 2.3 命令 → v2 六能力映射表（77 条全量）

映射依据：冻结 truth 的六能力合同（loop 跨 harness 编排；truth 吸收 plan/pre-freeze intent；design 仅 UI/视觉/交互；implement 只消费 frozen truth；qa-evidence 承接 evaluator；release 只做 go/no-go）。非六能力者标 `support-keep`（降为 kernel internal module，AC-125）或 `legacy-drop`（v2 面不保留，v1 轨按双轨继续存在）。

| 命令 | 原分组 | v2 归属 | 备注 |
|---|---|---|---|
| prd | core | truth | truth 包 authoring（`features/<slug>/truth.md`），v2 不再生成顶层 prd 文档 |
| user-story | core | truth | 同上，包内材料 |
| test-case | core | truth | 覆盖真相 `tests.md`（TC 编译归 truth，执行裁决归 qa-evidence） |
| prototype | core | design | 条件触发的 UI/UX 原型，产出 design primitive 绑定输入 |
| implement | core | implement | 唯一实现入口 |
| qa | core | qa-evidence | evaluator 裁决面 |
| release | core | release | human gate 不变量 |
| code2truth | support | truth | 代码反推真相草稿，仍走 freeze 流程 |
| freeze | support | truth | freeze 入口（顶层快捷命令） |
| plan | support | truth | plan 不设第七能力；intent 捕获进 `truth.pre_freeze_intent`（AC-111..114） |
| reopen | support | truth | truth 重开/change 链 |
| orchestrate | support | loop | 编排归 orchestrator |
| program | support | loop | program 编排面（phase graph 推进） |
| resume | support | loop | run checkpoint resume（AC-047） |
| real-project-validation | support | qa-evidence | 最高权重证据层，归 evaluator |
| test-progress | support | qa-evidence | 覆盖/进度读面 |
| design | support | design | design 能力正名入口 |
| board | support | support-keep | 看板读面（Spine projection） |
| help | support | support-keep | 帮助/路由说明面 |
| status | support | support-keep | internal status module（AC-125） |
| doctor | support | support-keep | internal doctor module；`verify-truth-integrity` 路由到 gate（AC-103） |
| archive | support | support-keep | internal archive module |
| ask | support | support-keep | internal router（意图路由，不裁决） |
| skill | support | support-keep | internal harness loader；统一 skill-score 在 P4 接入（AC-151） |
| profile | support | support-keep | 配置面 |
| report | support | support-keep | 报告投影面 |
| review | support | support-keep | support advisory，只提 route 不改 truth（Standard Flow 首步） |
| friction-intake | support | support-keep | friction 采集面 |
| feedback | support | support-keep | 反馈采集面 |
| remember | support | support-keep | memory 写入面 |
| recall | support | support-keep | memory 读面 |
| version | support | support-keep | 版本面治理读面 |
| evolution-summary | support | support-keep | 汇总投影面 |
| init | support | support-keep | 项目初始化/上下文装配 |
| debug | support | support-keep | internal debug module |
| deep | support | support-keep | deep 分析 advisory，不出 verdict |
| hooks | support | support-keep | internal hooks module |
| think | compat | truth | WorkflowStage 的 Think 变体归 truth pre-freeze |
| workflow | compat | loop | 旧编排名在 v2 删除，能力并入 loop |
| gate | compat | support-keep | internal/gate 横切四门禁（truth_freeze/qa/release/implement），不单设能力 |
| verify | compat | qa-evidence | 校验裁决归 evaluator |
| auto-test | compat | qa-evidence | 自动测试执行+证据 |
| brainstorm | labs | truth | pre-freeze brainstorm/review synthesis（AC-111..114） |
| ui-spec | labs | design | UI 规格归 design primitive |
| lighthouse | labs | support-keep | 巡检 advisory 面 |
| analyze | labs | legacy-drop | v2 面不保留（v1 轨自理） |
| api-design | labs | legacy-drop | 同上 |
| data-dictionary | labs | legacy-drop | 同上 |
| jtbd | labs | legacy-drop | 同上 |
| kano | labs | legacy-drop | 同上 |
| memory-friction-evolution | labs | legacy-drop | 同上 |
| moscow | labs | legacy-drop | 同上 |
| persona | labs | legacy-drop | 同上 |
| roadmap | labs | legacy-drop | 同上 |
| run | bridge | loop | orchestrator 统一 goal/run（D-03：吸收 beacon-loop + beacon-run） |
| team | bridge | loop | team 长任务归 orchestrator |
| ralph-bridge | bridge | loop | implement lane 桥接归 orchestrator/implement 编排 |
| prompt | deprecated | legacy-drop | 已废弃 |
| save | deprecated | legacy-drop | 已废弃 |
| work | deprecated | legacy-drop | `workflow start` 别名；能力归 loop，名删除 |
| change | internal | truth | change/refreeze 链（canonical branch hard gate，AC-136..137） |
| prd-bootstrap | internal | truth | truth 冷启动材料 |
| labs | internal | legacy-drop | 容器名删除；其子命令已逐条映射 |
| bridge | internal | loop | 容器名删除；其子命令（run/team/ralph）已并入 loop |
| goal | 工厂额外 | loop | Product Goal facade（`beacon/commands/goal.py`），v2 loop 的正名入口 |
| loop-goal | 工厂额外 | loop | 旧 loop 编排名，并入 loop |
| task | 工厂额外 | loop | goal 任务账本，归 orchestrator |
| eval | 工厂额外 | qa-evidence | agent/skill eval；skill-score 统一面 P4（AC-151..156） |
| truth | 工厂额外 | truth | truth 能力正名入口（gen/freeze/change/refreeze） |
| truth-map | 工厂额外 | truth | truth-map resolve/verify，orchestrator 启动预检（AC-041） |
| truth-cutover | 工厂额外 | truth | package-authoritative 切换恢复路由 |
| gen-truth | 工厂额外 | truth | `truth gen` 别名 |
| materialize-feature-package | 工厂额外 | truth | feature 包物化 |
| validate-feature-package | 工厂额外 | truth | feature 包校验（freeze 前置） |
| planner-review | 工厂额外 | truth | 多代理 review runtime 在 P2 落 orchestrator（AC-102），产物供 truth pre-freeze 消费 |
| manifest | 工厂额外 | support-keep | delivery/evidence manifest 是 TraceSpine 投影，非权威（AC-131） |
| workspace | 工厂额外 | implement | workspace admission 产出 `BranchContext.v1`，是 implement 准入前置（AC-116/128/129） |

**映射汇总（77 条）**：truth 18 ｜ loop 11 ｜ design 3 ｜ implement 2 ｜ qa-evidence 6 ｜ release 1 ｜ support-keep 23 ｜ legacy-drop 13。

注：v1 的 `spine` / `projects` 命令在 v1 工厂中**不存在**（只有内部 Spine 语义），是 v2 新增面（AC-017..021），Go 侧将在 P1 新增，不在本映射的 77 条内。

---

## 3. Python 基线（行为契约基线，不修代码）

环境：系统 `pip install -e .` 与 Debian 的 `typing_extensions` 冲突失败（`Cannot uninstall typing_extensions 4.10.0, RECORD file not found`），改用独立 venv `~/workspace/beacon-go-venv/`（`pip install -e ~/workspace/repos/beacon pytest`，成功；pytest 9.1.1）。基线运行在源仓产生的副产物（`beacon.egg-info`、各处 `__pycache__/*.pyc`）已全部清除，源仓 .py 文件零改动（已复核：残留 pyc/egg-info 计数为 0）。

| 基线项 | 命令 | 结果 |
|---|---|---|
| 版本面 | `python -m beacon --version` | `Beacon CLI v1.6.19`（pyproject `version = "1.6.19"`，Typer/Click/Rich/Pydantic 栈） |
| 全量收集 | `python -m pytest tests/ --collect-only -q` | **2004 tests collected in 5.55s，0 collection error** |
| 抽跑契约批 | `pytest tests/test_beacon.py tests/test_cli_factory_mainline_help.py tests/test_entrypoint_parity.py -q` | **79 passed, 0 failed in 18.45s** |
| — 其中 test_beacon.py | 单独 collect | 77 条 |
| — 其中 test_cli_factory_mainline_help.py | 单独 collect | 1 条 |
| — 其中 test_entrypoint_parity.py | 单独 collect | 1 条 |
| 测试文件数 | `find tests -name 'test_*.py'` | tests/ 下 496 个（tests/ 全部 .py 501 个）；全仓 `test_*.py` 500 个 |

说明：本阶段只抽跑入口/工厂/对等三组契约测试作冒烟基线，**未跑全量 2004 条**（全量绿/红分布留待迁移各 phase 以同一 venv 复测对比）。基线数字以本节为准，后续 Go 版对等验证（testscript/golden）以这批 Python 输出为 oracle。

---

## 4. 冻结真相提炼

### 4.1 AC 总数与分布

truth.md 中显式 `AC-xxx` 标记 **166 个不同编号**；其中 AC-084..090 以区间形式给出（`AC-084..AC-090：P0–P6 阶段出口`），补入区间内未逐个写出的 AC-085..089 后，**有效 AC 总数 = 171 个**（编号跨度 AC-001..173，缺号仅 AC-145、AC-146——这两个编号在包内只作为 TC-145/TC-146 存在，不是 AC）。tests.md 与之对应：TC 编号标记 175 个不同 token，另有负向 TC-N01..N40（40 个）与 E2E TC-E01..E05（5 个），且冻结了「每个 AC 至少一行行为级 TC、弱 TC 不得独过（AC-164）」。

| AC 段 | 数量 | 内容 |
|---|---:|---|
| AC-001..104 | 99（显式）+ 阶段出口区间 | 核心合同：双轨边界、存储三层、Spine API、crate 结构、双 FSM、证据链、orchestrator + agent 真集成、性能、harness surface、CLI 面、九表 schema、阶段出口、风险、验收总则、v1.6 治理移植（TIG/planner/doctor） |
| AC-105..108 | 4 | Loop Engineering 对齐：CLI 选 agent、无 daemon、stop_when maker≠checker、Developer Feedback checkpoint、三层 Loop 可观测 |
| AC-109..144 | 36 | Minimal Kernel 扩展：六能力定义、intent snapshot/prompt ledger/decision packet、design primitives gate、agent trace/stagnation/remediation、loop guardrails、QA confidence/FSM/defect chain、release human gate、progressive disclosure、BranchContext、TraceSpine、canonical truth branch、Standard Execution Flow |
| AC-147..156 | 10 | 长期双轨（UD-B）+ 统一 skill 评分（只评分、人改） |
| AC-157..173 | 17 | R18 场景完备 S-01..S-19：双轨安装/写域、archive 面、scaffold/schema 闭环、MD golden parity、phase 出口≠release、弱 TC 禁令、reason_codes 分层、跨轨证据污染、对抗式审查、共享合同白名单、skill-score 语料、GitScopeGraph→BranchContext 拓扑 admission |

### 4.2 P0 清单（Phase 0 / P0 出口前必须成立）

来自 truth 共存表、tasks.md Phase P0 与 R18 的 P0 标注：

- **结构闭环**：AC-001（Go workspace 等价物成立）、AC-022/023（五模块收敛 + 依赖无环）、AC-024（无 proto/gRPC 硬依赖，D-V2-001 accepted defer）、AC-160（core closure 无 Python 成员，`core_closure_python_present` 阻断）。
- **类型层**：AC-026..031（WorkflowStage/StageStatus 枚举、转换表、Blocked 带 reason_codes、Skipped 须 deferral 引用、Delivery 12 态与 runs.status CHECK 一致）、AC-042 的类型部分（IterationResult/AgentBackend 接口形状）。
- **存储骨架**：AC-004/005（truth 权威在 MD、DB 不存需求正文）、AC-012（lifecycle_events append-only）、AC-061（BEACON_STORE_PATH 隔离）、AC-067..076（九表字段合同 + 六 DAO 骨架）、AC-080（user_version 管理、禁止降级）、AC-161（schema 合同与实现不一致必须显式决策，`schema_contract_drift` 阻断）。
- **双轨/场景 P0**：AC-157（双轨安装 + 显式选轨矩阵）、AC-158（全局 DB 按轨写域/writer lease，`dual_track_writer_conflict`）、AC-159（archive 面显式 `--version` 可定位）、AC-163（phase 出口 scorecard 标 `authority=phase_exit_only`，不得冒充 release）。
- **program 入场**：TC-145（program surface：manifest/phase-graph/admission-gates/run-state 存在且 `fsm_state=canonical_truth_materialized`）。
- **P0 出口**：AC-084（core+store+skeleton 测试通过）。迁移在 P0 只做 **skeleton**（user_version + 备份钩子），**非** production 全量迁移（全量迁移在 P3）。

### 4.3 Phase 图（program phase-graph + truth 共存表对齐）

```text
program_intent_captured
  -> canonical_truth_materialized -> canonical_refreeze_complete
  -> canonical_commit_ready -> feature_worktree_reconciled
  -> governance_verified -> workspace_admitted
  -> P0_running -> P1_running -> P2_running -> P3_running
  -> P4_running -> P6_running -> qa_evidence_ready -> release_candidate

P0 core/store        五模块骨架 + 九表 schema + migration skeleton        出口 AC-084
P1 spine/projects    Spine read（show/list/artifacts/gate）+ projects 注册表 + JSON schema   出口 AC-085
P2 orchestrator      goal/resume/stop/status + decision_packet/BranchContext/TraceSpine
                     + Codex/Subprocess 真集成（AC-043/044）+ planner_review（AC-102）  出口 AC-086
P3 gate/migration    gate 写库 + 全量 migration + projection rebuild
                     + Claude/ACP/ImplementHarness 真集成（AC-045/046/099/100）+ TIG（AC-101） 出口 AC-087
P4 six-ability       六能力 harness surface 精确收敛 + progressive disclosure
                     + v2 路径内禁止 cross-track fallback（AC-062）+ 双轨/skill-score（AC-147..156, 168..170） 出口 AC-088
P5 proto             跳过（D-V2-001 accepted defer，AC-089）
P6 benchmark         性能门禁 AC-051..058（冷启动 p95<100ms、spine<50ms、resolve<200ms、
                     gate<50ms、hash<10ms、migration<60s、调度<100ms、DB 软上限 500MB）
                     + E2E + evaluator 全矩阵；P6 是 v2 成熟度出口，不是 v1 关停开关   出口 AC-090
```

准入规则（phase-graph 原文要点）：后一 phase 必须以前一 phase 证据绑定 AC/TC 为前提；缺 `decision_packet.v1` 不得启动 loop；无 TraceSpine 完整性不得 implement/qa/release；release 永远人工确认；已完成步骤重跑返回 `up_to_date`/`pass`（幂等）。

### 4.4 存储与运行态权威（一句话版）

L1 需求真相 = Markdown feature package（唯一权威）；L2 运行态 = 全局 SQLite `~/.beacon/beacon.db`（WAL，禁止 per-project DB，AC-008）；L3 大文件 = 项目 `<project>/.beacon/` blob 层，DB 只存绝对路径 + hash 指针（hash 不符标 `stale`，AC-010）。

---

## 5. Rust crate → Go package 映射

用户决定：用 Go 实现冻结设计，不追 Rust 代码。下表把冻结 truth 中的 Rust 结构 1:1 翻译为 Go 结构（本 Phase 0 骨架已按此落地）：

| 冻结设计（Rust） | Go 落地 | 职责 |
|---|---|---|
| `beacon-core` | `internal/core` | 纯类型层：WorkflowStage / StageStatus / Delivery RunStatus、转换规则、EvidenceChain、IterationResult |
| `beacon-store` | `internal/store` | 全局 SQLite `~/.beacon/beacon.db`、九表、六 DAO、migration、SpineDao |
| `beacon-orchestrator` | `internal/orchestrator` | goal/run/team 长任务编排、checkpoint/resume/stop、AgentBackend 实现、TraceSpine 写入 |
| `beacon-gate` | `internal/gate` | truth_freeze / qa / release / implement 四门禁 + reason_codes |
| `beacon-cli` | `cmd/beacon` + `internal/cli` | 主 CLI（Cobra 根命令 + 子命令注册）、spine/projects/run 等命令面 |
| `beacon-harness` | `internal/harness` | 六个一等能力的注册表与 contract metadata、progressive disclosure loader |
| `beacon-proto` | —（不建） | D-V2-001 accepted defer，post-v2.0.0 再议 |

依赖方向（AC-023 翻译）：`cli -> {gate, orchestrator, store, core, harness}`；`store -> core`；`orchestrator -> {store, core}`；`gate -> {store, core}`；禁止循环依赖（Go 编译器天然强制 import 无环）。

### 5.1 语言替换点（Rust → Go）

| 维度 | Rust 原设计 | Go 替换 | 说明 |
|---|---|---|---|
| 工程定义 | Cargo workspace + 各 crate `Cargo.toml` | 单 `go.mod`（module `beacon-go`）+ `internal/` 包布局 | Go 无 workspace 概念（go.work 可选，后续多模块再引入）；五 crate 边界用包 + 内部可见性守住 |
| 单元边界 | crate | package | `internal/` 阻止外部 import，等价 crate 的非 pub 面 |
| 错误类型 | `thiserror` typed error + reason_code | 标准库 `errors`/`fmt.Errorf("%w")` + 自定义 typed error struct 携带 `ReasonCode` 字段 | reason_code 必须是结构体字段而不是字符串拼接，对齐 AC-027/028/165 |
| 枚举/FSM | `enum` + 转换表 | `type X string` 常量集 + 显式转换表（map/switch）+ 表驱动测试 | Go 无 sum type；非法转换返回 typed error + reason_code；常量值与 DB CHECK 约束同源生成/校验 |
| Trait | `AgentBackend` trait | Go `interface`（iterate/health_check/abort/name） | IterationResult 在 `internal/core` 强类型定义（AC-042） |
| 并发/信号 | tokio + signal handling | 标准库 goroutine + `context.Context` + `os/signal`（SIGINT graceful stop） | 无 daemon 的 CLI 进程模型不变（AC-105） |
| SQLite 驱动 | `rusqlite` | **`modernc.org/sqlite`（选定）** | 见下 |
| 性能验收 | criterion bench | `go test -bench` + CI 归档 | AC-051..058 的阈值不变，只换测量工具 |
| CI | `cargo build/test` matrix | `go build ./...` / `go vet ./...` / `go test ./...` matrix（macOS + Linux，AC-091 翻译） | — |
| 模块行数 | 单文件 >500 行须拆分或 ADR（AC-025） | 不变 | Go 文件同样执行 |

**SQLite 驱动选型：选 `modernc.org/sqlite`，不用 `mattn/go-sqlite3`。**
理由：① `modernc.org/sqlite` 是纯 Go（SQLite C 代码转译），`CGO_ENABLED=0` 即可跨平台交叉编译出单一静态二进制，与 Go CLI 的分发方式和 AC-051 冷启动、AC-091 双平台 CI 矩阵最契合，不需要目标机 C 工具链；② `mattn/go-sqlite3` 需 CGO，交叉编译 macOS/Linux 必须各自备 C 编译器，CI 与发布复杂度显著上升。代价（诚实记录）：modernc 版本二进制更大、极端吞吐略低于 CGO 版，且其转译层跟随 SQLite 上游有滞后；本项目是单写者、低并发的本地治理库（WAL + append-only 事件），该代价可接受。Phase 0 骨架**尚未引入**该依赖（本阶段依赖只用 Cobra），store 真实接线时再 `go get modernc.org/sqlite`。

---

## 6. Go 骨架（已落地并验证）

### 6.1 结构树

```text
~/workspace/beacon-go/
├── go.mod / go.sum
├── cmd/beacon/main.go            # 入口（对应 v1 beacon/main.py + __main__.py）
├── internal/
│   ├── cli/root.go               # Cobra 根命令：--version + 六能力占位子命令
│   ├── core/core.go              # WorkflowStage/StageStatus/RunStatus/IterationResult/EvidenceChain 类型
│   ├── store/store.go            # BEACON_STORE_PATH 解析 + 九表合同（驱动未接线）
│   ├── orchestrator/orchestrator.go  # 包占位（职责与禁区注释）
│   ├── gate/gate.go              # GateType/Verdict/Result 类型
│   └── harness/harness.go        # 六能力注册表（AC-059/063/109 的数据源）
├── skills/
│   ├── skills.go                 # go:embed 占位
│   └── README.md                 # 被 embed 的占位文件（未拷全量 skills）
├── docs/phase0-inventory.md      # 本报告
└── bin/beacon                    # 构建产物
```

module 名 `beacon-go`，`go 1.27.3`。依赖只有 `github.com/spf13/cobra v1.10.2`（+ 传递依赖 `spf13/pflag v1.0.9`、`inconshreveable/mousetrap v1.1.0`），其余全标准库。

### 6.2 构建验证（实跑输出）

```text
$ go build ./...   → OK（无输出）
$ go vet ./...     → OK（无输出）
$ gofmt -l .       → （空，全部格式化）
$ go build -o bin/beacon ./cmd/beacon → OK
```

`./bin/beacon --version`：

```text
Beacon CLI v2.0.0-go-dev
```

`./bin/beacon --help`：

```text
Beacon v2 kernel (Go implementation of the frozen v2 design).
Six first-class abilities: loop | truth | design | implement | qa | release
Runtime state: global ~/.beacon/beacon.db + project blob pointers.

Usage:
  beacon [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  design      Design primitive contract (design_primitives.v1) gating implement when design is required
  help        Help about any command
  implement   Consume frozen truth + bound design + admitted worktree; run agent/backend loop
  loop        Cross-harness orchestration: truth -> design? -> implement -> qa-evidence -> release
  qa          qa-evidence: behavior evidence, confidence chain, business FSM coverage, defect repair chain
  release     Final go/no-go gate; human confirmation is an invariant
  truth       Pre-freeze intent capture, prompt snapshots, decision packet, freeze/refreeze

Flags:
  -h, --help      help for beacon
  -v, --version   version for beacon

Use "beacon [command] --help" for more information about a command.
```

占位子命令冒烟：`./bin/beacon truth` → `beacon truth: not implemented yet (Phase 0 skeleton; ability contract frozen in v2 truth)`，exit 0。六能力中 `qa` 子命令名按任务约定用 `qa`，其 contract 语义为冻结 truth 的 `qa-evidence`（help 文案已注明）。

---

## 7. 交付物清单（文件名 + 大小）

| 文件 | 大小（字节） |
|---|---:|
| `docs/phase0-inventory.md`（本报告） | 27,158（初版落盘值，勘误修订后约 27.2KB） |
| `go.mod` | 181 |
| `go.sum` | 900 |
| `cmd/beacon/main.go` | 327 |
| `internal/cli/root.go` | 1,685 |
| `internal/core/core.go` | 2,997 |
| `internal/store/store.go` | 1,483 |
| `internal/orchestrator/orchestrator.go` | 422 |
| `internal/gate/gate.go` | 970 |
| `internal/harness/harness.go` | 1,435 |
| `skills/skills.go` | 294 |
| `skills/README.md` | 507 |
| `bin/beacon`（构建产物） | 5,982,064 |
| 工具链 `~/workspace/toolchains/go/` | 276M（go1.27.3） |
| Python 基线 venv `~/workspace/beacon-go-venv/` | 独立 venv（不在新仓内，避免污染 Go 仓） |

## 8. 未决项

1. **全量 Python 基线未跑**：2004 条只做了 collect + 79 条抽跑；全量 pass/fail 分布要在 P0 实现期以同一 venv 补跑，作为 Go 对等 oracle 的完整基线。
2. **SQLite 驱动未接线**：选型已定（`modernc.org/sqlite`），但 Phase 0 按「依赖只用 Cobra」约束未引入；store 的九表 DDL/DAO 在 P0 实现期落地。
3. **二进制命名/选轨面**：冻结 truth 要求运行时选择显式（AC-060/157，如独立 `beacon-v2` 二进制或 `--runtime v2`）。Go 轨当前二进制名 `beacon` + 版本串 `v2.0.0-go-dev` 只是开发期区分，正式选轨面（改名 `beacon-go`/`beacon-v2` 还是加 `--runtime` flag）待拍板。
4. **module 名**：暂用 `beacon-go`（本地名）。若未来开源/内部分发需改为正式路径（如 `github.com/justin-mc-lai/beacon-go`），要在写更多 import 前尽早改，越晚越贵。
5. **AC-145/146 缺号**：冻结包内不存在这两个 AC（只有 TC-145/146），按缺号处理，不自造。
6. **golden parity 语料未建**：AC-162 要求 MD 解析与 v1 Python 输出的 golden 对照集（含中文/表格/frontmatter），P1 做 truth parser 前需先用基线 venv 生成并冻结这批 golden JSON。
7. **evidence.md 未深挖**：本阶段提炼以 truth/tests/tasks 为主，evidence.md（81 行）只确认其为冻结证据索引；P0 实现入场（program admission）时需按 TC-145/146 再核对 run-state 与 revision 对齐。
