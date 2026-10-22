# Beacon Go 重构 · P2d 实现报告（编排扩展 + P2 收口，版本 v2.0.0-go.1）

> 日期：2026-10-22 ｜ 范围：口径稿第 2 节 G-12/G-13/G-14/G-15/G-16 + P2 出口（版本号、收口总表、出口三件套）
> 依据：源仓 `~/workspace/repos/beacon/` 源码实测（只读未改）；已交付的 p2a/p2b/p2c 报告与 [scope-charter.md](scope-charter.md)。
> 写法：按用户规则用大白话。必要术语第一次出现时一句话解释。

---

## 0. 这一期做了什么（一句话版）

goal 的最后五块拼图装上了：**大活拆湖编排（program）、急活快车道（hotfix）、跨版本切段（multi-version）、任务记忆（memory）、主轴（axis）**。同时 P2 正式收口：版本号从开发版 `v2.0.0-go-dev` 升为第一个正式版 **`v2.0.0-go.1`**（按口径稿第 5 节维护约定，第一个稳定版对应 P2 出口）。

状态仍然只进全局数据库那一份账本：本期没有加任何新表、没有写任何自带的状态文件，冻结的九表结构一个字没动。

---

## 1. 逐 G 落地表

| # | 工作流 | 落地内容（大白话） | 源 → Go 对应 | 测试 |
|---|---|---|---|---|
| G-12 | program 编排 | 大目标拆成几个湖：每湖记下目标、依赖谁、碰哪些面；依赖图先验（有环、依赖不存在、自己依赖自己、重名都当场拒绝）；每湖入场逐个判（需求包在不在、真相冻没冻、依赖的湖完没完或入没入场）；总调度只读视图按依赖顺序给出下一批能开工的湖和每个湖的司机启动提示；计划冻结后不许直接改湖清单，必须走带说明的 change 记录 | `program_graph.py` → `ValidateProgramGraph`/`TopoOrder`（纯函数）；`program_admission.py` → `ProgramAdmission`/`judgeLakeAdmission`；`program_orchestrator.py` → `ProgramRoute`；`program_freeze_gate.py`+`program_change_intent.py` → `ProgramPlan` 冻结拒绝 + `ProgramChange`；命令面 `commands/program.py` → `goal program plan/admit/route/change/freeze` | 图校验五类问题、拓扑序、冻结拒绝+change 路径、逐湖入场四种结局、route 拓扑与完成后再派生 |
| G-13 | hotfix 快车道 | 急活直接出修复计划：从哪个版本修、修哪几个点、往哪个版本同步、怎么回滚——回滚方案没写就不给计划；发布前再校验一遍（计划完整、回滚声明可执行、验收门是 pass）；计划里写死「发布必须人点头」，完工路径一点没放宽 | `prod_ops_v168.py` `build_hotfix_plan` → `BuildHotfixPlan`；`validate_hotfix_for_release` → `ValidateHotfixForRelease`；`goal.py hotfix-plan` → `goal hotfix-plan build/validate` | 缺回滚/缺基线/缺同步目标/缺修复点四路拒绝、校验缺 qa pass 拒绝、补声明后通过、校验通过也不能让 Complete 自己过 |
| G-14 | multi-version plan | 一个目标跨几个版本时按版本切段：版本号必须递增；上一段没干完的活，必须明确被下一段接走或明确写「丢弃」，不许悄悄丢活；最后一段没干完的同样要交代 | 运行时 `plan_multi_version` / `multi_version_intent.build_multi_version_execution_plan` → `PlanMultiVersion`；`goal.py multi-version-plan` → `goal multi-version-plan` | 承接缺口检出（3 处）、全交代时 valid 且 carried_in 正确、版本不递增/不可解析/依赖后段三路拒绝 |
| G-15 | goal memory | 任务跑过的事件、产物指针、存档点能重新索引、按词搜回来复用；搜索结果每条都盖「只做参考」的戳，结构上不存在拿搜索结果当完工证据的路径（完工和准入的代码根本不读 memory） | 运行时 `goal_memory_reindex/search` + 监工的 `memory_reindex/search`（FTS5）→ `MemoryReindex`/`MemorySearch`（倒排索引）；`goal.py memory` → `goal memory reindex/search` | 索引后搜得到且带 reference_only、搜不到返回空、索引前搜返回空形态、轨迹里写满「已完成」字样也不能让未完工的 run 完工/过准入 |
| G-16 | axis 主轴 | 给大任务生成一条主轴说明：目标一句话、各阶段按什么顺序走、每阶段「算干完」的判据、湖按依赖顺序排；校验能查跑偏：阶段对不上、湖多了少了、湖的顺序和依赖图冲突、引用的湖根本不存在，逐项报 drift | `goalrun_main_axis.py` `generate_main_axis/validate_main_axis` → `GenerateMainAxis`/`ValidateMainAxis`；`goal.py axis` → `goal axis generate/validate` | 生成+校验干净通过、改湖清单后报 lake_missing/lake_extra、加依赖后报 order_conflict、无主轴时 validate 给编码错误 |

