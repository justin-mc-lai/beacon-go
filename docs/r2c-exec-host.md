# R2c 批次报告：执行与宿主组（H8 / H9 / H10 / M10 / M11 / M12）

对等目标：Python v1.6.19 源码（只读镜像 `~/workspace/repos/beacon/`）。
基线：go.8 + R2a + R2b，283 个测试全过。收尾：build / vet / test 全绿，
E2E 全链 PASS。未动版本号（仍 `2.0.0-go.8`），未动数据库九表，未提交未推送。

## 一句话结论

六项全部落地。这一批是把「只会判定、不会动手」的部分补成真能跑：
测试能真执行并留证据、技能能真装进宿主目录、项目状态能装进口袋
（仓库）里带走、归档和初始化按 Python 的文件面对齐、钩子能真触发。

## H8 auto-test 执行半边（`qa autotest run`）

- 新命令：`beacon qa autotest run --run-id <id> --by <谁> [--timeout-seconds N] [--json]`。
- 做什么：读需求包 tests.md 的覆盖矩阵，每一行 TC 有命令就真跑；
  没命令就回退读项目 `.beacon/config` 里的 `test_command` / `autotest_command`；
  两处都没有，明确报 Python 原码 `qa9_command_missing_auto_case`，不许假跑。
  tests.md 里写了 `BLOCKED:` 的行直接记 blocked（原因码 `tc_declared_blocked`），不执行。
- 执行方式：命令先经一个无 shell 的分词器拆成参数数组（引号/转义支持；
  管道、重定向、`$()`、反引号、`&&` 等 shell 语法一律拒绝，原因码
  `autotest_shell_syntax_unsupported`），再走既有 harness 执行器
  （进程组、超时击杀）。环境变量带 `BEACON_TC_ID / BEACON_AC_ID /
  BEACON_PROJECT_ROOT / BEACON_JUNIT_PATH / BEACON_TC_DIR`。
- 证据：每个 TC 在 `.beacon/autotest/<run-id>/<tc>/` 落
  `result.json + stdout.log + stderr.log`，经既有 artifacts 通道登记
  （kind=`test_result`）；汇总 `report.json` 登记为 kind=`test_results`；
  再追加一条 `autotest_executed` 事件。qa 门按既有词表直接消费，
  没有为它开任何后门。结果行序与 tests.md 原序一致。
- 一个真 bug 在本批修掉：证据文件一度是「先登记哈希、再回填字段重写文件」，
  登记的哈希和最终文件对不上，门复算会判证据过期。已改为
  「登记前写最终版」，并加了回归断言（登记哈希 == 磁盘最终字节哈希）。
  代价：result.json 文件本身不带自己的 artifact id——id 只存在于
  登记表和汇总报告里，这是哈希登记的固有形态，不是缺陷。

残留：
- Python auto_test.py 的浏览器/扩展类用例（要起浏览器、装扩展）不移植，
  属外部条件（审计 L13），只做命令行级执行。
- 断言数只在命令产出 junit.xml 时统计（读 `BEACON_JUNIT_PATH`），
  不像 Python 那样逐行解析各家测试框架的输出格式。

## H9 skill 宿主发现路径

- 内嵌技能从 6 份变 7 份：新增 `skills/plan/SKILL.md`（beacon-plan），
  内容提炼自 Python 的 beacon-plan 技能包：五种 mode、ask→sign 链、硬规则。
- 七份技能各补了一份 `references/core.md`，内容从 Python 对应技能包
  提炼并注明来源。
- 新命令：`beacon skills install [--host claude|codex|generic] [--dir <目录>] [--force] [--json]`，
  另有 `beacon skills references <id>` 查看引用文件。
  默认落点照 Python `agent_surface.py` 的公开约定：
  claude → `~/.claude/skills`，codex/generic → `~/.agents/skills`。
  物化形态是宿主能直接发现的扁平目录：`<目标>/beacon-<id>/SKILL.md`
  + `references/`。写前打印全部目标路径；任一同名目录已存在且没给
  `--force` 时整单拒绝（原因码 `skill_install_conflict`），不写一半。
  宿主名不认识报 `skill_install_host_unknown`。

残留：
- references 只有一份提炼版 core.md，不是 Python 技能包的全量文件
  （modes / evals / agents 等子目录未搬）。够宿主理解和调用，
  不是逐字节复刻。
- Python 里 `beacon-pln-review` 这类名字在 Go 版仍是 plan 技能的
  mode 名，不是独立技能——与 Python 技能包的组织方式一致。

## H10 状态随仓库走

