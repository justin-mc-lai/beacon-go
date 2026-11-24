# 开源 SDD 竞品拆解机制分析（2026-11-24）

> 目的：回答两个问题——(1) 开源竞品怎么驱动 LLM 拆需求、怎么收敛功能边界并交付；(2) beacon 的意图树该采用/不采用哪些机制。
> 方法：直接读各仓 HEAD 的 README、模板、命令 prompt、schema 与 CLI 文档；只记录仓内可查的事实。
> 性质：调研记录，不是需求真相，不构成 gate verdict。

## 1. 样本

| 项目 | 类型 | 核心工件 |
|---|---|---|
| [GitHub Spec Kit](https://github.com/github/spec-kit) | 命令/模板包 | constitution、spec.md、plan.md、tasks.md |
| [OpenSpec](https://github.com/Fission-AI/OpenSpec) | CLI + schema + skills | changes/<name>/{proposal.md, specs/**, design.md, tasks.md} |
| [BMAD-METHOD](https://github.com/bmad-code-org/BMAD-METHOD) | 角色 agent + 工作流 | initiative/epic/story + `tickets/tickets.toml` + 计划文件 |
| [Superpowers](https://github.com/obra/superpowers) | skills 包 | brainstorming → design doc → implementation plan |
| [Taskmaster](https://github.com/eyaltoledano/claude-task-master) | CLI + 任务图 | `tasks.json`（依赖 DAG、子任务、复杂度报告） |

## 2. 各自的拆解模型

### Spec Kit

- 结构：feature 级文档链。`spec.md` 按优先级写**可独立测试**的用户故事（Given/When/Then + Independent Test）；`plan.md` 写技术上下文、项目结构，并有 **Constitution Check** 门；`tasks.md` 按 Phase 分组，任务带 `[P]`（可并行）与 `[USn]`（所属故事），并声明阶段依赖。
- 扩散：每个命令是一份模板 prompt + 前置脚本；`clarify` 补问题，`checklist` 做质量清单。
- 收敛：`analyze` 是**只读**跨工件一致性审计（缺失、重复、歧义、欠规格）；`converge` 是**只追加**——只把未完成工作追加为新任务，绝不改 spec/plan。
- 交付：agent 逐任务实现；implement↔converge 循环到 `Converged`。

### OpenSpec

- 结构：capability 级规格 + 变更增量。一个 change 四个工件：`proposal.md`（Why/What/Capabilities）→ `specs/<capability>/spec.md`（ADDED/MODIFIED/REMOVED requirements，每条带 Scenario）→ `design.md` → `tasks.md`。
- 扩散：`schema.yaml` 声明工件依赖图（`requires`）、每个工件的 instruction 与模板；CLI 的 `status` / `instructions` 计算“现在只能写哪个工件”（ready/blocked/unlocks）。Capability 清单（`list --specs`、`show`）强制复用既有边界；零 delta 必须显式 `skip_specs`。
- 收敛：`openspec validate` 做结构/语法校验；`archive` 把 delta 合并进主 specs；写作规范要求一条 requirement 一个 SHALL/MUST、可观察、scenario 能变成测试。
- 交付：apply 读 tasks 与上下文执行；archive 保留历史。

### BMAD-METHOD

- 结构：initiative → epic（先薄信封，`refined:false`）→ story/bug；稳定 id 存在 `tickets/tickets.toml`，每个 epic 一个目录。六个角色 agent：Analyst、PM、PO、Architect、Dev、UX。
- 扩散：PO 先 refine 未决点（现在定 / 开 spike / 留给故事），再按**需求覆盖、依赖、验证方式**切片；支持 12 种规划文档；`advanced-elicitation` 用独立读者视角把初稿推离“模型最可能的答案”。
- 收敛：接受拆解前先 review；code review 追加带日期的审查块；epic 结束跑 retrospective 写证据与 verdict；replan 时保持 id 稳定、点名过期条目、不重写已完成内容。
- 交付：`bmad-build` 一次一票；计划文件承载状态与基线；auto 模式一次只派一票，不自己挑下一票。

### Superpowers

- 结构：brainstorming → design doc → implementation plan。计划写给“没看过代码库和 spec 的工程师”，必须写清文件、接口签名、每个任务对应的测试。
- 扩散：brainstorming 一次只问一个问题，先找“为什么”；计划按**可独立测试的交付物**切任务；步骤粒度是“一个动作 + 一个可检查结果”（写失败测试→跑它→最小实现→跑通→提交）。
- 收敛：每任务独立可测 + 任务级 review + `verification-before-completion`；测试先行；子代理执行时每个任务单独 review。
- 交付：`executing-plans` 或 `subagent-driven-development`，复选框跟踪。

### Taskmaster

- 结构：PRD → `tasks.json`：id/title/description/status/dependencies/priority/details/testStrategy/subtasks；另有单任务文件与复杂度报告。
- 扩散：`parse-prd` 生成任务；`analyze-complexity` 给每个任务打 1–10 分并推荐子任务数；`expand` 按推荐展开（复杂度高的先展开）。
- 收敛：依赖图 + `next` 只挑依赖满足的任务；`testStrategy` 是每任务的验证建议（文本）。
- 交付：agent 执行任务并更新状态。

## 3. 共同套路

1. **仓库文件是状态，agent 无状态**：spec/plan/tasks 都在 git 里；CLI 负责校验、路由、状态计算。
2. **扩散靠“模板 + 依赖 DAG”**，不靠自由发挥：先把“下一步只能写什么”编码进 schema/命令，再让 LLM 填内容。
3. **收敛分三层**：结构校验（validate/analyze）→ 人工 review → 追加式补齐（converge / 新增任务 / replan）。
4. **验证基本是建议级**：testStrategy、checklist、review 都是提示词或人工门；没有内容哈希、冻结指纹、证据绑定。
5. **树都很浅**：capability 路径 1–2 层；epic→story 2 层；Taskmaster 的树是执行 DAG，不是意图树。

## 4. beacon 采用 / 不采用

### 采用

| 机制 | 来源 | 落到哪 |
|---|---|---|
| 工件依赖 schema（requires/unlocks） | OpenSpec | `plan expand` 只允许对“当前可展开”的节点动手；非法跳层给稳定 reason code |
| 只读审计 + 只追加修复 | Spec Kit `analyze`/`converge` | `verify` 审计零写入；修复只能由 `change` 追加任务/变更记录 |
| 稳定 id + 覆盖/依赖切片校验 | BMAD | 节点 id 不可变；父节点汇总校验子覆盖与依赖 |
| delta 语义（ADDED/MODIFIED/REMOVED） | OpenSpec | `change` 归档把节点变更合并进版本线快照 |
| “写给陌生人”的计划粒度 | Superpowers | 叶子执行模板必须含精确接口、测试与步骤 |
| 复杂度→子节点数 | Taskmaster | 替换拍脑袋的“≤7 子节点”，按复杂度算扩散预算 |
| tracer-bullet 竖切片 + blocking 边 | mattpocock/skills | 票的切法（本仓已用） |

### 不采用

| 机制 | 原因 |
|---|---|
| 纯散文工件作为唯一真相 | 竞品的 validate 只查结构，testStrategy 只是文本；beacon 的差异必须是可执行证据 + 哈希 + 冻结门 |
| 多角色 agent 组织（BMAD 六角色） | 宿主 agent 已经承担；内核只保留契约与门 |
| 强制 Phase 1/2/Polish 骨架 | 小改动会被过度流程化；按切片右尺寸 |
| `tickets.toml` 与 issue tracker 双写 | 违反单一权威源；工作项用 GitHub issues，需求真相用仓库树 |
| 归档即改写主 spec | beacon 用不可变快照 + 节点级 change 记录，历史不重写 |
| 以 advisory `testStrategy` 作为验收 | 叶子必须有可执行验证规格，否则拒绝冻结 |

## 5. 大树 JSON 的 review / 分析性能

实测（本机，Python，作为上界；beacon CLI 是 Go）：

| 场景 | 规模 | 耗时 |
|---|---:|---:|
| 一次读取整棵 canonical JSON | 12,497 节点 / 3.3 MB | 12.8 ms |
| canonical 序列化 + SHA-256 | 同上 | 19.0 ms |
| 逐节点文件读取 + 解析 | 10,000 个文件 | 799.8 ms |

结论：**整树加载和哈希不是瓶颈，N+1 文件读取才是**。因此：

- `tree.json` 索引必须自带 review 所需字段（id/parent/kind/status/title/fingerprint），普通 review 不读节点正文。
- 读单个节点才加载 `node.json`；批量场景走索引。
- 父冻结记录绑定子指纹集合，形成 Merkle 式结构：子树 diff 只重算变化节点，不重哈希全树。
- 命令面：`truth tree`（索引）、`truth tree --node <id>`（单节点）、`truth tree --diff <snapshot>`（子树 diff）、`verify status`（汇总）、`release check`（门）。
- 大结果集提供 `--jsonl` 流式输出，避免一次性拼大 JSON。
- 目标（待 Go 实现后实测，不冒充已验证）：10k 节点索引加载 < 50 ms；子树 diff 与变更节点数成正比；review 查询 < 100 ms。

## 6. 风险

1. 竞品的“validate/analyze”是结构门，不是语义门；beacon 若只做到同层，会被认为没有差异。
2. 树节点文件数量是竞品的 5–10 倍；没有索引视图会把 review 成本吃光。
3. 采用 schema DAG 后，`plan expand` 的拒绝路径会变多，必须配矩阵测试，否则又回到“修一个入口漏两个”。
