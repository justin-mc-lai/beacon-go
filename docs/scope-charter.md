# Beacon Go 重构 · 口径方案（确认稿）

> 日期：2026-10-21 ｜ 状态：已拍板（2026-10-21，用户确认第 6 节 6 条全按推荐通过，见第 6 节决定记录）
> 依据：源仓 `~/workspace/repos/beacon/` 源码实测（HEAD `43c48a8a`，只读未改）；已交付的 [phase0-inventory.md](phase0-inventory.md)、[p0-implementation.md](p0-implementation.md)、[p1-implementation.md](p1-implementation.md)。
> 写法：按用户规则用大白话。必要术语第一次出现时一句话解释。

---

## 1. 口径一句话 + 三条判定标准

**一句话：Go 版只做一件事——把「想清楚 → 冻需求 → 写代码 → 验收 → 发布」这条编码主线做稳，其中 goal（总入口，一句话进来、带着任务一路跑到验收）底下的工作流一条不少全量迁移；其余凡是重复的、过时的、与这条主线无关的，全部剥离，不带进 Go 版。**

怎么判一件能力该不该留？过下面三条筛子，命中任意一条就剥离（文末第 3 节逐项执行）：

### 标准一：冗余 —— 同一件事有多个入口，或者同一份活被写了好几遍

留一个，其余合并或删掉。实证：

- **查状态有四张脸**：`status`（`beacon/commands/status.py` 916 行）、`board`（225 行）、`report`（967 行）、`help`（278 行）各查各的；它们底下算状态的代码又写了三遍：`runtime_projection.py` 1337 行 + `runtime_projection_board.py` 1977 行 + `runtime_projection_store.py` 2092 行，合计 5406 行，外加 `board_launcher.py` 1274 行、`help_support.py` 2012 行。Go 版 P1 已经只留一个查状态的面（spine，只认数据库查进度），这就是合并后的样子。
- **打分有三套**：给 skill 打分 `skill_eval_kit.py` 1267 行、给 agent 打分 `agent_eval_kit.py` 1433 行（另有 `agent_eval_grader.py` 86 行）、QA 侧的 `qa9.py` 441 行，维度各定各的、互相对不上。冻结真相本来就要求「统一评分、一套维度」（AC-151..156），Go 版只做一套。
- **品牌有两套**：beacon 和 loom 两套名字、两套 skill、两套状态目录，光品牌工具文件就 1037 行（`loom_brand.py` 179 + `loom_skill_score.py` 319 + `loom_slug_alias.py` 173 + `loom_subagent.py` 257 + `loom_trace.py` 109），内容基本同构，改一处要改两处。

### 标准二：过时 —— 已经被新面取代、只是没人敢删的老入口

源仓自己的命令目录（`beacon/command_catalog.py`）就把它们标成了「兼容」和「已废弃」，这是最硬的证据：

- **兼容五件**（目录里明标 compat）：`think`、`workflow`、`gate`、`verify`、`auto-test`。其中 `workflow.py` 3725 行和新的编排面做同一件事，`gate.py` 5091 行是全仓第二胖的命令文件，与门禁新面重复。
- **废弃三件**（目录里明标 deprecated）：`prompt`（502 行）、`save`（58 行）、`work`（`workflow start` 的别名）。
- **一次性工具**：`truth-cutover`（53 行）是当年需求格式切换时用的一次性迁移入口，切换早已完成，还挂在命令面上。

### 标准三：与编码需求真相无关 —— 不服务于主线任何一步

主线只有五步：想清楚、冻需求、写代码、验收、发布。一件能力如果既不帮着把需求说清/冻住，也不帮着写代码、验收、发布，就是无关。实证：

- **产品分析小工具箱**（目录里明标 labs，共 12 件）：`jtbd` 154 行、`kano` 170 行、`moscow` 129 行、`persona` 181 行、`roadmap` 144 行、`api-design` 133 行、`data-dictionary` 100 行、`analyze` 182 行等。它们是通用的产品方法练习，和某个具体需求的真相、代码、验收没有任何关系。
- 这类工具删掉，主线一步都不少；留着，则每件都要跟着 Go 版一起维护、测试、发版。

**一条总原则**：剥离的是 Go 版不迁，不是把 Python 旧版删掉。旧版按已冻结的长期双轨决定继续存在（见 phase0 报告 §4.1 的 UD-B），旧工具在旧版里照常用，所以剥离没有退路风险。

---

## 2. goal 工作流全量清单（核心，一条不能少）