- 快照格式 `beacon-state-snapshot.v1`，落在仓库内
  `.beacon/state/project-state.json`（可读 JSON：项目、版本、
  每个需求包的 lifecycle、每个包的最新门结论、run 摘要）。
- 命令：`beacon projects export-state / import-state`。
- 取舍（明确一条）：**全局 DB 仍是唯一事实源，快照只是可携带副本，
  冲突时以 DB 为准**。import 只填缺失行——项目、版本、需求包、
  lifecycle、门结论，本地已有的任何一行都不覆盖；二次 import
  填入数为 0（有测试钉住）。
- **run 只导出摘要、不导入**：历史执行记录不许靠一个 JSON 文件
  在另一台机器上「恢复」出来，那等于可以伪造历史。
- 自动识别：未注册的项目目录只要带 `.beacon/state/project-state.json`，
  不再一刀切拒绝——`gate / qa / release / truth` 面的项目解析、
  `projects show`、`spine --project` 都会先自动 import 再继续。
  只读命令（show / spine）自己连接写不进，会开一次短命的可写连接
  完成 import 再回读。本批还修了一个早退：B 机连 DB 文件都还没有时
  `projects show` 原来直接报「未注册」，现在先走自动识别再决定。

## M10 archive（`beacon archive`）

先纠正一个直觉：**Python 的 archive 并不移动需求包**。apply 做的事
是写治理元数据：`docs/beacon/<版本>/.machine/archive/` 下
`feature-archive-<包>.json` + `archive-index.json` + `archive-plan.json`。
Go 版逐项照做，没有自创「搬到归档区」的动作。

- 命令：`archive list / lineage / apply / status / explain`，顶层命令面。
- 扫描双根：`docs/beacon/<版本>/features/*` 与
  `docs/beacon/archive/<版本>/features/*`；
  状态模型照 Python：`active / superseded / archived-learnable /
  archived-reference-only`；AC 数 = truth.md 里不重复的 `AC-\d{3}` 计数；
  lineage 按版本号跨双根排序追踪一个包的一生。
- 版本低于 1.3.8 报 Python 原码 `archive_governance_not_supported`；
  包不存在报 `archive_feature_not_found`。
- 一处有意分歧：状态值非法时 Python 是静默归一成 active，Go 版
  直接拒绝并报新码 `archive_status_invalid`——静默改掉用户写的状态
  更危险，拒绝更诚实。此分歧已在代码注释标明。

残留：Python 若有 catalog 级别的批量搬迁视图未做；当前是
单包治理 + 全量清单（`archive list --all`）。

## M11 init（`beacon projects init`）

- 命令：`projects init --project <目录> [--version] [--project-mode
  greenfield|takeover] [--name] [--dry-run] [--force]
  [--with-claude] [--with-agents] [--json]`。
- 不止注册一行 DB：文件集逐个对 Python `init.py`——目录
  `docs/beacon/<v>/{execution,features,release}` 与 `.beacon/state`；
  `SUMMARY.md`、execution 下六份蓝图/索引文件、`release/<v>.md`
  内容用 Python 原文模板；takeover 模式另加
  `current-state-service-map.md`、`refactor-boundaries.md`；
  `CLAUDE.md` / `AGENTS.md` 用 Python 的兜底桩；
  `.beacon/state/project-onboarding.json` 用 Python 字段集
  （含 `lifecycle_adoption_level=2`）；apply 时另写
  `.beacon/state/init-report.json`。
- 语义照 Python 的 `_write_if_apply`：文件已存在不覆盖（记
  unchanged），`--force` 才刷新（记 refreshed），dry-run 零写入。
  模式非法报 Python 原码 `invalid_project_mode`。apply 同时完成
  注册项目 + 记录版本（版本缺省取已记录的 current 版本）。

残留（两处明确适配）：
- `global-boundaries.md`：Python 是从工具包同步权威文本，Go 版
  没有可同步的权威源，写项目自有桩并在报告声明，不冒充原文。
- Python `setup_context_files` 的富上下文引导块未移植，
  CLAUDE.md / AGENTS.md 只写兜底桩。

## M12 hooks（`beacon hooks`）

- 契约文件：主位置 `docs/beacon/hooks/<id>.md`（Python 的
  `hooks_root`），次位置 `.beacon/hooks/<id>.md`（任务书措辞），
  同 id 时前者胜。契约是带 frontmatter 的 Markdown，
  正文必须有 Purpose / Admission / Evidence / Failure 四节。
