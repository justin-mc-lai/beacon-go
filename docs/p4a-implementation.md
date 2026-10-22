# P4a 实施报告：六能力面收敛 + truth 能力补真

> 日期：2026-10-22 ｜ 版本：v2.0.0-go.2（不变，P4 整体出口再升版）｜ 状态：完成，173 个测试全过（既有 159 个零改动、零回归；本片新增 14 个）
> 依据：口径稿 [scope-charter.md](scope-charter.md) 第 3、4 节；冻结包 `beacon-v2-rust-kernel-redesign` truth.md 六能力面。评分（AC-151..156）不在本片，留 P4b。

## 1. 做了什么（大白话）

这一片补上了一个真空：**Go 版以前根本没法「生」出一个需求包**——包只能靠 P3b 从旧版搬进来，或者手工往库里塞数据。现在 truth 能力是真的了：一条命令建包（四件套骨架直接落盘）、一条命令冻结（先过完整性预检，不合格当场拦下并逐项说明）、冻结后要改走登记变更（只记账、绝不碰正文）、改完重新冻结销账。

同时把六个能力的门脸收拾成型：以前 truth/design/implement/qa/release 五个顶层命令都是 Phase 0 留下的占位（只会打印一句「还没实现」），现在 truth 是真命令组，其余四个是「薄脸」——只负责把人领到已经落地的真实现门口，判定逻辑一份都没有复制。顺手把 P2 欠的两笔小账还了：goal 的入口合同（entry-contract）和完整恢复包（resume-pack）各有了自己的命令。

## 2. truth 命令口径

### 2.1 包结构来源（不编造字段）

骨架形状逐项对着源仓真实包提炼：

| 骨架部分 | 来源 |
|---|---|
| truth.md frontmatter（slug/version/status/language/domain_required/ux_required/parser_contract/truth_source_model/package_maturity/materials_status/canonical_refs） | v1.6.19 真实包 `beacon-precheck-stage-and-bundle-freshness` 的 truth.md |
| canonical_refs 四键（truth/tests/tasks/evidence） | 冻结 v2 包 `beacon-v2-rust-kernel-redesign` 的 truth.md（v1.6.19 样本只有 prd/user_story/test_case 三键，integrity 的 packageFiles 读 truth/tests/tasks/evidence 四键，按冻结包取四键） |
| tests.md 覆盖矩阵（AC ID \| TC ID \| Layer \| Command \| Assertion，AC 在首列） | 冻结 v2 包 tests.md 第 20 行的权威矩阵（integrity 的 ParseCoverageRows 只认这个布局） |
| tasks.md（frontmatter 带 slug + Task Ledger 复选框 + Boundary） | v1.6.19 样本 tasks.md |
| evidence.md（无 frontmatter，Evidence Index 表 + Boundary） | v1.6.19 样本 evidence.md |

骨架有一条刻意的设计：**刚建出来的包必须能直接通过 block 模式的完整性检查**（这样作者填完内容就能冻结，不会被骨架自带的断裂拦住）。为此 AC-001 在 truth 里定义、在 tests 里有一条硬 TC（真实 Command + Assertion，非 soft）；evidence 索引只用裸文件名点名（truth.md / tests.md），不写完整路径引用——完整引用在 integrity 里算「产物引用」，要求已登记 + 文件在盘，骨架阶段两者都不成立，写了就是自带断裂。作者落下真证据后自行换成完整引用，报告与骨架的 Boundary 行都写了这条。

### 2.2 命令与语义

