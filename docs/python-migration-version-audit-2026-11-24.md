# Python Beacon 迁移审计：版本治理与假交付（2026-11-24）

> 对象：`justin-mc-lai/beacon`（Python v1.6.19 线） → `justin-mc-lai/beacon-go`（Go 线）。
> 本地克隆：`~/Developer/personal/beacon`（`--depth 1` + tags，约 455M，HEAD `01e7c95f`，2026-10-27；历史仍是 shallow）。
> 方法：仓内文件、git/GitHub API、真实 pytest、GitHub Actions 结论交叉核对。
> 性质：分析报告，不是需求真相，不构成 gate verdict。

## 0. 结论

1. **Python 包版本本身是标准 x.y.z**：`pyproject.toml` = `1.6.19`，`beacon/__init__.py` = `1.6.19`，`version-truth.json` = `1.6.19`。真正不规范的**不是数字格式，而是版本治理**：双真源、未定义状态、标签/发布缺失、发布文档是存根。
2. **Go 线此前不是 x.y.z**：`2.0.0-go.17` 是预发布串；已于 2026-11-24 规范为 `2.0.0`（`internal/version/version.go`）。`go.mod` 的 `module beacon-go` 仍缺 `/v2`，按 Go module 规则待修。
3. **存在可实证的假交付**：v1.6.19 的 tag 指向的提交里，release 文档仍是生成存根，delivery-manifest 自报 `draft` / `strict_delivery_ready=false` / `tag=""`；同期 GitHub Actions 的 release gate 结论是 failure。
4. **测试绿不等于交付真**：版本/发布相关的 40 个测试全绿，但它们只校验临时 fixture，不校验仓库真实版本数据；真实数据里的矛盾没人拦。
5. **迁移已过大半**：Go 已覆盖 6 能力、gate、driver、program、topology、skills、archive、hooks、code2truth、reopen、brainstorm、truth-map、review、autotest、quota、release check 等。剩余缺口集中在 6–8 项，详见第 4 节。
6. **顺序建议**：先修版本/发布真相与 gate，再继续迁移。否则 Go 会把 Python 的“版本可手改、交付可自报”继承过去。

## 1. 版本治理缺陷（实证）

### 1.1 双真源互相矛盾

| # | 事实 | 证据 |
|---|---|---|
| V1 | 顶层声明当前发布线是 `v1.6.19` | `beacon/version-truth.json:6` |
| V2 | 同一文件的逐线状态却把 `v1.6.12` 标成 `current_runtime_release` | `beacon/version-truth.json:71-72` |
| V3 | `v1.6.13`–`v1.6.19` 共 7 条标成 `active_delivery` | `beacon/version-truth.json:75-93` |
| V4 | 语义契约只定义 4 个状态，没有 `active_delivery` | `beacon/version-semantics.json:39-43` |

结论：`active_delivery` 是**未定义状态**；同一文件里同时存在两个“当前”候选（v1.6.19 顶层、v1.6.12 逐线）。任何按逐线状态消费的代码都会读到 v1.6.12。

### 1.2 第二套版本目录已滞后

| # | 事实 | 证据 |
|---|---|---|
| V5 | `docs/beacon/governance/version-truth-catalog.md` 的最后事务是 2026-08-17 | catalog payload `transaction.committed_at` |
| V6 | catalog 活跃窗口只到 `v1.6.17`（v1.6.10–v1.6.17） | `list_active_catalog_versions()` 实测 |
| V7 | `v1.6.18`、`v1.6.19` 不在 catalog；`beacon/version-truth.json` 有 | 两源差集实测 |
| V8 | catalog 里有 `v0.0.1-debug`，不是 x.y.z | catalog `versions` |
| V9 | `v1.4.13` 只在 version-truth，不在 catalog | 两源差集实测 |

结论：两个“版本真源”各自演化，catalog 落后两个发布线，且容纳了非规范版本名。

### 1.3 标签与发布断裂

| # | 事实 | 证据 |
|---|---|---|
| V10 | version-truth 有 70 条线，GitHub 只有 12 个 tag | API tags |
| V11 | 17 条 `v1.6.x` 线没有 tag（v1.6.0、v1.6.2–5、v1.6.7–18） | 两集合差 |
| V12 | GitHub Releases 只到 `v1.6.6`；`v1.6.19` 有 tag、无 release | API releases |
| V13 | Go 线 6 个 tag（go.7–10、16、17），只发了 3 个 release（go.10、16、17） | `git tag` / `gh release list` |
| V14 | Go `module beacon-go` + 主版本 2，缺 `/v2` | `go.mod:1` |

