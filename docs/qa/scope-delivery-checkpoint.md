# 范围交付实施检查点

状态：部分实现，S1-S6未核销，不得作为整体验收或发布结论。

## 范围锚点实施与真实链验证（2026-11-28 三）

- 用户确认门禁语义为“显式范围锚点”：不放宽旧单feature唯一性，由合同显式 run_anchor 指定 run 的整体验收入口。
- WorkflowContract 增加 run_anchor 与 acceptance_nodes。run_anchor 必须是 canonical feature、在 scope 内（或其祖先在 scope 内）、且 slug 解析到 run.feature；acceptance_nodes 不排期，但其 touchpoint leaf AC 纳入 required。旧合同无 anchor 时仍走唯一 slug 校验。
- 红测 TestRunAnchorResolvesAmbiguousFeature 证明：同名 f1/f2 在旧规则下 workflow_feature_ambiguous；声明 wrapper 为 anchor 后消歧；错配 anchor 仍拒绝。
- Portfolio prepare_stable_workflow.py 生成 run_anchor=n_a9a97c8f1a90（super-agent-delivery），acceptance_nodes 含两份 wrapper；scope_nodes 28、source_nodes 106、coverage 106，11 模块与 26 排期 feature 不变。
- 临时 binary 实跑 run_6efbf90a118c1607c0fcbde5：先显式 rebind（合同变化），check 从 workflow_feature_ambiguous 变为 workflow_artifacts_missing；prepare 成功，prepared_hash=e67480bc0dc70db8c6e966b8170cda989f210ad15e8051311b3c8aae5c3acf6e；下一门禁为 workflow_confirmation_required，必须由人类 reviewer 审查 HTML/coverage 后 confirm。
- 最新全量 go build/test/vet/version/kernel/E2E PASS（tool_222b2482c001Wq0W2GUGGJOb4X）。产品实现仍未提交、未安装、未发布；Portfolio 未 confirm/freeze/verify/release。

## 后续推进（2026-11-28 二）

- S3 全范围冻结门禁：scopeFrozenAdmissionItem 接入 checkAdmission，检查 scoped 中全部活跃 feature/touchpoint 为 frozen/implemented/verified/delivered；deprecated 跳过；draft -> scope_node_not_frozen，change_pending -> scope_change_pending。legacy 无绑定 not_applicable；无树/合同不可解时交由 workflow 门禁，不双重报错。Red（门禁未接入/桩返回 not_applicable）→ Green（4 项测试 PASS）。
- S2 任务核销投影：DeliveryTask.Status 从 canonical 叶子状态派生（pending/implemented/verified），纯只读投影，不建第二账本，不影响合同指纹与任务ID。测试证明状态变化不改身份。
- 复审 ses_ddd97ba87ffe7mxW6xur99LL69 发现旧绑定无 ScopeFingerprint 时可静默接受范围变化；已修为 workflow_scope_unbound 并要求显式 rebind，新增测试。复审其余发现：双重拒绝信息（两个独立门禁同根因，均阻塞，不影响安全）、Status 是投影非证据（已在注释/文档明确）、driver 并发入口无锁（未修，见下）。复审后未再次独立复审。
- 最新 full build/test/vet/version/kernel/E2E 全通过，输出 tool_222682c30001S2UhiVlmbDXTwM。

## 已知未覆盖边界（按复审）
- driver 的 load/Tick 不在 program.lock 内，LastTickKey 判重使用启动时快照；双执行者仍可能重复 tick。RecordInterrupt/Heartbeat 也未与持锁 Resume 共享互斥域。
- DeliveryTask.Status 只是派生视图，不能替代 S4 证据门；整体验收仍由 treeStageFacts/validateWorkflowDelivery 负责。
- 全范围 committed baseline 尚未逐 feature 检查；当前范围冻结以 canonical tree 状态为准。
- 跨进程 bind/checkpoint 恢复、S4 集成失败、S6 各入口统一与旧 program 路径尚未实现。
- Portfolio 的 workflow_feature_ambiguous 仍需改 run 身份语义（gate 语义变更，须用户确认）；本轮未放宽旧校验。

## 后续推进（2026-11-28）