先说清 goal 是什么：用户只说一句话（比如「给项目加登录功能」），goal 负责把这句话变成一个能跑几小时、出问题能接上、每一步有人（或规则）把关的长任务，最后交到验收和发布门口。**goal 不是一个命令，是一台机器。** 它的零件主要在两个大文件里：`beacon/utils/product_goal_runtime.py`（2034 行，下称「运行时」）和 `beacon/utils/loop_goal_state.py`（1204 行，下称「状态机」），加上 `goal_*` 家族 3049 行、`program_*` 家族 2102 行，配套命令面 `beacon/commands/goal.py`（574 行，23 个子命令）。

下面逐个列。每条给：大白话一句、源文件 + 关键函数 + 行数、迁到哪个 Go 包、哪一期做（P2 为主角；P0/P1 已部分落地的如实标注）。

| # | 工作流 | 大白话 | 源实证 | Go 包 | 期 |
|---|---|---|---|---|---|
| G-01 | 主流水线分阶段推进 | 一句话进来，先排好队：想清楚→冻需求→（要界面才加设计）→写代码→验收→发布，每步该加载哪份规矩（skill，写给干活的 agent 看的操作手册）都排好 | 运行时 `create_goal_run`、`build_stage_order`（基础顺序 plan/truth/implement/qa/release，有界面需求时插入 design）、`build_stage_plan`、`progressive_skills_for_stage`、`load_stage_skills`、`run_goal_facade`、`public_entry_contract`；`goal.py` 子命令 `run`、`entry-contract`、`status` | orchestrator + harness | P2 |
| G-02 | tick / continue 推进循环 | 机器的心跳：每「嘀嗒」一次就看当前走到哪、下一步该谁干、能不能往前走，能走就走，走不动就记明原因 | 运行时 `tick_goal_run`（约 290 行）、`goal_continue_run`；`goal.py` 子命令 `tick`、`continue` | orchestrator | P2 |
| G-03 | stage exit gates 出口检查 | 每一步出门都要过检查：比如冻需求那步，没冻住就不许出门去写代码 | `beacon/utils/goal_stage_exit_gates.py` 360 行（`evaluate_stage_exit`、`apply_stage_exit_block`）+ `goal_gates.py` 79 行 | gate + orchestrator | P2（判定）；检查结果写库的底座 P0 已落地 |
| G-04 | implement admission 准入 | 写代码前先验票：需求冻没冻、工作区（worktree，独立工位目录）对不对、设计该绑的绑没绑，缺一不放行 | `beacon/utils/goal_implement_admission.py` 126 行（`ensure_goal_implement_admission`）+ `beacon/commands/workspace.py` 114 行 + 写代码命令里的一串前置检查 | gate + orchestrator | P2–P3 |
| G-05 | blocker 记录 + verifier 挂载 + complete 闭环 | 卡住了要留字据（卡在哪、为什么）；完工不能自己说了算，必须挂一个独立检查者（verifier），它说过才算完 | 运行时 `record_blocker`、`attach_verifier`、`complete_goal_run`、`_qa_stage_passed`、`_backfill_closure_artifacts`；`goal.py` 子命令 `block`、`attach-verifier`、`complete` | orchestrator + gate | P2 |
| G-06 | driver 后台长跑 | 一个「司机」在后台一轮一轮推着任务走：一轮干一点、写好下一轮的提示、到点收工，绝不自己偷偷发布 | `beacon/utils/goal_agent_driver.py` 935 行（`driver_start`、`driver_run_once`、`driver_run_loop`、带锁防两个司机抢方向盘）；运行时 `goal_driver_start/status/stop/run_loop`；`goal.py` 子命令 `driver` | orchestrator | P2 |
| G-07 | durable supervisor 看护 | 司机之外的「监工」：定时看任务还活着吗、是不是卡死了、宿主中途断了要标记好以便恢复；长跑出毛病还有专门的体检入口 | `beacon/utils/goal_durable_supervisor.py` 552 行（`attach_supervisor`、`supervise_once`、`detect_stall`、`mark_host_interrupted`）；运行时 `goal_supervise_attach/detach`；`goal.py` 子命令 `supervise`、`longrun-doctor` | orchestrator | P2 |
| G-08 | heartbeat / checkpoint / resume-pack 断点恢复 | 跑着定期报平安（心跳）、存档（存档点记清下一步干什么）、真断了拿一份精简「恢复包」接着跑，不用从头来、也不用重读全部聊天记录 | 运行时 `goal_heartbeat`、`goal_checkpoint`、`goal_resume_pack`；监工文件 `write_checkpoint`、`build_resume_pack`；状态机 `record_host_interrupt`；`goal.py` 子命令 `heartbeat`、`checkpoint`、`resume-pack`、`resume`，另有 `beacon/commands/resume.py` 191 行 | orchestrator + store | P2；其中存档点表 P0 已落地（checkpoints 表 + 单调递增校验） |
| G-09 | discovery queue 消费 | 零散发现的新活先排队（队列），goal 按顺序取来干；排队的东西自己不能冒充「已完成」 | `beacon/utils/goal_queue.py` 201 行（`enqueue_discovery`、`consume_next`、防冒充完成的两道断言）；运行时 `consume_discovery_queue`；`goal.py` 子命令 `consume-queue`；状态机侧 `loop_goal.py` 子命令 `discover` | orchestrator + store | P2 |
| G-10 | delegate-ralph 实现外包 | 写代码这步可以整包外包给一个叫 ralph 的执行循环（一轮轮写、一轮轮自检），goal 只管派活和收活 | 运行时 `delegate_implement_to_ralph`；`beacon/commands/implement/ralph.py` 184 行 + `beacon/commands/ralph_bridge.py` 155 行；`goal.py` 子命令 `delegate-ralph` | orchestrator + harness | P2（派活面）；真集成 P3 |
| G-11 | parallel lakes + main-star / satellite 并行 | 多个子任务（湖）同时开干时，一人一个独立工位互不踩文件，卫星工位只能往主工位合并，不许反着来 | 运行时 `build_parallel_plan`、`detect_parallel_intent`、`progressive_worktree_refs`；`beacon/commands/implement/git.py` 125 行 | orchestrator | P2；工位全模式对齐 P3 |
| G-12 | program 编排（大海拆湖） | 目标太大一口吃不下时，先拆成几个湖、画好先后图、每个湖验票入场，再由 program 司机逐个推进 | `program_*` 家族 2102 行：`program_graph.py` 168（依赖图）、`program_admission.py` 334（入场检查）、`program_orchestrator.py` 244（总调度）、`program_driver.py` 684（司机）、`program_plan_runtime.py` 233、`program_plan_state.py` 99、`program_change_intent.py` 164、`program_freeze_gate.py` 176；命令面 `beacon/commands/program.py` 236 行；运行时 `sync_program_orchestrator_route` | orchestrator | P2（图+入场+调度）；司机长跑并入 G-06 口径 |
| G-13 | hotfix 快车道 | 线上出急事时走快车道：直接出修复计划（从哪个版本修、怎么回滚写清楚），不用排全流程，但不跳过发布前的校验 | `goal.py` 子命令 `hotfix-plan`；`beacon/utils/prod_ops_v168.py` 420 行（`build_hotfix_plan`、`validate_hotfix_for_release`） | orchestrator + truth | P2 |
| G-14 | multi-version plan | 一个目标横跨好几个版本时，先把每个版本干哪段排清楚再开跑 | 运行时 `plan_multi_version`；`goal.py` 子命令 `multi-version-plan` | orchestrator | P2 |
| G-15 | goal memory（reindex / search） | 一个长任务跑过的东西能被重新索引、按词搜回来复用，不用把全部历史读一遍（用的是本地全文检索，只做参考、不当完成证据） | 运行时 `goal_memory_reindex`、`goal_memory_search`；监工文件 `memory_reindex`、`memory_search`；`goal.py` 子命令 `memory` | store + orchestrator | P2 |
| G-16 | axis 主轴 | 给一个大 program 生成一条「主轴」说明：这条线到底在干什么、按什么顺序干，并能校验它没跑偏 | `goal.py` 子命令 `axis`；`beacon/utils/goalrun_main_axis.py` 417 行（`generate_main_axis`、`validate_main_axis`） | orchestrator + store | P2 |
| G-17 | trace 轨迹 | 一个任务全程留痕：谁在什么时候干了什么、出过什么事，事后能拼回完整 bundle 查账 | `goal.py` 子命令 `trace`；`beacon/utils/agent_trace.py` 462 行 + `loom_trace.py` 109 行的 `aggregate_goal_trace`（函数迁，loom 这个文件名随双品牌一起弃，见第 3 节） | store + cli | P2；留痕的表底座 P0 已落地（lifecycle_events 只许追加不许改） |
| G-18 | design-stage 设计阶段 | 主线里需要界面时，goal 专门跑一步设计：把设计原语（颜色、间距、组件这类定死的视觉规矩）绑好，才准进写代码 | 运行时 `run_design_stage_for_goal`；`goal.py` 子命令 `design-stage` | orchestrator + gate | P2 |
| G-19 | self-heal 自愈（pre / mid / post） | 三个固定体检点：开跑前查环境和版本面对不对；中途卡住先试着修证据清单、再查工作区准入；发布后装好本机环境（这步要人确认） | 运行时 `plan_goal_self_heal`；规则写在 `skills/beacon/references/public-surface-progressive-map.v1.json` 的 `goal_transversal.self_heal`（pre 两项自动、mid 两项一自动一手动、post 一项需确认） | orchestrator（执行）+ doctor/hooks 内部面（被调用） | P2 |
| G-20 | loop-goal 状态机 | 上面所有工作流共用的「红绿灯规则本」：一共 12 个状态（idle 待命、governance_precheck 先查治理、truth_routing 去冻需求、review_routing 去审查、standard_execution_precheck 标准前置检查、implement_routing 去写代码、qa_routing 去验收、qa_deep_routing 深验收、drift_routing 发现跑偏、release_gate 发布门口、done 完成、error_recovery 出错恢复），外加：发布门口必须人点头、修不好最多重试几轮要计数、人中途说的话要记下来不重复问、卡住统一走出错恢复 | `loop_goal_state.py` 1204 行：`VALID_STAGES`（12 态）、`transition`、`check_stop_when`、`check_release_gate`、`increment_repair_loop`、`record_intent_fork`、`record_user_decision`、`record_standard_execution_precheck`、`route_to_error_recovery`、`build_resume`；命令面 `beacon/commands/loop_goal.py` 86 行（`run`、`discover`、`mark-complete`——最后这个是故意做成「禁止」的，防止排队面冒充完成） | core（状态规则）+ orchestrator（执行） | P2；说明：P0 落地的 core 状态机是另一条轴（需求生命周期轴），不是这台 loop-goal 运行轴，这台还没迁，如实标注 |
| G-21 | goal 护栏组：预算 / 配额 / 转向 / 待办 | 长跑要有油量表：花了多少、配额还剩多少（超了要停）、中途改方向要留记录（转向）、任务自己的待办清单要和总进度对得上 | `goal_budget.py` 151 行、`goal_quota.py` 115 行、`goal_steering.py` 296 行、`goal_todos.py` 146 行、`goal_reason_codes.py` 56 行（卡住原因的统一词表）、`goal_ready.py` 32 行 | orchestrator + core | P2 |
| G-22 | implement 三条 lane（单人 / 团队 / ralph 循环） | 写代码这步有三种干法：单人直接干、团队交接干、ralph 一轮一轮循环干；goal 按活的大小选一条，三条的入场检查和收活口径必须一致 | `beacon/commands/implement/` 子包 6043 行：`commands.py` 5505（三条 lane 的主体）、`adapter.py` 127（执行器适配）、`ralph.py` 184、`team.py` 42（交接校验）、`git.py` 125（工位） | harness + orchestrator + cli | P2（口径与派活）；真集成 P3 |

