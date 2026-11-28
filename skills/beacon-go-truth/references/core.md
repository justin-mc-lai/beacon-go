# Truth core reference

## 持续树与版本范围

逻辑一棵需求树持续成长；canonical JSON 是唯一权威，SQLite 只是可重建索引。
需求正文保留 Markdown，HTML 由既有 CLI 渲染器派生，不建立第二需求账本。
每个版本保存不可变 baseline + 本轮 delta：baseline 记录已批准的历史合同，
delta 指明新增、修改、移除的节点及来源、指纹、依赖和本轮验收范围。
版本并非每轮复制或重建一套互相竞争的需求真相。

可同时存在 future draft，明确排除在本轮范围与发布依赖闭包之外，因而不拖本轮发布。
若本轮依赖尚未完成的 draft，必须解决依赖或取得范围取舍，不能仅改标签绕过内核。
这描述宿主工作流合同；CLI 若对范围隔离仍返回缺项，记录阻塞，不能宣称已支持或绕过。
已发布 baseline 保持不可变；本轮完成不等于所有 future draft 已完成。

## truth 完成条件

1. 将 plan 的调查、范围取舍与可选候选树整理为本轮唯一 canonical 树。
2. 宿主按既有工具命名 `plan init/expand/apply` 形成节点；每次展开不超过七个子节点。
3. 每叶有意图来源、可观察 AC、验证命令；覆盖清单列明 covered/excluded/unknown。
4. `truth tree --format html` 产出真实 HTML，记录路径、source_hash；用户审阅批准当前内容。
5. `truth node freeze` 自底向上执行并保留回执。叶冻结、父汇总、证据登记、
   harness 只读 hash 校验与 gate/release 评估分层，不互相冒充。

已有树的后续增删改、重组与 draft 调整统一走 change；无需用户再解释技术绑定清单。
内核只校验与记录，宿主生成语义、执行命令并处理 next/missing。
