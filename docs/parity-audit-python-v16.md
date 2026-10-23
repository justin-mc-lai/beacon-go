# 对等审计：Go 版对 Python v1.6.19（换尺重测）

> 日期：2026-10-23 ｜ Go 版：v2.0.0-go.8（HEAD `2ec1cf4`）｜ 参照：Python v1.6.19 源码镜像 `~/workspace/repos/beacon/`（HEAD `43c48a8a`，全程只读）
> 写法：大白话；必要术语第一次出现时一句话解释。
> 这份审计干什么：此前验收 Go 版用的尺是给 Rust 版画的冻结 v2 设计蓝图（从没在任何语言里落地）。用户已纠正参照系——**对标对象是 Python v1.6.\* 的实际行为**。本报告换尺重测：逐条命令、逐项主链行为，以两边源码为证，不以任何一期的实现报告自述为准。

---

## 1. 摘要

### 1.1 命令层四档计数（77 条顶层命令全覆盖）

| 档位 | 条数 | 含义 |
|---|---:|---|
| parity（对等） | **9** | Go 有独立入口且核心语义在（含按口径稿 slim 的薄面） |
| merged-verified（合并且验真） | **30** | 口径稿说并入别的入口，且在 Go 代码里真找到了承接点 |
| dropped-by-charter（拍板弃） | **16** | 口径稿第 6 节六条决定拍板不迁，不算缺口 |
| gap（真缺口） | **22** | 口径稿说保留或合并，但 Go 里找不到承接点，或只剩同名异实 |
| 合计 | 77 | 与 phase0 全量口径、口径稿总账一致 |

一句话：**口径稿的账面是 keep 12 / merge 49 / drop 16；实测只有 39 条（9+30）的承接验真，22 条落空。** 落空最扎眼的是 keep 名单本身：archive、init、hooks 三条承诺「瘦身保留」的命令在 Go 里零承接；doctor 的瘦身承诺（留三项检查）也没兑现，doctor 本体只剩一个 rebuild。

### 1.2 主链真缺口总数

命令层 22 条 gap 与行为层深挖出的缺口去重合并后（同名命令在两层只计一次），**真缺口共 42 条**：高严重度 10 条、中 16 条、低 16 条。逐条详单见第 4 节。

严重度的判法：高 = 冻结/验收/发布的判定质量或主链闭环受损，错误的东西能过门或对的东西被误判；中 = 一整块在用能力缺失，用户在具体场景会撞墙，但有体系外绕行；低 = 可见性、产物或便利损失，不伤判定正确性。

### 1.3 真缺口清单（按严重度，一行一条）

**高**

| # | 缺什么 | Python 在哪 | 影响什么 |
|---|---|---|---|
| H1 | 需求描述的模糊词拦截（「或者/至少/或」等二义表述拦冻） | `utils/truth_review_gate.py:check_e_vague`（654 行） | 冻结的需求可以合法地写二义句，实现漂移少一条防线 |
| H2 | 领域状态机（FSM，需求里描述业务状态怎么流转的表）的图校验退化成关键词检查 | `utils/truth_fsm.py:validate_truth_fsm` | 未声明状态、悬空转移、非法边与合法边矛盾的状态机在 Go 能冻住，下游照错图生成用例和实现 |
| H3 | integrity（完整性门）缺五组检查维度：需求清晰度、领域 FSM、确定性（占位符/空模板）、对齐面、用例包校验 | `utils/truth_integrity.py:build_truth_integrity_gate` | 同样的包在 Python 会被拦、在 Go 可能 pass，冻结质量下降 |
| H4 | 填充深度缺两条硬检：业务域必须有用户旅程、只写包生命周期的状态机冒充业务 FSM 要拦 | `utils/truth_review_gate.py:check_d_fill_depth`（554 行） | 「形式齐全、内容空心」的包能过冻 |
| H5 | 评审结论不回写编排状态机：评审分级（P0/P1…）、建议路线、需人拍板等路由闭环断裂 | `utils/loop_goal_state.py:record_review_decision` | P0 级问题拦不住写代码这步；发现跑偏时不自动走变更/重冻路线 |
| H6 | 配额判定五段逻辑缺失：健康异常/等人工/等证据/等焦点四种「先等着」状态 + 按时间窗计的槽位配额 | `utils/goal_quota.py:quota_should_run`、`normalize_quota_config` | 长跑司机分不清「在等东西」和「在干活」：要么误判预算耗尽停掉，要么不该跑时硬跑 |
| H7 | 发布判定只有单需求 5 项合成，缺版本级约 25 项检查；且没有发布档案降级（急修/纯文档时把部分不合格降为警告） | `commands/release.py:check`、`utils/prod_ops_v168.py:select_release_profile` | 以「版本」为单位发布时就绪判定覆盖不了；线上急修在 Go 只能全硬判或绕开 |
| H8 | 验收只剩「判证据」半边：自动跑测试产证据的能力没了，证据靠人工登记 | `commands/auto_test.py`（3390 行） | 测试跑没跑与验收判没判脱节，没人登记证据时验收门永远等不到东西 |
| H9 | 给干活的 agent 用的技能包没有宿主发现路径：宿主技能列表里根本没有 beacon；六份技能里还缺 plan、references 全空 | `skills/beacon/SKILL.md` 路由包 + `commands/skill.py` 物化面 | 编排跑到某阶段给出技能名，宿主也加载不到内容，等于没技能可用 |
| H10 | 状态不随仓库走：进度、评审结论、当前版本只活在本机数据库里，换机/协作/clone 全看不见，未注册的项目直接拒绝一切命令 | `.machine/` 记录 + `beacon/version-truth.json`（均入仓） | 协作和换机场景下项目状态归零，只能靠迁移导入派生重建，精度有损 |

**中**

| # | 缺什么 | Python 在哪 | 影响什么 |
|---|---|---|---|
| M1 | 给 agent 本身打分的整条线（六维评分卡+会话记录审查+实测准入） | `utils/agent_eval_kit.py`、`utils/agent_eval_grader.py` | 没法验收「干活的 agent 守不守边界」，如它偷写发布结论无人发现 |
| M2 | 按项目类型自动生成 QA 打法（策略矩阵） | `utils/qa9.py:build_qa_strategy` | 新项目没有现成 QA 打法，全靠人手写 |
| M3 | 一句话自动拆解成子意图/旅程/验收点并物化成整包 | `utils/intent_decomposition.py`、`utils/intent_first.py` | 「想清楚」阶段内核不产种子，包质量下限全靠宿主 agent 自觉 |
| M4 | 提问由内核生成（苏格拉底四问等）与 brainstorm 收敛环节 | `utils/brainstorm.py:run_beacon_brainstorm` | 问答账本只记不问，容易走形式（go.8 报告自认） |
| M5 | 推迟账本：全量承诺下推迟某项必须留账且要用户拍板 | `utils/planner_review_runtime.py:_deferral_ledger` | 范围被静默缩小时无记录、无拦截点 |
| M6 | 范围模式的双轴与自动推导（现为手填自由文本、无校验） | `planner_review_runtime.py:_scope_mode`、`intent_decomposition.py` | 填错词静默失去对等门；接管类项目无模式可表达 |
| M7 | 改需求后把测试/任务材料标「已过期」，发布时再查一道 | `truth_review_gate.py:mark_package_materials_stale`、`evaluate_release_truth_materials_gate` | 旧材料顶着「现行」面貌存活到重冻才被兜底 |
| M8 | 编排的记录类四件：意图分叉留痕、用户决定缓存（问过不重复问）、人工打断统计、统一送出错恢复的入口 | `utils/loop_goal_state.py` 的 `record_intent_fork/record_user_decision/_ensure_human_stop_metrics/route_to_error_recovery` | 长跑复盘说不清、重复问人、卡死处理各行其是 |
| M9 | 预算契约三件：交接文本限长、花费幂等（重复记账要拦）、来源白名单 | `utils/goal_budget.py` | 交接文本可无限膨胀；重复花费、轮询误记账无防护 |
| M10 | archive：按版本定位已归档需求包（keep 承诺未兑现） | `commands/archive.py` | 归档的版本找不回来、说不清谱系 |
| M11 | init：新项目装配上下文（keep 承诺未兑现，只有登记数据库行） | `commands/init.py` | 新项目起步的上下文文件无人写 |
| M12 | hooks：钩子框架与发布后自动装机执行（keep 承诺未兑现，自愈计划还引用着不存在的命令名） | `commands/hooks.py`、`utils/project_hooks.py` | 发布后多 agent 环境对齐仍要手工做 |
| M13 | code2truth：从存量代码反推需求真相草稿 | `commands/code2truth.py` | 存量项目接入只能从零手写需求包 |
| M14 | reopen：把已激活的包重开成新修订 | `commands/reopen.py` | 变更链（change/refreeze）不是重开，语义缺失 |
| M15 | review：对实现做规格/质量/设计审查并回写 | `commands/review.py` | 计划评审和完整性门都顶替不了实现审查 |
| M16 | truth-map：需求图谱的独立 resolve/verify 面（只剩准入束里一项不可独立调用的派生校验） | `commands/truth_map.py` | 编排启动预检的一项对外不可见、不可单独验 |

**低**

| # | 缺什么 | Python 在哪 | 影响什么 |
|---|---|---|---|
| L1 | 问题采集入口（friction-intake 与 feedback 口径稿承诺合并，实际未建） | `commands/friction_intake.py`、`commands/feedback.py` | 用中发现的问题没有回流需求改进的口子 |
| L2 | 意图路由：一句话先路由到该走哪条线（带置信度与预览） | `commands/ask.py:ask_entry`、`utils/plan_subintent.py` | Go 的 truth ask 是提问账本，同名异实；一句话只能直接建任务 |
| L3 | task：项目级定时任务系统（带唤醒与定时器） | `commands/task.py` | 靠定时唤醒跑的任务在 Go 无承接（run 内待办不是它） |
| L4 | test-progress：按测试用例计的进度与同步 | `commands/test_progress.py` | 只有阶段进度，没有用例级进度 |
| L5 | deep：深验收入口 | `commands/deep.py` | 只有同名状态，没有深验收面 |
| L6 | debug 与 lighthouse 并入 doctor 的承诺未兑现（doctor 只有 rebuild，自愈引用的 doctor 子命令不存在） | `commands/debug.py`、`commands/lighthouse.py` | 排障与巡检没有落点，自愈计划表有死引用 |
| L7 | profile：项目画像（Go 的 profile 是执行器档案，同名异实） | `commands/profile.py` | 项目级配置/画像无处放，只有一个数据库路径环境变量 |
| L8 | prototype 宿主桥接生成 + ui-spec 规约本体（口径稿承诺的桥接半边未做） | `commands/prototype.py`、`commands/ui_spec.py` | 设计只管「绑没绑」，原型生成与 UI 规约无人承接 |
| L9 | prd-bootstrap：从种子材料自举生成首版包 | `commands/prd_bootstrap.py` | 只有空骨架，没有内容生成 |
| L10 | release 产物：发布清单、版本变更单、发布评分卡 | `commands/release.py:create/scorecard` | 发版留痕与变更汇总要手工拼 |
| L11 | skill 评分的实测部分：用例回放与带/不带 skill 对照基准（维度名保住了，测法没了） | `utils/skill_eval_kit.py:replay_eval_case` | 技能改版后无法回归验证是否退化 |
| L12 | 状态面视角群：跨仓 git 状态、健康汇总、debug/团队/ralph 视角、叙事报告 | `utils/runtime_projection*.py`、`commands/report.py` | 多仓多 agent 时运维第一屏信息全丢 |
| L13 | 免疫三项仍无：浏览器扩展验收执行、loop 模式注册表、自愈沙箱（试错+自动回滚） | `utils/playwright_extension.py`、`utils/loop_patterns.py`、`utils/remediation_sandbox.py` | 扩展类交付验不了；模式治理只能改代码；长跑自愈只有「修证据清单」一级 |
| L14 | 治理类部分覆盖群：强制对抗评审车道、受保护路径的计划期风险预告、卫生清单、验收/就绪审计产物 | `planner_review_runtime.py`、`utils/protected_paths.py`、`utils/material_audit.py`、`utils/loop_readiness.py` | 信号大多在，但不强制、不可见、无单份审计结论 |
| L15 | humanizer 排序（审查过后才许润色人话段） | `truth_review_gate.py:evaluate_truth_review_gate` | 只丢行为规矩，Python 侧此项本身不计分 |
| L16 | real-project-validation：对真实项目跑一组校验并分级 | `commands/report.py:real_project_validation` | 状态面从「会自己复核」退化为「只读账本」 |