| 命令 | 做什么 | 写什么 |
|---|---|---|
| `truth create --slug --project [--version]` | 在已注册项目的已记录版本下建包；slug 已登记 → `truth_package_already_exists`；目录已在盘但未登记 → `truth_package_dir_exists`（提示走 migrate，不覆盖）；项目未注册/无可用版本 → 拒绝 | 四件套文件、feature_packages 行（status=draft）、lifecycle Think/NotStarted、`truth_created` 事件 |
| `truth freeze --feature [--by]` | 冻结草稿包（检查清单见 2.3）；已 frozen 且无 pending 时幂等返回（already_frozen，仅当 lifecycle 漂移时才对账回 Freeze/Completed 并留 `truth_freeze_reconciled` 事件）；有 pending change 时拒绝并指路 refreeze | truth.md frontmatter 的 `status:` 一行、package status=frozen、lifecycle Freeze/Completed、`truth_frozen` 事件（payload 带改前原文 sha256） |
| `truth change --feature --summary [--by]` | 对 **frozen** 包登记变更（gen-change）；未 frozen 拒绝（`truth_not_frozen`，草稿直接改正文）；已有 pending 拒绝（`truth_change_already_pending`） | 包目录 `changes/<chg_id>.md` 一份变更文档、package status=change_pending、`truth_change_requested` 事件。**正文一个字节不动**（测试以哈希钉死） |
| `truth refreeze --feature [--by]` | 关 pending 的变更（gen-refreeze）；无 pending 拒绝（`truth_no_change_pending`） | 同 freeze 的写半部；`truth_refrozen` 事件 payload 带被关的 change id 列表 |
| `truth show / list` | 只读 | 数据走既有 DAO 面（ResolveFeature / ListFeatureContexts，即 spine 的数据路径），视图组装是引擎的 `TruthPackage`，没有第二套查询实现 |

包状态的权威顺序：pending 以事件轨迹扫描为准（`truth_change_requested` 之后、最近一次 freeze/refreeze 之前），frozen 以 truth.md frontmatter 为准（与 P3a truth_freeze 门的主口径一致），store 的 package status 是随写动作同步的镜像。事件词汇与 P3a 的 `evalTruthFreeze` 挂起变更扫描完全对齐（change 事件名 `truth_change_requested`、清除事件名含 `truth_frozen`/`truth_refrozen`），所以 change 一登记，truth_freeze 门立刻能看见挂起、refreeze 一落门立刻放行——两边没有第二套约定。

### 2.3 freeze 检查清单（全部复用，无新判定）

1. 四件套齐全且非空（缺/空逐项给 `missing_or_empty_<member>` finding，码 `truth_integrity_package_incomplete`）。
2. Truth Integrity Gate 在 **block 模式**下 findings 为 0——直接调 P3a 的 `Engine.EvaluateIntegrity(ctx, by, "block")`（判定在 `internal/gate` 的 CheckIntegrity，本片一行判定都没新写）。未决 deferral 是 integrity 的 `deferral_pending` finding，自在本条之内，不另设检查。评估照常落 gate_results + 事件（freeze 因此在账本里留下一条正式 integrity 评估）。
3. 通过后才动笔：先算原文 sha256，再改 frontmatter。改法是 `setFrontmatterStatus`——只重写 frontmatter 区内 `status:` 一行（没有就插在 `slug:` 后），区外字节逐字保留（测试用整文比对钉死）。这是全 CLI 唯一允许改需求文件的动作；除此之外任何命令不改正文任何一字。

## 3. 六能力面收敛与对账

### 3.1 薄面怎么薄

design/implement/qa/release 四个顶层组（`internal/cli/abilities.go`）只做路由：

| 薄面 | 子命令 | 复用方式 |
|---|---|---|
| design | status / bind | 与 `goal design-stage status/bind` **同一个构造函数**（各建一份实例） |
| implement | plan / admit / delegate-ralph / receive-result / worktree | 与 goal 下同名命令同一个构造函数 |
| qa | status / evaluate / planner-review | status 与 `gate status` 同一构造函数；evaluate 是把 gate 类型钉死为 qa 的三行包装（判定走 `Engine.EvaluateGate` 本体）；planner-review 与 goal 下同一构造函数 |
| release | evaluate / complete | evaluate 同上钉死为 release；complete 与 `goal complete` 同一构造函数 |

