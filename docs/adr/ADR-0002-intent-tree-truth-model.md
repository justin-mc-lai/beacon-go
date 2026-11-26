# ADR-0002: 意图树是唯一需求真相模型，goal 单命令工作流保留

- Status: accepted
- Date: 2026-11-24
- Supersedes: `docs/sdd-kernel-scope.md` 中「goal 从 CLI 隐藏」「不做长跑 driver/supervisor/checkpoint」「runs/checkpoints 移出核心」的段落
- Authority: spec issue #15（产品边界）+ 本 ADR（架构决定）；两者冲突时以 #15 为准

## Context

项目此前有两套需求拆解口径：

1. Python v1 的「海/湖 + feature 平表」：`Decomposition{ScopeMode, Journeys, Lakes, SubIntents}`，无父子层级，叶子不可判定，变更只能按单 feature 包做。
2. 2026-11-24 提出的「意图树」：根意图 → intent/module/feature/touchpoint，递归冻结、级联失效、叶子验证绑定。

原型（#17）在两个领域做了双拆解对比：羽毛球社区与命令行笔记同步。两样本均满足「≥3 项不劣 + 叶子验证绑定率 100%」；同时暴露出旧引擎对这两个领域都产不出 journeys/lakes，且 readiness 标记不一致。

另一处冲突：`sdd-kernel-scope.md` 曾把 goal/driver 列为隐藏项，但用户在评审中明确要求保留 goal 单命令工作流，只移除内置 agent 后端。

## Decision

1. **意图树是唯一需求真相模型**；海/湖退役，旧 feature 包 1:1 导入为 `feature` 节点。
2. **goal 单命令工作流保留**：`goal run` 建 run + 全阶段计划，`goal tick` / driver / checkpoint / resume / verifier / 人工 release gate 全部保留。
3. **内核不调 LLM**：删除 `internal/llm` 直连与 ChiefOfStaff 路径；外部 LLM 由宿主 agent 按 skill 契约调用，内核只校验、记账、冻结。
4. **内置 agent 后端移出**：subprocess / ralph / profile / ACP 执行与 program / topology / desktop / agentapi 等外围面不进内核。worktree 除外——它是核心能力（见 AGENTS.md 真源 #3）。
5. **真相存储**：仓库内 canonical JSON 是权威源；SQLite 是可重建索引缓存；Markdown 与 HTML 是带来源哈希的渲染物。
6. **命令面**：`plan / truth / change / implement / verify / release / goal` 七组；`qa` 并入 `verify`，不留旧名。
7. **版本**：以上决定按 AGENTS.md 唯一真源执行；版本轴已重置，从 `0.0.1` 重新开始。

## Consequences

- `sdd-kernel-scope.md`、`scope-charter.md` 中与上述决定冲突的段落视为历史，不再作为当前边界。
- 冻结指纹改为 `tree-json-v1`（canonical JSON），旧记录按 `fingerprint_alg` 分派，不改写。
- schema 合同变化，需要显式迁移与 `VerifySchemaContract` 更新。
- 树节点数约为湖海的 2–2.5 倍（#17 实测），review 必须依赖 `truth tree` 聚合索引，不能逐节点读文件。
- 竞品机制（artifact schema DAG、analyze/converge 分离、稳定 id）采用记录见 spec #15 与 `docs/prototypes/competitor-analysis.md`。

## Evidence

- spec：[#15](https://github.com/justin-mc-lai/beacon-go/issues/15)
- 原型与指标：`docs/prototypes/intent-tree-vs-lake/`
- 竞品分析：`docs/prototypes/competitor-analysis.md`
- 返工根因：`docs/rework-retrospective-2026-11-24.md`
