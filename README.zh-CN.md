# Beacon Go

[English](README.md) | **简体中文**

Beacon Go 是单二进制的**需求真相内核**。它把一句话变成意图树，只在叶子有可执行验证后才允许冻结，记录实现证据，自底向上汇总验证，最后经人工确认才发布。

树是唯一需求真相：

```
intent -> module -> feature -> touchpoint
```

每个节点有不可变 id、canonical JSON 正文、`tree-json-v1` 指纹，以及五态生命周期。海/湖模型已退役；旧包以 `feature` 节点导入。

当前版本：**v3.0.3**（纯 SemVer，见 [version-scheme.md](docs/version-scheme.md)）。发布说明：[docs/beacon/v3.0.3/release/3.0.3.md](docs/beacon/v3.0.3/release/3.0.3.md)；迁移指南：[docs/migration-3.0.2.md](docs/migration-3.0.2.md)。

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

七个命令组，一棵树：

| 命令组 | 入口 | 做什么 |
| --- | --- | --- |
| **plan** | `beacon plan …` | 建树与展开：`init` 建根，`expand` 只校验提案，`apply` 写入 draft；记录展开 provenance（模型、prompt 哈希、tokens、成本）。 |
| **truth** | `beacon truth …` | 自底向上冻结节点；叶子必须有 AC 和验证规格，module 必须有子节点；渲染树（HTML 为三栏工作台：全树 / 当前需求 / 对应 FSM）、生成不可变快照。 |
| **change** | `beacon change …` | 改一棵子树：后代变 draft，全部祖先变 `change_pending`；再自底向上 refreeze。变更记录不可改。 |
| **implement** | `beacon implement …` | 宿主干活，`implement report` 记录叶子、执行者、产物哈希与失败原因。 |
| **verify** | `beacon verify …` | 按 feature/node scope 登记证据、验证叶子、汇总父级、审计树；原 qa 门禁与评分也在这里。 |
| **release** | `beacon release …` | 发布门与人工确认；根未 verified 不放行。 |
| **goal** | `beacon goal …` | 单次长跑的驱动：建阶段计划、把阶段判定委托给树门、verifier + 人工确认后才完成。 |

横切能力：

- **证据门**：验证 = 已登记产物 + 哈希，内核不信散文。
- **宿主执行**：没有内置 agent 执行器，也没有内核 LLM client；宿主用 `goal tick --by` 记账。
- **不可变历史**：冻结指纹 + 版本快照。
- **依赖门**：已删的 LLM/HTTP/执行器面不得回到二进制（`scripts/check_kernel_deps.sh`）。

## 快速上手

需要 Go 1.27+。

```sh
go build -o bin/beacon ./cmd/beacon
./bin/beacon --version          # Beacon CLI v3.0.3
```

典型树流程：

```sh
beacon projects register --path /path/to/project --version v0.1.0
beacon plan init  --project . --title "my product" --intent "一句话"
beacon plan expand --project . --node <id> --children proposal.json   # 只校验
beacon plan apply  --project . --node <id> --proposal proposal.json   # 写 draft
beacon truth node freeze --project . --node <id>                     # 先冻叶子
beacon implement report  --project . --node <leaf> --by dev --artifact proof.txt
beacon verify leaf       --project . --node <leaf> --artifact proof.txt --by qa
beacon verify rollup     --project . --node <parent>
beacon verify audit      --project . --json
```

从根开始建树；子节点未全部就绪时，父节点冻结会被拒绝。`proposal.json` 由宿主 agent 生成（见 `beacon-go-plan` skill）。

### Skills

二进制内置 7 个 skill。安装到宿主 skill 目录后，宿主能发现 `beacon-go` 路由与各能力 skill。

| Skill | 什么时候用 |
| --- | --- |
| `beacon-go-goal` | 一句话要跑成一段有阶段的交付 |
| `beacon-go-plan` | 规划、展开树、评审、FSM、调研、棕地接手 |
| `beacon-go-truth` | 建包、签字、冻结、变更 |
| `beacon-go-design` | UI 实现前绑定设计原语 |
| `beacon-go-implement` | 准入、worktree 隔离、实现回报 |
| `beacon-go-verify` | 证据、叶子验证、父级汇总、审计、评分、并行 code review |
| `beacon-go-release` | 发布门与人工确认 |

```sh
beacon skills list                     # id、名称、一句话用途
beacon skills show plan                # 完整 SKILL.md
beacon skills hosts                    # 检测到的宿主目录
beacon skills install --host codex     # 写入 beacon-go-<id> 与路由
beacon skills install --dir <path>     # 指定目录
beacon skills verify --host codex      # 已安装与内嵌是否漂移
```

安装名带命名空间（`beacon-go-goal`、`beacon-go-verify` …），不会和别的 Beacon 安装冲突。原 `qa` skill 已改名 `beacon-go-verify`。

## 验证情况

- `go test ./... -count=1`：行为级测试全绿。
- `bash scripts/check_version_consistency.sh`：版本只有一个源。
- `bash scripts/check_kernel_deps.sh`：已删面不得回归。
- [`e2e/run_e2e.sh`](e2e/run_e2e.sh)：manual lane 全链（注册 → 需求包 → 冻结 → 基线 → goal run/tick → 证据 → 门禁 → verifier → 完成）。
- [`e2e/run_dogfood.sh`](e2e/run_dogfood.sh)：羽毛球队列 22 节点 / 12 叶子 / 2 快照；冻结、实现、验证、汇总、change+refreeze、HTML、审计全过。
- v3.0.2 证据汇总：[docs/beacon/v3.0.3/release/3.0.3.md](docs/beacon/v3.0.3/release/3.0.3.md)。

## 已知边界（如实写明）

- 内核不跑 agent。没有宿主时，goal run 会停在原地说清楚原因。
- orchestrator 内部 `program_*` 实现仍编译在包内，但没有 CLI/API 入口，属待删死代码。
- `wails.json` 仍保留：它绑在冻结的 desktop 需求包上；desktop 与 HTTP 面已在 3.0.0 移除。
- 真实模型驱动的 skill eval 不在 CI 内；当前 skill eval 是结构校验。

## 文档

各期实现报告在 [`docs/`](docs/)：`phase0-inventory`（命令盘点）、
`scope-charter`（收敛口径）、`p0`–`p6` 各期报告、两份修复报告、
以及标准流与拓扑的 `stdflow-s1/s2/s3`。全部用大白话写，推导口径
与取舍都明示。

## 与 Python 版的关系

Python 仓（`justin-mc-lai/beacon`）是冻结 v2 设计的原始出处，保持
不动；本仓是该设计的 Go 实现，日常使用以本仓为准。老项目用
`beacon migrate v1` 搬过来。

## 协议

[MIT](LICENSE) © 2026 Kunqi Lai

## 维护者

**Kunqi Lai** (<goodlive.lkq1128@gmail.com>)
- GitHub: [@justin-mc-lai](https://github.com/justin-mc-lai)