每个薄面的 help 都写明与 goal 的关系：goal 是编排总入口（一句话进来、带着 run 走完全程），能力面是单项工具入口；两边结论必须一致，因为底下只有一份实现——测试 `TestThinFacesAgreeWithGoalAndGate` 把这条钉死（design 状态文档两边逐字相等、admit 两边同码同判、qa 与 gate 的门表 DeepEqual、release 两边同 verdict 同 reason_codes）。

### 3.2 顶层命令对账（vs 口径稿 keep 12）

P4a 后顶层命令共 14 个（不计 Cobra 框架的 help/completion——口径稿 3.1 已定 help 并入框架自带）。逐个对账：

| 顶层命令 | 对账 | 依据 |
|---|---|---|
| goal | keep 12 之一（loop 能力的正式入口） | 拍板第 5 条 |
| truth | keep 12 之一（本片补真） | 第 4 节总账 |
| design | keep 12 之一（薄面） | 第 4 节总账 + 3.4 slim |
| implement | keep 12 之一（薄面） | 第 4 节总账 |
| qa | keep 12 之一（薄面） | 第 4 节总账 |
| release | keep 12 之一（薄面） | 第 4 节总账 |
| doctor | keep 12 之一（slim，既有） | 3.5 |
| gate | **总账外的保留面**：口径稿 merge 表把 v1 的 `gate` 命令「并入 gate 包」——Go 的顶层 `gate` 就是这个包的正式评估命令面（evaluate/status/integrity），不是 v1 compat 别名 | 3.2 merge 表 + P3a |
| spine | v2 新增只读面，不占 77 条名额（口径稿第 4 节原注） | P1 |
| projects | v2 新增注册表面，同上 | P1 |
| migrate | P3b 的一次性迁移工具面（AC-077..079），与 doctor 同属运维面；常驻的是「工具」不是「旧命令兼容」 | P3b |
| loop | 指针（打印一行指路 goal），不是别名、无自身行为 | 拍板第 5 条 |
| program | 不设顶层：`goal program` 子命令组（keep 12 里注明「归 loop 的独立入口」） | 第 4 节总账 |
| archive / skill / init / hooks | 不设顶层：archive 的 slim 子集（按版本定位包）在 migrate/doctor rebuild 与 spine 里；skill 的 loader 是引擎的 ProgressiveSkills + skills 嵌入包（统一评分入口留 P4b 随评分落）；init 的最小装配由 `projects register` 承担；hooks 唯一保留的发布后钩子由 `goal self-heal` post 步引用 | 3.5 逐项 |

结论：keep 12 全部有落点（6 个能力面 + program 在 goal 下 + doctor + 4 个 slim 支撑面折叠在既有面内）；总账外多出的 4 个顶层（gate/spine/projects/migrate）逐个有说法，无一是默默长出来的；Phase 0 的五个占位命令至此全部消除。

## 4. P2 两笔欠账清偿

| 欠账 | 清偿 |
|---|---|
| G-01 `entry-contract` 未单设子命令 | 新增 `goal entry-contract --run-id`：v1 `public_entry_contract` 的公开面形状（public_primary=goal、public_cli="beacon goal"、single_entry=true、dual_public_orchestration=false）+ 该 run 的 stage plan 与逐阶段技能清单 + verifier/human 不变量，数据源就是已有 stage plan 与 run 快照，只读 |
| G-08 `resume-pack` 未单设子命令 | 新增 `goal resume-pack --run-id`：run 快照 + resume 决策 + 最近 checkpoint（state 已解码）+ open blockers（run 当前 blocker 与 checkpoint 里记的合并去重）+ 未 ack 的 steering 摘要，只读聚合 |