**已落地部分汇总（不冒充）**：P0 落了存档点表、运行/事件/门禁结果等账本表和生命周期状态规则（属 G-08/G-17/G-03 的底座）；P1 落了 goal 状态的只读查询面（spine，属 G-01 的 `status` 读面）。**P2 已于 2026-10-22 收口（v2.0.0-go.1）**：G-01..G-22 逐条最终状态见 [p2d-implementation.md](p2d-implementation.md) 第 5 节收口总表——主体全部落地，明确留 P3 的是各条的真集成部分（真 agent 后端、真 worktree、gate 评估写库、ralph 真执行）。

---

## 3. 剥离清单

处置只有三种：**drop** 直接不迁；**merge** 命令名消失、能力并入某个保留面；**slim** 只留最小子集，明说留什么、弃什么。
分两层列：先是命令层（计入第 4 节 77 条总账），再是模块层（命令背后的大文件，不重复计入 77 条，但决定实际瘦身幅度）。

### 3.1 冗余类

| 项 | 源实证 | 处置 |
|---|---|---|
| status / board / report 三张状态脸 | 命令 `status.py` 916、`board.py` 225、`report.py` 967 行 | merge → 全部并入 spine（P1 已落地 show/list/artifacts/gate）；HTML 看板启动器 `board_launcher.py` 1274 行 drop，不做网页看板 |
| help 自研帮助面 | `help.py` 278 行 + `help_support.py` 2012 行 | merge → 用 Go 框架（Cobra）自带 help 代替，自研帮助内容只留一份简明路由说明 |
| 状态计算三件套 | `runtime_projection.py` 1337 + `runtime_projection_board.py` 1977 + `runtime_projection_store.py` 2092 = 5406 行（模块层） | merge → 只留一个投影实现，数据源只认数据库（store），其余两套 drop |
| 评分三套（skill / agent / QA） | `skill_eval_kit.py` 1267 + `agent_eval_kit.py` 1433 + `agent_eval_grader.py` 86 + `qa9.py` 441 行，命令侧 `eval.py` 124 行（模块层+命令层） | merge → 一套统一评分（冻结真相 AC-151..156：同一套维度、只评分、人来改，评分不能冒充验收和发布）；命令 `eval` 并入 qa 能力 |
| 双品牌 beacon / loom | `loom_*.py` 1037 行（模块层）+ 两套 skill 目录与状态目录（`.beacon` / `.loom`）+ 运行时里的双品牌防误调函数 `docs_forbid_dual_call` | drop（loom 侧）→ 只留 beacon 一套；连带 G-17 的 trace 聚合函数改名迁入，不再叫 loom。**待拍板第 1 条** |
| 进度/汇总类投影 | `test-progress`（169 行）、`evolution-summary`（370 行）、`manifest`（164 行） | merge → test-progress 并入 qa/spine；manifest（交付清单，本质是轨迹的投影）并入 spine；evolution-summary drop（就是 spine 能查到的东西换个排版） |
| ask 意图路由 | `ask.py` 72 行 | merge → 并入 goal 入口的意图识别（G-01 的一部分），不单设命令 |
| remember / recall 两张脸 | `remember.py` 182 + `recall.py` 178 行 | merge → 并入 goal memory（G-15），一套记忆一个入口 |
| profile 配置面 | `profile.py` 82 行 | merge → 并入全局配置（环境变量 + 配置文件），不单设命令 |
| friction-intake / feedback 两个采集面 | `friction_intake` 命令 170 行、`feedback.py` 482 行 | merge → 合成一个「问题采集」入口，归 truth 的改进输入 |
| debug 诊断面 | `debug.py` 1149 行 | merge → 并入 doctor（诊断本来就该一个入口） |
| deep 深度分析面 | `deep.py` 131 行 | merge → 并入 qa 的深验收（qa-deep），不单设 |
| lighthouse 巡检面 | `lighthouse.py` 116 行 + 对应 gov skill | merge → 巡检项并入 doctor |

