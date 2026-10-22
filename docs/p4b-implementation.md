# P4b 实现报告：准入误报修复（A）+ 统一评分（B）

> 日期：2026-10-22 ｜ 版本保持 v2.0.0-go.2（P4 整体出口再升）
> 依据：口径稿 scope-charter.md；冻结真相 AC-151..156、AC-169、AC-170（动手前已读原文核对）；P4a 报告第 5 节观察项。

## 做了什么（大白话）

这一片干了两件事。第一件是修一个别扭点：需求包明明已经冻结在文档里，新建任务后查"能不能开工"却误报"没冻结"，要先跑一次正式门禁评估才转绿。原因是以前每个地方各看各的证据——有的看进度快照、有的看门禁结论，就是不看文档本身。现在全引擎只有一个地方回答"到底冻没冻"，先看有没有挂着没销账的变更，再看文档自己写的冻结标记，文档看不了时才认正式门禁结论，进度快照只当参考、永远不算数。第二件是把旧版三套评分（一套评技能、一套评 agent、一套评 QA）合成一套：给技能包打体检分。结构好不好是机器逐项查的；实效好不好要人拿着固定题目去测、把分录进来，引擎不编。评分只是体检分，不是通行证——任何门、任何准入、任何完工判定都不读它，这条是代码结构上钉死的，不是嘴上说的。

---

## A. 准入误报修复

### A.1 共享判定函数口径

新文件 `internal/orchestrator/frozen_fact.go`：`Engine.FrozenFact(ctx) -> FrozenFact`，全引擎唯一的"冻没冻"判定。判定顺序：

| 步 | 看什么 | 结论 |
|---|---|---|
| 1 | 事件轨迹里有没有未销账的变更（truth_change_requested 之后无 refreeze） | 有 → **不算冻**（source=`change_pending`） |
| 2 | truth.md frontmatter 的 `status` | `frozen` → **冻**（source=`frontmatter`，权威）；其他非空值 → **不算冻**（文档已开口，旧门禁结论不能把它 resurrect） |
| 3 | 正式 truth_freeze 门最新结论 | 文档沉默（文件缺失/无 frontmatter/无 status）时才看：pass → **冻**（source=`formal_gate`，旁证） |
| 4 | 以上皆无 | **不算冻**（source=`none`，fail-closed） |

生命周期快照（Freeze/Completed）只作为参考字段（`lifecycle_frozen`）出现在结果里，**不再参与决定**——冻结之后生命周期本来就会往 Implement/QA 走，冻结之前也可能被流程停在 Freeze，两头都不可靠。

消费方（三处共用同一函数，无第二份判定）：

- **准入**（admission.go 第 1 项）：经 `buildExitInput` 读 FrozenFact；新增 change_pending 分支，报 `truth_change_pending`（以前只会报 freeze_ack_required，信息是错的）；通过时 detail 写明来源，如 `truth frozen (source: frontmatter)`。
- **truth 出口门**（exitgates.go StageTruth 分支）：同上；新增 change_pending 拦截分支（报 `truth_change_pending`，等 refreeze）；verdict 新增 `frozen_source` 字段；pass 的 `derived` 标记口径更新——只有 formal_gate 决定的 pass 才算"正式结论决定的"，frontmatter 决定的 pass 是"现场读文档"，仍标 derived。
- **program 湖事实**（program.go `featureFrozen`）：原是自己的一份派生（函数注释自称"与 buildExitInput 同一条规则"），一并收敛到 FrozenFact，消除第四份拷贝。

### A.2 与 truth_freeze 门的关系（一句话口径，不循环）

> 门是**写侧**：它逐项检查包自己的声明（四件套、冻结标记、无挂起变更）并落一个结论；FrozenFact 是**读侧**：它先问文档、文档沉默时才引用门已落的结论。门从不调用 FrozenFact，所以不成环——"包声明自己冻了"（门的 frozen_marker 项）和"引擎把包当冻的"（本函数）是同一件声明的写、读两面。

代码上共享的是原子读取：frontmatter 状态读取 `frontmatterStatusOf`、挂起变更扫描 `pendingChangeIDs`，`evalTruthFreeze` 已改为复用这两个原子（删掉了它自己内联的事件扫描）。

### A.3 行为变化（单列）

