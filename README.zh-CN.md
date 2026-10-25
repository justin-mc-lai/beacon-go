# Beacon Go

[English](README.md) | **简体中文**

Beacon Go 是 Beacon 内核的 Go 语言重构版。它保留 Python 版 Beacon 的
纪律——*先想清楚、冻结需求真相、照着真相写代码、拿证据验收、人点头
才发布*——同时剥掉 1.x 时代积攒的冗余、过时和与主线无关的摊子。

成果是一个单文件二进制，核心是 **goal 工作流引擎**：长时编码任务
自己排阶段、在硬门禁下逐步推进、断了能续跑，并且永远不能绕过独立
检查者和人工确认自称完成。

当前版本：**v2.0.0-go.9** · 327 个行为级测试 · 端到端全链跑通。
go.9 补齐了对 Python v1.6.19 对等审计查出的 26 条高/中缺口
（报告见 `docs/parity-audit-python-v16.md`）。

## 为什么重构

Python 版 Beacon（v1.6.19，77 条顶层命令）能用，但有三个毛病：

- 门禁是“软”的——靠宿主 agent 的提示词自觉，而不是可执行的判定；
- 表面长胖了：三套查状态的面、三套评分、beacon/loom 双品牌、还有
  一层兼容老命令压着真命令；
- 慢：每条命令都要把全项目状态从文件重算一遍（约 1 秒才开始干活）。

Go 版按一份书面口径执行（[`docs/scope-charter.md`](docs/scope-charter.md)）：
77 条命令收敛为**保留 12、合并 49、丢弃 16**，goal 底下的 22 条工作流
全量迁移。需求真相只存在于 Markdown 文档，运行状态只存于一个 SQLite
账本，任何功能不许再自带 JSON 状态文件。

## 它能干什么

六大能力，一个总入口：

| 能力 | 入口 | 干什么 |
| --- | --- | --- |
| **goal** | `beacon goal …` | 编排总入口。一句话建任务，自动排阶段（想清楚 → 冻需求 →〔需要界面才加设计〕→ 写代码 → 验收 → 发布），用 tick/司机逐步推进，卡住留原因码，存档断点续跑，完工必须在发布门口、挂独立检查者、且有人确认。 |
| **truth** | `beacon truth …` | 需求包（truth.md + tests.md + tasks.md + evidence.md）：建包、冻结、给冻结稿登记变更（不动正文）、重新冻结。冻结有分支纪律（只能在主分支），且提交了才算数。 |
| **design** | `beacon design …` | 带界面的活，先绑好设计规矩才准写代码。内核只管“规矩绑没绑”，不画图。 |
| **implement** | `beacon implement …` | 三种干法（单人 / 团队交接 / ralph 循环）共用一套准入检查、一套收活标准；真执行外部命令，真建 git 工位（卫星只能往主工位合并）。 |
| **qa** | `beacon qa …` | 拿证据验收：正式门禁评估并把结论写进账本、多评审聚合、一套统一评分引擎（评分是体检分，永远不能代替门禁放行，也永远不改被评的 skill）。 |
| **release** | `beacon release …` | 发布门：各门全绿 + 人工确认，缺一就拦住并逐项点名。 |

横切的硬机制：

- **四道门正式评估**（冻需求 / 设计 / 写代码 / 验收，及发布门）：
  读真事实、出结论、写账本；另有需求完整性检查，专查四件套互相
  引用对不对得上。
- **长跑看护**：司机一轮轮推任务（租约防双开、预算封顶、到发布门
  口自停），监工靠心跳判卡死，急停开关一放全停，人中途改方向会
  被各入口持续摆出直到确认。
- **标准执行流**：八步固定顺序从主分支写真相到恢复开工，跳步直接
  拦死。
- **拓扑门**：认得单仓库、多包大仓、多仓拼工作区三种形状；能加载
  v1.6.7 的作用域图谱并投影为分支上下文；准入必须带拓扑结论。
- **受保护路径**：卫星工位的 `.env`、密钥、auth、支付、迁移类文件
  永远合不进主线。