### 3.2 过时类

| 项 | 源实证 | 处置 |
|---|---|---|
| think（compat） | `think.py` 251 行；目录明标 compat | merge → 能力本就是「想清楚」，并入 truth 的冻前环节；名字 drop |
| workflow（compat） | `workflow.py` 3725 行；目录明标 compat | merge → 编排能力并入 loop（G-20/G-02）；名字 drop |
| gate（compat） | `gate.py` 5091 行；目录明标 compat | merge → 四道门禁（冻需求/写代码准入/验收/发布）的判定并入 gate 包；顶层命令名 drop |
| verify（compat） | `verify.py` 65 行；目录明标 compat | merge → 并入 qa（校验裁决本就归验收） |
| auto-test（compat） | `auto_test.py` 3390 行；目录明标 compat | merge + slim → 执行测试 + 留证据的核心并入 qa；与 qa 重复的编排外壳 drop |
| prompt / save / work（deprecated） | `prompt.py` 502、`save.py` 58 行，work 为别名；目录明标 deprecated | drop（三件全弃，能力早被新面覆盖） |
| loop-goal 旧名 | `loop_goal.py` 86 行 + 运行时 `loop_goal_support_metadata` | merge → 与 goal 是同一台机器（第 2 节 G-20），名字并入 goal/loop，不留两个入口 |
| orchestrate / run / team / task 等旧编排名 | `orchestrate.py` 57、`run.py` 382、`team.py` 862、`task.py` 258 行 | merge → 全部并入 loop 编排（G-01/G-06/G-22）；run 的「统一 goal/run」口径由 goal 接管，team 的交接校验进 G-22 |
| truth-cutover 一次性迁移入口 | `truth_cutover.py` 53 行 | drop（切换已完成）；若 P3 迁老项目时需要读旧格式，在数据导入器里做，不做常驻命令。**待拍板第 6 条** |
| gen-truth 等别名群 | `gen-truth`、`materialize-feature-package`、`validate-feature-package`、`prd-bootstrap`、`code2truth`、`freeze`、`reopen`、`truth-map`、`planner-review`、`review` | merge → 全部收进 truth 的子命令（gen/freeze/change/refreeze/review 等），顶层别名一律 drop——别名越留越多正是旧版变胖的原因之一 |
| prd / user-story / test-case 顶层文档面 | `prd.py` 1875、`user_story.py` 652、`test_case.py` 2304 行 | merge → 冻结口径早已定：需求只存在于需求包（features/<slug>/truth.md 等）里，不再生成顶层文档；三个命令并入 truth 的包内材料生成 |

