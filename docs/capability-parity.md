# 双仓能力对照表（Python v1.6.19 → Go）

> 日期：2026-10-25 ｜ 仓：`beacon-go` HEAD `d9326f4`（CLI 从 HEAD 源码重建到 `/tmp/beacon-parity`，`v2.0.0-go.9`）
> 回答一件事：**Python v1.6.19 有的 77 条顶层命令，Go 侧有没有、怎么处置。**
> 真值来源三处：`docs/scope-charter.md` §3/§4（处置口径）、`docs/ac-matrix.md`（AC 编号）、Python 仓 `products/beacon` 的 CLI 注册代码（命令是否真的存在，不凭记忆）。

## 0. 两侧怎么测出来的

Python 侧（`products/beacon`，注册入口 `beacon/cli_factory.py`，目录清单 `beacon/command_catalog.py`）：

```bash
# typer 组名 + app.command 名，合并去重
{ grep -o 'name="[^"]*"' beacon/cli_factory.py | sed 's/name="//; s/"//'
  grep -o 'app\.command("[^",]*"' beacon/cli_factory.py | sed 's/app\.command("//; s/"//'; } | sort -u | wc -l
# 实测 77；另有两个非命令注册名：beacon（app 名）、ralph（ralph-bridge 的别名组）
```

Go 侧（`beacon-go`）：

```bash
go build -o /tmp/beacon-parity ./cmd/beacon && /tmp/beacon-parity --version   # v2.0.0-go.9
/tmp/beacon-parity --help            # 顶层：design doctor gate goal implement loop migrate projects qa release skills spine truth（+completion/help）
/tmp/beacon-parity <cmd> --help      # 逐个查子命令
```

**77 条与 charter 对齐结果**：charter §4.1 列的 77 个名字，在 Python 注册代码里**逐条都在**（差集为空）；代码侧比 charter 多出 `beacon`、`ralph` 两个非命令注册名。下面的表按 §4.1 的 12 / 49 / 16 三档展开，共 77 行。

## 1. keep（12 条）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 1 | `goal` | 是 | `beacon goal`（22+ 子命令：run/tick/driver/flow/program/memory/todo…） | keep | §4.1 L156；AC-040/AC-109 |
| 2 | `truth` | 是 | `beacon truth`（create/decompose/freeze/reopen/change/refreeze/ask/answer/map/drift…） | keep | §4.1 L156；AC-004 |
| 3 | `design` | 是 | `beacon design`（bind/status，只绑不生成） | keep | §3.4 L127；AC-115 |
| 4 | `implement` | 是 | `beacon implement`（admit/plan/worktree/delegate-ralph/receive-result） | keep | §4.1 L156；AC-100 |
| 5 | `qa` | 是 | `beacon qa`（evaluate/status/score/score-history/autotest/strategy/agent-eval…） | keep | §4.1 L156；AC-097 |
| 6 | `release` | 是 | `beacon release`（check/evaluate/complete） | keep | §4.1 L156；AC-124 |
| 7 | `program` | 是 | `beacon goal program`（独立入口，挂在 goal 下） | keep | §4.1 L156 |
| 8 | `doctor` | 是 | `beacon doctor rebuild`（slim：只留重建投影） | keep（slim） | §3.5 L135；AC-011 |
| 9 | `archive` | 是 | `beacon archive`（slim；**可执行但未列入 `--help` 列表**） | keep（slim） | §3.5 L139；AC-159 |
| 10 | `skill` | 是 | `beacon skills`（复数；list/show/references/install） | keep（slim） | §3.5 L136；AC-125 |
| 11 | `init` | 是 | `beacon projects init`（slim：装配新项目上下文） | keep（slim） | §3.5 L140 |
| 12 | `hooks` | 是 | `beacon hooks`（slim；**可执行但未列入 `--help` 列表**） | keep（slim） | §3.5 L138 |

## 2. merge（49 条）

> 「Go 命令 / 等价路径」列就是合并去向；名字在 Go 侧一律消失（并列明等价子命令）。

