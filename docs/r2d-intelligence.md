# R2d 智能面组补修报告（M1/M2/M3/M4/M13/M14）

本批是 Python 对等补修的最后一批（第四批）。补的是六条「智能面」：给干活的 agent 打分的引擎、QA 策略生成器、一句话拆需求的分解器、内核自己生成问题的头脑风暴、从存量代码反推需求的 code2truth、把冻结包重开为新修订的 reopen。参照一律是 Python v1.6.x 源码（`~/workspace/repos/beacon/beacon/`），不是审计文字；审计与源码不一致处按源码走，并在本报告「对审计/任务书的纠正」一节留痕。

收尾状态：`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿；测试计数 307 → 327（新增 23 个测试函数，其中含子断言合并计数差异，净增 20）。E2E 全链 PASS（`e2e/run_e2e.sh`，本批动了 ask 账本表形，必须验证主链——已跑通，`e2e/last_run.log` 为运行产物）。版本号未动（升 go.9 由发版环节处理）；未提交、未推送。

---

## M1 agent 评分整线（agent-eval）

**是什么**：给「干活的 agent」打分的独立引擎，和给 skill 包打分的 `qa score` 完全分开（两套卡、两个落盘目录、互不引用）。

落点：

- 纯规则层 `internal/core/agenteval.go`：
  - 六维评分卡逐字移植 `utils/agent_eval_kit.py` 的 `COMPOSITE_WEIGHTS`（行为证据质量 0.25 最重、边界保持与目录覆盖各 0.20、路线稳定 0.15、来源锚点 0.10、防假交付 0.10）；及格线 70（`REGRESSION_COMPOSITE_THRESHOLD`）、模拟跑封顶 85（`SIMULATED_COMPOSITE_CAP`）均按源码。
  - 维度公式移植 `_compute_dimension_scores` 一式不差，包括一个源码怪癖：`ci_default` 档的行为分是 `host_pass × 100`（不是比例，两次通过就是 200 分）——逐字保留并在代码注释标明，没有「顺手修对」。
  - 会话审查移植 `utils/agent_eval_grader.py`：先查禁词（release verdict / formal_qa_pass / truth_write / --refreeze-chain，命中即不合格），再查 `ROUTE:` 探针缺失，再查路线不符，顺序与源码一致。
  - 准入处置 `AgentAdmissionFromRecord`：无记录=警告放行；最新评分卡低于 70 或最近一次审查有边界违规=拦截。
- 引擎层 `internal/orchestrator/agentscore.go`：目录（capability catalog，技能项查内嵌技能是否真实存在、CLI 项对 Go 的真实命令面、边界负例 4 条逐字）、评分落盘 `.beacon/state/agent-eval/{grades,scorecards}/`、`GradeRunTranscript`（从 run 的事件轨迹拼会话记录再审查）、历史查询。
- 接线：执行器选 profile 时（`exec_resolve.go` 的 profile 解析与迭代执行两处）调用 `CheckProfileAdmission`——只有 blocked 才拒绝，warn 静默放行（警告在 `qa agent-eval admission` 面可查）。
- CLI 面：`qa agent-score`（建卡）、`qa agent-eval grade|history|catalog|admission`。

**本批修掉的一个真 bug**：评分/评分卡文件名原来只精确到秒，同一秒写两条会互相覆盖（测试里 3 条评分只剩 1 条暴露了它）。已改为纳秒戳。

## M2 qa9 策略生成（qa strategy）

**是什么**：看一个项目的家底（什么语言、有没有界面、有没有 API 规格、有没有技能文件），推断它是什么类型的项目，再按类型决定九类测试里哪些必测、哪些不适用，并给出每类的推荐命令。

落点：

- 纯规则层 `internal/gate/qastrategy.go`：逐字移植 `utils/qa9.py`——九类测试名、项目扫描（语言/框架/API 规格/风险点）、四种项目类型推断（有 SKILL.md 或 skills 目录→ai-agent-skill；有前端→web-app；有 API 规格→api-service；其余→sdk-lib）、四类适用矩阵、命令预设覆盖规则（有 Postman 集合→newman、有 OpenAPI→schemathesis、有 docker-compose.test→集成测试走它）、每类的默认断言文案。产物 schema 1.0.2 与 runner 策略（功能测试主跑 browser-use/playwright 等）均按源码。
- 引擎层 `internal/orchestrator/qastrategy.go`：落盘 `.beacon/qa-strategy.json`；Go 侧增补一个字段 `default_command`（冒烟基线，没有冒烟就单元基线）——这是给 H8 的 autotest 命令发现链消费的单一入口，已接线：`autotest.go` 的发现顺序在项目配置之后追加读这份策略文件（没有策略文件时行为与从前完全一致）。
- CLI 面：`qa strategy [--project] [--types] [--ui-runner] [--write=false] [--json]`。

## M3 意图分解与物化（truth decompose）

**是什么**：把一句话需求按确定性规则（关键词表、切句规则，全是抄源码的词表，没有调模型）拆成子意图、用户旅程种子、验收点，并直接写进包的四件套。

落点：

- 纯规则层 `internal/core/decompose.go`：移植 `utils/intent_decomposition.py`——能力/平台/海洋/来源四张词表逐字、7 条旅程种子（触发词+步骤种子+验收种子）逐字、湖标签提取（只认显式标签 `L1-orders`、`湖3`，见纠正一节）、湖/海判定（海洋词命中或显式湖 ≥3 即为海）、就绪判定；外加 `utils/intent_first.py` 的按句捕获（≥10 字一句、来源标记 `utterance_sentence_N:`）。
- 物化层 `internal/orchestrator/decompose.go`：选定形态是「先 `truth create` 建包、再 decompose 填充」（不另开建包路径，包的诞生只有一条路）。物化内容：AC 表（每条子意图子句一条 + 每条旅程验收种子一条，来源可追溯）、`## 用户旅程` 一节由种子**真生成**（不是占位）、覆盖矩阵（来源=子句序号/旅程种子）、tests.md（每条 AC 一条用例，命令按项目家底扫描给真实命令）、tasks.md（带每条 AC 的证据路径）、意图快照（占位或 focused 时才填，不覆盖人写过的内容）、分解产物 JSON 落 `.beacon/state/decompose/<slug>.json`。
- 守卫：已冻结的包拒绝分解（`decompose_frozen_package`）；有待处理 change 时拒绝；拆不出任何实质子句时拒绝（`decompose_no_intents`）。
- 签字作废：分解后 `intent_confirmed` 置 false——分解永远不替人签字，人仍要走 intent→confirm→freeze。CLI 冒烟测试实证：分解出的包能完整走通回答问题→确认→冻结全部门禁（包括 R2a 的新硬检），没有生成「会被自家门拦死的空心包」。