### 3.3 无关类

| 项 | 源实证 | 处置 |
|---|---|---|
| labs 产品分析工具箱（除已在上面归类的 brainstorm / ui-spec / lighthouse / memory-friction-evolution 外的） | 目录明标 labs：`analyze` 182、`api-design` 133、`data-dictionary` 100、`jtbd` 154、`kano` 170、`moscow` 129、`persona` 181、`roadmap` 144 行 | drop（8 件全弃出 Go 版；旧版里继续存在）。`brainstorm` 是冻需求前的想清楚环节，merge 进 truth（不算无关）；`ui-spec` merge 进 design；`memory-friction-evolution` 131 行是 labs 的演化实验，drop（它和 G-15 的 goal memory 不是一回事）。**待拍板第 2 条** |
| labs / bridge 两个容器名 | 目录 internal 组 | drop（容器名删除；bridge 底下的 run/team/ralph 已在 3.2 逐项 merge，不随容器保留） |

### 3.4 design / prototype 的诚实边界（slim，不是 drop）

design 不能全砍：**冻结口径里，凡是带界面的需求，写代码之前必须先绑好 design primitive（定死的视觉规矩），没绑不许开工**（G-18 + G-04 的一部分）。要留的：

- 留：design primitive 绑定合同本身、要不要做设计的触发判断、设计基线最小检查（够判断「绑没绑、能不能开工」即可）、`prototype` 作为 design 的一个子命令走宿主桥接生成（活儿交给宿主 agent 干，和 ralph 外包一个道理）。命令侧 `design.py` 363 行、`design_contract.py` 428 行按此口径瘦身迁入。
- 弃：`prototype_utils.py` 6805 行——全仓最胖的文件，里面是自己造的原型生成/渲染机器，属重复造轮子；`design_company_templates.py` 276 行的公司模板库；`design_md_lint.py` 240 行的全套 lint 只留最小校验；看板/预览类渲染（已在 3.1 随 board 处理）。
- 一句话边界：**Go 版只管「设计规矩定没定、绑没绑、够不够开工」，不管「原型长什么样怎么画出来」。待拍板第 3 条。**