### 1.4 先说清三件容易误读的事

1. **有几处 Go 是故意比 Python 严的，不算缺口**：完工四条件全硬（Python 有 force 逃生阀和异常降级）、非发布门口不许进 done、挂检查者时新增「干活者≠检查者」。这些是判定口径变化，单列在 3.10，不混进缺口数。
2. **有几处是同名异实，最容易误判对等**：`ask`（Python 是意图路由，Go 是提问账本）、`profile`（Python 是项目画像，Go 是执行器档案）、`report`（Python 能生成报告和跑实证，Go 的 spine 只是只读投影）、`task`（Python 是定时任务，Go 的 todo 是 run 内待办）。本审计一律按实质判，不按名字判。
3. **go.8 的签字门是真超出，不是补齐**：Python 没有「未签字不许冻」的带状态账本，Go 这块比 Python 硬，应记为 Go 的净增量（见第 5 节），不能拿它抵扣想清楚环节其他缺口。
---

## 2. 命令层全表（77 条）

证据列：Go 侧给文件+函数/命令名，Python 侧给命令文件。Go 顶层命令实测只有：design、doctor、gate、goal、implement、loop（指针）、migrate、projects、qa、release、skills、spine、truth（外加框架的 completion/help）。

| Python 命令 | 档位 | Go 承接 | 证据 | 备注 |
|---|---|---|---|---|
| goal | parity | 顶层 `goal`，40+ 子命令 | Go `internal/cli/goal.go:newGoalCmd`；Py `commands/goal.py` | keep 主入口，语义最完整 |
| truth | parity | 顶层 `truth`（create/freeze/change/refreeze/baseline/show/list + ask/answer/intent/confirm/drift/rebase） | Go `internal/cli/truth.go:newTruthCmd`、`truth_intent.go`；Py `commands/truth.py` | Py 的 gen/review/prose-check 等子面被重组承接，非逐名对应 |
| design | parity | 顶层 `design`（status/bind） | Go `internal/cli/abilities.go:newDesignCmd`；Py `commands/design.py` | 薄面，与 `goal design-stage` 同一构造器；符合 §3.4 slim 边界；Py 的 route contracts 无对应 |
| implement | parity | 顶层 `implement`（plan/admit/delegate-ralph/receive-result/worktree） | Go `abilities.go:newImplementCmd`、`orchestrator/lanes.go:PlanImplement`、`admission.go:CheckImplementAdmission`；Py `commands/implement/commands.py` | 与 goal 下同一批构造器共实现 |
| qa | parity | 顶层 `qa`（status/evaluate/planner-review/score/score-history） | Go `abilities.go:newQACmd`、`cli/qa_score.go`；Py `commands/qa.py` | 薄面；Py qa 的 team-run/evolve 等无对应 |
| release | parity | 顶层 `release`（evaluate/complete） | Go `abilities.go:newReleaseCmd`；Py `commands/release.py` | 仅发布门裁决+完成；清单面缺失见 H7/L10 |
| program | parity | 降级为 `goal program` 子命令组（plan/change/freeze/admit/route） | Go `internal/cli/goal_program.go:newGoalProgramCmd`、`orchestrator/program.go`；Py `commands/program.py` | `beacon program` 顶层不存在；validate-graph/guard-sea/drive 仅部分对应 |
| doctor | parity | 顶层 `doctor`，仅 `rebuild` 一个子命令 | Go `internal/cli/doctor.go:newDoctorCmd`、`internal/migration.Rebuild`；Py `commands/doctor.py` | 与 §3.5 承诺错位：承诺留的查上下文/查版本面/修证据清单在 doctor 面不存在，只以字符串躺在 `orchestrator/selfheal.go:selfHealTable` 里（引用的还是 v1 命令名） |
| skill | parity | 无顶层 `skill`；复数名 `skills`（list/show） | Go `internal/cli/skills.go:newSkillsCmd`；Py `commands/skill.py` | 名字漂移：必须打 `skills`；只剩六个内嵌 skill 的只读 loader；register/bind/prefer/discover 全无；评分在 `qa score` |
| archive | **gap** | 无 | Go 全仓无 archive 命令；仅 `migration/scan.go` 扫 archive 目录、`store/spine_derive.go` 有 `"archive"` 字符串；Py `commands/archive.py`（plan/apply/status/explain/lineage/inventory） | M10：slim 承诺的「按版本定位归档包」无任何面 |
| init | **gap** | 无（近亲 `projects register`） | Go `internal/cli/projects.go:newProjectsRegisterCmd`；Py `commands/init.py` | M11：register 只登记 DB 行，不写项目上下文/装配文件 |
| hooks | **gap** | 无 | Go 唯一出现是 `selfheal.go` 里字符串 `beacon hooks run …`；Py `commands/hooks.py`（list/plan/run/disable/status） | M12：钩子框架与发布后装机钩子均无实现，自愈计划引用的是不存在的命令（死引用） |
| prd | merged | `truth create` 的包内 truth.md | Go `orchestrator/truth.go:CreateTruthPackage`、`truth_skeleton.go:truthSkeleton`；Py `commands/prd.py` | 按口径稿并入包内材料；无 Py 的模板渲染级生成，只有骨架 |
| user-story | merged | `truth create` 的意图快照+提问节 | Go 同上；Py `commands/user_story.py` | 弱承接：无 user story 句式生成 |
| test-case | merged | `truth create` 的 tests.md 覆盖矩阵 | Go `truth_skeleton.go`；Py `commands/test_case.py` | 包内材料形态承接；无独立生成器 |
| code2truth | **gap** | 无 | Go 全仓 grep `code2truth` 零命中；migrate v1 只导 MD+JSON 不是代码；Py `commands/code2truth.py` | M13：从存量代码反推真相的整条能力 |
| freeze | merged | `truth freeze` | Go `cli/truth.go:newTruthFreezeCmd`、`orchestrator/truth.go:FreezeTruthPackage`；Py `commands/freeze.py` | 含完整性+integrity 预检，比别名时代更实 |
| plan | merged | `goal program plan`（另 `goal implement plan`、`goal flow`） | Go `cli/goal_program.go`、`orchestrator/program.go`；Py `commands/plan.py` | 归属错位：口径稿记「并入 truth」，实际落在 goal/program；能力在 |
| reopen | **gap** | 无 | Go 全仓 grep `reopen` 零命中；Py `commands/reopen.py`（从当前激活版建新 revision 并标材料 stale） | M14：change/refreeze/rebase 都不是重开 |
| review | **gap** | 无同义面（仅碎片 `gate integrity`、`qa planner-review`） | Go truth 子命令表无 review；Py `commands/review.py`（spec/quality/design/ceo/accept/writeback） | M15：实现审查与回写整面缺失；计划评审与完整性门不能冒充 |
| change | merged | `truth change` + `refreeze` | Go `orchestrator/truth_change.go:RequestTruthChange`；Py `commands/change.py` | 不动真相本体，语义对齐 |
| planner-review | merged | `goal planner-review`、`qa planner-review` | Go `orchestrator/planner_review.go:RunPlannerReview`；Py `commands/planner_review.py` | 真子进程多评审；归属错位（记在 truth 下，实在 goal/qa）；行为层问题见 H5 |
| think | merged | truth 冻前环节 `truth intent/ask/answer/confirm` | Go `orchestrator/intent.go:UpdateIntent/AskQuestion/ConfirmIntent`；Py `commands/think.py` | 只承接冻前意图固化；Py 的产品思考画布类无对应 |
| brainstorm | **gap** | 无（仅技能模式标签） | Go 唯一命中 `orchestrator/meta.go` 的 `"mode:pln-brainstorm"` 字符串；Py `commands/brainstorm.py` | M4：发散探索与收敛本体均无 |
| prd-bootstrap | **gap** | 无（`truth create` 只算半个） | Go create 只渲染空骨架，不吃种子材料；Py `commands/prd_bootstrap.py` | L9：自举生成首版包的语义缺失 |
| truth-map | **gap** | 无 truth 子命令；仅内部碎片 | Go `orchestrator/bundle.go:bundleTruthMap`（注释自认无独立 artifact）+ `store/spine.go:ResolveFeature`；Py `commands/truth_map.py`（resolve/verify/重建 machine JSON） | M16：口径稿称并入 truth 子命令，实测无 |
| gen-truth | merged | `truth create` + `truth intent` | Go `cli/truth.go:newTruthCreateCmd`；Py `commands/truth.py:gen_cmd`（别名注册见 `cli_factory.py`） | Py 的 intent-first 一次成文被拆成骨架+分步，非一键生成 |
| materialize-feature-package | merged | `truth create`（四件套落盘+store 注册） | Go `orchestrator/truth.go:CreateTruthPackage`；Py `commands/feature_package.py:materialize_feature_package_entry` | 语义对齐；Py 的确定性脚手架/任务闭合分支无对应 |
| validate-feature-package | merged | freeze 预检 + `gate integrity` | Go `orchestrator/truth.go:freezePrecheck`、`orchestrator/gates_integrity.go:EvaluateIntegrity`；Py `commands/feature_package.py:validate_feature_package_entry` | 无独立 validate 子命令，校验绑在两处（弱承接） |
| friction-intake | **gap** | 无 | Go grep `friction` 仅 meta.go 技能标签；Py `commands/friction_intake.py` | L1：与 feedback 的合并承诺落空 |
| feedback | **gap** | 无 | Go 全仓 grep `feedback` 零命中；Py `commands/feedback.py`（482 行） | L1：同上 |
| orchestrate | merged | `goal` 编排本体 | Go `orchestrator/engine.go:NewEngine/CreateRun`；Py `commands/orchestrate.py` | 编排即 goal 引擎，承接实 |
| resume | merged | `goal resume` + `goal resume-pack` | Go `orchestrator/pack.go:ResumePack`、`resume.go`；Py `commands/resume.py` | 含恢复决策，非仅续跑 |
| ask | **gap** | 无同义承接（同名异实） | Go `truth ask` 是提问账本（`orchestrator/intent.go:AskQuestion`）；全仓无意图路由/置信度代码；Py `commands/ask.py:ask_entry`（auto/rule/llm 路由+置信度门+预览/执行） | L2：口径稿称并入 goal 入口意图识别，Go goal 入口无此面 |
| remember | merged | `goal memory reindex/search` | Go `orchestrator/memory.go:MemoryReindex/MemorySearch`；Py `commands/remember.py` | 弱承接：无显式写入命令，靠 run 留痕后索引 |
| recall | merged | `goal memory search` | Go 同上；Py `commands/recall.py` | 会话恢复由 resume 分担 |
| workflow | merged | `goal run/tick/continue` + `goal flow` + loop 状态机 | Go `cli/goal_flow.go:newGoalFlowCmd`、`core/loopgoal.go`；Py `commands/workflow.py`（3725 行） | 全流程由 goal 机器承接；Py 的单命令直跑各阶段形态无对应 |
| run | merged | `goal run` | Go `orchestrator/engine.go:CreateRun`；Py `commands/run.py` | 「统一 goal/run」口径兑现 |
| team | merged | implement 的 team lane（选路+统一收活） | Go `orchestrator/lanes.go:PlanImplement/HarvestLaneResult`；Py `commands/team.py`（862 行） | 只承接口径稿点名的交接校验；Py 的团队状态机/HUD/worker 管理无对应 |
| ralph-bridge | merged | `goal delegate-ralph` + `receive-result` | Go `orchestrator/delegate.go:DelegateRalph`、`exec_ralph.go:NewRalphExecutor`；Py `commands/ralph_bridge.py` | 配置命令后是真循环，非占位 |
| loop-goal | merged | `goal` 本体 + `goal queue` + 顶层 `loop` 指针 | Go `core/loopgoal.go`、`cli/goal_watch.go:newGoalQueueCmd`；Py `commands/loop_goal.py` | mark-complete 的「禁止」语义原样移植，本组最忠实一条 |
| task | **gap** | 无（`goal todo` 不是它） | Go 只有 run 内待办（`cli/goal_watch.go:newGoalTodoCmd`）；全仓无定时任务/定时器；Py `commands/task.py`（create/list/enable/disable/wake/…/install-timer） | L3：口径稿把两者混同，实为无承接 |
| real-project-validation | **gap** | 无 | Go 仅 meta.go 技能标签；Py `commands/report.py:real_project_validation`（工厂注册为顶层） | L16：对真实项目跑验证并分级的能力 |
| test-progress | **gap** | 无 | Go spine 是阶段进度（`cli/spine.go:printSnapshotHuman`）；Py `commands/test_progress.py`（show/update/sync） | L4：用例级进度不可由阶段进度互替 |
| deep | **gap** | 无（仅状态名沾边） | Go 只有 `qa_deep_routing` 状态名；qa 子命令表无深验收入口；Py `commands/deep.py` | L5：口径稿称并入 qa 深验收，未兑现 |
| verify | merged | `qa evaluate` / `gate evaluate` | Go `cli/gate.go:newGateEvaluateCmd`、`orchestrator`（gate 包）`EvaluateGate`；Py `commands/verify.py` | 裁决落 gate_results（注：域 4 另记 verify 的 profile 化验证面属拍板弃，两处口径不同——命令层按口径稿 merge 判，行为层按 compat drop 判） |
| auto-test | **gap** | 无 | Go qa 只判已登记证据，全仓无测试执行器；Py `commands/auto_test.py`（3390 行） | H8：merge+slim 承诺的「执行+留证据」半边不存在 |
| eval | merged | `qa score` / `qa score-history` | Go `cli/qa_score.go:newQAScoreCmd`、`orchestrator/score.go`；Py `commands/eval.py` | 一套维度、只评不改；但 agent 评分线行为层缺失见 M1 |
| board | merged | `spine show/list`；HTML 看板按口径稿 drop | Go `cli/spine.go:newSpineShowCmd`；Py `commands/board.py` | 状态读取由 spine 承接，视角损失见 L12 |
| status | merged | `spine show/list` + `goal status` | Go `store/spine.go:ResolveFeature` 等；Py `commands/status.py`（916 行） | 四张状态脸收敛为一张，符合口径稿 |
| report | merged | `spine` 只读投影群（show/list/artifacts/gate） | Go `cli/spine.go:newSpineCmd`；Py `commands/report.py`（967 行） | 只承接只读汇总半边；generate 叙事与实证面分别见 L12/L16 |
| manifest | merged | store 注册表 + `spine list`/`truth list` 派生清单 | Go `store/dao_p3b.go:ListFeatures`；Py `commands/manifest.py` | 清单变派生视图，符合「manifest 本质是投影」 |
| prototype | **gap** | 无（design 面明确不生成） | Go `goal_p2c.go:newGoalDesignStageCmd` 注释自认 "no prototypes are generated"；全仓无宿主桥接生成；Py `commands/prototype.py` | L8：§3.4 承诺的桥接生成未实现，只剩绑定合同半边 |
| ui-spec | **gap** | 无 | Go design 只有 primitive 绑定（`orchestrator/design.go:DesignStageStatus/BindDesignPrimitive`）；Py `commands/ui_spec.py` | L8：UI 规约本体无人承接 |
| debug | **gap** | 无 | Go doctor 只有 rebuild；Py `commands/debug.py`（1149 行） | L6：称并入 doctor，doctor 里无任何 debug 子集 |
| lighthouse | **gap** | 无 | Go doctor 无巡检项；selfheal 引用的 doctor 子命令不存在；Py `commands/lighthouse.py` | L6：同上 |
| help | merged | Cobra 自带 help + root 长说明 | Go `internal/cli/root.go:NewRootCmd`；Py `commands/help.py`（按阶段给下一步建议，278 行+support 2012 行） | 按口径稿定义验真；Py 的阶段感知导航能力随之弃掉，只剩框架级帮助 |
| profile | **gap** | 无（同名异实） | Go profile 是执行器档案（`internal/harness/profile.go`）；配置面仅 `BEACON_STORE_PATH` 一项；Py `commands/profile.py`（项目画像 create/show/discovery） | L7：口径稿称并入全局配置，Go 既无配置文件也无项目画像 |
| gate | merged | 顶层 `gate`（evaluate/status/integrity） | Go `cli/gate.go:newGateCmd`、`orchestrator/gates_eval.go:EvaluateGate`；Py `commands/gate.py`（5091 行） | 超额保留：口径稿说顶层名 drop、判定并入 gate 包，Go 把名也留了；判定真实存在 |
| workspace | merged | `goal admit`/`implement admit` + `goal topology` + `goal worktree` | Go `orchestrator/admission.go:CheckImplementAdmission`、`cli/goal_topology.go`；Py `commands/workspace.py` | admit 与 topology 有实承接；release 子项靠 worktree 操作分担 |
| version | merged | 内部化：`--version` 标志 + projects/spine 输出带 versions | Go `cli/root.go:Version`、`cli/projects.go:buildProjectView`；Py `commands/version.py` | 实测 `beacon version` 为 unknown command；按 slim 口径判 merged；Py 的版本规划面（create/experiment/fleet）整体无承接 |
| analyze | dropped | — | Py `commands/analyze.py`；Go 无 | labs 工具箱，§3.3 拍板弃 |
| api-design | dropped | — | Py `commands/api_design.py`；Go 无 | 同上 |
| data-dictionary | dropped | — | Py `commands/data_dictionary.py`；Go 无 | 同上 |
| jtbd | dropped | — | Py `commands/jtbd.py`；Go 无 | 同上 |
| kano | dropped | — | Py `commands/kano.py`；Go 无 | 同上 |
| moscow | dropped | — | Py `commands/moscow.py`；Go 无 | 同上 |
| persona | dropped | — | Py `commands/persona.py`；Go 无 | 同上 |
| roadmap | dropped | — | Py `commands/roadmap.py`；Go 无 | 同上 |
| memory-friction-evolution | dropped | — | Py `commands/memory_friction_evolution.py`；Go 无 | labs 演化实验；与 friction-intake 是两条不同命令，不混 |
| evolution-summary | dropped | — | Py `commands/evolution_summary.py`；Go 无 | 拍板第 6 条确认 drop |
| prompt | dropped | — | Py `commands/prompt.py`；Go 无 | deprecated，drop |
| save | dropped | — | Py `commands/save.py`；Go 无 | deprecated，drop |
| work | dropped | — | Py 无独立文件，是 `cli_factory.py:work_alias`；Go 无 | deprecated 别名，drop |
| labs | dropped | — | Py 容器见 `cli_factory.py:labs_app`；Go 无容器 | 容器名 drop；其下子命令已逐项判定，不随容器算承接 |
| bridge | dropped | — | Py 容器见 `cli_factory.py:bridge_app`；Go 无容器 | 容器名 drop；其下 run/team/ralph 已逐项判 merged |
| truth-cutover | dropped | — | Py `commands/truth_cutover.py`；Go 无常驻命令 | 一次性迁移入口，拍板第 6 条确认 drop；老格式兼容在 migrate 导入器方向 |
---