结论：tag、release、changelog、docs 四条发布痕迹没有统一约束；Go 线同样漏 tag/release。

## 2. 假交付证据

定义：**对外声称某个版本已交付/当前发布，但该版本的必要交付物缺失、互相矛盾或自报未就绪。**

| # | 证据 | 位置 |
|---|---|---|
| F1 | v1.6.19 的 release 文档只有标题 + 生成器存根 | `docs/beacon/v1.6.19/release/v1.6.19.md:3` |
| F2 | v1.6.18 同样是存根 | `docs/beacon/v1.6.18/release/v1.6.18.md:3` |
| F3 | 共 8 个 release 文档是生成存根 | `grep "version-line stub" docs/beacon` |
| F4 | **在 `v1.6.19` tag 指向的提交里**，delivery-manifest 仍自报 `delivery_status=draft`、`strict_delivery_ready=false`、`tag=""`、reason 含 `git_branch_target_mismatch` / `git_version_ref_not_satisfied` | `docs/beacon/v1.6.19/.machine/execution/delivery-manifest.json`（tag 版本） |
| F5 | v1.6.16/17 的机器验收 `acceptance_release_ready=true`，但 release 文档写“待 QA 验收后由人类 gate 确认” | `docs/beacon/v1.6.16/.machine/execution/qa-acceptance-*.json:7` vs `docs/beacon/v1.6.16/release/v1.6.16.md:5` |
| F6 | v1.6.18/19 的机器验收 `release_ready` 缺失（None），release 文档是存根，但版本线被标 `active_delivery` | 实测 QA JSON + `version-truth.json:90-93` |
| F7 | README 的 PyPI 徽章指向 `beacon`，但 PyPI 上该名字是无关包 `0.0.2`（“collects usage data of your Python apps”），不是本仓 | README 第 5 行；PyPI JSON |
| F8 | v1.6.19 晋升提交 `0e8657f`（2026-09-21 09:16:46）之后，GitHub Actions `beacon-release-gate` 在 09:18:59 结论仍为 **failure** | run `35582572793` |
| F9 | 当前 main（2026-10-27）全量 pytest：**173 failed / 1837 passed / 1 skipped** | 本地 Python 3.9.6，2m15s |
| F10 | 版本/发布相关的 40 个测试全绿，却不校验仓库真实数据——它们只用 `tmp_path` 造 fixture | `tests/test_version_truth_archive_lineage.py`、`tests/test_release_version_consistency.py` |

### 2.2 全量测试失败分类（本地 Python 3.9.6）

两次运行：`173 failed / 1837 passed` 与 `174 failed / 1836 passed`——两次数字不同，说明还有非确定性用例。

| 失败数 | 测试文件 | 直接原因 | 性质 |
|---:|---|---|---|
| 34 | `test_v160_beacon_loop_automation_retirement.py` | `packages/beacon-loop` 整个目录不在 clone 里；本机另缺 `rg` | **干净 clone 不可测** |
| 19 | `test_v200_goal_durable_longrun.py` | v2.0.0 设计物料缺失 | v2 未交付但测试已在 main |
| 10 | `test_v167_goal_agent_driver.py` | `docs/beacon/v1.6.7/...`（实际在 archive）、`docs/beacon/v2.0.0/fixtures/steering/continuation.md` 缺失 | 测试引用失效路径/缺失 fixture |
| 8 | `test_v167_goalrun_main_axis.py` | `goalrun-main-axis.md` 缺失；返回 `program_feature_graph_missing` | 产物缺失 |
| 5 | `test_v200_loom_*` | `skills/loom/`、v2.0.0 loom 物料缺失 | v2 未交付 |
| 4 | `test_docs_current_contract.py` | README 当前段仍引用 v1.6.17；`CLAUDE.md` 版本标记是 v1.6.18，docs 是 v1.6.19 | **文档版本漂移** |
| 2 | `test_workflow_gate_release_consistency.py` | 期望 `Pass`，实际 `Blocked` | 真实行为回归，需单独查 |
| 其余 | v1.6.7–v1.6.9 的 skill/public-surface/fixture 测试 | 1+6 surface、steering、golden commerce 等物料漂移 | 版本线物料不闭合 |