### 3.5 slim 保留的支撑面（留最小子集，防又长回胖子）

| 面 | 留什么 | 弃什么 |
|---|---|---|
| doctor（2141 行命令 + 诊断工具约 3600 行） | G-19 自愈要调的几项：查上下文、查版本面、修证据清单；版本解析 fail-closed（查不到版本就报错，不许猜） | 全套巡检编排的重复实现，只留被 goal 自愈和日常排障真正调用的子集 |
| skill（696 行） | 按需加载规矩的 loader（先给目录、用到才展开，省上下文）+ P4 的统一评分入口 | 与 loom 双份的 skill 副本（随双品牌 drop） |
| version（371 行命令 + 工具约 2480 行） | 版本解析与「运行版 vs 文档版不一致就拦住」这一条硬规矩，进 store/truth 内部 | 独立顶层命令面（查版本并入 projects/spine 输出） |
| hooks（118 行） | 发布后装机这一个被 G-19 引用的钩子 + 钩子执行框架最小集 | 其余实验性钩子 |
| archive（312 行） | 按版本显式定位已归档需求包（冻结 AC-159 的口径） | 批量归档的治理外壳 |
| init（416 行） | 新项目装配上下文的最小集（projects register 的前置） | 与 doctor 重复的环境拼装 |

---

## 4. 修订后的迁移总账（77 条重算）

数法先说清，三种处置按「命令名和入口」计：

- **keep**：命令名在 Go 版保留为独立入口（含瘦身保留，标注 slim）。
- **merge**：命令名消失，能力并入括号里的保留面——活还在，只是没自己的摊子了。
- **drop**：命令名和能力都不迁。

### 4.1 新口径总账

| 处置 | 数量 | 明细 |
|---|---:|---|
| keep | 12 | goal、truth、design、implement、qa、release、program（归 loop 的独立入口）、doctor (slim)、archive (slim)、skill (slim)、init (slim)、hooks (slim) |
| merge | 49 | 并入 truth 19：prd、user-story、test-case、code2truth、freeze、plan、reopen、review、planner-review、think、brainstorm、change、prd-bootstrap、truth-map、gen-truth、materialize-feature-package、validate-feature-package、friction-intake、feedback；并入 loop 11：orchestrate、resume、ask、remember、recall、workflow、run、team、ralph-bridge、loop-goal、task；并入 qa 6：real-project-validation、test-progress、deep、verify、auto-test、eval；并入 spine 4：board、status、report、manifest；并入 design 2：prototype、ui-spec；并入 doctor 2：debug、lighthouse；并入 cli/配置 2：help、profile；并入 gate 1：gate；并入 implement 1：workspace；并入内部 version 1：version |
| drop | 16 | analyze、api-design、data-dictionary、jtbd、kano、moscow、persona、roadmap、memory-friction-evolution、evolution-summary、prompt、save、work、labs（容器）、bridge（容器）、truth-cutover |
| **合计** | **77** | 与 phase0 的全量口径一致（77 条顶层命令名） |

注：spine / projects 是 v2 新增面（不在旧 77 条内），P1 已落地，不占上表名额。

### 4.2 与 phase0 对比，差额逐项可追溯