## 3. 主链行为对账（九个域）

判定词：parity 对等 / partial 部分对等 / gap 缺口 / dropped-by-charter 拍板弃 / go-exceeds Go 超出。每行证据为「Python 文件+函数 ｜ Go 文件+函数」。

### 域 1 · 想清楚（plan / intent / brainstorm）

结论：10 项 — parity 1、partial 3、gap 5、go-exceeds 1。Go 落的是「问答账本+签字门」，Python 的是「自动拆解+主动生成+审查合成」；账本段 Go 更硬，生成段基本不在 Go。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| 意图快照记录 | 快照含原话、`strength_words`（「完整复刻/不要 MVP」类强度词提取）、`silent_scope_change_allowed: False`（`utils/planner_review_runtime.py:_intent_snapshot`；`program_plan_runtime.py:build_plan_start_payload`） | 五字段快照+硬门（`orchestrator/intent.go:UpdateIntent`、`gate/intent.go:CheckIntentGate`） | partial：字段更全，但强度词保真与「禁止静默缩范围」标记丢失 |
| scope_mode 取值与效果 | 自动推导 `full_parity/mvp/patch/brownfield_takeover/research`（`planner_review_runtime.py:_scope_mode`），另有 lake/ocean 第二轴（`intent_decomposition.py`） | 手填自由文本 `focused/full_parity/mvp/explore`，无枚举校验，唯一效果是 full_parity→要求 Parity Matrix 存在（`gate/intent.go`） | partial：取值对不上、推导变手填、填错静默失去门 |
| 推迟账本（deferral_ledger） | full_parity 下命中推迟词即生成「blocked+必须用户拍板」账本项（`planner_review_runtime.py:_deferral_ledger`） | 无此概念 | **gap（M5）** |
| truth 内未决推迟拦截 | fake-delivery 模式表 `pending_deferral`，freeze 硬拦（`utils/truth_integrity.py`） | integrity 第 4 步 `deferral_pending` 拦（`gate/integrity.go`）+ Skipped 态必须带推迟依据（`core/fsm.go:NewSkippedState`） | parity |
| 提问生成 | 内核主动产苏格拉底四问+第一性原理拆解（`utils/brainstorm.py:run_beacon_brainstorm`）、七维发散模板（`commands/brainstorm.py:create`） | 只记账不生成；骨架种一条固定 Q1（`orchestrator/intent.go:AskQuestion`、`truth_skeleton.go`）；r1 报告 §7.1 自认无自动访谈器 | **gap（M4）** |
| 一问一答账本+签字门 | 无等价物（只有协议叙述，无带 ID/状态的账本） | ask/answer/questions/intent/confirm 五命令，未签字或有开放问题拦 freeze，改意图自动作废旧签字（`orchestrator/intent.go`、`gate/intent.go`） | **go-exceeds** |
| 意图自动分解 | 切句→子意图（带平台/能力/范围分类）→7 类旅程种子→高风险域识别→就绪判定（`intent_decomposition.py:decompose_product_intent`，经 `plan_artifact.py` 在用） | 无；只有把已有 AC 统一追溯为同一句泛化来源（粒度远粗于 `intent_first.py:build_provenance` 的逐句溯源） | **gap（M3 的一半）** |
| 一句话物化整包 | 按句捕获意图→推导 AC→渲染带溯源的四件套（`intent_first.py:capture_intents_from_utterance/derive_ac_from_intent/render_intent_first_*`） | 只有固定骨架（单条占位 AC + Q1），靠人/宿主填 | **gap（M3 的另一半）** |
| plan 子意图路由 | 关键词打分把原话分到 friction/source/review/brainstorm/program 五轨（`plan_subintent.py:detect_plan_subintent`，主链在用） | 无意图驱动路由（阶段技能表是阶段驱动，不是同一件事） | **gap（L2）** |
| 评审合成产物 | 确定性合成：意图快照/范围/能力清单/对等矩阵/覆盖映射/推迟账本/自动修复路由等十项，落 `.beacon/state/planner-review/`（`planner_review_runtime.py:build_planner_review_runtime_artifact`） | 真子进程评审，产物只有结论+工件引用（`orchestrator/planner_review.go`） | partial：执行真实性 Go 更强，分析字段整体不在 |
| brainstorm 收敛 | 只读 sidecar+收敛契约+人工收尾脚手架+拓扑写回（`brainstorm.py:_load_convergence_contract`） | 无 | **gap（并入 M4）**：口径稿处置是 merge 不是 drop，合并形态不存在 |

### 域 2 · 冻需求审查清单（truth review A/B/C/D/E + 执行层）

