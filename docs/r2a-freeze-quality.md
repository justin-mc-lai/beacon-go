# R2a 批次报告：冻结质量组 7 项补齐（H1/H2/H3/H4/M5/M6/M7）

本批把 Python v1.6.19 在「冻结前把关」这一段的判定深度补进 Go 版。
判定规则全部照 Python 源码移植，词表和阈值在代码注释里标了出处行号；
有两处我中途理解偏了，用真 Python 跑了一遍才纠正过来，写在第 3 节。

最终状态：`go build ./...`、`go vet ./...`、`go test ./... -count=1` 全绿，
测试 252 → 265（新增 13 个），E2E 全链 PASS。未升版本号，未提交未推送。

## 1. 逐项落点

### H1 模糊词拦截

- Python 出处：`truth_review_gate.py` 的 `check_e_vague`（约 654 行起），
  词表在 660–675 行：中文的「或者/至少/或/等」和英文的 or/at least 等分支词，
  命中就是 error（拦），先剥掉反引号代码段再匹配。
- Go 落点：`internal/gate/intent.go` 的 `checkEVague`，接在意图门
  （签字 confirm 和冻结 freeze 共用同一道门）里，原因码 `ac_vague_disjunction`。
- 配套：`internal/gate/truthmd.go` 的 `ExtractACDescriptions` 是这项检查的
  取数口——从需求文档里把每条验收标准的描述抽出来。抽取规则按 Python 改齐
  （见第 3 节第 1 条）。

### H2 领域状态机图校验

- Python 出处：`truth_fsm.py` 的 `validate_truth_fsm`——把状态机当图来查：
  转移没有目标状态、事件缺失、非法转移没声明、非法边同时又是合法边、
  声明了不存在的终态，等等。error 级拦，warning 级只报。
- Go 落点：新文件 `internal/gate/fsm.go`（`ParseTruthFSM` 解析五元组表格、
  非法转移小节、终态声明；`ValidateTruthFSM` 做图校验）。
  意图门里原来那段「看关键词猜状态机全不全」的检查已删除，改用图校验结果，
  原因码 `truth_fsm_invalid`。

### H3 integrity（完整性审查）补五组维度

integrity 是冻结后对整个需求包做的全面体检。原来 Go 版只查 7 步，
Python 的 `truth_integrity.py`（`build_truth_integrity_gate`）还多五组，本批补齐：

- 清晰度扫描（`internal/gate/clarity.go` 的 `scanClarityFile`）：
  正文里留着「待确认/待补充/TBD」这类没定的话就报，原因码一组 `requirement_clarity_*`。
- 领域状态机组（`internal/gate/domainfsm.go`，移植 `domain_fsm.py`）：
  包里出现领域内容时，实体模型、状态机、非法转移槽、验收标准与状态机的
  绑定关系都要齐，缺了按 Python 的 REASON_* 码报。
- 确定性扫描（`scanDeterminismFile`）：占位符（「后续补充」之类）拦，
  空模板话术（「具有良好的扩展性」之类）只警告不拦——这是照 Python 的
  warn 口径新给 Finding 加了 Severity 字段，体检结论只被 error 级发现挡住。
- 对齐面扫描（`scanExtensionFiles`）：大包（ocean）必须有对齐面和用户意图面，
  验收标准里有占位语言、测试断言写软了，都在这一组报。
- 用例包结构校验（`internal/gate/pkgvalidate.go` 的 `ValidateFeaturePackage`）：
  frontmatter 必填键、parser_contract 取值、证据权威表形状、任务勾选账本
  形状等结构问题，原因码 `feature_package_validation_failed`。

聚合顺序照 Python 的 TIG（完整性门）顺序追加为第 8–12 步，原有 7 步没动。

### H4 填充深度两条硬检

- Python 出处：`truth_review_gate.py` 的 `check_d_fill_depth`（约 554 行起）。
- Go 落点：`internal/gate/intent.go` 的 `checkDFillDepth`。
  两条硬检：①业务类需求包必须有「用户旅程」一节（`journey_missing`）；
  ②只拿包自己的生命周期状态（draft→frozen 那套）冒充业务状态机要拦
  （`scaffold_meta_fsm`）。同一函数里顺带落了骨架未填标记
  （`truth_scaffold_not_filled`）和脚手架占位行检查，D 维度其余各项原已在位。