为保证恢复包的决策和 `goal resume` 的决策永远一致，本片把 Resume 的判定抽成纯函数 `decideResume`（resume.go）：Resume 在它外面套副作用（清软 block、消费 interrupt），ResumePack 直接调它、什么都不消费不清。`TestResumePackDoesNotConsumeInterrupt` 钉死：看过 pack 之后，Resume 仍能且只能重放一次被打断的步。既有 Resume 行为不变（159 个旧测试含全部 resume 用例原样通过）。

## 5. 观察项（如实记录，非本片引入）

实跑中发现一处 P2a 既有口径，初看像 bug、查清后是既定派生规则，记录在此供后续讨论，本片未改：

- 包已经 frozen，但刚 `goal run` 建完 run 时立刻 `admit`，truth_frozen 项会报 `freeze_ack_required`（未冻结）。机理：准入的 frozen 派生（P2a `buildExitInput`）只认两个证据——lifecycle 快照在 Freeze/Completed，或正式 truth_freeze 门 pass；而 `CreateRun` 启动时会把 feature 的 lifecycle 推进到 Prd/InProgress（引擎以 run 驱动 feature 的既有行为），第一个证据随之消失。
- 转绿路径是现成的：跑一次 `gate evaluate truth_freeze`（P3a 的正式门以 frontmatter 为主口径，包真 frozen 就 pass），正式 pass 落库后准入与 truth 出口门都认（formal-first 是 P3a 已定的消费口径）。本片实跑样例第 6 步演示了这条路径：评估 pass 后 admit 只剩 design_bound 一项拦（带界面的 run 没绑规矩，本就该拦）。
- 是否要改这条派生（比如让派生也读 frontmatter），涉及 P2a 出口门语义，不在本片范围；如要改，按维护约定先补测试再动。

## 6. 实跑样例（真二进制，临时库 /tmp/p4a-smoke）

```text
$ beacon projects register --path /tmp/p4a-smoke/proj --version v2.0.0 --json
{ "project_key": "proj--876fb41e97", "current_version": "v2.0.0", ... }

$ beacon truth create --project $R --slug login-2fa --json
{ "slug": "login-2fa", "status": "draft", "frozen": false,
  "package_dir": "docs/beacon/v2.0.0/features/login-2fa",
  "lifecycle_stage": "Think", "lifecycle_status": "NotStarted", ... }

（作者改稿：把「人话」一段写实）

$ beacon truth freeze --project $R --feature login-2fa --by lai --json
{ "slug": "login-2fa", "status": "frozen", "frozen": true,
  "lifecycle_stage": "Freeze", "lifecycle_status": "Completed",
  "truth_sha256": "2ec090a574a1a21d39b9291c654c9e2644f432b8e05f37a7116e3be6a1402a65" }

$ beacon truth change --project $R --feature login-2fa --summary "AC-001 扩围：含 SSO 登录" --by lai --json
{ "status": "change_pending", "change_pending": true,
  "pending_change_ids": ["chg_980c0a7043fa"],
  "change_doc": "docs/beacon/v2.0.0/features/login-2fa/changes/chg_980c0a7043fa.md" }

（作者改正文后）$ beacon truth refreeze --project $R --feature login-2fa --by lai --json
{ "status": "frozen", "frozen": true, "change_pending": false, ... }
$ grep -n "^status:" .../login-2fa/truth.md
4:status: frozen

$ beacon goal run "给登录加二次验证" --project $R --feature login-2fa --ux --executor-id host-1 --json
{ "run_id": "run_b0a20e11ee855b420bdbd83c", ... }

$ beacon design status --run-id $RUN
required: true bound: false stage: pending
bridge: prototype generation is delegated to the host agent via the design bridge (P3); ...

$ beacon implement admit --run-id $RUN --json   （exit 1）
"allow_implement": false, "reason_codes": ["freeze_ack_required", "design_primitive_not_bound"]
（truth 项的机理见第 5 节观察项；design 项是真拦：没绑规矩）

$ beacon gate evaluate truth_freeze --project $R --feature login-2fa --evaluator qa-bot
verdict: pass   [ok] frozen_marker  truth.md frontmatter status=frozen (primary source) ...

$ beacon goal admit --run-id $RUN --json       （重跑）
allow: False reasons: ['design_primitive_not_bound']   ← truth_frozen 已 ok，只剩真拦的一项

$ beacon qa status --project $R --feature login-2fa
truth_freeze     pass       [] (...)
design           not_evaluated
implement        not_evaluated
qa               not_evaluated
release          not_evaluated
truth_integrity  pass       [] (...)        ← freeze 预检留下的正式评估

$ beacon release evaluate --project $R --feature login-2fa --evaluator qa-bot   （exit 1）
verdict: blocked
  [BLOCK] gate_design_passed    gate design has never been evaluated; ...
  [BLOCK] human_confirmation    no human release confirmation on record ...
reasons: gate_not_evaluated, release_requires_human_confirmation

$ beacon goal entry-contract --run-id $RUN
run: run_b0a20e11ee855b420bdbd83c (login-2fa)
entry: beacon goal (single_entry=true)
  plan  active  skills=[beacon-plan mode:pln-program ...]
  truth pending skills=[beacon-truth mode:gen-truth mode:gen-truth-init mode:gen-change mode:gen-refreeze]
  ...

$ beacon goal resume-pack --run-id $RUN
run: run_b0a20e11ee855b420bdbd83c (running)
action: load_next_skill   next: load_next_skill:beacon-plan   blockers: []
checkpoint: ckpt_ed431df1ae2116fe3916a6e9 (iteration 1)
```