结论：9 项 — parity 0、partial 6、gap 2、go-exceeds 1。六检合起来 Go 只完整覆盖了 C 的骨架；B 大幅弱化、D 只剩约两条、E 整体缺失。Python 总入口 `utils/truth_review_gate.py:evaluate_truth_review_gate`，freeze 时双门拦截（`commands/freeze.py:145-206`）。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| A 意图覆盖 | 查「每条必达意图有没有落点」：矩阵行 intent→AC，带 strength/status，must 未落地单独报错（`check_a_intent_matrix`、`extract_intent_coverage_matrix`） | 查「每条 AC 有没有来源」：方向反转，要求全 AC 覆盖且来源非占位（`gate/intent.go:CheckIntentGate/ParseIntentCoverage`） | partial：strength/status/落点语义无对应；Go 好处是无条件全查（Python 非必需时可整检跳过） |
| B 领域 FSM 审查 | 五元组缺失、转移无目标、状态未声明、无 event、illegal 段缺失=错、illegal 与合法边重合=错、终态未声明=警告；与 freeze 共用同一校验器，另有双摘要与图投影回写（`check_b_fsm_audit` 委托 `utils/truth_fsm.py:validate_truth_fsm`） | 仅当 `domain_required: true` 时查：有表、每行五列非空（首个坏行即停）、章节文本命中「挂起」「非法」各一词（`gate/intent.go`） | partial（显著弱化，**H2**）：不做图校验，写错的状态机可过门 |
| C 可测性（AC→TC） | 每条 AC 须有带命令+断言的 TC，一条 TC 可覆盖多 AC，弱 TC 单独 finding，零 AC=fail-closed（`check_c_testability`） | AC 无覆盖/未知 AC/软 TC 均 finding（`gate/integrity.go:CheckIntegrity/ParseCoverageRows`） | partial：Go 只认特定表头且按固定列位取命令/断言（多 AC 单 TC、无表头续表解析不到）；无「零 AC 即拦」条款 |
| C_exec 执行层/docs-only | 仅业务域要求 ≥1 条真执行 TC；docs-only 判定细（rg 变体、test -f 指文档、print 假桩、true/echo 均算假）（`check_c_domain_exec_layer`、`_is_docs_only_command`） | 行首正则逐行判，全包生效（`gate/integrity.go:docsOnlyCommandRe`） | partial（双向偏移）：范围 Go 更严（不分域、有一条 rg TC 就拦，Python 有真 TC 时不拦）；细度 Go 更松（print 假桩、复合命令不命中） |
| D 填充深度 | 五类：成熟度=scaffold、scaffold 标记残留、业务域缺用户旅程=fail、只建模包生命周期的 FSM=冒充、L0 通用套话（`check_d_fill_depth`） | 仅成熟度与人话占位句两类（`gate/intent.go`） | partial（**H4**：最能防空心的两条硬检缺失） |
| E 模糊词 | AC 含 或者/至少/不少于/等/若干/视情况 或 or/at least 等即拦冻（`check_e_vague`，654 行） | 无（全仓 grep `vague` 零命中） | **gap（H1）** |
| 签字前置门 | review 不要求签字标记 | 未签字/快照占位/开放问题任一即拦（`gate/intent.go`，freeze 串行执行） | **go-exceeds** |
| 审查报告与材料过期 | 报告落仓库内 `.machine/execution/<slug>.truth-review-gate.json`；旧版本整检跳过；change/reopen 时把 tests/tasks 标 stale；release 时对 stale/报告缺失 fail-closed（`run_truth_review_gate/mark_package_materials_stale/evaluate_release_truth_materials_gate`） | 结论落 DB，无报告文件；无版本跳过（更严）；**change 不标 stale，release 不查材料新鲜度**（`orchestrator/truth_change.go` 无 materials_status 写入） | partial（**M7**） |
| Humanizer 排序 | 审查过后、冻结前才许润色人话段且保护机器字段（`evaluate_truth_review_gate` 返回 `humanizer_order`） | 无（r1 报告 §7.4 明示未迁） | **gap（L15，低）**：Python 侧此项本身不计分 |

### 域 3 · goal / loop-goal 语义

结论：22 项 — parity 8、partial 8、gap 4、变严 2（另计）。骨架（12 态、司机、监工、队列、心跳/存档/恢复）迁得完整；丢的主要是状态机上的「记录类」函数和 budget/quota 的判定模型。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| 12 态集合 | `loop_goal_state.py:VALID_STAGES` 12 态 | `core/loopgoal.go:AllLoopStages` 同 12 态同名 | parity |
| 状态转换+停止条件 | 非法态→blocked；stop_when 为字符串列表、多次转换累积合并，求值内置（`loop_goal_state.transition`） | 同 reason_code；但 StopWhen 是单个字符串，求值靠注入接口、无内置求值器时只执行机械规则（`core/loopgoal.go:LoopState.Transition`） | partial |
| 转换→done | 只拦「在发布门口且未确认」一条边，其他态直达 done 不拦 | 任何非 release_gate 态进 done 一律拦 | **变严**（见 3.10） |
| 发布门判定 | 须在 release_gate + 人工确认，确认后代行迁移（`check_release_gate`） | 只判定不迁移，迁移在完工逻辑内（`core/loopgoal.go:CheckReleaseGate`、`orchestrator/complete.go`） | parity |
| 修复循环上限 | 通用计数器，上限 3，超限送出错恢复（`increment_repair_loop`、`MAX_REPAIR_LOOPS=3`） | 同上限同码；但只有 tick 在写代码/验收两阶段失败时才计数（`orchestrator/tick.go:handleStageFailure`） | partial：触发面收窄 |
| 意图分叉留痕 | 路由决策点追加 {阶段，原话片段，决定，原因码，时间}（`record_intent_fork`） | 无（全仓 grep 无 IntentFork） | **gap（M8）** |
| 用户决定缓存/关卡卡 | 同一问题答过不再问、重复确认报错、未确认禁止进下一阶段（`record_user_decision`；`goal_gates.py:build_gate_card/resolve_gate/assert_gate_resolved`） | 无通用机制（steer 的 ack 门与 release 确认是专用件） | **gap（M8）** |
| 评审决定路由 | fail-closed 路由：P0/需拍板→出错恢复、跑偏→drift 态、P1 可降级续行并记审计（`record_review_decision`） | 评审结论只写门结果与 run 记录，不回写状态机；无 severity/route（`orchestrator/planner_review.go:RunPlannerReview`） | **gap（H5）** |
| 标准执行预检 | 五项必查+束新鲜度+陈旧重冻隔离，评审后独立一关（`record_standard_execution_precheck`） | 五项检查并入写代码入场束（`orchestrator/bundle.go`、`tick.go:admissionBundleForEntry`）；新鲜度/隔离 unverified | partial |
| 宿主中断记录 | 记 last_substep+人工停统计，恢复动作=replay_substep（`record_host_interrupt`） | `orchestrator/resume.go:RecordInterrupt/decideResume` 同义 | parity |
| 统一送出错恢复 | 任意调用方统一送 error_recovery（`route_to_error_recovery`） | 无通用入口，只有修复超限与阶段失败两条内置路径 | partial（入口并入 M8） |
| 恢复决策 build_resume | 无 run 时自动取最新；done→none、发布门→等确认、无下一步→禁止空恢复、其余 load_next_skill；输出含人工停统计（`build_resume`） | 五分支齐，另加软堵清除；但必须显式给 run-id，输出无人工停统计（`orchestrator/resume.go:Resume`） | partial |
| 人工停统计 | 每 run 打断次数、按原因分布、p1 续行计数、伪停滞标记（`_ensure_human_stop_metrics`） | 只有 InterruptCount 一个计数 | **gap（并入 M8）** |
| goal 23 子命令面 | run/status/tick/continue/resume/block/attach-verifier/complete/entry-contract/consume-queue/delegate-ralph/design-stage/supervise/heartbeat/checkpoint/resume-pack/memory/driver/axis/trace/hotfix-plan/longrun-doctor/multi-version-plan | 23 个全部有对应（部分拆成二级子命令），另有增项 queue/steer/todo/self-heal/flow/program/worktree/planner-review/topology/artifacts/admit | parity |
| tick 推进条件 | 默认 record-only 只记日志；execute 才派工，过阶段出口门硬拦；plan→truth 带 advisory 诊断 hint（`product_goal_runtime.tick_goal_run`、`goal_stage_exit_gates.evaluate_stage_exit`） | 无 record-only，无执行器直接拦；有 tick 幂等（Go 超出）；advisory hint 无（`orchestrator/tick.go:Tick`、`exitgates.go`） | partial（变严见 3.10） |
| continue 解堵条件 | 先刷新 program 路由落盘再 tick，无 streak 条件（`goal_continue_run`） | streak<3 才清堵再 tick；program 路由同步不在 continue 内（`tick.go:Continue`） | partial（互有严松，见 3.10） |
| complete 闭环条件 | 只查检查者在场+收尾门；force 可全跳；收尾门异常降级放行（`complete_goal_run`） | 四条件全硬：release_gate+人工确认+检查者+qa pass；force 明示不绕过（`orchestrator/complete.go:Complete`） | **变严**（见 3.10） |
| block / attach-verifier | 只记录（`record_blocker/attach_verifier`） | 同义+新增干活者≠检查者校验（`complete.go:Block/AttachVerifier`） | go-exceeds |
| driver 长跑 | 文件锁防双司机、真跑 turn、blocked_streak≥3 或待发布视为 terminal（`goal_agent_driver.py`） | start/status/stop/run-once/run-loop 齐，急停开关与预算停齐（`orchestrator/driver.go`） | parity |
| durable supervisor | attach/supervise_once/detect_stall/mark_host_interrupted（`goal_durable_supervisor.py`） | 同义，心跳年龄判 stall、默认阈值 1800s（`orchestrator/supervisor.go`） | parity |
| discovery queue 防冒充 | 入队/消费/两道防冒充断言（`goal_queue.py`） | 同义，且完工入口对队列类 run 硬拒（`orchestrator/queue.go`、`complete.go`） | parity |
| budget 护栏 | 记的是轮次契约不是 token：五步 turn、无已验证写回不许 spend、交接文本截断、spend 账本（幂等键+来源白名单+quiet-skip 不记账）（`goal_budget.py`） | 只记轮次+时长上限（`orchestrator/guardrails.go:ConfigureBudget`、`meta_p2b.go:BudgetState`） | partial（契约部分 **gap，M9**） |
| quota 护栏 | 五段判定（健康/人工门/等证据/等焦点/配额）+时间窗槽位模型（`goal_quota.py:quota_should_run/normalize_quota_config`） | 无等价判定，只有预算耗尽停 | **gap（H6）** |
| steering 转向 | 按模板把转向渲染成注入司机提示的 bundle；含目标指纹/变更检测（`goal_steering.py`） | 记录+ack 门在；注入降级为摘要；目标指纹无（`guardrails.go:Steer/AckSteering`、`driver.go:nextPromptDigest`） | partial |
| todos 待办 | 带 capability_gate 的全生命周期（new/claim/release/update/supersede/next_open）（`goal_todos.py`） | add/list/done/remove/claim 在；release/supersede/capability_gate/next_open 无（`guardrails.go`） | partial |

### 域 4 · QA 家族（integrity / planner-review / 三套评分）