### 1.1 program 的存储论证（为什么不加表）

program 需要一个「家」：计划本身、冻结状态、入场判定记录、变更记录。结论是**挂在一个宿主 goal run 上**，理由和 P2b 队列的论证同一条线：

- 冻结九表没有 program 表，加表就破 AC-067..075 的结构合同，本期边界明确不动冻结结构；
- runs 表本来就是一切编排状态的锚：计划进宿主 run 的 `metadata_json`（湖清单、依赖边、冻结标记、每湖的入场判定、变更记录都在里面），动作进 `lifecycle_events` 事件轨迹（`program_plan_created/frozen/changed`、`program_admission_evaluated`）；
- 湖和 program 的绑定规则只有一条：**湖 = 宿主 run 所在项目版本里的需求包（feature slug）**。所以不需要额外的绑定表，查湖就是按 slug 查同版本的包；
- 每个湖的进度（完没完、有没有它自己的 goal run）**不缓存第二份**：route 每次都从各湖自己的生命周期和 run 实时派生，和口径稿「状态只有一个账本」一致；
- program 不配自己的司机：G-06 的 driver 一次推一个湖的 goal run，route 给每个湖输出等价的 driver 启动命令作为提示（源仓 `program_driver.py` 的逐湖 drive 就是这个形状）。

一个口径分界（开发中定的，写在这里备案）：**入场判定和能不能开工是两把尺**。入场看「依赖湖已完成**或已入场**」（任务口径，文书链可以先通）；route 的「可启动」只认依赖湖**已完成**（源仓 orchestrator 的 `await_upstream_lake_closure` 就是这个规矩）。所以一个湖会出现「已入场但 route 显示 waiting」——不是矛盾，是还没轮到它真开工。

### 1.2 memory 的存储论证（FTS5 实测结果）

源仓用的是 SQLite 的全文检索（FTS5）建一个每 run 的小数据库文件。Go 侧先实测：**modernc.org/sqlite v1.60.1（内嵌 SQLite 3.53.4）支持 FTS5**——建虚拟表、插入、MATCH 查询都实测通过。但本期不用它，两个原因：

- FTS5 虚拟表是新的数据库结构对象（还带影子表），建在全局库里就动了冻结九表结构；
- 学源仓另开一个小数据库文件，则违反口径稿第 7 条（状态只存全局库一份，不许自带状态文件）。

所以用最小替代：**倒排索引**（把每个词对应到哪些条目，查时取交集）存进 run 的 `metadata_json`，索引源只认库里的三样东西——该 run 的事件、产物指针、存档点，不读任何外部文件。索引是派生物，随时能从权威表重建，丢了不心疼。若将来要上 FTS5，走一次结构变更流程再说。

---

## 2. 语义差异 / 推导清单（逐项，不藏）

