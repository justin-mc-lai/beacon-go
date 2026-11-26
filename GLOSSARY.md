# GLOSSARY

本仓的领域词汇。写 issue、spec、ADR、测试名时用这里的词，不要换同义词。

| 术语 | 含义 |
|---|---|
| **意图树（intent tree）** | 需求的唯一真相模型：根意图逐层拆到不可再拆的叶子。 |
| **intent** | 节点 kind：意图层，可递归嵌套；根节点是最终用户意图。 |
| **module** | 节点 kind：模块层，只能挂在 intent 下，可嵌套 module。 |
| **feature** | 节点 kind：功能边界层，只能挂在 module 下；业务域 FSM 挂在这一层。 |
| **touchpoint** | 节点 kind：用户触达点，唯一可作叶子的类型；必须带可执行验证规格。 |
| **叶子（leaf）** | 没有子节点的 touchpoint；冻结前必须有 AC 与验证规格。 |
| **冻结（freeze）** | 把节点正文与 AC 固化为不可原地修改的版本，并记录指纹。 |
| **refreeze** | 变更后自底向上重新冻结；父节点绑定有序子指纹集合。 |
| **级联失效（cascade）** | 任意后代 change 后，全部祖先立即变为 `change_pending`。 |
| **验证绑定（verification binding）** | 叶子声明“怎么证明做对了”的方法（可执行测试或具名人工验收 + 工件哈希）。 |
| **真相指纹（truth fingerprint）** | `tree-json-v1`：对 canonical JSON 与 AC 计算的内容哈希；证据追加不改指纹。 |
| **版本线快照（version snapshot）** | 某条版本线冻结后的不可变树集合；未变节点按指纹引用。 |
| **canonical JSON** | 键排序、字段顺序固定、UTF-8、无空白歧义的 JSON；仓库内的权威源。 |
| **quarantine** | 代码保留但从 CLI 隐藏的过渡状态；不等于已删除。 |
| **goal** | 单命令工作流入口：`goal run` 建 run + 全阶段计划，驱动 plan→truth→implement→verify→release（固定五阶段，无 design）；阶段执行归宿主 agent。 |
| **湖海（sea/lake）** | 旧需求模型：扁平 journeys + lakes + features；已退役，仅作旧数据导入来源。 |
| **宿主 agent（host agent）** | 在仓库里干活的编码 agent；负责调外部 LLM、执行叶子任务、提交证据。 |
| **内核（kernel）** | beacon 本体：真相存储、门禁、冻结、账本视图；不调 LLM、不跑 agent。 |

## 执行器与推进

内核不执行 agent。下表区分常被混用的五个词：

| 词 | 含义 |
|---|---|
| **run** | 一次交付实例（goal run），从创建到 release。 |
| **stage** | run 的阶段计划项（plan/truth/implement/verify/release）。 |
| **step** | `goal flow` 的固定八步流中的一步。 |
| **task** | 需求包 `tasks.md` 里的一行任务。 |
| **turn** | `goal driver` 的一次循环推进（tick + heartbeat + checkpoint）。 |

`--executor` 只有三种有效值：`manual`/`external`（宿主执行，`goal tick --by` 记录）、`recording`（无真实工作的占位）。`--exec-cmd`/`--ralph-cmd`/`--profile` 只作为宿主配置记录在 run 上，内核从不执行它们。旧的 `subprocess`/`ralph` 选择器已被拒绝。

## 内容哈希的命名

同一 SHA-256 机制按覆盖范围有几个名字，不是几种算法：

| 名称 | 对象 | 随证据变化？ |
|---|---|---|
| `tree-json-v1` 真相指纹 | 节点 canonical JSON + AC | 否 |
| `pkgcontract-v1` 合同指纹 | truth/tests/tasks/restatement 四文件 | 否 |
| `pkgsurface-v1` 表面指纹 | 含旧四层文件的包表面 | 否 |
| `content_hash` / `hash=` | 注册工件的字节 | 不适用（工件本身） |

## 退出码

对外字段统一叫 `reason_code`（常量 `Reason*` 定义在 `internal/core/reasons.go`）。常量名与字符串值不完全同名——值是 v1 契约，禁止改写；查值请用 `core/reasons.go` 的常量而非字面 grep。`qa` 作为 gate 类型（`GateQA="qa"`）是冻结 JSON 值，保留；命令面已并入 `verify`。

## 作用域标识

`scope_id` 在不同命令里指不同东西，读到时先看它的 `scope_kind`：

- `--scope-kind node|feature --scope-id <id>`：证据归属的需求节点（`goal artifacts`）。
- `GitScope.ScopeID`：多仓拓扑里的 git 仓库标识（`git_root/role`）。
- `artifact_refs.run_id` 里的 `node:<id>` / `feature:<id>` 前缀：为兼容旧 schema 而复用的作用域引用，**不是** goal run id。

字段名暂不统一——统一需要 SQLite schema 迁移与 JSON 契约变更。新代码不要再把作用域塞进 `run_id`。

新增术语时先确认它是否只是已有概念的同义词；同义词一律不加。