结论：红灯不是“少数偶发”，而是三类结构性问题——**clone 不自包含、测试引用失效、文档版本漂移**。这三类都会让“绿灯”失去证明力。

### 2.3 为什么这些能同时成立

1. **Gate 只查字符串，不查交付物**：`version-gate.yml` 只比对 `pyproject.toml`、README、SKILL、CHANGELOG；不查 `__init__.py`、逐线状态、catalog、tag、release、release 文档、delivery-manifest。
2. **两套真源都能手改**：`beacon/version-truth.json` 与 catalog 没有 schema/一致性校验；`active_delivery` 不在语义契约里也能存在。
3. **测试只验合成数据**：真实仓库文件的矛盾不会被 40 个版本测试发现。
4. **tag 先于 gate**：tag 由人打，release gate 在 tag 之后跑；失败不撤销 tag，也不阻止版本面晋升。
5. **验收与发布脱节**：QA acceptance 是“湖级”结论，delivery-manifest 是“版本级”结论；前者 true 不等于后者 ready，但版本面只读前者或手写。

## 3. 与 Go 线的关系

Go 线已经用独立账本、硬 gate、合同指纹、E2E 修掉了一部分同类问题（见 `docs/rework-retrospective-2026-11-24.md`），但仍有：

- 已修：版本规范为 `2.0.0`（`internal/version/version.go`）；待办：`module beacon-go` 缺 `/v2`。
- tag/release 漏发（go.11–15）。
- 版本常量已集中到 `internal/version/version.go`；仍未由 tag/ldflags 注入。
- 文档版本漂移（AGENTS.md 曾写 go.15，本轮已修；CI 已加版本一致性检查）。

**迁移时不要把 Python 的“版本可手改、交付可自报”搬过来。** Go 侧的版本单一源与发布 gate 应作为上游规范，反向约束 Python。

## 4. Go 迁移现状与剩余缺口

### 4.1 已覆盖（证据：Go 源码与命令面）

goal / truth / design / implement / qa / release；gate；driver / supervisor / checkpoint / resume；program；topology；skills 安装；archive；hooks；code2truth；reopen；brainstorm；truth-map；review；autotest；quota；release check；agent score/corpus；desktop + agentapi。

### 4.2 剩余缺口（按 Go 仓 grep 与命令面）

| # | Python 能力 | Go 现状 | 建议 |
|---|---|---|---|
| G1 | `ask` 意图路由（auto/rule/llm + 置信度 + 预览） | 无；Go `truth ask` 是提问账本，同名异实 | P0，迁为 `goal intend` 或 `truth route`，保留证据 |
| G2 | `feedback` + `friction-intake` | 无命令；仅 skill/注释里出现 friction | P0，合并为一个改进输入入口 |
| G3 | `doctor debug` / `lighthouse` | doctor 只有 `rebuild`；selfheal 仍引用 v1 命令名 | P0，补 `doctor diagnose` / `doctor lighthouse` 或删除死引用 |
| G4 | `report` / 运行时投影（跨仓 git、健康、团队/ralph 视角） | spine 只读账本；无报告面 | P1，先做只读 `spine report` |
| G5 | `profile` 项目画像 | Go profile 是执行器档案，同名异实 | P1，定义项目画像或明确 drop |
| G6 | `prototype` 宿主桥接 + `ui-spec` | design 只绑不生成 | P1，按 charter 3.4 只做桥接合同，不做渲染 |
| G7 | `prd-bootstrap` 内容生成 | truth create 只出骨架 | P1，做“吃种子材料生成首版包” |
| G8 | immune：playwright 扩展、loop patterns、remediation sandbox | 仅 QA 策略里提到 playwright，无执行器 | P2，先拍板 keep/drop，避免半成品 |
| G9 | agent eval 实测回放 | 有 score/corpus；live replay 不明确 | P2，补 replay 或标 deferred |
| G10 | 定时 `task` 系统 | Go `goal todo` 是 run 内待办，语义不同 | P2，按需决定是否迁 |

## 5. 版本规范化方案（x.y.z）

### 5.1 统一规则

- 版本格式：`MAJOR.MINOR.PATCH`，tag 为 `vX.Y.Z`。
- 预发布只允许 `-rc.N`，且必须显式 gate；禁止 `-go.N`、`-simplified`、`-debug` 作为发布线。
- 每个发布线只允许一个权威状态；状态枚举必须与语义契约完全一致。
- 每条 `released` 线必须有：tag、GitHub Release、非存根 release 文档、CHANGELOG 条目、证据包。
- 版本号由 tag 注入，不在源码/文档手写多个副本。