结论：9 项 — parity 2、partial 3、gap 3、dropped-by-charter 1。统一评分的「只评不改、不冒充门」守住了；丢的是三套评分各自的实测执行部分与 integrity 的五组维度。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| integrity 检查维度 | 聚合七组：清晰度门、领域 FSM 门、四件套缺失、扩展行扫描（占位 AC 语言/非法闭合路径/对齐面缺失/意图缺失等）、假交付扫描+闭合载荷、确定性扫描（占位符拦、空模板仅警告）、用例包校验（`truth_integrity.build_truth_integrity_gate`） | 交叉引用族齐全（缺文件/frontmatter/未知 AC/软 TC/docs-only/无 TC/推迟未决/证据未登记/假交付模式）（`gate/integrity.go:EvaluateIntegrity`） | partial（五组无承接，**H3**） |
| integrity 模式降级 | mode=audit 且已冻→有 findings 也不 blocked；空模板恒 warn-only | 已冻包默认 audit_only 在（`gate/integrity.go` 的 IntegrityModeBlock/AuditOnly） | parity |
| planner-review 机制 | 确定性构造评审产物（是否真并行由环境决定）+结论经状态机按 severity/route 路由 | 真子进程多评审、fail-closed 聚合、诚实标 sequential；但只有 pass/block 二值，不回写路由（`orchestrator/planner_review.go`） | partial（真执行超出，路由缺失 H5） |
| skill 评分（skill_eval_kit） | 7 维且每用例强制带齐；按 3 维实测回放；带/不带 skill 双配置对照基准；产 HTML 复盘（`skill_eval_kit.py`） | 7 维逐字保留为人工打分键，内嵌 10 条语料；引擎不回放、不跑对照（`orchestrator/score.go:ScoreSkill`、`score_corpus.go`） | partial（实测部分 **gap，L11**） |
| agent 评分（agent_eval_kit+grader） | 6 维加权评分卡（行为证据 .25、目录覆盖 .20…）+ 会话记录越权词审查 + live 执行准入分层（`agent_eval_kit.py`、`agent_eval_grader.py:grade_host_behavior_transcript`） | 无 agent 评分对象（p4b 明示只做 skill） | **gap（M1）** |
| qa9（QA 策略） | 按项目类型生成 QA 策略矩阵与默认命令/断言，被 auto_test/gate/qa/test_case 消费（`qa9.py:build_qa_strategy`） | 无分类、无策略生成；qa 门只判已有证据 | **gap（M2）** |
| 统一评分（qa score） | 三套各自为政 | structural 60（机械 8 项）+effectiveness 40（人工）；明示评分不是门；评分不影响任何门有测试钉死（`score.go`、`score_checks.go:structuralItems`） | parity（就口径稿目标而言；维度丢失清单见 M1/L11 与域 2 注） |
| 只评不改 | 评分产物只落状态目录 | --apply 明拒+评分前后文件哈希不变有测试 | parity |
| verify 命令 | 项目 profile 化验证+profile 门，blocked 退出码 2（`commands/verify.py:run`） | 无 verify 命令 | dropped-by-charter（口径稿 compat 五件弃名；命令层 verify 行的 merge 判据是「裁决归 gate/qa」，profile 化验证面本身按此处置不迁） |

### 域 5 · 发布（release）

结论：7 项 — parity 1（另 1 项 hotfix 计划 parity）、partial 3、gap 3。核心差异：Python 的 release 是「版本级」发布判定，Go 的是「单需求/run 级」五项合成，两者不是同一个对象。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| release 命令面 | create（清单）/check（判定）/scorecard（评分卡）三命令（`commands/release.py`） | evaluate/complete 两命令（`cli/abilities.go:newReleaseCmd`） | partial |
| check 判定细节 | 对整个版本跑约 25 项检查（自动测试状态、门报告、证据清单、覆盖工件、元数据合同、切版预检、浏览器阻塞项带修复建议等），任一 fail 即 blocked | 对单个 feature 查 5 项：四门最新结论须 pass+人工确认在录（`gate/evaluate.go:EvaluateRelease`） | **gap（H7）**：广度差一个量级 |
| profile 降级 | --profile full/hotfix/docs_only，急修时部分 fail 改写为 warn 放行（`prod_ops_v168.py:select_release_profile`、release.py:1281-1286） | 无 profile 概念、无降级分支 | **gap（并入 H7）** |
| 人工确认形式与留痕 | 确认在 loop 轴 release_gate 与清单勾选项；check 结论写进 memory 决策条目+lifecycle 事件（release_decision_recorded） | --human-confirmed+--confirmed-by 署名落事件（`cli/goal.go:newGoalCompleteCmd`、`complete.go`） | partial：Go 的确认更硬；Python 的「结论写进 memory 决策条目」无对应 |
| 拒绝后状态 | check fail 只留痕不迁移任何状态 | complete/evaluate 被拒同样只留痕不迁移 | parity |
| release scorecard / create 产物 | 版本级评分卡（`utils/release_scorecard.py`）；清单+版本变更单（含 feature 实现顺序分析） | 均无（qa score 是 skill 分，不是发布分） | **gap（L10）** |
| hotfix 发布校验 | `prod_ops_v168.py:validate_hotfix_for_release` 与 hotfix-plan 联动 | `goal hotfix-plan build/validate` 在（`orchestrator/hotfix.go`） | parity（与 H7 的 profile 降级是两回事，后者仍缺） |
### 域 6 · 被合并的状态面（status / board / report）

结论：11 项 — parity 2、partial 2、gap 5、dropped-by-charter 2。spine 保住了「单个需求进度」视角；Python 状态面里信息量最大的三块——跨仓 git 视角、健康汇总、叙事/实证报告——基本全丢。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| 单 feature 生命周期视角 | 当前阶段/进度标签/阻塞原因/下一步/运行模式/debug 状态（`runtime_projection.py:_collect_feature_lifecycle`） | spine 快照字段覆盖此视角（`store/spine.go:SpineSnapshot`） | parity |
| 跨项目视角 | 看板 portfolio 多项目卡片墙（`runtime_projection_board.py:_portfolio_view`） | `spine list` 可跨项目列举 + projects 注册表；无 portfolio 级汇总（每项目健康/阻塞计数） | partial |
| git 工作区视角 | 每仓根/分支/提交/脏否/角色/工位路径、失败仓计数（`runtime_projection.discover_git_scopes`） | spine/goal 字段中无 git scopes | **gap（L12）** |
| 材料完整度视角 | prd/user_story/test_case 各自齐缺 + 需求完整度与子门状态（`build_material_completeness`） | 只答「产物指针新不新鲜」（stale 判定） | partial |
| 健康汇总 | 总体健康度、陈旧 run/阻塞 run/失败仓计数、同步状态（`runtime_projection` 健康段） | 无聚合，需人肉逐行数 | **gap（L12）** |
| debug/会话/团队视角 | debug 问题、宿主会话、团队 HUD、ralph 轮次、qa 会话（`_collect_debug_issues/_collect_host_sessions` 等） | 无对应字段（最接近的是单 run 的 `goal trace`） | **gap（L12）** |
| status 特化子面 | show-status/loop-budget/预检面/loop 轴面四子命令（`commands/status.py`） | 分散在 goal status/flow status；预检独立读面无 | partial（loop-budget 读面完整度 unverified） |
| board 看板 | 四列泳道卡（八种卡）、attention 卡、文档在线预览、HTML 渲染 | 无；口径稿明示 merge→spine、网页启动器 drop | dropped-by-charter（视角损失记 L12） |
| board dispatch | 可从看板直接执行动作命令（`commands/board.py:dispatch`） | 无（动作都在 goal 子命令） | dropped-by-charter |
| report 叙事汇总 | 人话叙事 + 一致性快照（`report.py:_human_summary/_compute_consistency_snapshot`） | 无 report 面 | **gap（L12）** |
| report 实证面 | 真对项目跑校验命令产 overlay + acceptance-audit（`report.py:real_project_validation`） | 无等价 | **gap（L16）** |

### 域 7 · host / skill 面

结论：8 项 — parity 2、partial 4、gap 2。Python 的交付方式是「技能包物化进宿主的技能目录、宿主原生发现、先给目录用到才展开」；Go 是「二进制内嵌 6 份文件、靠 CLI 吐文本」——宿主 agent 没有原生发现路径。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| 宿主发现机制 | 技能包顶层放 SKILL.md 即被宿主（Claude Code/Cursor 等）原生发现加载；内部治理包只放 SUPPORT.md 故不可见（防泄漏）；`doctor setup-agent-surface` 把包物化到宿主技能目录；`commands/skill.py` 管理物化与校验 | `//go:embed` 嵌 6 份 SKILL.md，只有 list/show 两命令吐文本；无 materialize/install 到宿主目录的命令（`skills/skills.go`、`cli/skills.go`） | **gap（H9）**：宿主技能列表里根本没有 beacon |
| 公共面构成（1+6） | 路由包 + plan/truth/design/implement/qa/release 六核，每核有 SKILL.md + references + modes + agents + evals | 嵌的是 goal/truth/design/implement/qa/release——**plan 被 goal 顶替**，无独立 plan 技能；每份仅单文件无子结构 | **gap（并入 H9）** |
| 先目录后展开 | 每阶段（plan 还按意图细分）逐个展开内部技能并记入 run 状态（`references/public-surface-progressive-map.v1.json`；`product_goal_runtime.progressive_skills_for_stage/load_stage_skills`） | list 省略正文、show 给全文，目录层成立；但阶段技能表里的 mode 名与内部技能在 Go 仓内无文件可展开 | partial |
| 阶段→技能映射 | 映射表返回的名字在技能包里真实存在（`loop_goal_state.get_skill_for_stage`） | 映射表逐字同（`core/loopgoal.go:LoopSkillForStage`），但被映射的名字（beacon-pln-review、beacon-eval-qa-deep、beacon-gov-debug…）在内嵌技能集里不存在，show 取不到 | partial：表迁了、被指的东西没迁 |
| references 按需加载 | 每核 references/ 随包物化，宿主按索引按需读 | skills.go 注释称 references 是宿主侧文件，但仓内**没有任何 references 文件** | **gap（并入 H9）** |
| skill 命令面 | 适配器注册表（list/show/register/bind/prefer/doctor/discover）+ 包管理 + eval 入口（`commands/skill.py`，696 行） | 只有 list/show；适配器改走 executor/profile 概念 | partial（口径稿 slim 只留 loader+评分，registry 不迁有据） |
| 只评不改的执行 | 评分产物只落状态目录，不回写被评对象 | --apply 硬拒 + 哈希不变测试 | parity |
| 技能内容本身 | 操作手册：教宿主怎么干活（路由表、硬门、模式索引） | 瘦版手册：教宿主怎么调 beacon 二进制（5 步 CLI+边界） | parity（形态不同，与 Go 引擎定位一致，不算缺口） |

### 域 8 · 数据面（文件状态 vs 全局账本）

结论：9 项 — parity 2、partial 5、gap 2。正文的文件约定守住了，但「状态住在哪里」整体搬家：Python 的状态可从仓库文件读出、随仓库走；Go 的状态只在机器级全局 DB 里。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| 版本目录约定 | `docs/beacon/<version>/features/<slug>/` 四件套；旧版在 archive/ 同构；支持目录重定向（`utils/artifact_paths.py:version_root`、`feature_package.py:package_paths`） | 新建与扫描的主约定一致（`orchestrator/truth.go:Create`、`migration/scan.go` 双根识别） | parity（细差：Go 的根是 DB 字段，Python 是文件约定+catalog 解析） |
| truth 正文权威 | 包文件即权威，投影/账本只是读模型 | DB 永不存需求正文，冻结只改 frontmatter 一行（`internal/store/store.go` 包注释、`orchestrator/truth.go`） | parity |
| 项目注册 | 无注册动作：项目=目录本身，命令对任意目录直接工作 | 必须先 register；未注册时 truth create 拒绝、spine 报 not registered（`cli/projects.go`、`orchestrator/truth.go:Create`、`cli/spine.go:resolveSearchRoot`） | partial（行为变化对用户可见） |
| 当前版本来源 | 入仓文件 `beacon/version-truth.json` 声明，可 review | DB 字段 `project_versions.is_current`；该文件 Go 不读 | partial：同一仓库在两台机器上「当前版本」可不同且无文件痕迹 |
| `.machine` 记录 | 修订状态/审查报告/qa 矩阵/trace 全部躺在版本目录内，随仓库可见可提交（`artifact_paths.py:machine_execution_file` 等） | 只读不写：仅迁移时消费（`migration/scan.go:scanRevision/scanGateFile`）；运行时等价物全在全局 DB | **gap（H10 的一半）**：翻仓库看不到任何包的进度与评审结论 |
| 投影输出 | status 每次从文件现算投影再同步进账本（`runtime_projection.py:build_runtime_projection`、`runtime_projection_store.py:sync_projection`）；另有投影包导出/PocketBase/备份 | spine 为 DB-first 只读；文件改了而没跑 register/migrate/rebuild 之一，显示旧状态；无导出/备份面（`cli/spine.go`） | partial |
| 跨项目视图 | 账本内跨项目列举与详情（`runtime_projection_store.py:list_projects`） | 全局单 DB 天然跨项目，spine list 可走全局作用域 | parity |
| doctor 对账 | 文件→投影单向同步 + 从投影事件日志重建账本 | 双向对账三类 drift、默认只报、--apply 克制写、永不覆盖已有快照与门结论（`migration/doctor.go`） | partial：Go 多了 drift 清单，少了事件日志整库重建 |
| 状态随仓库迁移 | 包+.machine+version-truth.json 一起 clone，状态基本随行 | 四件套随行，但注册/当前版本/lifecycle/gate/轨迹/指纹基线全留在原机器 DB；新机只能 migrate 派生重建且精度折损（gate 时间用文件 mtime、lifecycle 带 derived 标记，p3b §2/§8 自述） | **gap（H10 的另一半）** |

