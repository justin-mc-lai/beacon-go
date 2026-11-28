# 范围交付：单 feature 假设全链路核查

状态：调查与实施规格；未实现、未独立验收。
依据：用户确认按“需求树 → 本轮范围 → run → 证据”修正完整链路。
canonical intent tree 为需求权威；本文是变更提案与调用点扫描，不替代冻结需求。

## 已核实的调用链

| 链路 | 实际入口/函数 | 当前假设及影响 | 修正后必须验证的拒绝路径 |
| --- | --- | --- | --- |
| 创建与上下文 | `CreateRun`、`workflowInput` | run 用 FeatureID 定位项目/版本；不能把定位容器直接视为整个范围的需求权威 | 跨项目、跨版本、未知范围、空范围拒绝 |
| 绑定与准备 | `BindWorkflow`、`validateWorkflow`、`workflowRunFeature`、`PrepareWorkflow` | 合同已有多模块/scope_nodes；仍要求run slug唯一匹配且属于排期feature | 重复/重叠范围、身份冲突、缺失AC、未授权重绑拒绝 |
| 自动拆分 | `ModuleRun`、`moduleChildrenInDeclarationOrder`、`admitModuleChild` | 只展开单module直接feature；每次CreateRun，非整个范围的可恢复幂等拆分 | 重放不重复创建；依赖循环/未知依赖、非冻结子节点拒绝 |
| 执行准入 | `checkAdmission`、`buildExitInput`、`evalBaselineMerged` | workflow范围事实之后仍读取单feature FrozenFact/package/baseline | 全范围冻结/确认/基线/worktree/dirty/topology均保留，不能因包装层冻结而放行 |
| 阶段出口 | `treeStageFacts`、`buildExitInput` | 前者已查scope冻结/verified；后者仍查单feature生命周期与formal gate | 局部PASS不能让整体推进；缺范围证据拒绝 |
| 任务账本 | `taskPackage`、`TaskStatus`、`validateDeliveryTasks` | tasks绑定单feature package；非SDD包装层直接跳过任务账本要求 | 任一必需任务未完成、缺账本、候选/合同漂移拒绝；零任务不算成功 |
| 证据与独立验收 | `validateWorkflowDelivery`、`currentQACandidate`、`currentQAReview`、`AttachVerifier` | 叶证据已有scope验证；产品候选/report/refs仍从run单feature上下文派生 | 错范围、旧候选、旧指纹、缺整体链路证据、自审、篡改拒绝 |
| 中断恢复 | `Checkpoint`、`checkpointOn`、`Resume`、`decideResume` | 保存阶段/子步骤，但checkpoint无范围合同/需求指纹；Resume按旧阶段决策，可能清soft block | 合同变化后先重评；中断重放不丢失败历史、不重做已核验任务、不清human gate |
| 发布与旧program旁路 | `Complete`、`ProgramComplete`、`releaseFacts` | feature级生命周期/发布事实；旧program有另一条汇总/确认路径 | 总体和局部证据都齐全且人工批准；旧program入口不能绕范围检查 |

## 统一模型与实施顺序

1. 定义唯一范围解析结果：项目、版本、明确节点ID、完整依赖闭包、活跃feature/leaf集合、整体验收集合、合同指纹。
   validate/persist/query共用该结果。名字和路径仅供展示/发现，不能决定证据归属。
   复用workflow contract；数据库旧FeatureID可暂作兼容定位字段，但不作为范围验收权威。
2. 从该结果确定性生成任务DAG，声明依赖与稳定任务ID。只拆已确认工作，不在内核调用LLM/agent。
   任务创建/重放使用同一写入口，结果追加到既有事件与artifact账本，不建第二事实源。
3. admission/阶段出口/任务账本/候选/独立审查/发布共同消费范围结果。
   叶级证据与整体验收证据分开要求；汇总不等于逐项PASS，逐项PASS也不等于整体PASS。
4. checkpoint与恢复绑定合同hash、需求指纹、候选及任务引用。
   恢复先重评事实；需求变化走change，重算影响，不复用过期批准。
5. 更新实际CLI fixtures与E2E，最后才替换workflowRunFeature的旧约束。
   不先删除或绕过它，也不以单项prepare成功代替端到端验收。

## 原子验收任务

- S1 范围规范化：单feature/多模块成功；空白/重复key/重叠/别名/反向归属/跨版本/零活跃目标拒绝。
- S2 自动拆分与绑定：按依赖生成稳定任务；重试/恢复幂等；未知/循环依赖拒绝；并发写入需双执行者同步屏障探针。
- S3 准入与执行：冻结和committed baseline覆盖整范围；worktree仍为硬门禁；包装层不能冒充范围冻结。
- S4 证据汇总：局部成功但集成失败保持阻塞；缺失/篡改/过期/自审证据拒绝；所有必需任务核销。
- S5 中断恢复：跨进程恢复保留成功与失败历史；范围/候选变更使相关结果失效；human gate不能由resume/recover清除。
- S6 完整入口：status/check/admit/tick/prepare/confirm/tasks/verifier/complete/release及旧program路径一致。

每项先实跑行为Red，再Green；最终运行仓库必需检查及全新上下文独立审查。

## 实施准入与当前检查点

本轮未写产品实现。现有Portfolio run不是beacon-go内核变更的实施run。
实际运行 `beacon-go implement admit --run-id run_6efbf90a118c1607c0fcbde5` 返回blocked：
`workflow_feature_ambiguous`、`canonical_refreeze_not_committed`、缺truth baseline、需求包dirty及topology阻塞。
不能以删除歧义检查、自签冻结、擅自commit/stash或伪造baseline绕过。

接续：先将本提案纳入beacon-go本次变更的canonical需求与run，按change/refreeze确认冻结并取得真实实施准入。
需求确认/提交仍走现有授权边界；Portfolio继续停在truth，不推进implemented/verified或发布。

未覆盖：本表是已读关键入口的静态扫描；所有间接调用和旁路尚未穷尽，S6实现前须补齐全引用清单。