### 2.1 并入 truth（19）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 13 | `prd` | 是 | `beacon truth create` / `truth decompose`（需求只住包内，不再出顶层文档） | merge→truth | §3.2 L114 |
| 14 | `user-story` | 是 | `beacon truth create` / `truth decompose` | merge→truth | §3.2 L114 |
| 15 | `test-case` | 是 | `beacon truth create` / `truth decompose`（tests.md 随包生成） | merge→truth | §3.2 L114 |
| 16 | `code2truth` | 是 | `beacon truth code2truth` | merge→truth | §4.1 L157 |
| 17 | `freeze` | 是 | `beacon truth freeze` | merge→truth | §3.2 L113 |
| 18 | `plan` | 是 | `beacon implement plan` / `beacon goal axis`（**口径差异**：charter 记并入 truth，Go 落在 implement/goal） | merge→truth | §4.1 L157 |
| 19 | `reopen` | 是 | `beacon truth reopen` | merge→truth | §3.2 L113 |
| 20 | `review` | 是 | `beacon review`（可执行、未列 `--help`）+ `beacon gate integrity` | merge→truth | §3.2 L113 |
| 21 | `planner-review` | 是 | `beacon goal planner-review` / `beacon qa planner-review` | merge→truth | §3.2 L113；AC-102 |
| 22 | `think` | 是 | `beacon truth intent` + `truth confirm`（冻前想清楚） | merge→truth | §3.2 L104 |
| 23 | `brainstorm` | 是 | `beacon truth brainstorm` | merge→truth | §3.3 L120 |
| 24 | `change` | 是 | `beacon truth change` | merge→truth | §3.2 L113 |
| 25 | `prd-bootstrap` | 是 | `beacon truth create` | merge→truth | §4.1 L157 |
| 26 | `truth-map` | 是 | `beacon truth map` | merge→truth | §3.2 L113 |
| 27 | `gen-truth` | 是（`app.command` 别名） | `beacon truth create` / `truth decompose`（别名消失） | merge→truth | §3.2 L113 |
| 28 | `materialize-feature-package` | 是（`app.command` 别名） | `beacon truth create` | merge→truth | §3.2 L113 |
| 29 | `validate-feature-package` | 是（`app.command` 别名） | `beacon gate integrity` | merge→truth | §3.2 L113；AC-101 |
| 30 | `friction-intake` | 是 | `beacon truth ask`（问题采集入口归 truth 改进输入） | merge→truth | §3.1 L95 |
| 31 | `feedback` | 是 | `beacon truth ask` / `truth answer` | merge→truth | §3.1 L95 |

### 2.2 并入 loop / goal（11）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 32 | `orchestrate` | 是 | `beacon goal program` / `goal delegate-ralph` | merge→loop | §3.2 L111 |
| 33 | `resume` | 是 | `beacon goal resume`（+ `resume-pack`） | merge→loop | §3.2 L111 |
| 34 | `ask` | 是（`app.command` 别名） | `beacon truth ask` / `truth answer`（**口径差异**：charter 记并入 goal 意图识别，Go 落在 truth） | merge→loop | §3.1 L92 |
| 35 | `remember` | 是 | `beacon goal memory` | merge→loop | §3.1 L93 |
| 36 | `recall` | 是 | `beacon goal memory` | merge→loop | §3.1 L93 |
| 37 | `workflow` | 是 | `beacon goal flow`（AC-138 标准执行流） | merge→loop | §3.2 L105 |
| 38 | `run` | 是 | `beacon goal run` | merge→loop | §3.2 L111 |
| 39 | `team` | 是 | `beacon goal implement --lane team` | merge→loop | §3.2 L111 |
| 40 | `ralph-bridge` | 是 | `beacon goal delegate-ralph` / `implement delegate-ralph` | merge→loop | §3.3 L121 |
| 41 | `loop-goal` | 是 | `beacon goal`（旧名不留两个入口） | merge→loop | §3.2 L110 |
| 42 | `task` | 是 | `beacon goal todo`（run 自带任务清单，含在 goal status） | merge→loop | §3.2 L111 |

### 2.3 并入 qa（6）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 43 | `real-project-validation` | 是（`app.command` 别名） | `beacon qa agent-eval` / `qa agent-score` | merge→qa | §4.1 L157 |
| 44 | `test-progress` | 是 | `beacon qa status` + `beacon spine show` | merge→qa | §3.1 L91 |
| 45 | `deep` | 是 | `beacon qa strategy` / `qa evaluate`（深验收不单设命令） | merge→qa | §3.1 L97 |
| 46 | `verify` | 是 | `beacon qa evaluate` / `gate evaluate` | merge→qa | §3.2 L107 |
| 47 | `auto-test` | 是 | `beacon qa autotest`（执行 tests.md 命令并登记证据） | merge→qa | §3.2 L108 |
| 48 | `eval` | 是 | `beacon qa score` / `qa score-history`（一套统一评分） | merge→qa | §3.1 L89；AC-151..156 |

### 2.4 并入 spine（4）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 49 | `board` | 是 | `beacon spine show` / `spine list`（不做网页看板） | merge→spine | §3.1 L86 |
| 50 | `status` | 是 | `beacon spine show` / `spine list`（run 级另有 `goal status`） | merge→spine | §3.1 L86 |
| 51 | `report` | 是 | `beacon spine show` | merge→spine | §3.1 L86 |
| 52 | `manifest` | 是 | `beacon spine artifacts` | merge→spine | §3.1 L91 |

### 2.5 并入 design（2）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 53 | `prototype` | 是 | `beacon design bind` / `design status`（按 §3.4 只绑不生成，原型交宿主） | merge→design | §3.4 L127 |
| 54 | `ui-spec` | 是 | `beacon design bind` / `design status` | merge→design | §3.3 L120 |

### 2.6 并入 doctor（2）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 55 | `debug` | 是 | `beacon doctor rebuild`（诊断一个入口） | merge→doctor | §3.1 L96 |
| 56 | `lighthouse` | 是 | `beacon doctor rebuild`（巡检项并入 doctor） | merge→doctor | §3.1 L98 |