### 5.2 Python 侧

1. 选一个真源：`beacon/version-truth.json`；catalog 由脚本生成，不手改。
2. 修 `active_delivery`：要么加入 `version-semantics.json` 并定义，要么从 truth 中移除。
3. 逐线只保留一个 `current_runtime_release`，且必须等于顶层。
4. 把 `v1.6.18/19` 补进 catalog；把 `v0.0.1-debug` 移到 archive 或改名 `v0.0.1`。
5. 给 17 条未打 tag 的 `v1.6.x` 补 tag 或标记 `historical-untagged`。
6. 为 v1.6.19 建 GitHub Release，并写真实 release notes（替换存根）。
7. 移除 PyPI 徽章，或改名发布到唯一包名并校验 homepage。

### 5.3 Go 侧

1. 模块路径改为 `github.com/justin-mc-lai/beacon-go/v2`（若主版本保持 2）。
2. 版本常量已改为 `internal/version/version.go` 的 `const Version = "2.0.0"`；后续发布可用 ldflags 注入（待办）。
3. 发布版本用 `2.0.0`（已完成）；后续功能 `2.1.0`，预发布只用 `-rc.N`。
4. 每次发布补 tag + GitHub Release + release notes。
5. CI 增加“tag 与版本常量一致”检查。

## 6. 反假交付 Gate（可执行）

| Gate | 检查 | 失败后果 |
|---|---|---|
| G-REL-1 | `version-truth.json` 顶层 current 唯一，且逐线只有一个同值 `current_runtime_release` | block |
| G-REL-2 | 所有状态都在 `version-semantics.json` 枚举内 | block |
| G-REL-3 | catalog 与 version-truth 的线集合一致（archive 例外） | block |
| G-REL-4 | 每条 released 线有 tag、GitHub Release、非存根 release 文档 | block |
| G-REL-5 | delivery-manifest `strict_delivery_ready=true` 且 `tag` 非空，才允许版本面晋升 | block |
| G-REL-6 | release 文档不含 `version-line stub`，且引用 tag/commit | block |
| G-REL-7 | `pyproject.toml` / `__init__.py` / version-truth / catalog 四者一致 | block |
| G-REL-8 | PyPI 包名/homepage 与仓库匹配，或徽章不存在 | block |
| G-REL-9 | 全量测试绿；测试必须跑真实仓库数据至少一遍 | block |
| G-REL-10 | tag 创建前 gate 必须绿；tag 后 gate 失败要自动撤销/告警 | block |

## 7. 迁移顺序

1. **Phase 0（先做）**：修版本/发布真相 + G-REL-1..10；把 Python 的 173 个红灯分类（v2.0.0 设计测试应 skip/xfail，v1.6 回归要修）。
2. **Phase 1（P0）**：G1 意图路由、G2 feedback/friction、G3 doctor debug/lighthouse。
3. **Phase 2（P1）**：G4 report、G5 profile、G6 prototype/ui-spec、G7 prd-bootstrap。
4. **Phase 3（P2）**：G8 immune、G9 live replay、G10 task；先拍板 keep/drop。
5. 每个迁移项必须带：Python 行为对照测试、Go 真实 CLI 跑通、证据绑定 commit+binary+指纹。

## 8. 验证与边界

已执行：

- 克隆 `justin-mc-lai/beacon` 到 `~/Developer/personal/beacon`（shallow；已补拉 12 个 tag）。
- `pytest --collect-only`：2011 tests collected。
- 版本/发布定向测试：40 passed。
- 全量 pytest（Python 3.9.6）：两次 `173 failed / 1837 passed`、`174 failed / 1836 passed`。
- GitHub Actions：`beacon-release-gate` 在 v1.6.19 晋升后仍 failure（run 35582572793）。
- Go 侧：`beacon --version` = `v2.0.0`；tag/release/go.mod 核对；module 路径待办。

未执行/未确认：

- 未用 CI 的 Python 3.11 复跑全量；本地 3.9.6 的失败里可能混有版本差异，但 v2.0.0/loom fixture 缺失与版本治理矛盾与 Python 版本无关。
- 未访问 Actions 失败日志（API 返回 log not found），只采信 run conclusion。
- 未修改 Python 仓任何文件；本报告只读分析。
- 未发布 PyPI，未验证包名可用性。
