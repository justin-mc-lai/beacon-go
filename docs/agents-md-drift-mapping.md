# AGENTS.md 九项规范比较

范围：#47，feature `n_ca6fe1322405`，叶 `n_86e6a7a520c7`。本文说明实现与验收边界，需求合同以 canonical JSON 为准。

```sh
beacon plan drift --project . --agents-md --json
```

裸入口读取项目根 `AGENTS.md` 和 `docs/agents-md-drift-map.json`，沿用现有项目、文档版本和节点作用域。`--agents-map` 可指定项目相对路径或绝对路径。比较直接读取当前 canonical 节点，不使用快照；显式 `--snapshot` 被拒绝。检查只读。

## 九项 canonical 对应字段

用户批准后，九条完整编号规范已通过 change request → node update → bottom-up freeze 写入现有 feature 的 AC 字段。映射 JSON 只保存编号与字段选择器，不保存规范正文、期望值或基线哈希。AGENTS.md 的权威顺序保持不变；需求合同变化仍须走 change/refreeze。

| 编号 | canonical 节点 | 字段 |
|---|---|---|
| 1 | n_ca6fe1322405 | /acs/NORM-001/text |
| 2 | n_ca6fe1322405 | /acs/NORM-002/text |
| 3 | n_ca6fe1322405 | /acs/NORM-003/text |
| 4 | n_ca6fe1322405 | /acs/NORM-004/text |
| 5 | n_ca6fe1322405 | /acs/NORM-005/text |
| 6 | n_ca6fe1322405 | /acs/NORM-006/text |
| 7 | n_ca6fe1322405 | /acs/NORM-007/text |
| 8 | n_ca6fe1322405 | /acs/NORM-008/text |
| 9 | n_ca6fe1322405 | /acs/NORM-009/text |

当前九条正文一致时，返回 `evaluated: 9`、`blocked: 0`、`changed: []`。修改规范正文时，对应 `items` 标为 `changed`；`changed` 数组按节点去重，所以同 feature 的多个规范变化只产生一个节点记录。治理内容修改不进入需求漂移报告。字段缺失或显式 gap 仍逐项 blocked，并非零退出；不能将缺合同解释为对齐。

## 确定性合同

产品规范定位为 `唯一真源（最高口径）` H2 下的顶层连续编号项，每项必须且只能绑定一次。禁止整段 H2 映射。其他 H2 必须逐个显式分类为治理，产品 H2 不能归入治理。

可比较字段为 `/title`、`/intent`、`/doc`、`/acs/<稳定AC ID>/text`。AC 选择器按稳定 ID 查询，不使用数组偏移；重复 ID 或不存在的 ID 拒绝。比较只归一化 CRLF 与首尾空白；Markdown、否定词、内部空白保留。无 LLM、关键词相似度或语义推断。

整个映射与文档始终校验，作用域只过滤评估结果。未知节点、路径别名 ID、空集合、空字段、重复 JSON key、重复编号、未知字段、未分类 H2、未闭合围栏、零作用域项均拒绝。规范差异是报告成功，合同缺口则输出诊断并返回 `agents_md_drift_blocked`。

H1、产品编号前的说明段是文档元信息；本合同只比较九项编号正文。新增规范应新增编号，新增 H2 需重新分类。围栏内的 H2 不当作章节。编号必须是 `1. ` 形式，前导零、乱序、重复编号拒绝。

## 公开 CLI 验证合同

叶的机器合同执行 `go test ./e2e/driftcontract -run ^TestPortableAgentsMD`，绑定该独立包的完整测试文件和两个 required tests。计数与 JSON 参数由 capture 正常归一化添加。

- `TestPortableAgentsMDGovernance`：隔离复制真实 AGENTS 与 canonical 树，通过公开 CLI 建立 fixture；九项对齐，追加治理说明仍对齐。
- `TestPortableAgentsMDReverse`：反转真实第二条规范，要求九项均被评估且只有第二项 changed。

测试只依赖标准库，从受测源码构建生产 CLI。构建失败在 TestMain 作为前置阻断退出，不能算作行为 Red。fixture 的变更使用其当前 canonical 指纹，经过公开 change/update 入口，不修改主项目。

真实九项文本对齐只证明规范正文一致，不证明九项产品行为全部交付。正式叶子仍须同一候选的有效 Red/Green、独立评审和合法回执，不能由字段比较直接获得 verified 或发布批准。

## 同类入口扫描

| 调用点/入口 | 测试 | 拒绝或差异路径 |
|---|---|---|
| `newPlanDriftCmd` 默认与显式映射 | `TestPlanDriftAgentsMDNumberedCLI` | 显式快照、无模式带 map、空白 map、零作用域、合同阻断 |
| 真实仓库公开入口 | `TestPlanDriftAgentsMDRealRepositoryAligned` | 九项实际对齐，不能靠 fixture 代替 |
| `AgentsMDDrift` 映射规范化 | `TestAgentsMDNumberedComparison` | 空集合/字段、重复 key/item/target、未知节点/字段、别名 ID |
| canonical 字段读取 | `TestAgentsMDCanonicalFieldsAndWhitespace` | 重复 AC ID、非法选择器、空 canonical 文本 |
| Markdown H2/编号解析 | `TestAgentsMDNumberedComparison` | 无编号、重复/额外编号、未分类 H2、未闭合围栏 |
| 真实九项字段 | `TestAgentsMDRepositoryMappingIsAligned` | 治理修改不变，第三项反转只定位对应规范 |
| 生产 CLI 跨历史验证 | `TestPortableAgentsMDGovernance`、`TestPortableAgentsMDReverse` | 构建失败与行为失败分开；第二项反转定位 |
| 原有 snapshot drift | 原有 `TestPlanDrift*`、`TestDrift` | 保留原快照合同，agents 模式不访问快照 |

未覆盖边界：不判断语义等价，不比较编号前元信息，不验收全部产品行为；比较器不执行 gate、release、证据登记或生命周期迁移。