### 2.7 并入 CLI / 配置（2）

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 57 | `help` | 是（`app.command` 别名） | Cobra 自带 `--help`（自研帮助面不复刻） | merge→cli | §3.1 L87 |
| 58 | `profile` | 是 | 环境变量 + 配置文件（`BEACON_STORE_PATH` 等），不单设命令 | merge→cli | §3.1 L94 |

### 2.8 其余 1+1+1

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查 |
|---:|---|---|---|---|---|
| 59 | `gate` | 是（hidden typer） | `beacon gate evaluate` / `status` / `integrity`（**口径差异**：charter 写顶层名 drop，Go 保留同名但换成正式门评估语义） | merge→gate | §3.2 L106；AC-013 |
| 60 | `workspace` | 是 | `beacon implement worktree` / `goal worktree` | merge→implement | §4.1 L157 |
| 61 | `version` | 是 | `beacon --version`（版本解析进 store/truth 内部，无顶层命令） | merge→内部 | §3.5 L137 |

## 3. drop（16 条）

> 名字与能力都不迁；旧版在 Python 源仓继续存在。逐条实测：Go 侧 `<name> --help` 全部 `absent`。

| # | Python 命令 | Python 有 | Go 命令 / 等价路径 | 处置 | 反查（剥离理由） |
|---:|---|---|---|---|---|
| 62 | `analyze` | 是 | 无 | drop | §3.3 L120：labs 产品分析工具箱，与编码主线无关 |
| 63 | `api-design` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 64 | `data-dictionary` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 65 | `jtbd` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 66 | `kano` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 67 | `moscow` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 68 | `persona` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 69 | `roadmap` | 是 | 无 | drop | §3.3 L120：同上（labs） |
| 70 | `memory-friction-evolution` | 是 | 无 | drop | §3.3 L120：labs 演化实验，与 goal memory 不是一回事 |
| 71 | `evolution-summary` | 是 | 无 | drop | §3.1 L91：就是 spine 能查到的东西换个排版 |
| 72 | `prompt` | 是 | 无 | drop | §3.2 L109：deprecated，能力已被新面覆盖 |
| 73 | `save` | 是 | 无 | drop | §3.2 L109：同上（deprecated） |
| 74 | `work` | 是（`app.command` 别名） | 无 | drop | §3.2 L109：`save` 的别名，一并弃 |
| 75 | `labs` | 是 | 无 | drop | §3.3 L121：容器名删除（子命令已逐项归类） |
| 76 | `bridge` | 是 | 无 | drop | §3.3 L121：容器名删除（run/team/ralph 已逐条 merge） |
| 77 | `truth-cutover` | 是 | 无（能力由 `beacon migrate v1` 承接） | drop | §3.2 L112：一次性切换工具，切换已完成 |

## 4. 计数复核

| 处置 | 本表行数 | charter §4.1 口径 | 一致 |
|---|---:|---:|---|
| keep | 12 | 12 | 是 |
| merge | 49 | 49 | 是 |
| drop | 16 | 16 | 是 |
| **合计** | **77** | **77** | 是 |

复跑命令（应输出 `keep 12 / merge 49 / drop 16 / rows 77`）：

```bash
awk -F'|' '/^\| [0-9]+ \|/{n++; v=$6; gsub(/^ +| +$/,"",v); if (v ~ /^keep/) k++; else if (v ~ /^merge/) m++; else if (v ~ /^drop/) d++} END{printf "keep %d\nmerge %d\ndrop %d\nrows %d\n", k,m,d,n}' docs/capability-parity.md
```

## 5. 与 charter 的口径差异（5 处，只记录不改）

| 差异 | charter 写法 | Go 实际 | 影响 |
|---|---|---|---|
| `program` | keep 为**顶层独立入口** | 挂在 `goal program` 下 | 入口层级不同，能力在 |
| `skill` | keep `skill` | `skills`（复数）+ 子命令 list/show/references/install | 名字不同 |
| `archive` / `hooks` / `review` | keep（slim）/ merge→truth | 命令**可执行，但未列入 `--help` 列表** | 可发现性缺口，非能力缺口 |
| `ask` | merge→loop（goal 意图识别） | 落在 `truth ask/answer` | 归属面不同 |
| `plan` | merge→truth | 落在 `implement plan` / `goal axis` | 归属面不同 |
| `gate` | 「顶层命令名 drop」 | 保留 `gate` 顶层，换成正式门评估语义 | 名字在、语义新 |

## 6. 边界

- 本表只回答「有没有、怎么处置」，不评判处置是否合理；处置口径的权威是 `docs/scope-charter.md`。
- Python 侧「有」= 注册代码里存在该名字（typer 组名或 `app.command` 别名），不等于该命令在当前版本仍被文档推荐；`hidden=True` 的命令（gate/workflow/think）也算有。
- `bin/beacon` 是旧构建（`v2.0.0-go.8`），本表所有 Go 侧结论取自 HEAD 源码重建的 `/tmp/beacon-parity`（`v2.0.0-go.9`）。