1. **hotfix 出计划比源仓严**：源仓缺基线时自动拿当前发布线顶上、缺同步目标时默认 main，计划照样出、只是标 blocked。Go 版按本期任务口径：基线、同步目标、修复点、回滚方案缺任何一样**直接拒绝出计划**、逐项给原因码（前三个原因码与源仓同字面：`hotfix_baseline_missing`、`hotfix_forward_port_required`、`hotfix_rollback_notes_required`；`hotfix_fix_points_required` 是新码，源仓没有这个检查）。理由：能执行的计划才叫计划，猜出来的基线更危险。
2. **hotfix 发布校验多了两项**：源仓 `validate_hotfix_for_release` 只查计划三字段。Go 版按任务口径加「回滚声明可执行」（`hotfix_rollback_not_executable`，新码）和「该 run 的 qa 门最新判定 = pass」（复用既有 `qa_gate_not_passed`）。源仓要求的六道门（`HOTFIX_REQUIRED_GATES`）原样记在计划里，其中 human_release 永远不由校验勾掉。
3. **multi-version 从「解析一句话」改成「显式切段」**：源仓从用户一句话里猜版本和湖（带双轨、loom 入口等历史包袱）。Go 版按任务口径收成显式输入：段 = 版本号 + 该段活清单 + 段间依赖，承接检查（上一段未完成项必须被接走或明确丢弃）是源仓没有显式做、任务要求补上的，原因码 `multi_version_carry_missing` 为新码。双轨/品牌字段不迁（口径稿拍板第 1 条）。
4. **program 入场判定的「冻没冻、完没完」不另造口径**：冻 = 与出口门同一派生（生命周期 Freeze/Completed 或 truth_freeze 门 pass）；完 = 生命周期 Release/Completed 或该湖有 run 成功。源仓是读文件 frontmatter 和证据文件，Go 版按 DB-first（AC-015）换成库内派生，与 P2a 出口门的换法一致。
5. **program plan 替换时入场记录的去留**：未冻结的计划重排，仍在清单里的湖保留其入场判定；走 change 换清单则**清空全部入场判定**——结构变了，旧判定作废重判（源仓结构变更后重跑 freeze gate 是同一个道理）。
6. **axis 的判据是「转述」不是第二套规则**：主轴里每阶段的「算干完」一句话，转述的是已有门面的规则（出口门、准入、Complete 四条件）。axis 只描述和查漂移，本身不判任何东西能不能过——校验发现 drift 也只是报告，不拦不放。
7. **axis 不再产文件**：源仓生成 markdown + machine json 两个文件。Go 版按状态唯一账本口径只存 run 快照 + 事件，不落文件；要看主轴用 `goal axis generate/validate --json` 或 `goal status`。
8. **命令动作拆成子命令**：源仓 `goal memory <动作>`、`goal axis <动作>` 是单命令带动作参数，`hotfix-plan` 是建+验一体。Go 版按 P2b/P2c 已立的形状拆子命令（`memory reindex/search`、`axis generate/validate`、`hotfix-plan build/validate`），动作词与源仓一致。
9. **新原因码都在 core 统一词表登记**：program 家族（图结构类为新拼写，源仓散在 payload 错误列表里）、hotfix 补码、multi-version 家族、axis 家族（`main_axis_artifact_missing`/`main_axis_validation_failed` 与源仓同字面）。没有散落在各文件里的自创字符串。

---

## 3. 实跑样例（真二进制，v2.0.0-go.1）

环境：`BEACON_STORE_PATH=/tmp/p2d-smoke/beacon.db`，先 `projects register` 建项目，再用 sqlite3 种 4 个需求包（宿主 feat-a + lake-a/b/c）+ truth_freeze 门 pass + 宿主 qa 门 pass（需求包的创建界面属 truth 面，P4 才有；此处仅为冒烟种子，种子写生命周期时注意枚举大小写 `Release`/`Completed`）。

