# Beacon Go：需求真相内核 / SDD 规范边界（2026-11-24）

> **2026-11-24 更新**：本文中「goal 从 CLI 隐藏」「不做长跑 driver / supervisor / checkpoint」「runs / checkpoints 移出核心」等段落已被 [ADR-0002](adr/ADR-0002-intent-tree-truth-model.md) 与 spec #15 取代。当前口径：goal 单命令工作流保留；内置 agent 后端与外围面移出。其余段落仍有效。

> 决定：Python Beacon 只作参考，不再维护；Go 线不再追求 1:1 迁移。
> 目标：一个**需求真相内核**，只负责 SDD（Spec-Driven Development）规范的定义、冻结、变更与验收证据绑定。
> 本文是范围决定，不是需求真相；具体 feature 仍走 `beacon truth` 冻结流程。

## 1. 一句话定义

**Beacon Go = SDD 规范内核：把一句话变成可冻结、可变更、可验证、可追溯的需求真相包，并用硬 gate 阻止“没想清楚就开工”。**

它保留一条命令跑完整条工作流（plan→truth→design→implement→verify→release），但不替 agent 写代码、不内置 agent 后端、不负责发版部署。那些是宿主 agent / CI / 人的事。

## 2. 内核必须有的六件事

| # | 能力 | 说明 |
|---|---|---|
| K1 | **需求真相包生命周期** | `truth create → intent → ask/answer → confirm → freeze → change → refreeze → drift/rebase`。正文只在 Markdown，账本只存索引、事件、gate 结果。 |
| K2 | **真相质量门** | 清晰度（模糊词）、FSM 图校验、填充深度、包契约、完整性（AC↔意图↔测试↔任务）、可追溯性、表面声明、复述门。全部 fail-closed。 |
| K3 | **证据绑定** | `tests.md` / `tasks.md` / `evidence.md` 与 AC 绑定；证据追加不改合同指纹；工件按 feature 作用域登记并校验哈希。 |
| K4 | **版本线** | `docs/beacon/<version>/features/<slug>/`；版本线可开、可 bump、可归档；合同指纹按版本线冻结。 |
| K1b | **goal 单命令工作流** | `goal run` 建 run + 阶段计划；`goal tick` / driver / checkpoint / resume 推进；出口 gate、准入、verifier 与人工 release gate 保留；阶段执行走 manual/external lane。 |
| K5 | **只读账本视图** | `spine show/list/artifacts/gate`，JSON 契约稳定，reason code 稳定，退出码可脚本判断。 |

## 3. 内核明确不做

| 不做 | 原因 |
|---|---|
| 1:1 迁移 Python 77 条命令 | Python 是参考实现，不是目标合同。 |
| ~~长跑 driver / supervisor / checkpoint~~（已由 ADR-0002 改为保留） | goal 单命令工作流保留；仅内置执行后端移出。 |
| agent executor（subprocess / ralph / ACP / team lane） | 执行归宿主；内核只验收证据。 |
| program / sea / topology / worktree | 多湖编排与多仓拓扑不是 SDD 规范的最小必需。 |
| 桌面端 / Web 控制台 / agentapi / llm / session | UI 与对话不是真相内核。 |
| v1 迁移 / bench / archive 治理外壳 / hooks | 与 SDD 规范无关；需要时由外部脚本做。 |
| labs 产品分析工具箱、loom 双品牌 | 与编码主线无关。 |

## 4. 目标 CLI 面（最小）

```text
beacon projects register|list|bump
beacon truth create|decompose|intent|ask|answer|confirm|freeze|change|refreeze|drift|rebase|show|list|map
beacon gate evaluate <truth_freeze|design|implement|qa|release>
beacon gate integrity
beacon gate status
beacon spine show|list|artifacts|gate
beacon doctor rebuild
beacon version
```

可选（只有 SDD 包含交付验收时才保留）：

```text
beacon qa evidence        # 只登记/校验验收证据，不跑 agent
beacon release check      # 只做发布门前置检查，人工确认
```

以下命令**从内核 CLI 隐藏**（代码可先保留，标 quarantine）：

```text
implement, design, program, worktree,
delegate-ralph, acp-probe, ui, migrate, hooks, skills install,
archive, review, bench, agent-eval
```