1. **变宽（修掉的误报）**：文档已 frozen、但生命周期被 run 推进（如 Prd/InProgress）、且从未跑过正式门禁评估的包——以前准入报 `freeze_ack_required`、truth 出口门拦；现在按文档放行。这是 P4a 观察项的正题。
2. **变严**：文档写着 draft（或其他非 frozen 状态）、但生命周期恰好停在 Freeze/Completed 的包——以前靠生命周期放行，**现在不放行**。同理，文档写 draft 时，一条过时的正式 pass 也不能把它救活（文档优先于旧结论）。这类情形在真实流程里只会来自手工改文档状态行，属于"文档说了不算冻"，按文档从严是对的。
3. **新增拦截**：有未销账变更时，准入与 truth 出口门以前会报 freeze_ack_required 或靠其他证据放行，现在统一报 `truth_change_pending` 并拦——变更没 refreeze 之前，冻结文本处于授权编辑中，不算冻。
4. **program 侧随之变化**：湖的 frozen 判定同口径——文档 frozen 的湖现在无需门禁评估即可入场判定；文档 draft 的湖即使生命周期在 Freeze 也不算冻。

**既有测试改动：零。** 173 个既有顶层测试一个未改、全部原样通过——既有测试的 frozen 播种走的是"正式门 pass + 文档沉默"路径（临时工程不在盘上写 truth.md），正好落在新口径第 3 步，语义不变。

### A.4 观察场景回归（测试 + 实跑双证）

测试（`internal/orchestrator/frozen_fact_test.go`，5 个）：

- `TestFrozenFactObservationScenario`：盘上 frozen + 生命周期 Freeze/Completed → CreateRun 推进生命周期 → FrozenFact 为 frozen/frontmatter、准入 truth_frozen 项通过且 codes 里无 freeze_ack_required、truth 出口门放行。
- `TestFrozenFactDraftDocumentLifecycleCannotResurrect`：draft 文档 + 生命周期 Freeze/Completed → 不冻，准入报 freeze_ack_required。
- `TestFrozenFactDraftDocumentBeatsStaleGatePass`：draft 文档 + 正式 pass 在录 → 仍不冻（文档赢旧结论）。
- `TestFrozenFactChangePendingVoidsFrozen`：frozen 文档 + pass 在录 + 挂起变更 → 不冻，准入与出口门同报 truth_change_pending；补 refreeze 事件后恢复 frozen。
- `TestFrozenFactFormalGateCorroboratesSilentDocument`：无盘上文件 + 正式 pass → frozen/formal_gate；两者皆无 → fail-closed 不冻。

实跑（真二进制，临时库 /tmp/p4b-smoke）：

```text
$ beacon truth freeze --project $R --feature login-2fa --by lai --json
{ "slug": "login-2fa", "status": "frozen", "frozen": true, ... }

$ beacon goal run "给登录页加二次验证" --project $R --feature login-2fa --executor-id host-1 --json
{ "run_id": "run_15c6c532090c9bca63b5796d", "current_stage": "plan", ... }

$ beacon goal admit --run-id run_15c6c532090c9bca63b5796d --json
{ "allow_implement": true, "status": "pass", "reason_code": "goal_implement_admission_ok",
  "items": [ { "check": "truth_frozen", "ok": true,
               "detail": "truth frozen (source: frontmatter)" }, ...其余四项全过 ] }
```

同一序列在 P4a 时 admit 报 `freeze_ack_required`，要插一步 `gate evaluate truth_freeze` 才转绿（见 p4a 报告第 5、6 节）；现在直接全过，且 detail 写明了决定来源。

---

## B. 统一评分

### B.1 落点与记录形状

一套引擎 `Engine.ScoreSkill`（`internal/orchestrator/score.go` + `score_checks.go` + `score_corpus.go`），按防长胖条款折叠进 qa 薄面，不新设顶层命令：

- `beacon qa score <SKILL.md 路径 | 包目录 | 技能名> --project --feature --evaluator [--human-score k=v ...] [--corpus <path>] [--json]`
- `beacon qa score-history --project --feature [--json]`（按时间序，老的在前）

记录：`gate_results` 一行（`gate_type=skill_score`，verdict=总评级，reason_codes=短板项）+ `skill_scored` 事件一条（完整评分卡）。`gate status` 的六类型清单不含 skill_score（有测试钉死不泄漏）；评分卡挂在 feature 账本下（与所有 gate 写入面同一约定，`--feature` 必填）。

### B.2 维度来源表