### 域 9 · Python 独有在用能力（免疫七项逐个核实 + hygiene/handoff/audit）

结论：核实 11 项 — parity 1、partial 5、gap 5。先把账对清：p3b §7 的「未覆盖 7 项」= v1.6.3 段 2 项（post-release hook、spec fingerprint）+ v1.6.4–1.6.6 段 5 项（chrome e2e、methodology 合同、pattern registry、protected-path、remediation sandbox）。在 go.8 的真实状态：**真补齐 1 项、部分补齐 2 项、仍无 4 项**。p4c 报告把其中 3 项记为「按拍板结案」——那是 Go 项目内部处置决定，不是行为已存在，本审计仍记 gap。

| 行为项 | Python 行为 | Go 现状 | 判定 |
|---|---|---|---|
| spec fingerprint 包权威路径（免疫 #6） | 指纹按包权威路径计算（v1.6.3 修复湖） | `core/fingerprint.go:PackageFingerprint` 只算包自身两文件、freeze 写基线、doctor 新增正文 drift 对账；go.8 另有逐 AC 指纹 | parity（7 项中唯一真补齐，且超出原修复范围） |
| post-release hook 名单（免疫 #4 前半） | 修复后名单以 `utils/agent_surface.py` 为单一事实源 | 名单从注册表实时派生、无第二份硬编码（`orchestrator/selfheal.go:postReleaseCoverageDetail`） | partial：名单不漏了，但执行本体见下行 |
| post-release 装机执行（免疫 #4 后半） | hook 真执行装机对齐（含临时工位、原根脏拦截）（`utils/project_hooks.py:_run_beacon_post_release_local_install`） | 只出计划+需确认，永不执行，且无 hooks 命令面 | **gap（并入 M12）** |
| Chrome 扩展 E2E（免疫，v1.6.4） | 可加载扩展、按 manifest 算 ID、不符 fail-closed，有实际 runner（`utils/playwright_extension.py`、`adapters/playwright_extension_runner.py`） | 无任何路径（p4c 结转 P6） | **gap（L13）** |
| QA 方法论合同（免疫，v1.6.4） | 第一性原理+对抗审查写进行为契约，且 QA/release 前强制补 adversarial 评审 lane（`planner_review_runtime.py:ensure_qa_release_adversarial_lane`） | 无合同面、无强制 lane；planner-review 不是 qa/release 门的硬前置 | partial（**L14**）：对抗审查靠调用方自觉 |
| loop pattern registry（免疫，v1.6.5） | 机器可读注册表文件（按版本存放、可校验），被 readiness/budget/stagnation 三处真实消费（`utils/loop_patterns.py`） | 无注册表；模式集是代码内静态定义 | **gap（L13）** |
| protected-path denylist（免疫，v1.6.5） | 匹配+阻断载荷+planner 阶段风险预告三件（`utils/protected_paths.py`） | 五类匹配+合并前三点守门（`core/protected_paths.go`、`orchestrator/worktree.go`） | partial（**L14**）：写面已对等甚至更靠主干；缺动手前的风险预告 |
| remediation sandbox（免疫，v1.6.6） | 工位内 overlay 试错、冒烟评估、失败自动回滚、保 canonical 不被改；被 readiness 消费（`utils/remediation_sandbox.py`） | 无；self-heal 自动动作只有修证据清单一项 | **gap（L13）** |
| hygiene 卫生清单 | 证据卫生 8 项+发布运维 8 项随审查产物产出（`planner_review_runtime.py:_qa_evidence_hygiene/_release_ops_review`）；`utils/material_audit.py:build_material_audit` 被七处主链调用 | hygiene 清单面无；material_audit 的包级硬拦部分由 integrity 承接，跨版本材料闭合汇总无输出 | partial（**L14**） |
| handoff 交接 | 写文件交接，被 implement 主链调用（`utils/team_surface.py:write_implement_to_qa_handoff`；`commands/team.py:generate_handoff_report`） | 最小交接结构+缺一即拒+收活校验（`orchestrator/meta_p2c.go:TeamHandoff`、`lanes.go:ValidateTeamHandoff`） | parity（人读版报告与仓库内交接文件消失，属域 8 同源折损） |
| audit 类产物 | acceptance audit 对外产物；loop readiness 审计汇多信号出单一就绪分（`utils/acceptance_audit.py`、`utils/loop_readiness.py:build_loop_readiness_audit`，经 `commands/doctor.py` 对外） | 无审计产物；信号分散在 self-heal/longrun-doctor/gate status 三处，无单一就绪评分 | partial（**L14**）：「拿一份审计报告走人」无落点 |

另：扫描中见若干无主链调用证据的面（如 `utils/agent_eval_live_executor.py` 的独立入口形态等），未逐一完成调用链验证，按口径不计缺口，记 unverified（见第 6 节边界）。

### 3.10 语义变严 / 变松清单（判定不一致，不是缺口，但迁移用户会撞上）

**变严（Python 放行或降级 → Go 硬拦）：**

1. **完工**：Python 只查检查者在场+收尾门，force 可全跳、收尾门抛异常时降级放行 → Go 四条件全硬，`--force` 明示不绕过（`product_goal_runtime.complete_goal_run` ｜ `orchestrator/complete.go:Complete`）。
2. **状态进 done**：Python 只锁发布门口一条边 → Go 非 release_gate 态一律不许进 done（`loop_goal_state.transition` ｜ `core/loopgoal.go:Transition`）。
3. **挂检查者**：Python 只记录 → Go 新增干活者≠检查者校验，同源拒绝（`complete.go:AttachVerifier`，超出型变严）。
4. **tick**：Python 默认只记录可空转 → Go 无执行器直接拦（`tick_goal_run` ｜ `tick.go:Tick`）。
5. **发布判定**：Python 急修/纯文档档案可把不合格降为警告 → Go 无档案、无降级（`release.py:check` ｜ `gate/evaluate.go:EvaluateRelease`）。
6. **评审聚合**：Python P1 级可 advisory 续行 → Go 评审只有 pass/block 二值，任一有效 block 即拦，无 advisory 通道（与 H5 同根）。
7. **docs-only 用例**：Python 仅业务域且「至少一条真 TC」即可 → Go 全包逐行判（域 2 C_exec 行）。

**变松（Python 硬拦 → Go 放行或不查）：**

8. **integrity 维度缺失型变松**：Python 五组 findings 可拦冻 → Go 根本不产这些 findings（H3）。
9. **标准执行预检时机**：Python 在评审路由后即独立拦截（束不新鲜、陈旧重冻未隔离均拦）→ Go 并入写代码入场束，前段阶段不受此检约束；束新鲜度/隔离子项 unverified（`record_standard_execution_precheck` ｜ `orchestrator/bundle.go`+`tick.go`）。
10. **continue 条件错位（互有严松）**：Python continue 无条件先刷新 program 路由再 tick → Go 加了 streak<3 才解堵（更严），但 program 路由同步这一前置动作在 continue 中消失（更松）（`goal_continue_run` ｜ `tick.go:Continue`）。

---

## 4. 真缺口详单（42 条，按主题分组；编号与第 1.3 节清单一致）

每条给：Python 证据（文件+函数）｜ Go 缺失证据 ｜ 影响面（谁在什么场景撞上）｜ 补齐建议（落点）｜ 工作量（小/中/大）。

### 4.1 冻结质量组

**H1 模糊词拦截缺失**
- Python：`utils/truth_review_gate.py:check_e_vague`（654 行），词表含 或者/至少/不少于/等/若干/视情况 与 or/at least/at most/and so on，先剥代码引号防误伤，命中产 `ac_vague_disjunction` 并计入总判定，freeze 走该入口。
- Go：全仓 grep `vague` 零命中；`internal/gate/` 两文件无此逻辑。
- 影响：任何人写需求都会撞——「支持 A 或者 B」这类句子在 Go 能合法冻结，实现者只能猜，验收时无据可依。
- 补齐：`internal/gate/intent.go` 的 CheckIntentGate 增 E 段（或 integrity 增一项），词表照 Python 移植＋引号剥离。工作量：小。

**H2 领域 FSM 图校验退化**
- Python：`utils/truth_fsm.py:validate_truth_fsm` 做图级校验（未声明状态、悬空目标、无 event 边、illegal 缺失、illegal 与合法边重合），`build_truth_fsm_gate_payload` fail-closed，freeze 与 review 共用同一校验器防两套口径。
- Go：`internal/gate/intent.go` FSM 段只有「表在、五列非空、命中挂起/非法关键词」；无图结构解析。
- 影响：带业务状态机的需求（订单、审批、设备状态类）写错图也能冻，下游按错图生成用例与实现，错误到验收才爆。
- 补齐：`internal/gate/` 新增 FSM 图校验器（解析五元组表建图），intent 门 B 段调用。工作量：中。

**H3 integrity 缺五组维度**
- Python：`utils/truth_integrity.py:build_truth_integrity_gate` 聚合清晰度门、领域 FSM 门、扩展行扫描（对齐面缺失/意图面缺失等）、确定性扫描（占位符拦、空模板警告）、`validate_feature_package` 结果。
- Go：`internal/gate/integrity.go:EvaluateIntegrity` 只有交叉引用族（AC↔TC、证据登记、假交付模式、推迟未决）。
- 影响：需求含糊、意图面对不齐、占位符文本混进冻结包——Python 会拦，Go 看不见。
- 补齐：integrity 增五组扫描函数，模式可先 audit_only 上线再转 block。工作量：中。

**H4 填充深度缺两条硬检**
- Python：`check_d_fill_depth` 五类，含业务域缺 `## 用户旅程` 即 fail、FSM 只建模包生命周期（draft/frozen…）判冒充。
- Go：`gate/intent.go` 尾段只查成熟度=scaffold 与人话占位句。
- 影响：赶工时最容易出现的「骨架填满、内容空心」包在 Go 能过冻，审查形同虚设。
- 补齐：intent 门 D 段补 journey 必备与 meta-FSM 冒充两检。工作量：小。

**M7 材料过期标记与 release 材料门缺失**
- Python：`mark_package_materials_stale` 在变更/重开时把 tests/tasks 标 stale；`evaluate_release_truth_materials_gate` 在 release 时对 stale/无报告 fail-closed。
- Go：`orchestrator/truth_change.go` 无 materials_status 写入；release 门不查新鲜度。只能靠 refreeze 时 integrity 全查兜底，release 前无第二道提醒。
- 影响：改了需求正文、测试材料还是旧的，发布时无人提醒。
- 补齐：truth change 写 stale 标记（包 frontmatter 或 store），release 评估增材料新鲜度一项。工作量：小。

### 4.2 编排闭环组

**H5 评审结论不回写状态机**
- Python：`loop_goal_state.py:record_review_decision` 按严重度/路线 fail-closed 路由：包不完整/P0/需人拍板→出错恢复；跑偏路线→drift 态走变更/重冻；P1 在自动模式下可降级续行并记审计与计数，人可升格为停。
- Go：`orchestrator/planner_review.go:RunPlannerReview` 结论只有 pass/block 二值，只写门结果与 run 记录，不回写 loop 状态、无 severity/route/需拍板字段。
- 影响：评审发现 P0 问题也拦不住下一步写代码；发现需求跑偏不会自动触发变更流程，靠人记得。
- 补齐：评审结论结构增 severity/route 字段，tick 在评审后调用路由判定写回 loop 状态。工作量：中。