## M4 提问生成与收敛（truth brainstorm）

**是什么**：内核自己往 ask 账本里生成问题（和人问的问题同账本、靠来源列区分），并把「问题清零=收敛」接到既有的 confirm 前置上，不新造第二道门。

落点：

- 纯规则层 `internal/core/brainstorm.go`：移植 `utils/brainstorm.py`——四种模式与别名表、苏格拉底四问（英文原文逐字）、第一性原理 3 条、冗余切除 3 条、按模式给的建议文案与下一跳路由。
- 账本升级：`gate.OpenQuestion` 加 `source` 字段，账本表加第六列 `Source`（旧账本无此列可兼容解析，人问的问题缺省记 `human`）。R2a/R2b 的既有测试全过，表形变化无回归。
- 引擎层 `internal/orchestrator/brainstorm.go`：候选问题=苏格拉底四问（来源 `kernel:brainstorm-socratic`）+ 按包的真实缺口生成的问题（来源 `kernel:brainstorm-gap`：快照还是占位、声明了领域却缺用户旅程、缺 FSM 五元组、有 AC 没有用例覆盖、材料过期——上限 4 条，问法照 Python 收敛契约的形状）；按问题文本去重，重复运行生成 0 条；有新增才写账本并作废签字（新问题出现后旧签字不再算数，与既有口径一致）。
- 收敛口径：视图直接报开放问题数与 confirm 的现成拦截原因（`IntentState.BlockReasons`），`converged = 开放问题为 0`。confirm 的门本身没动。