```text
$ beacon --version
Beacon CLI v2.0.0-go.1

$ beacon goal run "ship the platform" --project /tmp/p2d-smoke/proj --feature feat-a --executor-id host-1 --json
{ "run_id": "run_5ecbd2597eaa788753b89f7d", "status": "running", "current_stage": "plan",
  "loop_stage": "governance_precheck", "stage_plan": [plan, truth, implement, qa, release], ... }

$ beacon goal program plan --run-id <R> --program platform --goal "ship the platform" \
    --lake lake-a --lake lake-b --lake lake-c --depends lake-b=lake-a --depends lake-c=lake-b --freeze
program: platform (frozen=true)
lakes:   lake-a, lake-b, lake-c
changes: 0

$ beacon goal program plan --run-id <R> --program platform --lake lake-a     # 冻结后直接改
Error: program.Plan: program plan is frozen; record a change with goal program change instead (program_plan_frozen)

$ beacon goal program admit --run-id <R>
lake lake-a           admitted=true
lake lake-b           admitted=true
lake lake-c           admitted=true
all admitted: true

$ beacon goal program route --run-id <R>
program: platform (frozen=true, all_complete=false)
  lake-a           admitted
    drive: beacon goal run --feature lake-a (then: beacon goal driver start --run-id <lake run>)
  lake-b           waiting    program_dependency_unclosed
  lake-c           waiting    program_dependency_unclosed
next batch: lake-a

# lake-a 的生命周期推到 Release/Completed 后，route 实时重派生：
$ beacon goal program route --run-id <R>
  lake-a           completed
  lake-b           admitted
    drive: beacon goal run --feature lake-b (then: beacon goal driver start --run-id <lake run>)
  lake-c           waiting    program_dependency_unclosed
next batch: lake-b

$ beacon goal hotfix-plan build --run-id <R> --description "login 500s in prod" \
    --from-version v1.6.8 --target-version v1.6.9 --fix-point "nil guard in session loader"
Error: goal.BuildHotfixPlan: hotfix without rollback notes: ... (hotfix_rollback_notes_required)

$ beacon goal hotfix-plan build --run-id <R> ... --rollback "redeploy the v1.6.8 tag" --rollback-executable
hotfix plan for run <R>: v1.6.8 -> v1.6.9, 1 fix point(s), rollback recorded, human release required: true

$ beacon goal hotfix-plan validate --run-id <R>
hotfix release validation: ok=true blocked=false codes= (human release required: true)

$ beacon goal multi-version-plan --run-id <R> --segment v1.7.0=alpha,beta --segment v1.8.0=beta,gamma
v1.7.0: items=alpha,beta completed= dropped= carried_in=
v1.8.0: items=beta,gamma completed= dropped= carried_in=beta
problem: multi_version_carry_missing — segment v1.7.0 leaves "alpha" unfinished: ...
problem: multi_version_carry_missing — segment v1.8.0 leaves "beta" unfinished: ...
problem: multi_version_carry_missing — segment v1.8.0 leaves "gamma" unfinished: ...
valid: false        （命令 exit 非 0）

$ beacon goal memory reindex --run-id <R>
indexed 6 entries for run <R> (authority: support_advisory, reference only)

$ beacon goal memory search --run-id <R> -q "hotfix rollback"
status: empty hits: 0 (authority: support_advisory, can_complete: false)
# 正确为空：rollback 原文只在计划快照里，不在事件/产物/存档点三类索引源内

$ beacon goal memory search --run-id <R> -q "hotfix" --json
{ "status": "ok", "query": "hotfix", "authority": "support_advisory",
  "reference_only": true, "can_complete": false,
  "hits": [ { "source_kind": "event", "label": "hotfix_plan_created",
              "snippet": "hotfix_plan_created {\"fix_points\":[\"nil guard in session loader\"], ...}",
              "reference_only": true } ] }

$ beacon goal axis generate --run-id <R>
axis for run <R>: ship the platform
  plan       stage plan recorded; plan exit gate passes
  truth      truth package frozen (...)
  implement  implement admission passes; artifacts registered and stage evidence harvested
  qa         qa gate verdict = pass
  release    verifier attached and human release confirmation recorded (Complete)
lakes: lake-a -> lake-b -> lake-c

$ beacon goal axis validate --run-id <R>
axis ok: true

$ beacon goal program change --run-id <R> --note "descope lake-c" \
    --lake lake-a --lake lake-b --depends lake-b=lake-a
program: platform (frozen=true)
lakes:   lake-a, lake-b
changes: 1

$ beacon goal axis validate --run-id <R>
axis ok: false
  drift lake_extra: axis lake lake-c is not in the program plan
Error: goal axis validate: main axis drift detected (main_axis_validation_failed)
```