phase0 口径是「先全量对齐再说」：truth 18 / loop 11 / design 3 / implement 2 / qa-evidence 6 / release 1 / support-keep 23 / legacy-drop 13，即 **64 条保留成面、只 drop 13 条**。
新口径是 **keep 12 / merge 49 / drop 16**。两个差额：

- **drop 多 3 条**（13 → 16），逐项追溯到第 3 节：`evolution-summary`（3.1，support-keep → drop，就是 spine 换个排版）、`bridge` 容器（3.3，support 侧原计 loop → 容器名 drop，其子命令已逐项 merge）、`truth-cutover`（3.2，truth → drop，一次性工具）。
- **保留成面少 52 条**（64 → 12 个独立入口）：这 52 条不是能力被砍，而是按第 3 节逐项 merge——其中 49 条计入 merge 总账，另 3 条即上面新增的 drop。换句话说，新口径真正砍掉的能力只有 16 条 drop 里的无关/废弃部分，其余全是「合并摊子」：入口从 77 个收敛到 12 个，goal 底下的工作流（第 2 节）一条没动。

---

## 5. 稳定 Go 版维护约定（一页）

1. **单二进制发布**：Go 版永远是一个文件发版（`beacon` 一个二进制，数据库驱动用纯 Go 实现、不用装任何运行环境），下载下来就能跑。这是选 Go 的主要理由之一，不许退回「先装 Python 再装一堆包」。
2. **版本号规则**：稳定版用 `v2.0.0-go.N`（N 从 1 起，每完成一期出口发一个）；开发中间产物用 `v2.0.0-go-dev`，不许拿开发版当交付版报。当前二进制是 `-go-dev`，第一个稳定版 `-go.1` 对应 P2 出口。
3. **每期出口三件套**：全量测试通过（一条不许跳）、真二进制实跑样例贴进当期报告（命令 + 真实输出）、交付物逐个报文件名 + 大小。缺一件不算出口——P0/P1 已按此执行，后面不许降格。
4. **冻结真相不口头改**：需求真相的任何变更走既有流程 gen-change → gen-refreeze（先写变更、再重新冻结、留记录）。本方案若被拍板，同样要以一次 change 的形式落进冻结包，Go 侧的 AC 清单随之更新，不靠聊天记录当依据。
5. **goal 工作流改行为先补测试**：第 2 节任一工作流要改行为，先照旧版行为写一条会失败的测试（拿旧版输出当标准答案），改完测试变绿才算改对；不许先改代码后补测试，更不许无测试改编排逻辑。
6. **防长胖条款**：新加任何顶层命令或新面，必须在当期报告里写明「它替代或合并了哪个现有面」；说不出来的，不加。旧版从 37 条默认命令长到 77 条就是没有这条规矩。
7. **状态只有一个账本**：运行状态只存全局数据库一份、只从 spine 一个面读；任何新功能不许再自带一套 JSON 状态文件和旧账本抢权威（旧版双写抢权威的教训，冻结 AC-016）。
8. **推进顺序**：P2 编排器是主角（第 2 节全部 G 条目）→ P3 门禁写库与老项目数据迁移、ralph/worktree 真集成 → P4 六能力面收敛 + 统一评分 → P6 性能门禁。不许跳期先做表面功夫（比如先做看板、先做评分），因为主线没稳之前表面都是空壳。

---

## 6. 拍板清单（6 条，已于 2026-10-21 全部拍板）

**决定记录**：2026-10-21，用户确认本节 6 条**全部按推荐（A）走**。逐条决定标注在每条下方（✅ 已拍板）。

1. **双品牌只留 beacon？** 选项：A 只留 beacon，loom 整套（品牌文件、skill 副本、`.loom` 状态目录）不迁；B 两套都留。**推荐 A**：两套内容同构、双倍维护，loom 本是另一条实验轨道的名字，Go 版没有包袱，没必要背。
   → ✅ **已拍板（2026-10-21）：选 A**，双品牌只留 beacon，loom 整套不迁。
2. **labs 产品分析工具箱（8 件）全弃出 Go 版？** 选项：A 全弃（旧版 Python 里继续可用，不受影响）；B 挑一两件留档迁入。**推荐 A**：它们与编码主线无关，旧版双轨还在，弃的只是 Go 版不背。
   → ✅ **已拍板（2026-10-21）：选 A**，8 件全弃出 Go 版，旧版 Python 不受影响。
3. **prototype 瘦身边界按 3.4 执行？** 选项：A 按 3.4（只留绑定合同 + 最小检查 + 宿主桥接生成，6805 行的自研生成机器不迁）；B 把原型生成也完整迁入。**推荐 A**：生成活儿宿主 agent 干得更好，Go 版管「够不够开工」这一道关就够了。
   → ✅ **已拍板（2026-10-21）：选 A**，按 3.4 slim 边界执行。