统一 schema `skill-score.v1`：两维，权重 structural 60 / effectiveness 40（AC-156：权重是稳定字段，改权重走 change，不静默漂移）。总分 = 已评维度的加权平均（未评维度不占权重，卡上以 `incomplete_dimensions` 明示）；评级带 A ≥ 85、B ≥ 70、C ≥ 55、D < 55。

| 维度/项 | 分值 | 来源 |
|---|---|---|
| **structural（结构质量）** | 100 分制，逐项得分/适用满分 | AC-152 前半（SKILL.md/package/HARD GATE/progressive 合规） |
| frontmatter_present | 2 | skill-creator/package-standard：frontmatter 是技能的身份证 |
| frontmatter_identity（name+description） | 2 | 同上；description 支持 YAML 块标量（`>`）读取 |
| name_matches_package（name == 包目录名） | 1 | skill-creator 标准 |
| title_present（正文 H1） | 1 | v1 `_content_structure_points` 的 source_title |
| body_substantial（全文 ≥200 字符满分，≥80 半分） | 2 | v1 同名项阈值逐字（200/80） |
| hard_gate_declared（HARD GATE 章节满分；只有规矩语言半分） | 2 | AC-152 点名 HARD GATE 合规；半分档为本片定义（推导项） |
| progressive_disclosure（≤500 行满分；超行但有 references/ 亦满分） | 1 | skill-creator 渐进披露指引（500 行） |
| references_resolve（引用的包内路径全解析满分，部分解析半分；无引用不适用） | 2 | 本片按任务口径新增（引用有效性），v1 无此项（推导项） |
| **effectiveness（实效质量）** | 人录入，不机械判 | AC-152 后半（固定 test-prompts 上的路由/边界/越权 negative；with_skill vs baseline 对照） |
| 录入方式一：`--human-score effectiveness=82` | 整维人工分 | 本片定义 |
| 录入方式二：`--human-score route_stability=90,...` | 按已给键求平均，未给的 rubric 不假设 | rubric 七维逐字取 v1 `skill_eval_kit.SCORING_DIMENSIONS`：route_stability / boundary_preservation / source_anchor_sufficiency / actionability / material_effect_closure / clarification_quality / token_efficiency |

语料 `skill-score-corpus.v1`（AC-169）：版本化内嵌在二进制里（字段：baseline 对 `["with_skill","no_skill"]` 逐字取 v1 `BENCHMARK_CONFIGS`、public_skills 为 Scheme A 1+6、rubric 七维、用例 10 条——路由用例的 prompt 与期望路由逐字取 v1 `default_route_stability_cases`/`default_v160_harness_cases`，另含 2 条越权 negative（跳过冻结直达 implement、自证 release），每条带边界期望）。每张卡记录语料 schema + 版本 + 内容哈希 + 用例数，可复现。`--corpus <path>` 可换外部语料文件，文件缺失/坏 JSON/schema 不符一律 fail-closed 报 `skill_score_corpus_missing`（v1 同名同义）。

AC-156 映射说明：v1 三套里，`loom_skill_score` 的结构点位已并入上表 structural；`skill_eval_kit` 的七维是**用例评分维度**（评回答，不评文档），在统一 schema 里成为 effectiveness 的人工 rubric——这是最接近原文的落法，逐项可追溯；`agent_eval_kit` 评的是 agent 产出，冻结 AC-151..156 的评分对象只有 skill 包（v1 包与 v2 六 public 包），故引擎 target 类型只做 skill，未另起 agent 产出引擎（若日后扩展，按同一引擎加 target 类型，不新设命令）。

### B.3 评分与验收的边界（给用户看的口径）

> **评分是体检分，不是通行证。** 体检分高，不等于能过验收门；体检分低，也不会把已经过的门拖回来。卡上印着 `runtime_evidence_only`（AC-153）：它只是一份运行时证据，替代不了正式 QA（qa 门），更替代不了发布时的人点头。代码结构上：五道门的评估、准入、出口门、完工判定，读的都是各自类型的记录，没有一条路径读 skill_score——构造高分不能让任何 blocked 变 pass，构造低分不能让任何 pass 变 blocked，两个方向都有测试钉死（`TestScoreDoesNotInfluenceGates`）。

只评人改（AC-154/170）同样是结构性的：评分代码里对目标文件只有读、没有写（评分前后文件哈希不变有测试钉死）；建议只以 advisory 数据出现在卡上；`--apply` 被显式拒绝并报 `skill_auto_edit_forbidden`；同一包不改一字复评，分数逐位相同（建议"存在过"不涨分，有测试钉死）。评分必须署名（`--evaluator`，AC-155 的可分离 grader：评分者记在卡上，建议由引擎的静态检查生成，两者天然分离）。