---

## 4. 验证（出口三件套之一：全量测试）

- `go build ./...` / `go vet ./...` / `gofmt -l .`（无输出）全干净。
- `go test ./... -count=1` 全过：**103 个测试**（P2c 收口时 90 个，零回归；本期新增 13 个：orchestrator 12 个 + CLI 全链路 1 个）。
- 本期覆盖点逐项有测试：图环检出 ✓、依赖不存在/自依赖/重名拒绝 ✓、逐湖入场判定（冻/未冻/无包/依赖未闭四种结局）✓、route 拓扑顺序与下一批 ✓、冻结后改清单被拒且 change 路径可用 ✓、hotfix 缺回滚拒绝 ✓、hotfix 校验缺 qa pass 拒绝 ✓、multi-version 承接缺口检出 ✓、memory 索引后搜得到且带 reference_only ✓、memory 命中「已完成」字样也不能让未完成 run 完工/过准入 ✓、axis 与 plan 不一致报 drift（含 order_conflict）✓。
- 版本残留检查：全仓 `.go` 文件搜 `go-dev` 无结果（版本常量唯一处 `internal/cli/root.go` = `2.0.0-go.1`，`--version` 输出 `Beacon CLI v2.0.0-go.1` 已实跑确认）。

---

## 5. P2 收口总表（G-01..G-22 逐条对账）

状态口径：**已落地** = 行为已实现且有测试；**部分落地** = 主体已落地、个别子面未单设（逐项写明）；**派活面已落地，真集成留 P3** = 口径/打包/收活已实现，真正执行（外部执行器、真工位）按冻结 phase 图在 P3。