4. **compat 五件（think/workflow/gate/verify/auto-test）直接弃名？** 选项：A 名字直接不留，能力按第 3 节 merge（Go 版是新二进制、没有存量用户喊着要兼容）；B 留一期只读兼容别名，到 P4 再删。**推荐 A**：留别名正是旧版从 37 条长到 77 条的起点；真有老脚本依赖，旧版 Python 还在。
   → ✅ **已拍板（2026-10-21）：选 A**，直接弃名、能力 merge，不留兼容别名期。
5. **用户入口叫 goal 还是 loop？** 冻结设计里六能力之一叫 loop，但旧版用户习惯的入口叫 goal（23 个子命令都挂在它下面）。选项：A 命令入口统一叫 `goal`（内部包名仍可叫 orchestrator，六能力对外说法随之写成 goal/truth/design/implement/qa/release）；B 按冻结名叫 `loop`，goal 只作别名保留一期。**推荐 A**：名字跟着用户习惯走，少一次全员改口；但这条改的是冻结口径，拍板后要按第 5 节第 4 条走一次 change 落账。
   → ✅ **已拍板（2026-10-21）：选 A**，用户入口统一叫 `goal`，内部包名仍可叫 orchestrator。**落账待办**：此条改冻结口径，**需对冻结包走一次 gen-change 落账，列入 P3 前待办**（尚未执行，不得视为已落账）。
   → **落账进展（2026-10-22，P3a）**：Go 侧变更记录已落账：`docs/changes/change-goal-entry-rename.md`（变更内容/理由/受影响 AC 逐条/Go 侧清单调整齐备）。上游冻结包在只读镜像中**仍未改**，待源仓下一次正式 gen-change 时合并本记录——不得视为已对上游执行 gen-change。
6. **truth-cutover 和 evolution-summary 确认 drop？** 选项：A 确认 drop（老项目若还有没切换完的，P3 数据迁移时在导入器里兼容读，不做常驻命令）；B 先留着进 Go 版再说。**推荐 A**：一次性工具常驻化是旧版的典型病，导入器兼容是一次性成本，比常驻命令便宜。
   → ✅ **已拍板（2026-10-21）：选 A**，确认 drop；老格式兼容读取以后放 P3 数据导入器，不做常驻命令。

---

## 附：自查记录（写完后对源仓复核）

- `goal.py` 子命令实测 23 个（run/status/tick/continue/resume/block/attach-verifier/complete/entry-contract/consume-queue/delegate-ralph/design-stage/supervise/heartbeat/checkpoint/resume-pack/memory/driver/axis/trace/hotfix-plan/longrun-doctor/multi-version-plan），逐个对第 2 节：run/entry-contract/status→G-01，tick/continue→G-02，block/attach-verifier/complete→G-05，consume-queue→G-09，delegate-ralph→G-10，design-stage→G-18，supervise/longrun-doctor→G-07，heartbeat/checkpoint/resume-pack/resume→G-08，memory→G-15，driver→G-06，axis→G-16，trace→G-17，hotfix-plan→G-13，multi-version-plan→G-14。**23/23 全覆盖，无遗漏。**
- `product_goal_runtime.py` 公开函数实测 49 个（另 4 个下划线私有助手）：逐个有归属——流水线组（create/load/tick/continue/facade/stage_plan 等）→G-01/G-02，program 组（`_goal_program_slug`、`_program_binding_diagnostics`、`sync_program_orchestrator_route`、`attach_program_takeover_hint`）→G-12，恢复组→G-08，看护/司机组→G-06/G-07，memory 组→G-15，自愈 `plan_goal_self_heal`→G-19；两个不迁的已注明：`host_visible_whitelist`（双品牌名单，随第 1 条拍板 drop）、`docs_forbid_dual_call`（双品牌防误调，同上）。
- `loop_goal_state.py` 状态名实测 12 个（idle/governance_precheck/truth_routing/review_routing/standard_execution_precheck/implement_routing/qa_routing/qa_deep_routing/drift_routing/release_gate/done/error_recovery），与 G-20 所列 **12/12 一致**；其公开函数（transition/check_stop_when/check_release_gate/increment_repair_loop/record_intent_fork/record_user_decision/record_standard_execution_precheck/route_to_error_recovery/build_status/build_resume 等）均在 G-20 覆盖。
- 第 4 节总账自加校验：keep 12 + merge 49 + drop 16 = 77，与 phase0 全量口径一致；merge 分组自加：19+11+6+4+2+2+2+1+1+1 = 49，一致。
- 本文档行数与字节数见文末落盘记录（以文件系统实测为准，写在交付回报中）。