隐藏不等于删除。先隐藏、跑通 SDD 链路、更新测试，再分两批删除死代码。

## 5. SDD 包规范（truth / tests / tasks / evidence）

### 5.1 `truth.md`

必备 frontmatter：

```yaml
slug: <feature-slug>
version: <docs version line>
status: draft | frozen | change_pending
language: zh | en
domain_required: true | false
ux_required: true | false
parser_contract: beacon-feature-package-v2
truth_source_model: feature_package_authoritative
package_maturity: draft | filled
materials_status: draft | current | stale
intent_confirmed: true | false
surfaces: [cli]            # 必填（首次冻结）
canonical_refs:
  truth: ...
  tests: ...
  tasks: ...
  evidence: ...
```

必备章节：

1. User Intent Snapshot（utterance / goal / non_goals / constraints）
2. 人话（给非实现者读的一段）
3. Open Questions（逐条关闭后才能确认）
4. Intent Coverage（每个 AC 必须指向意图来源）
5. Scope（in / out）
6. Execution Contract（CLI/JSON/退出码/reason code）
7. Domain FSM（`domain_required: true` 时必填）
8. Acceptance Criteria（AC 表）
9. Deferred（推迟项必须留账）
10. Evidence Index（只追加）

### 5.2 `tests.md`

- AC ↔ TC 矩阵；每个 AC 至少一个可执行 selector。
- 红基线 → 绿证据；禁止只写“已测试”不写命令。
- 断言变化必须走 `truth change → refreeze`。

### 5.3 `tasks.md`

- 任务 DAG；每个任务绑定 AC；每个任务有证据引用。
- 任务状态只由证据推进，不由声明推进。

### 5.4 `evidence.md`

- 只追加，不改历史行。
- 每条证据含：命令、退出码、时间、source commit、binary SHA、输出路径/hash。
- 合同指纹只覆盖 `truth/tests/tasks/restatement`；**evidence 不进入合同指纹**。

## 6. 冻结与变更规则

### 6.1 冻结（fail-closed）

`truth freeze` 必须同时通过：

1. 结构完整：四件套存在、frontmatter 合法、`surfaces` 非空。
2. 意图确认：`intent_confirmed=true`，Open Questions 全部 answered。
3. 清晰度：无未定义的模糊词（“或者/至少/尽量/等”这类二义表述）。
4. FSM：状态/转移/非法边结构合法；domain 必填，非 domain 不编造。
5. 填充深度：业务域有用户旅程；不得用包生命周期状态机冒充业务 FSM。
6. 完整性：AC↔意图↔测试↔任务交叉引用闭合。
7. 可追溯：每个 AC 有意图来源与证据入口。
8. 复述门：restatement 与用户原话一致，`confirmed_by` 有记录。
9. 分支纪律：只在 canonical 分支冻结；冻结只在提交后生效。

### 6.2 变更

- `truth change`：只登记变更意图，不改冻结正文。
- `truth refreeze`：记录新合同哈希；只允许变更集内的 AC 移动。
- `truth drift`：列出移动过的 AC。
- `truth rebase`：显式接受当前正文，必须带 change id。
- 取消未改动正文的变更：走 `truth change --cancel`；不得改写冻结正文或变更文档字节。

## 7. 账本最小模型

保留：

| 表 | 用途 |
|---|---|
| `projects` | 项目根路径登记 |
| `project_versions` | 版本线（docs root、is_current） |
| `feature_packages` | 包索引（slug、truth/tests 路径、status） |
| `feature_lifecycle` | 阶段/状态/reason codes |
| `gate_results` | 每次 gate 判定与逐项检查 |
| `artifact_refs` | 证据工件（feature-scoped，含 hash） |
| `lifecycle_events` | append-only 事件流 |

移出核心（quarantine）：

`program_*` 运行态、worktree/topology 记录。`runs`、`checkpoints` 属 goal 单命令工作流，保留。

需求正文永远在 Markdown，不进数据库。

## 8. 现有 Go 代码处置