### M5 推迟账本

- Python 出处：`planner_review_runtime.py` 的 `_deferral_ledger`（约 376 行）。
- 规则：范围模式是 full_parity（全量对齐承诺）时，只要意图原话或验收标准里
  出现推迟信号（mvp/defer/延期/裁剪等），需求文档里就必须有一张
  `## Deferral Ledger`（推迟账本）表，且至少一行是已批准状态、批准人不是占位符；
  否则签字和冻结都拦，原因码 `unapproved_scope_reduction`。
  Python 的怪癖照样保留：「不要 mvp」因为含子串 mvp 也会触发，要靠账本结案。
- Go 落点：`internal/gate/truthmd.go` 的 `ParseDeferralLedger` +
  `CheckIntentGate` 里的 full_parity 段。账本就写在 truth.md 里，不新增存储。

### M6 scope_mode（范围模式）双轴与推导

- Python 出处：`planner_review_runtime.py` 的 `_scope_mode` 与
  `intent_decomposition.py` 的推导——两个轴的取值合起来是一张受控词表，
  不是自由文本。
- Go 落点：新文件 `internal/gate/scope.go`。词表 =
  full_parity/mvp/patch/brownfield_takeover/research（规划轴）+
  lake/ocean（分解轴）+ focused（Go 保留值，Python 分解轴同位叫 lake）。
  `truth intent` 写入时校验（`internal/orchestrator/intent.go` 的 `UpdateIntent`）：
  非法值直接拒写，原因码 `scope_mode_invalid`（新登记）；
  空值按原话信号推导（全量信号→full_parity、mvp→mvp、修复→patch、大包→ocean、
  其余→focused）。手写进文档的非法值在意图门里再拦一道。
  CLI `--scope-mode` 默认值由写死的 focused 改成空（走推导），help 文案换成词表说明。

### M7 材料过期标记

- Python 出处：`truth_review_gate.py` 的 `mark_package_materials_stale` 与
  发布侧的 `evaluate_release_truth_materials_gate`。
- Go 落点：新文件 `internal/orchestrator/materials.go`。
  变更打开（`RequestTruthChange`）时把 tests.md/tasks.md 的 frontmatter
  标 `materials_status: stale`（这两份材料是照旧需求写的，先标过期）；
  重新冻结成功（`applyFreeze`）时清回 current；
  发布评估（`internal/gate/evaluate.go` 的 `EvaluateRelease` +
  `internal/orchestrator/gates_facts.go` 的 `evalRelease`）多查一道：
  任一面还挂着 stale 就拦发布，原因码 `materials_status_stale`。
- 配套修了一个自伤问题：过期标记写在材料文件里，而文件的「内容指纹」
  本来会把这次标记当成内容变化——轻则让「空变更不许重新冻结」的守卫误放行，
  重则让体检误报漂移。已把 `materials_status:` 行和原有的 `status:` 行一样
  纳入指纹规范化（`internal/core/fingerprint.go`）：这两行是内核自己的记账，
  不是需求内容。

## 2. 骨架变更（1 处，影响所有新建包）

`internal/orchestrator/truth_skeleton.go`：状态机一节的标题从
`## Domain FSM` 改为 `## FSM 五元组`。原因：Python 的领域判定认的是
v1.6.19 实际包的方言；旧标题里的 "domain fsm" 字样会让每个骨架包都被
误判成「领域包」，新硬检会全线拦死。这是移植时发现的方言冲突，
新旧标题在 Go 的解析里都认（`fsmSectionText`），老包不受影响。
同一处顺手改了覆盖矩阵的占位文案（去掉「或」字），否则自家骨架会被 H1 拦。

## 3. 验证中纠正的两处（附证据）

1. **验收标准描述取哪段**：Python 的抽取是「最长描述胜」（按字符数，
   `add_description` 只在更长时替换）。我最初写成「先到先得」。
   用真 Python 在同一夹具上跑 `extract_acceptance_criteria` 核对：
   覆盖矩阵的证据格（29 字符）比验收描述（11 字符）长时，Python 取证据格，
   模糊词检查看的是证据格那段。已按此改齐（`ExtractACDescriptions`），
   测试夹具也按这个口径构造（模糊描述必须长于证据格才会被查到）。