**H6 配额五段判定缺失**
- Python：`utils/goal_quota.py:quota_should_run` 五段序（健康拦→人工门→等证据→等焦点→配额），前四段命中返回 skip（且 skip≠run）；配额本体是时间窗槽位模型（`normalize_quota_config`：窗口小时×60/槽分钟）。
- Go：无等价函数；只有 `budgetExhausted`（轮次/时长双上限）在司机每轮前置检查，命中即停（`orchestrator/driver.go:DriverRunOnce`、`meta_p2b.go:budgetExhausted`）。
- 影响：长跑等外部证据/等人工时，Go 司机要么空转烧轮次、要么被误判成预算耗尽停掉，真停因不可见。
- 补齐：`internal/orchestrator/` 新增 quota 判定（先落四种 skip 态，槽位模型可后补），driver 每轮先问 quota 再问 budget。工作量：中。

**M8 loop 记录类四件缺失**
- Python：`record_intent_fork`（分叉留痕）、`record_user_decision`+`goal_gates.py` 关卡卡（答过不重复问、未答禁推进）、`_ensure_human_stop_metrics`（人工打断计数与原因分布、伪停滞标记）、`route_to_error_recovery`（通用送恢复入口）。
- Go：四者均无；steer 的 ack 与 release 确认是专用件，不成体系；resume 只有 InterruptCount 一个计数。
- 影响：长跑复盘说不清在哪句话改了路线、同一个人被重复问、停了几次为什么停没数据；非 tick 路径发现卡死只能各自 block。
- 补齐：`internal/core/loopgoal.go` 补四个记录函数 + store outputs 落盘；关卡卡可先做 run 级决策缓存。工作量：中。

**M9 预算契约三件无执行点**
- Python：`utils/goal_budget.py` 的轮次契约（五步 turn、无已验证写回不许 spend）、`enforce_handoff_budget`（交接文本截断 16 行/1800 字）、spend 账本（幂等键冲突拦、来源白名单、quiet-skip 类来源不 spend）。
- Go：`orchestrator/guardrails.go:ConfigureBudget` 只记轮次+时长上限；相关 reason code 常量在 `core/reasons.go` 有定义，无执行点（执行 unverified）。
- 影响：交接文本可无限膨胀吃上下文；重复 spend、轮询类来源误 spend 无防护。
- 补齐：guardrails 增 spend 账本（落 run 快照）与 handoff 截断。工作量：中。

### 4.3 发布组

**H7 版本级 release check 与档案降级缺失**
- Python：`commands/release.py:check` 对整个版本跑约 25 项（自动测试状态、门报告、证据清单、覆盖工件、元数据合同、切版预检、版本面治理、浏览器发布阻塞项等），fail 即 blocked；`--profile full|hotfix|docs_only`（`prod_ops_v168.py:select_release_profile`）在急修/纯文档时把部分 fail 改写为 soft-warn 放行（release.py:1281-1286）。
- Go：`internal/gate` 的 release 评估只查单个 feature 的四门最新结论+人工确认（`EvaluateRelease`），无版本级概念、无 profile。
- 影响：真正按版本发版的团队用 Go 判不了「这一版能不能发」；急修只能绕开 release 面或被全硬判卡住。
- 补齐：新增版本级 check（先落 manifest/coverage/metadata 三类高频项）与 hotfix profile 降级。工作量：大。

**L10 release 产物（清单/变更单/评分卡）缺失**
- Python：`release.py:create` 写发布清单+版本变更单（含 feature 实现顺序分析），`scorecard` 出版本级评分卡（`utils/release_scorecard.py`）。
- Go：无任何发布文档产物。影响：发版留痕、给人看的变更汇总要手工拼。补齐：`release` 面增 create/scorecard，从 store+包文件生成。工作量：中。

### 4.4 验收执行组

**H8 auto-test 执行半边缺失**
- Python：`commands/auto_test.py`（3390 行）真执行测试套件并留证据（含 junit 与断言计数要求，scope=qa9 时按 TC 条目执行）。
- Go：qa 只判已登记证据；证据靠 `goal artifacts register` 人工登记；orchestrator 的子进程执行器跑的是 agent 阶段，不是测试套件。
- 影响：验收证据的生产环节在 Go 体系外——没人手工登记，qa 门就永远等不到证据；「跑没跑测试」与「判没判过」脱节。
- 补齐：qa 面增执行器（按 tests.md 的 Command 逐条跑、自动登记证据），先支持本地命令型 TC。工作量：中。

**M1 agent 评分整条线缺失**
- Python：`utils/agent_eval_kit.py` 六维加权评分卡（行为证据质量 .25、目录覆盖 .20 等）+ `agent_eval_grader.py:grade_host_behavior_transcript`（解析会话记录查越权词）+ live 执行准入分层，经 `commands/eval.py` 对外，受众是宿主 agent 行为验收。
- Go：p4b 报告明示引擎只评 skill 包，未做 agent 产出引擎；transcript grader、live 准入、目录覆盖均无。
- 影响：唯一针对「干活的 agent 本身守不守边界」的验收面没了，如 agent 在会话里偷写发布结论，无机制发现。
- 补齐：`internal/orchestrator/` 增 agent score target（先落 transcript grader 与评分卡，live 准入后补）。工作量：大。

**M2 qa9 策略生成缺失**
- Python：`utils/qa9.py:build_qa_strategy/build_qa_llm_plan` 按项目类型生成 QA 策略矩阵与每类 QA 的默认命令/断言，被 auto_test/gate/qa/test_case 消费。
- Go：qa 面只有判门与 skill 评分，无类型分类、无策略矩阵。影响：新项目没有自动生成的 QA 打法，全靠人写 tests.md 命令列。补齐：随 H8 执行器一并落策略生成（`internal/gate/` 或 qa 面）。工作量：中。

**L4 test-progress 缺失**：Python `commands/test_progress.py` 按 TC 记进度并从 auto-test 同步；Go 的 spine progress 是阶段进度不是测试进度。补齐：随 H8 落 TC 级进度读面（spine 或 qa）。工作量：小（依赖 H8）。
**L5 deep 深验收入口缺失**：Python `commands/deep.py`（run/status/report/contract）；Go 只有同名状态 `qa_deep_routing`。补齐：qa 面增 deep 子命令路由到深度检查集。工作量：小。
**L11 skill 评分实测部分缺失**：Python `skill_eval_kit.py:replay_eval_case/run_skill_eval_benchmark` 做用例回放与带/不带 skill 双配置对照，产 HTML 复盘；Go 的 7 维只剩人工录入键（`orchestrator/score.go`），语料在但不执行。影响：技能改版后无法回归验证路由是否退化。补齐：score 引擎增 replay 模式跑内嵌语料。工作量：中。
**L16 real-project-validation 缺失**：Python `report.py:real_project_validation` 真跑一组校验命令并分级表面；Go 只有 skill 模式标签字符串。补齐：doctor 或 qa 增真实项目验证运行器。工作量：中。

### 4.5 想清楚组

**M3 意图分解与一句话物化缺失**
- Python：`intent_decomposition.py:decompose_product_intent`（子意图+旅程种子+高风险域+就绪判定）与 `intent_first.py`（逐句溯源地渲染四件套），主链经 `plan_artifact.py` 在用。
- Go：`truth create` 只有固定骨架；intent 的自动动作只是给已有 AC 贴同一句泛化来源。
- 影响：「想清楚」阶段内核不产任何种子，全靠宿主 agent 自由发挥，包的质量下限无保障——这是想清楚环节最大的产能差。
- 补齐：`internal/orchestrator/` 增 intent 分解器（先落切句+分类+旅程种子，就绪判定随后）。工作量：大。

**M4 提问生成与 brainstorm 收敛缺失**（含命令层 brainstorm）
- Python：`utils/brainstorm.py:run_beacon_brainstorm` 产苏格拉底四问+第一性原理拆解+冗余切除，带版本材料上下文；另有七维发散模板与收敛契约 sidecar。
- Go：ask 只记账（r1 §7.1 自认）；`internal/orchestrator/meta.go` 只有一个 `"mode:pln-brainstorm"` 技能标签字符串。
- 影响：问答账本容易走形式，问题质量无下限。补齐：truth ask 增 `--generate` 由内核按模板+包上下文产题（仍入同一账本）。工作量：中。

**M5 推迟账本缺失**：Python `planner_review_runtime.py:_deferral_ledger` 在 full_parity+推迟词时强制入账并要求用户拍板；Go 无（integrity 的 deferral_pending 只拦已写下的未决项，不生成「缩范围即入账」记录）。影响：全量承诺被静默降级时无记录无拦截点。补齐：intent 门在 full_parity 下扫描 utterance/AC 的推迟词生成账本行，confirm 时要求逐行处置。工作量：小。
**M6 scope_mode 双轴与推导缺失**：Python 两轴均为推导值（五值+ocean 触发旅程冻结）；Go 手填四值无校验。影响：填错静默失去 parity 门；接管类项目无模式可表达。补齐：intent 记录时按 Python 规则推导并落盘，手填只作覆盖且校验枚举。工作量：小。
**L2 意图路由缺失**（含命令层 ask 与 plan 子意图路由）：Python `commands/ask.py:ask_entry` 是自然语言→命令路由（auto/rule/llm 三模式+置信度门+只预览/真执行），`plan_subintent.py` 是 plan 侧五轨路由；Go 的 truth ask 是提问账本（同名异实），goal 入口无路由器。影响：用户一句话进来只能直接建 run，不能先看路由预览。补齐：goal run 前增 route 预览模式（先做规则路由）。工作量：中。
**L9 prd-bootstrap 缺失**：Python `commands/prd_bootstrap.py` 从种子材料/技能后端自举生成首版包；Go create 只渲染空骨架。补齐：随 M3 分解器落种子成文。工作量：中（依赖 M3）。
**L15 humanizer 排序缺失**：低影响（Python 侧不计分），只丢「润色时机+机器字段保护」行为。补齐：freeze 前在 truth 面加一步人话润色钩子。工作量：小。

### 4.6 数据与宿主组

**H9 skill 宿主发现/物化缺失（含 plan 技能与 references）**
- Python：技能包物化进宿主技能目录、宿主原生发现；公共面 1+6 含独立 plan 技能；每核带 references/modes/agents/evals；阶段映射返回的名字都有实物。
- Go：`skills/skills.go` 只 `//go:embed` 6 份 SKILL.md（goal/truth/design/implement/qa/release，无 plan、无 references），`cli/skills.go` 只有 list/show；`core/loopgoal.go:LoopSkillForStage` 返回的名字（beacon-pln-review 等）在技能集里不存在。
- 影响：宿主 agent 的技能列表里根本没有 beacon，编排跑到某阶段给出 next_skill 名字也加载不到内容——这是 Go 版「靠宿主 agent 干活」模式的地基缺口。
- 补齐：`skills` 面增 materialize（把内嵌包写到宿主技能目录）+ 补 plan 技能与六核 references 骨架 + 映射名字与技能集对齐校验测试。工作量：中。

**H10 状态不随仓库走**
- Python：`.machine/` 记录与 `version-truth.json` 入仓，clone 即见状态与结论。
- Go：状态只在 `~/.beacon/beacon.db`（`internal/store/store.go:DefaultPath`）；未注册项目全系命令拒绝；换机只能 migrate 派生重建（gate 时间用 mtime、lifecycle 带 derived 标记）。
- 影响：协作、换机、CI 场景下包进度/评审结论/当前版本不可见；DB 丢失即历史丢失大半。
- 补齐：增「状态导出/导入」面（把 lifecycle/gate/版本标记以文件形式随仓同步），或至少让 truth show 在未注册时降级为读文件模式。工作量：中。

### 4.7 keep 承诺未兑现组