| # | 工作流 | 最终状态 | 落地期 | 交代 |
|---|---|---|---|---|
| G-01 | 主流水线分阶段推进 | 部分落地 | P1/P2a | run/status/阶段序/分阶段 skill 表已落地；子命令 `entry-contract`（入口合同面）未单设，列 P3 补齐候选 |
| G-02 | tick / continue 推进循环 | 已落地 | P2a | 含 tick 幂等 |
| G-03 | stage exit gates 出口检查 | 已落地（判定） | P2a | 判定为只读派生；判定结果写回 gate_results 是 P3 的 gate 评估写库 |
| G-04 | implement admission 准入 | 已落地 | P2c | 五项共享检查器，tick/delegate/admit 三路共用；工位全模式（真 worktree 绑定校验）随 P3 真工位对齐 |
| G-05 | blocker + verifier + complete 闭环 | 已落地 | P2a | complete 四条件比源仓严（按冻结真相） |
| G-06 | driver 后台长跑 | 已落地 | P2b | 租约防双开（启动层排他，边界见 p2b 报告） |
| G-07 | durable supervisor 看护 | 已落地 | P2b | 含 longrun-doctor 的 per-run 体检口径 |
| G-08 | heartbeat / checkpoint / resume 断点恢复 | 部分落地 | P2a | heartbeat/checkpoint/resume（决策面）已落地；源仓独立的 `resume-pack` 子命令未单设，resume 的决策视图承担其核心（下一步动作 + 技能 + 中断重放），完整 digest 恢复包列 P3 视需要补 |
| G-09 | discovery queue 消费 | 已落地 | P2b | 借 runs 表存队列项，九表未加表；防冒充完成双保险 |
| G-10 | delegate-ralph 实现外包 | 派活面已落地，真集成留 P3 | P2c | 打包/收活/防冒充已落地且比源仓严；ralph 执行器本体 P3 接 |
| G-11 | parallel lakes + main-star/satellite | 计划面已落地，真工位留 P3 | P2c | 只出计划（planned-only 处处标注），不真建 worktree |
| G-12 | program 编排 | 已落地 | P2d | 图+入场+调度+冻结/change 已落地；program 司机按口径并入 G-06（route 给逐湖 driver 提示），不另造司机 |
| G-13 | hotfix 快车道 | 已落地 | P2d | 出计划比源仓严（见差异 1/2） |
| G-14 | multi-version plan | 已落地 | P2d | 显式切段 + 承接检查（见差异 3） |
| G-15 | goal memory | 已落地 | P2d | 倒排索引替代 FTS5（论证见 1.2）；结构性非权威 |
| G-16 | axis 主轴 | 已落地 | P2d | 生成/校验已落地，不产文件（见差异 7） |
| G-17 | trace 轨迹 | 已落地 | P2a（底座 P0） | 事件聚合只读面；bundle 的文件导出形态未做，`goal trace --json` 即查账面 |
| G-18 | design-stage 设计阶段 | 已落地（绑定面） | P2c | 只管绑没绑；原型生成走宿主桥接，桥接本体 P3 |
| G-19 | self-heal 自愈 | 已落地 | P2b | pre/mid 自动项真执行，post 装机永远只出计划待人确认 |
| G-20 | loop-goal 状态机 | 已落地 | P2a（core） | 12 态 + release 人工确认 + repair 上限 + intent/决策记录 |
| G-21 | 护栏组（预算/配额/转向/待办） | 已落地 | P2b | 预算只做轮次+时长（不编 token 数）；reason codes 统一词表在 core |
| G-22 | implement 三条 lane | 口径与派活已落地，真集成留 P3 | P2c | 选路/交接校验/统一收活已落地；三 lane 的真执行器 P3 接 |

没有一条无交代：未单设的子面只有两处（G-01 的 entry-contract、G-08 的 resume-pack），都写明了现状与去向；其余「留 P3」项全部是冻结 phase 图本来就排在 P3 的真集成（执行器、worktree、gate 写库），不是 P2 欠账。

---

## 6. 交付物（出口三件套之三：文件大小清单）

- 二进制：`bin/beacon` **15,017,114 字节**（P2c 为 14,809,037），`--version` → `Beacon CLI v2.0.0-go.1`。
- 本期新增：`internal/orchestrator/meta_p2d.go` 14,191 B（356 行）、`program.go` 11,699 B（374 行）、`program_route.go` 7,649 B（210 行）、`hotfix.go` 4,842 B（119 行）、`multiversion.go` 5,833 B（184 行）、`memory.go` 6,167 B（199 行）、`axis.go` 5,621 B（156 行）、`internal/cli/goal_program.go` 9,892 B（292 行）、`internal/cli/goal_p2d.go` 12,928 B（385 行）；测试 `p2d_program_test.go` 8,990 B、`p2d_test.go` 11,109 B、`internal/cli/goal_p2d_test.go` 5,886 B。单文件均 <500 行。
- 本期改动：`internal/core/reasons.go` 9,707 B（P2d 原因码家族）、`internal/orchestrator/meta.go` 9,461 B（runMeta 加 P2d 五个字段）、`engine.go` 12,953 B（RunView 加四个面）、`orchestrator.go` 1,068 B（包注释更新为 P2 全貌）、`internal/cli/goal.go` 16,216 B（注册 P2d 命令组）、`internal/cli/root.go` 2,931 B（版本号 → `2.0.0-go.1`）。
- 文档：本报告；`docs/scope-charter.md` 第 2 节末尾的落地汇总段已同步为 P2 收口状态（只改该段，正文与拍板记录未动）。
- 边界守住：未接真 agent 后端、未真建 worktree、未做 gate 四道门评估写库、未改 Python 源仓、冻结九表零改动。