## M13 code2truth（truth code2truth）

**是什么**：扫一个存量代码库（文件清单、入口点、导出符号/接口路由/数据模型），反推出「这个项目大概在做什么」的候选结论，落成一个 draft 包等人确认。

落点：

- `internal/orchestrator/code2truth.go`：移植 `utils/code2truth.py` 核心子集——跳过目录集、源码扩展名集、文档名集、文件分类规则、入口点规则、500 文件上限与各列表上限、候选结论 CLAIM-001..004 与歧义 AMB-001/002（文案逐字）、understand-anything 七个产物路径的探测（只作证据）。
- Go 侧增补（任务书要求）：对 .go/.py/.ts/.js 的轻量静态提取（导出符号数、路由、模型名），写进清单摘要。
- 产物两层：① Python 原形态的证据文件 `.beacon/code2truth/<slug>/intake.json|md` 先落盘（权威级别 candidate_only）；② 再走正常建包路径建 draft 包，truth.md 打上 `code2truth_candidate: true` 并追加「## Code2Truth 反推草稿」一节（大字标注**反推草稿待人确认**、结论表、歧义清单）。结论只进这一节，**不冒充 AC**；冻结照常走意图门，反推没有任何豁免。目标目录不存在时以 `code2truth_target_missing` 拒绝。

## M14 reopen（truth reopen）

**是什么**：把已冻结的包「重开」成新一轮修订：修订号递增（R1→R2），旧冻结的一切记录原样留痕，新修订从旧正文派生、回到 draft、重新走签字冻结。与 change 的区别：change 是冻结期内记账、refreeze 后还是同一个修订；reopen 是开一个**新修订**。与 rebase 的区别：rebase 只是把待处理 change 换到更新的冻结正文上，不改状态。

落点 `internal/orchestrator/truth_reopen.go`，移植 `commands/reopen.py` + `utils/revision_state.py` 的 `reopen_revision_state`：

- 守卫：非主干分支拒绝（与 Python 一致的分支守卫）；有待处理 change 时拒绝（`truth_change_pending`，先 refreeze 关掉）；未冻结时拒绝（`truth_reopen_not_frozen`，draft 本来就能直接改）。
- 动作：正文 frontmatter 改四处（status→draft、revision_id→新号、source_revision_id→旧号、intent_confirmed→false）；store 里包状态回 draft、生命周期回 Think/NotStarted；tests/tasks 材料标 stale（它们描述的是旧修订，沿用 R2a 的材料过期标记）；事件 `truth_reopened` 留痕，带旧修订号、新修订号、原因、经手人、**旧冻结正文的哈希与包指纹**。
- 修订号解析照 Python：读正文自己的 revision_id，没有就读最近一次冻结记录的，再没有按 R1；递增解析失败按 1 算。

---

## 对审计/任务书的纠正（以源码为准）

1. **code2truth 在 Python 里不建包**。`commands/code2truth.py` 只产 `.beacon/code2truth/<slug>/intake.json|md` 候选证据（authority=candidate_only，promotion 必须用户确认）。任务书要求落 draft 包——Go 按任务书多走一步，但证据层保持 Python 原形态先落盘，包内结论明确标候选、不进 AC 表。审计把它描述成「反推建包命令」不够准确。
2. **agent-eval 的「准入」在 Python 里不直接卡 executor profile**。Python 的硬拦截是 live runner 的 admission 文件与评分卡自身的 blocked 状态（composite<70 → status=blocked）。任务书要求接到 profile 选择处——Go 的映射是：有记录且（低于 70 或边界违规）才拦，无记录只警告。若按 Python 字面，profile 选择根本不查评分。这一映射口径在此声明。
3. **湖的提取只认显式标签**。`_extract_lakes` 只读 `L<n>-name` 与 `湖<n>` 两种显式写法，不从「订单系统、会员中心」这类名词里猜湖。本批开发中曾按错误预期写测试，已按源码改正并留了反向断言（名词不算湖）。

## 残留差异清单（有意保留，均已在代码注释或上文标明）