- 命令：`hooks list / plan / run / disable / status`，flag 对齐
  Python（`--project/-p`、`--project-root` 别名、`--version`、
  `--feature/-f`、`--hook-code`、`--dry-run`、`--confirm`、`--json`）。
- 校验照 Python `validate_hook_contract` 移植，诊断码原样：
  hook_code 四值、trigger_policy 三值、failure_route 五值、
  skill 与 command_template 至少其一、produces_evidence 必填、
  权威越界检查 `hook_authority_bypass`（关键词表照抄）。
  本批修了一个细差：frontmatter 里**缺失** hook_id 时 Python 也判
  `hook_id_mismatch`（拿 None 和文件名比），Go 版已对齐。
- 运行的门照 Python：准入不过报 `hook_admission_blocked`；
  `--dry-run` 只写 planned 证据；`dry_run_only` 策略拒绝真跑
  （`hook_dry_run_only`）；`manual_confirmed` 没给 `--confirm`
  报 `manual_confirmation_required`；`auto_allowed` 没确认报
  `hidden_auto_run_not_allowed`。证据落在
  `.beacon/state/hooks/<id>/<runid>.json`，最新读模型在
  `docs/beacon/.machine/hooks/<id>.latest.json`（均照 Python）。
- 发布后触发：`goal complete` / `release complete`（共用同一
  构造器）成功后自动调 `RunPostReleaseHooks`：发现 enabled 的
  post_release 钩子，auto_allowed 的确认执行、manual_confirmed 的
  只报 `awaiting_confirmation` 绝不偷跑、dry_run_only 的只写
  dry-run 证据；汇总追加一条 `post_release_hooks_triggered`
  事件。钩子失败不推翻已完成的完工。
- 内置钩子 `beacon-post-release-local-install` 的 followup 真做事：
  把内嵌技能装进 `~/.claude/skills` 与 `~/.agents/skills`
  （复用 H9 的 Install，force 对齐）。
- 死引用修正：自愈计划里引用的
  `beacon hooks run beacon-post-release-local-install -p . --confirm`
  本批起是真命令，自愈文案无需改动。

残留 / 有意分歧（一处，必须明说）：
- **command_template 在 Go 版是真执行的**（经 H8 同款分词器 +
  harness，占位符 `{project}/{version}/{feature}/{hook_id}/{hook_code}`）。
  Python 只校验模板、除内置钩子外不执行模板。任务书要求
  「发布后钩子执行走 harness」，故 Go 版有意更强；skill-only 的
  钩子无法凭空执行，只记「已确认触发」且 `side_effect_executed=false`。
- 内置钩子的 followup 只落了「装机」子集；Python followup 里
  其余后续动作未移植。

## 测试

- 总量：283 → **307**（`go test -v` 的 RUN 计数口径，与基线同口径），
  净增 24 条运行项，全绿。新增测试文件：
  `skills/skills_test.go`、`internal/orchestrator/r2c_test.go`、
  `internal/cli/r2c_test.go`。
- 覆盖要点：分词器引号与 shell 语法拒绝；autotest 真跑
  pass/fail/blocked 三态 + 回退命令 + 证据哈希不漂移；
  状态快照 export → 新 DB import 圆桌 + 二次 import 验证 DB 胜出
  + run 不导入；archive 双根扫描 / lineage / apply 三文件 /
  非法状态与低版本拒绝；init 双模式文件集 / 幂等 / dry-run /
  非法模式；hooks 诊断码 / dry-run 证据 / 确认门 / 模板真执行 /
  三种策略的发布后触发 / 内置钩子装机；
  CLI 面：skills install 冲突与 force、init→export→换 DB 自动
  识别、archive、hooks、qa autotest 冒烟。

## 既有测试改动声明

仅 1 处：`internal/cli/intent_test.go` 的 `TestSkillsListAndShow`
断言技能数 6 → 7。理由：H9 要求内嵌第 7 份 plan 技能，集合是
合法变大；断言本身没有放宽（仍是精确计数）。其余既有测试零改动。
R2a / R2b 的行为面（评审回写、quota、release check 等）全部
保持，回归全绿即为证。

## 纪律核对

- schema 九表：`internal/store/` 零 diff，未动。
- 版本号：仍 `2.0.0-go.8`。
- 新原因码全部登记在 `internal/core/reasons.go`，Python 原样码与
  Go 新码（注释标 NEW）分开注明。
- E2E：`e2e/run_e2e.sh` 全链 PASS，**脚本未改动**；
  `e2e/last_run.log` 是本次运行的产物日志（内容随运行更新，
  与前两批同例）。
- 未 git 提交、未推送。