2. **领域验收的回退行**：Python 在找不到验收表时是裸扫编号、描述恒为空，
   于是粗体行写的验收标准永远绑不上状态机，孤儿拦截照样触发。
   我一度自作主张给回退行补标题文本，被真 Python 的运行结果证伪
   （同一「看似完整」的夹具，Python 同样报 `orphan_ac_without_fsm_path`），
   已按源码改回恒空。结论：现有 Go 行为与 Python 一致，包括这条严苛口径。

## 4. 既有测试改动声明（只修夹具数据，断言一条未放宽，新判定一条未放宽）

1. `internal/gate/gate_test.go`：`goodTruth` 夹具补全 frontmatter
   （version/language/parser_contract/canonical_refs），`goodEvidence` 夹具
   补 Authority 权威列。理由：新包校验维度按 Python 要求完整包形，
   旧夹具是残缺包，本就会被新维度拦。
2. `internal/orchestrator/gates_eval_test.go`：`frozenTruthDoc` 同样补全
   frontmatter，evidence.md 夹具补权威表。理由同上（标准流的
   verify-materials 步骤跑完整性体检时暴露）。
3. `internal/orchestrator/intent_test.go`（`TestIntentFSMRequiredWhenDomain`）：
   替换目标标题随骨架改名更新为 `## FSM 五元组`；状态机夹具补
   Illegal Transitions 小节（选的 idle -> hung 不在合法边内）。
   理由：新图校验按 Python 要求非法转移槽必须存在。

新增测试：`internal/gate/r2a_test.go`（10 个）、`internal/orchestrator/r2a_test.go`（3 个）。

## 5. 与 Python 的残留差异（如实清单）

1. `truth_fsm.py` 的 mermaid 图渲染与 digest（变更检测摘要）未移植——
   那是展示与变更检测用途，不参与门判定。
2. 清晰度门里 diagram-truth（图与文一致性）输入、研究闭环、
   support-topology（支撑拓扑）输入在 Go 侧没有对应物，未移植。
3. 包结构校验只移植了结构层：Python 的 L0 人话子门、任务行
   ac=/evidence= 绑定强制、canonical-ready 机器 JSON 检查未移植。
4. 领域状态机的 requiredness（是否必须有领域状态机）里，
   complexity/diagram（图复杂度）驱动的那一路未移植，Go 按缺失处理；
   其余三路（frontmatter 显式声明、章节出现、fixture 期望）已对齐。
5. scope 推导：brownfield_takeover / research 在 Python 里要 source_refs
   证据才能定，Go 的推导器没有这个输入，这两个值只能显式填写、
   不会被自动推导出来；focused 是 Go 保留别名（见 M6）。
6. M7 的 Python 发布材料门另一半（评审报告文件必须存在且结论 pass）
   在 Go 的对应物是已落库的门结论加冻结前置检查，未另行移植。
7. AC 描述抽取的表格块切分沿用 Go 既有口径，与 Python 在个别畸形表上
   可能有细差，未逐例对账。

## 6. 改动文件清单

新增：`internal/gate/fsm.go`、`internal/gate/domainfsm.go`、
`internal/gate/clarity.go`、`internal/gate/pkgvalidate.go`、
`internal/gate/scope.go`、`internal/gate/truthmd.go`、
`internal/orchestrator/materials.go`、两份 `r2a_test.go`、本报告。

修改：`internal/core/reasons.go`（新原因码登记）、
`internal/core/fingerprint.go`（记账行规范化）、
`internal/gate/integrity.go`（第 8–12 步 + Severity）、
`internal/gate/intent.go`（E/D 检查、FSM 图校验接入、推迟账本、scope 校验）、
`internal/gate/evaluate.go`（发布门材料新鲜度）、
`internal/orchestrator/intent.go`（scope 校验与推导）、
`internal/orchestrator/truth_change.go`（打开变更标过期）、
`internal/orchestrator/truth.go`（冻结清标记）、
`internal/orchestrator/gates_facts.go`（发布事实接线）、
`internal/orchestrator/truth_skeleton.go`（骨架标题与占位文案）、
`internal/cli/truth_intent.go`（--scope-mode 默认改推导）、
三份既有测试文件（仅夹具，见第 4 节）。
