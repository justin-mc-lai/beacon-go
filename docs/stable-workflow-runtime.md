# Stable workflow：实际运行合同

实现源：`internal/cli/goal_workflow.go`、`internal/orchestrator/workflow*.go`。
canonical JSON 是唯一需求真相。workflow JSON 是 run 的目标、调度范围和来源映射合同，不是第二棵需求树。
**当前仍由宿主编写合同、处理 canonical ID 和命令。** 用户决定目标、取舍并审阅批准；内核不自动完成语义规划，也不代替用户批准。

## CLI（已核对实际 help）

```bash
beacon goal workflow bind --run-id <run> --file <workflow.json> --by <owner>
beacon goal workflow prepare --run-id <run>
beacon goal workflow check --run-id <run>
beacon goal workflow confirm --run-id <run> --by <identity> --prepared-hash <hash>
beacon goal status --run-id <run> --json
beacon goal tick --run-id <run> --by <host> --json
```

`goal --help` 可见 workflow。四个 workflow 子命令直接输出 JSON，没有独立 `--json` flag。
project/version/feature 来自显式 run，不是 workflow 子命令的 flag。
confirm 必须传 prepare/check 返回的 **prepared_hash**，不是 source_hash；没有 `--source-hash` flag。

- bind 仅在 plan/truth，记录合同 hash、原文件绝对路径及文件 hash；修改文件必须显式重绑。
- prepare 生成 `.beacon/workflows/<run>/prepared-*/index.html` 与 `coverage.json` 并追加准备事件，不改 canonical、不批准。
- check 输出判断；`ready=false` 时返回失败。bind/prepare/confirm 写入成功不保证阶段 ready，要读取返回的缺项。

## Schema 与节点填写

plan 无树时的最小草稿：

```json
{
  "schema": "beacon-workflow.v1",
  "goal": "提交后得到明确结果",
  "scope": "本轮只交付提交功能",
  "scope_nodes": ["planned-feature-id"]
}
```

plan 只要求非空目标、范围和无重复的显式 scope 草稿，不要求节点已经存在。
truth 时需绑定完整合同；示例中的 feature 与 leaf 都有同 ID、同文的 AC-001：

```json
{
  "schema": "beacon-workflow.v1",
  "goal": "提交后得到明确结果",
  "scope": "提交功能；future 功能不在本轮",
  "scope_nodes": ["f-submit"],
  "source_nodes": ["f-submit"],
  "modules": [
    {"id": "submission", "node_id": "m-orders", "depends_on": [], "features": ["f-submit"]}
  ],
  "coverage": [
    {"source_node": "f-submit", "ac": "AC-001", "target_node": "t-submit"}
  ]
}
```

- `scope` 是文字说明，不能代替 `scope_nodes`。所有节点字段用 canonical ID，不用 slug、文件路径或 latest。
- `scope_nodes` 是本轮子树根集合，包含其后代，根不能重叠。选整个 root 就纳入其全部活跃功能，不会自动只选完成部分。
- `modules[].id` 是合同内依赖名称；`depends_on` 引用这些名称。重复、未知依赖及环拒绝。
- `node_id` 是 canonical module ID，`features` 是该模块内的 canonical feature ID。活跃范围内每个 feature 必须有唯一归属。
- `source_nodes` 独立声明原始 AC 来源，不从 coverage 反推。范围内带 AC 的非叶节点必须列为来源；leaf 可只作 target。leaf 自映射须显式列为来源。
- 每个活跃来源 AC 恰好覆盖一次，每个活跃目标 leaf AC 至少一个来源。多个同文来源可归并一个 target；AC ID 或正文不同则拒绝。
- 活跃目标 leaf 必须有 AC 与验证规格；零活跃目标不能通过。deprecated 子树中的未完成 draft 不阻塞 prepare，但墓碑仍在 HTML 与审阅合同指纹中。
- 来源集合、调度与覆盖随合同、manifest、confirm 一并批准。schema 没有独立 unknowns 字段，也不自动求解依赖闭包；未知项与依赖取舍仍由宿主处理。

## 阶段与 confirm

| 阶段 | 实际判断 |
| --- | --- |
| plan | 调查、头脑风暴、方案决策，绑定目标/范围草稿；**不要求树、HTML、AC**。 |
| truth | 树存在，结构与覆盖有效，当前活跃 scope 冻结，HTML/manifest 有效且明确确认；保留既有 truth 出口。 |
| implement | 继续复验合同、产物、确认与范围冻结，保留 admission/worktree/review。 |
| verify | 当前范围活跃 feature/leaf 已 verified/delivered，所有范围叶子证据复验，保留 QA 门禁；内部阶段值仍为 qa。 |
| release | 本轮范围验证及证据，保留 review、独立验证者、人工发布批准；workflow confirm 不代替发布确认。 |

prepare → 读取真实 HTML/覆盖清单 → 用户批准 → 显式 confirm → check → tick。
内核记录传入的确认身份，不自动鉴别人类身份或执行浏览器审阅。
确认绑定准备代、binding hash、scoped tree contract hash、HTML hash、manifest hash。
重新 bind/prepare 都使旧确认失效，即使内容相同；合同/产物变化会阻塞。
生命周期、实现记录、验证证据不进入审阅合同 hash，正常推进不要求逐阶段重确认。
HTML 复用既有 renderer，输入含当前范围、显式来源、墓碑与祖先导航壳；合法的范围外 future 变化不强制重备。
bind/prepare/confirm 使用 project 级跨进程文件锁，confirm 再比 expected hash，只写捕获的 A；随后生成 B 不继承批准。

## 回执、只读与 legacy

回执字段：run_id、mode、current_stage、ready、missing、next_action、owner、module_count；有准备产物后还有 html_path、coverage_manifest、source_hash、prepared_hash。
ready 只回答当前阶段可否推进，不是全树/版本总体 PASS。

- status/check 只读文件和账本，不生成产物、不补批准、不启动 agent。
- tick/complete、direct admission、ExitGate、direct gate 共用底层事实，避免 WorkflowCheck→admission→WorkflowCheck 递归。
- stable direct gate 必须显式 run，不借其他 run 的确认。
- 无 binding 的 run 为 legacy，沿用旧门禁，不代表 stable 通过。
- 删除 stable 合同文件只会阻塞，不退回 legacy；没有删除 binding 的降级命令。

## 两轮交付迭代

第一轮：plan 无树绑定草稿，truth 形成并冻结本轮范围；prepare、明确确认、验证与人工 release 分别保留真实回执。
第二轮：沿同一逻辑树生长，后续节点修改统一走 change；版本 baseline/delta 和树位置解析归版本/lineage 实现。
新 run 显式填写第二轮 scope/source，bind、prepare、confirm 新合同，不复制第二需求权威。
未来 root 尚未 verified 不自动拖住当前范围；宿主仍须选齐本轮依赖，本模块不证明依赖闭包语义完整。

测试依据：`internal/orchestrator/workflow*_test.go`、`internal/cli/stable_workflow_test.go`。
审查和并发证据见 `docs/qa/stable-workflow-checkpoint.md`。