1. **qa9 的 LLM 计划（build_qa_llm_plan）未移植**：Go 内核不调模型，策略的确定性部分已完整对等。
2. **分解的 task-closure advisory 尾巴未移植**：Python decompose 末尾有一段 v1.6.11 的任务闭合建议，依赖 plan 侧的闭合分析，Go 侧暂无消费方。
3. **分解的用例命令不用 intent_first 的 rg 形命令**：Python 的 tasks/tests 渲染里有 `rg ...` 式命令；Go 的 integrity 硬检对这类「只读文档」命令逐行拦截，照抄会让分解产物被自家门拦死。Go 改用项目扫描得到的真实测试命令，形状不同、语义更严。
4. **一条旅程验收种子在 AC 渲染处单点改写**：种子原文「旅程有测试或清单证据」含「或」，会被 R2a 的模糊词硬检拦。种子在 core 与旅程节保持逐字原文，仅写进 AC 表时改写为「旅程具备测试证据与清单证据」。用户子句里的模糊词只警告、不改写（那是用户的话）。
5. **agent-eval 的 live runner 未移植**：真实宿主矩阵执行、live admission 文件、回归监视属于运行时执行面；Go 落的是评分/审查/落盘/准入处置与 run 轨迹审查。评分卡的执行模式因此是 replay，降级标记（degraded）以「无评分记录」为据，与 Python 的「非 live 准入即降级」口径不同源、已在代码注释声明。
6. **code2truth 的 lighthouse 架构图与 truth-map/源码架构状态块未移植**：只移植 intake 核心子集（任务书口径）。
7. **reopen 的正文派生形态**：Python 的修订状态机维护独立的修订记录；Go 包只有一个工作正文，修订链落在 frontmatter（revision_id/source_revision_id）+ 事件轨迹里，不另叉一份正文文件。
8. **brainstorm 只移植提问生成与收敛**：Python 的 brainstorm 还有由模型产出的选项/决策草案部分，不在确定性内核范围内。
9. **舍入边界**：Python 的 round 是银行家舍入，Go 用四舍五入，恰好落在 .005 边界时可差 0.01（评分卡与既有批次同此说明）。
10. **MarkJourneysFrozen 已移植暂无消费方**：旅程冻结标记函数在 core 就位，引擎侧的旅程冻结流程未接（Python 里它服务于 ocean 分解后的人工旅程冻结，Go 的 freeze 链目前以 AC 矩阵为准）。

## 测试与既有测试改动声明

- 新增测试文件 4 个：`internal/core/r2d_intelligence_test.go`（评分三分支/维度公式手算对拍/准入三态/分解两路/捕获/模式别名）、`internal/gate/qastrategy_test.go`（类型推断/适用矩阵/预设覆盖/类型名归一）、`internal/orchestrator/r2d_intelligence_test.go`（评分全链→profile 拦截、策略落盘→autotest 发现、分解物化与守卫、头脑风暴生成→去重→收敛、code2truth 入包、reopen 全链与两道守卫）、`internal/cli/r2d_test.go`（两个冒烟：qa 五面、truth 四面含分解包走通冻结后再 reopen）。
- **既有测试改动：零**。本批没有修改任何既有测试文件，也没有放宽任何既有判定；新代码与既有 307 个测试直接全过（账本加列、autotest 发现链加回退、profile 准入钩子三处与既有行为的衔接点均有专门验证：无策略文件/无评分记录时行为与从前一致）。
- 新增原因码（登记于 `internal/core/reasons.go`，Python 原样 8 个 + Go 新增 7 个）：`boundary_forbidden_phrase`、`route_probe_missing`、`route_probe_mismatch`、`agent_eval_live_runner_not_admitted`、`agent_eval_admission_missing`、`prompt_fixture_missing`、`catalog_harness_missing`、`live_skill_matrix_mismatch`（以上为 Python 原码，部分仅登记暂无发出点）；新增 `agent_eval_below_threshold`、`agent_eval_no_record`、`brainstorm_mode_invalid`、`decompose_frozen_package`、`decompose_no_intents`、`code2truth_target_missing`、`truth_reopen_not_frozen`。
- store schema 九表未动；版本号未动；未 git 提交/推送。