- **迁移与重建**：`beacon migrate v1` 把 Python 版项目幂等导入；
  `beacon doctor rebuild` 让账本和磁盘真相重新对齐，永不删行。

## 快速上手

需要 Go 1.27+。

```sh
go build -o bin/beacon ./cmd/beacon
./bin/beacon --version          # Beacon CLI v2.0.0-go.9
```

日常五条命令：

```sh
beacon projects register --path /项目/路径        # 注册项目
beacon truth create --project . --slug my-feature   # 建需求包骨架
beacon truth freeze --project . --feature my-feature
beacon goal run "给登录页加二次验证" --project . --feature my-feature
beacon goal driver run-loop --run-id <任务id>       # 推到发布门口
beacon spine show --project . --feature my-feature  # 看唯一账本视图
```

### 先问清楚，再签字

需求包结构完整不等于想清楚了。意图门要求：记录用户原话、关掉开放问题账本、
每条 AC 都能追溯到意图来源，然后显式签字：

```sh
beacon truth ask      --feature my-feature --question "这个功能给谁用？" --recommend "..."
beacon truth answer   --feature my-feature --id Q1 --answer "..."
beacon truth intent   --feature my-feature --utterance "用户原话" --goal "..." \
                      --non-goals "..." --constraints "..."
beacon truth confirm  --feature my-feature        # 还有开放问题就拒绝
beacon truth freeze   --feature my-feature        # 完整性 + 一致性 + 意图
```

需求变更后，按 AC 级别自检：

```sh
beacon truth change   --feature my-feature --summary "放宽 AC-001"
# 编辑需求包，然后：
beacon truth refreeze --feature my-feature
beacon truth drift    --feature my-feature        # 指出哪条 AC 动了
beacon truth rebase   --feature my-feature --change <id> --accept-current
```

六个公共 skill 已随二进制打包：

```sh
beacon skills list
beacon skills show truth
```

运行状态只存一个全局 SQLite 数据库（`~/.beacon/beacon.db`，WAL
模式）。需求真相永不进库，只住在
`docs/beacon/<版本>/features/<slug>/` 下的 Markdown 包里。

## 验证情况

- `go test ./...`——327 个行为级测试全过。
- [`e2e/run_e2e.sh`](e2e/run_e2e.sh)——真二进制全链剧本（注册 →
  建包 → 冻结 → 提交基线 → 真执行器干活 → 司机推进 → 四门评估 →
  挂检查者 → 完工），15 步，退出码 0。
- [`docs/ac-matrix.md`](docs/ac-matrix.md)——冻结 v2 的 171 条验收
  标准逐条终局：有测试 81、已实现 64、待外部条件 17、按拍板不迁 9。
- [`bench/`](bench/)——性能门禁。读路径比 Python 版快 50–200 倍；
  写路径已按“合并落盘”改造（见 `docs/p6-fix-write-perf.md`）。还剩
  一项（本机共享磁盘较慢，评估写结论的延迟）高于阈值，如实报，
  不调阈值迁就。

## 已知边界（如实写明）

- **ACP** 只到探针级（initialize + session/new 两步握手），真会话
  迭代等有真 ACP 程序时再验。
- **厂商执行器**（codex / claude 命令行）已按档案模板接好端到端
  形状，真机验证等已登录的真程序。
- 四个外部验证项待外部条件（`docs/p6-implementation.md` 第 4 节）；
  AC 层面为 17 条 `deferred-external`。逐条前置条件、复跑命令与责任
  角色见 [`docs/deferred-external.md`](docs/deferred-external.md)。

## 文档

各期实现报告在 [`docs/`](docs/)：`phase0-inventory`（命令盘点）、
`scope-charter`（收敛口径）、`p0`–`p6` 各期报告、两份修复报告、
以及标准流与拓扑的 `stdflow-s1/s2/s3`。全部用大白话写，推导口径
与取舍都明示。

## 与 Python 版的关系

Python 仓（`justin-mc-lai/beacon`）是冻结 v2 设计的原始出处，保持
不动；本仓是该设计的 Go 实现，日常使用以本仓为准。老项目用
`beacon migrate v1` 搬过来。