| 处置 | 包/文件 |
|---|---|
| **保留（核心）** | `internal/core`（类型/错误/reason/fingerprint/protected_paths）、`internal/store`（上面的最小表）、`internal/gate`（truth 门）、`internal/orchestrator` 的 `truth*.go`、`intent.go`、`decompose.go`、`truthmap.go`、`feature_map.go`、`gates_eval.go`、`gates_facts.go`、`artifacts.go`、`internal/workspace`、`internal/versionmanifest`、`internal/sddroles`、`internal/changequality`、**goal 编排面：`engine.go`、`tick.go`、`exitgates.go`、`admission.go`、`complete.go`、`guardrails.go`、`driver.go`、`supervisor.go`、`external_stage_evidence.go`、`exec_resolve.go` 的 manual/external lane**、`internal/cli` 的 goal/truth/gate/spine/projects/fsm/helpers/version/doctor |
| **quarantine（代码在，CLI 隐藏）** | `program*`、`topology*`、`worktree*`、`implement*`、`exec_*`、`delegate.go`、`harness/acp.go`（仅握手探针）、`benchgate`、`evalgate`、`stride`、`changequality`、`effectjournal`、`archive`、`hooks`、`skills install`、`migrate` |
| **已删（2026-11-26 包袱清理）** | `internal/session`（desktop 遗留、零调用）、`internal/progressreview`（仅冻结 truth、实现未落地）、`harness/ralph.go`（内置 ralph 后端）、`harness/acp_session.go` 与 `AcpBackend`（ACP 全会话后端）、`AgentBackend` 接口；28 个残留间接依赖经 `go mod tidy` 移除。删除前 build/test/vet 全绿、kernel-deps 守卫通过。保留 `ParseArgvFlag` 与 `ProbeACP`。 |
| **删除（第二遍）** | quarantine 中确认无 SDD 依赖、测试已迁移的部分 |

原则：先隐藏 CLI，后删代码；删除前必须有 SDD-only e2e 绿。

## 9. SDD-only 验收链路（替代当前全链 e2e）

```bash
beacon projects register --path <tmp> --version v0.1.0
beacon truth create --project <tmp> --slug demo
beacon truth intent --feature demo --utterance "..." --goal "..." --non-goals "..." --constraints "..."
beacon truth ask --feature demo --question "..." --recommend "..."
beacon truth answer --feature demo --id Q1 --answer "..."
beacon truth confirm --feature demo
beacon truth freeze --feature demo
beacon truth change --feature demo --summary "widen AC-001"
beacon truth refreeze --feature demo
beacon truth drift --feature demo
beacon gate integrity --feature demo --evaluator verifier --json
beacon spine show --project <tmp> --feature demo --json
```

验收标准：

- 干净 clone、无 Python 依赖、无外部 agent。
- 冻结失败路径必须非零退出并给稳定 reason code。
- 变更只动变更集；冻结正文字节不变。
- 证据追加不改变合同指纹。
- `spine` JSON 字段稳定。

## 10. Python 参考清单（只读，不迁）

以下能力**不是 SDD 内核必需**，需要时再单独决策：

- `ask` 意图路由（auto/rule/llm）
- `feedback` / `friction-intake`
- `report` / 运行时投影
- `profile` 项目画像
- `prototype` / `ui-spec`
- `prd-bootstrap` 内容生成
- playwright 扩展验收、loop patterns、remediation sandbox
- 定时 `task`
- agent eval 实测回放
- labs 产品分析工具箱

## 11. 风险

1. **goal 是核心，不隐藏**：`goal run/tick/driver/checkpoint/resume` 与 runs/checkpoints 保留；e2e 改为 manual/external lane，不再依赖内置 subprocess 执行器。
2. **store schema 契约**：runs/checkpoints 属核心，不能移；仅当移除 program/topology 专用表时才需要显式迁移并同步 `VerifySchemaContract`。
3. **测试面大**：`internal/*` 有大量 goal/executor 测试；goal 相关保留，内置 executor 后端相关随 quarantine 迁移或删除。
4. **版本线 docs 与运行时版本**：两轴不要混；发布时对齐，平时分开。
5. **不要用“命令隐藏”冒充“能力删除”**：隐藏只是过渡，最终要么保留为核心，要么删除并记录决定。

## 12. 下一步

1. 确认本边界（哪些进核心、哪些 quarantine）。
2. 先做版本规范化（`docs/version-scheme.md`）与 SDD-only e2e。
3. 再按 K1–K5 收敛 CLI；删除放在最后。
4. 任何“保留/删除”决定都要有一条冻结 truth 记录，避免再次出现“账面无能力、代码有实现”或反之。