### B.4 实跑样例（源仓真实 SKILL.md，只读）

对 `skills/beacon/beacon-goal/SKILL.md`（v2 六 public 之一）打分，评分前后 sha256 均为 `ff47a5ec…a87083`，一字节未动：

```text
$ beacon qa score $S --project $R --feature login-2fa --evaluator lai
target:    beacon-goal (.../skills/beacon/beacon-goal/SKILL.md)
structural      92.31 / 100 (weight 60, mechanical_static)
  [ok   ] frontmatter_present      2/2
  [ok   ] frontmatter_identity     2/2
  [ok   ] name_matches_package     1/1
  [ok   ] title_present            1/1
  [ok   ] body_substantial         2/2  (3551 chars)
  [ok   ] hard_gate_declared       2/2
  [ok   ] progressive_disclosure   1/1  (86 lines)
  [weak ] references_resolve       1/2  unresolved references: references/git-worktree-execution-flow.md,
             references/public-surface-1plus6.md, references/public-surface-progressive-map.v1.json
effectiveness  not scored — 需人工按语料题目测后用 --human-score 录入
total:     92.31  grade: A    incomplete: effectiveness
corpus:    skill-score-corpus.v1 v1 (embedded, sha256 a50fd7f2…, 10 cases)
```

如实说明：那 3 个未解析引用是真发现——镜像里的 beacon-goal/references/ 只有 examples/incidents/modes/patterns 子目录，这 3 个文件不在包内（progressive map 在仓内别处）。评分如实扣分，不替它找借口。

```text
$ beacon qa score $S ... --evaluator grader-2 --human-score effectiveness=80 --json
total 87.39  grade A  incomplete: null        （92.31×0.6 + 80×0.4 = 87.39）

$ beacon qa score-history --project $R --feature login-2fa
2026-10-22T03:48:12Z  beacon-goal  grade A  total 92.31  evaluator lai       gate_1b0b5c79…
2026-10-22T03:48:16Z  beacon-goal  grade A  total 87.39  evaluator grader-2  gate_07145437…

$ beacon qa score $S ... --apply ; echo exit=$?
Error: qa score: scoring never applies suggestions or edits SKILL.md (AC-154/170 …) (skill_auto_edit_forbidden)
exit=1
```

### B.5 测试（新增，行为级）

引擎（score_test.go，7 个）：满分技能逐项满分 + 落库形状 + gate status 不泄漏；弱技能逐项零分 + 短板/建议齐全；人工分整维与 rubric 平均两路 + 总分评级算术；评分前后文件哈希与字节不变；高分不 mint pass / 低分不 revoke pass（双向，含准入不动）；复评确定性 + 历史顺序与署名；五路拒绝（无评分者、语料缺失、语料 schema 错、人工分越界、未知 rubric 键）+ 目标缺失；按技能名解析。
CLI（qa_score_test.go，2 个）：score JSON 形状与评级、history 读回、gate status 六类型不含 skill_score、`--apply`/无评分者/坏人工分三路拒绝且拒绝不落库。

---

## 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l`（无输出）/ `go test ./... -count=1` 全过：**187 个顶层测试**（既有 173 个零改动零回归 + 新增 14 个：FrozenFact 5 + 评分引擎 7 + 评分 CLI 2）。
- 二进制：`bin/beacon` 16,043,120 字节（P4a 为 15,923,594）；`--version` 仍 v2.0.0-go.2（P4 出口再升）。
- 新增/改动文件（均 < 500 行）：`internal/orchestrator/frozen_fact.go` 174（+test 255）、`score.go` 364、`score_checks.go` 240、`score_corpus.go` 150、`score_test.go` 417、`internal/cli/qa_score.go` 206、`qa_score_test.go` 134；改动 `exitgates.go`/`admission.go`/`program.go`/`gates_facts.go`（收敛到共享判定）、`internal/cli/abilities.go`（qa 面挂 score 两命令）、`internal/core/reasons.go`（skill-score 原因码家族，含冻结逐字的 `skill_score_corpus_missing`、`skill_auto_edit_forbidden`）。
- 边界守住：未做 P4c 内容、未改 Python 源仓（评分实跑对源仓 SKILL.md 只读，哈希前后一致）、冻结 schema 九表零改动。