## 7. 验证与交付物

- `go build ./...` / `go vet ./...` / `gofmt -l .` 全净；`go test ./... -count=1` **173 个顶层测试全过**（既有 159 个零改动零回归；新增 14 个：orchestrator 11 + cli 3）。**本片没有改任何既有测试**；唯一的既有代码改动是 resume.go 把 Resume 的判定抽成纯函数 `decideResume`（行为不变，旧 resume 用例原样通过）与 root.go/goal.go 的接线。
- 任务覆盖点逐项有测试：create 骨架形状 + 重复 slug 拒绝 ✓、freeze 被缺内容拦 / 被 integrity（未决 deferral）拦 ✓、freeze 后 frontmatter 与 lifecycle 一致且事件带原文哈希 ✓、重复 freeze 幂等（不重写文件、不堆事件）✓、change 对未 frozen 拒绝 / 对 frozen 登记且正文哈希不变 ✓、refreeze 清 pending 且事件点名 change id ✓、薄面与 goal/gate 同判 ✓、entry-contract 与 resume-pack 字段完整 ✓、resume-pack 不消费 interrupt ✓。
- 边界守住：未做统一评分（P4b）、未做免疫清单剩余项与 ACP 深化（P4c）、未改 Python 源仓、未动冻结 schema；除 freeze/refreeze 改 frontmatter status 一行外，CLI 不改需求正文任何内容。

| 交付物 | 大小 |
|---|---|
| docs/p4a-implementation.md（本报告） | 以落盘为准 |
| internal/orchestrator/truth.go（407 行） | 包服务：create/freeze/视图/预检 |
| internal/orchestrator/truth_skeleton.go（145 行） | 骨架模板 + setFrontmatterStatus |
| internal/orchestrator/truth_change.go（122 行） | change / refreeze |
| internal/orchestrator/pack.go（141 行） | entry-contract + resume-pack |
| internal/orchestrator/resume.go（改，抽 decideResume） | 行为不变 |
| internal/cli/truth.go（274 行） | truth 命令组 |
| internal/cli/abilities.go（156 行） | 四个薄面 |
| internal/cli/root.go / goal.go（改，接线 + 两子命令） | — |
| internal/orchestrator/truth_test.go（397 行）+ pack_test.go（121 行） | 新增测试 |
| internal/cli/p4a_test.go（279 行） | 新增测试 |
| bin/beacon | 15,923,594 字节（v2.0.0-go.2） |