- 绑定完整范围时，binding与workflow_tasks_planned同事务持久化；相同输入重复绑定不追加事件。
- 范围任务ID和指纹纳入ProjectID/VersionID命名空间。
- 显式绑定记录scope_fingerprint，树变更后拒绝静默重算；显式rebind写新计划。
- 无活跃任务的deprecated中间模块传递前置任务，避免依赖截断。
- checkpointOn仅接受事务前采集的绑定hash/范围指纹；Checkpoint和Resume持program锁。
- Resume拒绝旧checkpoint跨新绑定复用，且不消费interrupt。
- Driver在执行前采集绑定，执行后持program锁重评；变化时保留本轮计数与原始checkpoint并阻塞，不提前返回丢失收尾。
- 新增双goroutine同步屏障bind唯一计划探针及race通过；尚未等价为双进程证明。
- 独立review ses_dddb7d14effeXEPO4ZS6mrzJ1U发现三问题后已修正，修正后独立复审未执行；不得报告review全绿。
- 最新全量build/test/vet/version/kernel全部exit0，E2E RESULT PASS；完整输出tool_2224e29cf0016DgRbHVJin62Px。
- 最新workflow check范围指纹拒绝补丁后需定向回归；全S1-S6仍未完成。

以下为前次检查点历史，后续推进段优先；其中“checkpoint未实现”等描述不再代表当前局部实现。

## 已授权基线

- a76d0cf：canonical新增scope-delivery-runtime feature与六叶S1-S6，经change grow、plan expand/apply、bottom-up freeze生成。
- 7be1c56762b0975bfaeb5bd35319f549d26cdf70：goal run自动生成的tree-derived package兼容基线。
- feature节点 n_d8566e18c1a1，需求指纹254c91ec8b0c13b2d033d19f875c585c554fabc972185148adeee83874d0d174。
- run run_ef052571c184b4ba742e55ae，executor scope-delivery-maker；当前阶段plan（不得把admission pass当阶段已完成）。
- truth baseline实际登记committed；implement admit实际返回goal_implement_admission_ok。
- 仅提交需求基线，产品实现未提交；未push、安装、发布。

## 本轮部分实现

- scope_delivery.go：resolveWorkflowDelivery复用原合同校验，按明确节点ID构造确定性DAG任务视图、稳定任务ID与范围合同hash。
- workflow.go：validateWorkflow统一调用resolver；旧workflowRunFeature校验保留，尚不能让Portfolio整体run通过prepare。
- goal_workflow.go：新增只读goal workflow tasks --run-id，展示计划，不创建执行回执或通过门禁。
- resume.go：stable恢复前重评当前合同/artifacts，合同改变时阻塞且不消费interrupt/清soft block。
- scope_delivery_test.go：解析/依赖顺序/重放/生命周期hash隔离/需求hash变化、8拒绝探针、合同改变恢复拒绝。
- Portfolio prepare_stable_workflow.py：修复派生合同遗漏依赖，引用已确认module ID/depends_on，重绑仍明确workflow_feature_ambiguous。

## 实跑证据

Red1：TestScopeDeliveryResolution因scope delivery resolution unavailable行为失败；Green后通过。
Red2：TestScopeDeliveryResumeRejectsChangedContract显示旧Resume返回ok/replay并消费interrupt；Green后返回blocked且interrupt保留。

本轮最终主体候选执行go build ./...、go test ./... -count=1、go vet ./...、version/kernel检查均exit 0；
E2E RESULT PASS（/tmp/beacon-e2e.UESilJuX），包含预期拒绝探针。最后临时checkpoint实现回退后定向回归通过。
输出位置：OpenCode tool_221599a36001oo3oPEV0BTTwld（全量+E2E）、随后全量检查调用输出。

## 必须接续

1. 补充实际CLI tasks成功/拒绝/只读探针；任务计划加入明确项目/版本命名空间（当前ID version+node，node为随机全局ID但未显式project绑定）。
2. 自动绑定与任务创建要有单写入口、幂等和真并发屏障；当前仅read-only任务视图，不能报S2完成。
3. 全范围baseline/admission、任务账本、集成证据、独立verifier、Complete及旧program旁路尚未修正。
4. checkpoint完整绑定/跨进程恢复尚未实现。一次尝试在checkpointOn的store事务内调用workflowBinding造成嵌套store读取超时，已完整回退；接续必须在事务前采集并在锁内核对，不在tx中调用Engine store读取。
5. Portfolio包装层11AC尚未进整体验收合同；不能将26feature95AC范围冒充全部106AC已验收。
6. 最终固定候选重新全部必需检查与独立review；独立验收未执行。

旧reconcile/move实现与e2e日志仍为工作区既存未提交改动，不能混入本次需求基线提交。