**M10 archive 缺失**：口径稿 §3.5 承诺留「按版本显式定位归档包」；Go 全仓无 archive 面（`migration/scan.go` 只是扫描时认 archive 目录）。影响：归档版本找不回、说不清谱系。补齐：truth/spine 增 archive 定位读面（先只读）。工作量：小。
**M11 init 缺失**：承诺留「新项目装配上下文最小集」；Go 只有 `projects register` 登记 DB 行，不写任何项目文件。补齐：register 增 `--init` 写最小上下文（版本文件+目录骨架）。工作量：小。
**M12 hooks 缺失（含 post-release 装机执行）**：承诺留钩子框架最小集+发布后装机钩子；Go 无命令无执行器，`selfheal.go` 计划表引用 `beacon hooks run …` 等不存在的命令名（死引用，应先修文案或落实现）。Python 本体 `utils/project_hooks.py:_run_beacon_post_release_local_install`。影响：发布后多 agent 环境对齐仍需体系外手工做。补齐：先落 hooks list/run 最小集+post-release 一个内置钩子。工作量：中。
**L6 debug/lighthouse 并入 doctor 未兑现**：Python `commands/debug.py`（1149 行，hook/提示调试面）与 `lighthouse.py`（巡检）；Go doctor 只有 rebuild，自愈计划引用的 `doctor verify-context/verify-version-surfaces` 在 Go 不存在。补齐：doctor 增 verify-context/verify-version-surfaces 两项（自愈已引用，是现成欠账）。工作量：小。
**L7 profile 项目画像缺失**：Python `commands/profile.py`（create/show/discovery）；Go 的 profile 是执行器档案（同名异实），配置面只有一个 `BEACON_STORE_PATH` 环境变量、无配置文件。补齐：projects 增 profile 读写（落项目文件）。工作量：小。

### 4.8 truth 周边组

**M13 code2truth 缺失**：Python `commands/code2truth.py` 从存量代码反推真相草稿（仍走 freeze 流程）；Go 全仓零命中，migrate 只导 MD+JSON 不是代码。影响：存量项目接入只能从零手写需求包。补齐：truth 增 code2truth 入口（扫代码产草稿包，复用 skeleton+intent 门）。工作量：中。
**M14 reopen 缺失**：Python `commands/reopen.py` 从当前激活版建新 revision 并标材料 stale；Go 的 change/refreeze 是冻结后变更闭环、rebase 是并行变更对基线，都不是重开。补齐：truth 增 reopen（建新 revision+联动 M7 的 stale 标记）。工作量：小。
**M15 review 实现审查面缺失**：Python `commands/review.py` 对实现做 spec/quality/design/ceo/accept 五类审查并 writeback 回写；Go 只有完整性门与计划评审，均不能冒充。补齐：qa 或 truth 增 review 面（先落 spec/quality 两类+writeback）。工作量：中。
**M16 truth-map 独立面缺失**：Python `commands/truth_map.py` 有 resolve/verify 与 machine JSON 重建，且是编排启动预检；Go 只有 admission bundle 内一项不可独立调用的派生校验（`orchestrator/bundle.go:bundleTruthMap` 注释自认无独立 artifact）。补齐：truth 增 map resolve/verify 子命令，复用 bundle 逻辑。工作量：小。
**L1 问题采集入口缺失**（friction-intake+feedback）：口径稿承诺两者合成一个入口归 truth 改进输入；Go 全仓零承接（friction 仅技能标签、feedback 零命中）。补齐：truth 增 intake 子命令落包外改进清单。工作量：小。

### 4.9 命令残余组

**L3 task 定时任务缺失**：Python `commands/task.py` 是项目级调度任务系统（create/list/enable/disable/wake/suspend/resume/log/events/install-timer）；Go 的 `goal todo` 只是单 run 待办，口径稿把两者混同。影响：靠 beacon 定时唤醒任务的用法在 Go 无承接。补齐建议：先评估真实使用量，若确在用，落独立 task 面（可挂系统 cron/timer）。工作量：中。
**L8 prototype 桥接生成 + ui-spec 缺失**：口径稿 §3.4 承诺 prototype 作为 design 子命令走宿主桥接生成；Go design 只有 status/bind，子命令注释自认不生成原型；ui-spec 的 UI 规约本体也无人承接。补齐：design 增 prototype 子命令走 executor/profile 桥（与 ralph 外包同构）。工作量：中。

### 4.10 状态面组

**L12 状态面视角群缺失**：git 工作区视角（哪仓脏、停在哪个提交）、健康汇总（阻塞/陈旧/失败计数）、debug/会话/团队/ralph 视角、report 叙事+一致性快照——Python 均在 `runtime_projection*.py` 与 `commands/report.py`；Go spine 只有单 feature 结构化字段。影响：多仓多 agent 场景下运维第一屏信息全丢，只能逐个 feature 翻。补齐：spine list 增汇总段（先落健康计数+git scopes 两项）。工作量：中。

### 4.11 免疫与治理组

**L13 免疫三项仍无**：Chrome 扩展 E2E（`utils/playwright_extension.py`，需宿主扩展加载能力，工作量大且依赖外部条件）、loop pattern registry（`utils/loop_patterns.py`，工作量：中，落为版本化文件+load/validate+readiness 消费）、remediation sandbox（`utils/remediation_sandbox.py`，工作量：大，落 overlay 试错+评估+回滚）。三项共同点：Python 侧均有真实消费方，不是实验残留；p4c 的「结案」是处置决定不是实现。
**L14 治理 partial 群**：强制 adversarial lane（Python `ensure_qa_release_adversarial_lane`；补齐：qa/release 门前置检查 planner-review 含 adversarial lane，工作量：小）、protected-path 的 planner 阶段风险预告（写面已对等，补计划面呈现，工作量：小）、hygiene 清单（补进 planner review 产物，工作量：小）、acceptance/readiness 审计产物（`utils/acceptance_audit.py`、`loop_readiness.py`；补齐：doctor 增 audit 产物面，工作量：中）。

---

## 5. Go 超出 Python 的部分（单列：不算对等，也不算缺口）

| 项 | 内容 | 来源 |
|---|---|---|
| 问答账本+签字门 | ask/answer/questions/intent/confirm 五命令：未签字、有开放问题、快照占位任一即拦冻；改意图自动作废旧签字（`orchestrator/intent.go`、`gate/intent.go`） | Go 首创（go.8）。Python 只有协议叙述，无带状态的账本与签字门 |
| 冻结基线维度 | 冻结要「已提交」才算数：canonical 分支守卫+baseline 记录+admission 要求 committed（`truth baseline`、FrozenFact 基线维度） | 蓝图标准流（AC-139 等）+ Go 落地；Python 的 freeze 无提交基线概念 |
| 标准执行流八步状态机 | 八步固定顺序、跳步 fail-closed、admission bundle 七/八项（`goal flow`） | 蓝图（AC-138/141/142）为主，其中五项预检源自 Python `record_standard_execution_precheck` |
| 拓扑门 | GitScopeGraph 加载校验→BranchContext 投影→admission 强制消费，三拓扑真 git 行为（`goal topology`） | 混合：图谱格式源自 Python v1.6.7（`utils/git_scope_graph.py`），投影+强制消费是 Go 落地扩展 |
| 逐 AC 指纹+drift 命令 | 冻结时逐条 AC 留指纹，`truth drift` 逐条报新增/改动/删除（`core/fingerprint.go:RequirementFingerprints`） | Go 首创（go.8）；超出 Python 包级 spec fingerprint 的修复范围 |
| 并行变更基线守卫 | change 带 base 指纹，refreeze 基线不符 fail-closed，`truth rebase --accept-current` 显式放行（go.8） | Go 首创；Python 的变更链无此守卫 |
| 全局单账本+spine 读面 | 全局 SQLite 一本账、spine 四视图 DB-first、doctor rebuild 双向对账（永不删行/不覆盖） | 蓝图（AC-017..021、AC-011）；Python 是文件投影+账本同步的双写形态 |
| 完工四条件硬门 | release_gate+人工确认+检查者+qa pass 全硬性、force 不绕过 | Go 口径（比 Python 严，见 3.10 第 1 条） |
| 干活者≠检查者 | attach-verifier 时 source==executor 直接拒绝 | Go 首创（Python 只记录） |
| tick 幂等 | 同输入重复 tick 返回 up_to_date 不重复推进 | Go 首创（Python tick 无幂等机制） |
| 统一评分的机械分 | structural 60 分含 `references_resolve` 等静态项（p4b 自述 v1 无此项），评分落统一账本可查历史 | Go 首创（评分合一本身是口径稿/蓝图要求） |
| 受保护路径合并拦截 | 五类敏感路径在卫星工位合并前 fail-closed 拦截+人工路线（`core/protected_paths.go`、`orchestrator/worktree.go`） | Python 有匹配+预告，合并点硬拦是 Go 主干化（预告面缺失记 L14） |
| 性能 | 读路径比 Python 快 50–200 倍；写路径合并落盘（bench/、p6 修复报告） | Go 实现收益 |
| 单二进制分发+三平台 CI | 一个静态二进制、技能内嵌、Ubuntu/macOS CI+Windows advisory（go.8） | Go 形态（代价见 H9：内嵌≠宿主可发现） |
| 失败模式目录 | 6 个失败模式入 core 供 longrun-doctor 解释（P4c） | Go 落地（Python 对应物 loop-failure-mode-catalog 当时记部分覆盖） |

---

## 6. 审计边界（没验的部分，如实写）

1. **全程源码静态比对，未做双边行为实跑对照**：没有把同一个需求包/同一个 goal 场景分别喂给 Python 与 Go 两个二进制比输出。判定基于函数级语义阅读；凡「实现存在但输出字段级是否逐字一致」的问题（如 spine 输出 vs Python status 输出的字段全集、goal status 中 budget 字段完整度）未穷尽，个别处已标 unverified。
2. **Python 侧「在用」判定**靠主链调用点与测试存在性取证；域 9 另见若干无调用证据的面未逐一完成调用链验证（标 unverified），未计入缺口——若其中有实际在用者，缺口数会增加。
3. **迁移数据面只对行为不对数字**：按任务要求未重跑 migrate 导入计数（P3b 的导入数字未复验）；「状态不随仓库走」的精度折损引自 p3b 自述+migration 代码阅读。
4. **go.8 的 macOS/Windows 行为未验**：路径规范化修复了 55 个 macOS 测试失败是 commit 自述，本审计在 Linux 上未复跑跨平台测试；CI 配置存在性已确认，CI 实际结果未查。
5. **implement 子包深水区未逐行对**：Python `commands/implement/commands.py` 5505 行的三 lane 内部细节（团队交接的全部字段、ralph 循环的全部退出条件）只对了 lane 选路、准入、收活三处主干，未逐函数穷尽；域 3/域 9 中标 partial 的若干项可能还有更细的差异。
6. **技能内容质量未评**：域 7 只对交付机制与构成，不评两边 SKILL.md 文案的操作质量；Go 六份技能引用的 references 不在镜像包内一事（P4b 已发现）属 Python 侧包完整性问题，不在本审计范围。
7. 本报告只落盘，未提交、未推送。

---

## 附：审计方法与证据纪律

- 三路并行静态核查（命令面全表 / 主链 A 组：域 1、2、8、9 / 主链 B 组：域 3、4、5、6、7），每条判定要求文件+函数级证据；主审计对三条最关键判定做了独立复核（见下）。
- 口径稿拍板弃的 16 条只在命令表出现并标依据，不进缺口清单；口径稿处置为 merge/keep 而未兑现的，一律记 gap 并注明与口径稿的出入。
- go.8 新增（意图门、逐 AC 指纹、rebase、内嵌 skills、路径规范化）全部按当前代码核，不按 go.7 旧文档下结论。

主审计独立复核的三条关键判定：
1. **archive/init/hooks 零承接**：`grep '"archive"|"hooks"|"init"' internal/cli/` 无命令注册命中（root.go 唯一 archive 命中是注释里的 "archived"）；与子审计的全仓 grep 结论一致。
2. **E 模糊词缺失**：Python `utils/truth_review_gate.py:654 def check_e_vague` 实存；Go 全仓 grep `vague` 零命中。
3. **内嵌 skills 构成**：`skills/skills.go:12` 的 `//go:embed` 行实测只嵌 goal/truth/design/implement/qa/release 六份 SKILL.md，无 plan、无 references 文件（`ls skills/` 只有六个目录+README+skills.go）。
