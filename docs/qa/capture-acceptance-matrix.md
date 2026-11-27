# Capture 入口验收矩阵

需求来源：`red-green-capture` 三个冻结叶子及 `strict-delivery-contract` 六叶。
本表是测试与缺口登记，不是 verified 回执；规范状态仍以 canonical tree 为准。

## 入口矩阵

| 入口 | 成功证据 | 拒绝证据 | 范围 |
|---|---|---|---|
| implement capture | TestCaptureRealProcessAndTestHash | TestCaptureExplicitRedPair、TestCaptureRejectsZeroSkipAndEnvironmentFailure | 真实 Go 测试，先 Red 后 Green |
| verify leaf / MarkVerified | TestVerifyLeafAndParentRollup、TestCaptureBinaryE2ESuccessAndRefusal | TestCaptureRejectsHandwrittenDelivery、TestCaptureMakerVerifyHasNoRegistrationSideEffect | 含直接服务绕过探针 |
| verify rollup | 同上 | TestCaptureRollupRefusalHasNoPartialWrites、TestDeliveryReceiptStaleBlocksRollupAndDelivery | 拒绝前不写入；不宣称 IO 故障原子性 |
| truth node deliver | TestTruthNodeDeliverRequiresVerifiedChildren、binary E2E | TestDeliveryReceiptStaleBlocksRollupAndDelivery | 复验已 verified 证据 |
| verify evaluate | TestCaptureGoalFullSuccessChain：实际 capture 与产品 QA 执行 | binary E2E：候选改变后拒绝 delivery_capture_stale | reviewer 文档为合成 fixture |
| release evaluate | TestCaptureGoalFullSuccessChain、TestCaptureReleaseConfirmationRunIsolation | 无 owner 确认拒绝、stale capture 拒绝、旧 run 不借新 run 确认 | 有树 SDD 未核销账本专测仍需补充 |
| goal tick | TestCaptureGoalFullSuccessChain：QA 推进 release | binary E2E：stale capture 拒绝 | QA 前阶段由 fixture 定位，不冒充走完所有前置阶段 |
| goal complete | TestCaptureGoalFullSuccessChain：人类确认后完成 | binary E2E：stale capture 拒绝 | TaskStatus 的 SDD 模式需独立专测 |

binary E2E 构建并调用真实 beacon 程序；review JSON 为合成 fixture，不能声称独立 reviewer 已验收产品。

## 独立逐叶核对

独立 reviewer：Task session `ses_de387e012ffetaNYNWruZrtj5z`，只读检查与定向测试。
`satisfied` 表示其检查范围满足，不代表具备可登记的全部来源材料。

| 冻结叶子 | 核对结论 | 未关闭项 |
|---|---|---|
| n_a11841bbc44c plan-first | 宿主合同范围 satisfied | 缺真实节点 capture 合同配置与候选固定，不能登记 verified |
| n_a3b9d2c7f296 external-ledger | 现有合同与实现范围 satisfied | 同上 |
| n_ea9234b5e82c deterministic-tools | 现有合同与实现范围 satisfied | 同上 |
| n_5eaf5288ed83 capture-process | blocked | 自定义机器结果协议未实现；预检失败不保留 capture 尝试 |
| n_d8063fefcf41 pair-and-register | blocked | 未提供两个真实执行者同步屏障证明；登记检查/写入并发原子性未证明 |
| n_54606a05e1b6 transition-integration | blocked | 新 capture 到 direct gate / goal 的完整成功链和 SDD 账本拒绝未全覆盖 |
| n_a78ea52722a0 red-green | blocked | 预期业务失败的独立语义确认；预检中取消/超时无完整采集证据 |
| n_ec5052dd2cc5 blind-review | blocked | 现有审查是代码修复审查，未形成九叶逐项 Green capture 绑定回执 |
| n_6ee4eee57ba1 evidence-gate | blocked | 无固定到已提交源码的最终证据包及完整 diff 扫描处置记录 |

## 真实登记前置阻塞

1. 九叶状态 frozen，尚不满足 verify leaf 的 implemented 前置条件；不为登记而伪造实现状态。
2. 九叶 Verification.Spec 是已冻结的文字验收规格，而捕获器要求 JSON beacon-capture-command.v1。
   修改冻结合同必须另走 change request/refreeze；本轮不直接改 node JSON。
3. 本仓 tracked 实现与未跟踪新增源码尚未提交。自动证据加头会记录 source_state=dirty，
   ValidateEvidenceFile 会拒绝。不漏写 source_state、不借用旧 HEAD 假称 committed，也不擅自提交。
4. 原始 Red 实跑发生在新增测试时，现存九叶尚未配置采集合同；不能事后制作假的 Red capture。

本轮后续补修已增加 TestCapturePreflightFailureRetainsEvidence、
TestCaptureConcurrentProcessesUseBarrier、TestCaptureCustomProtocolRedGreenAndRejections
和 TestCaptureGoalFullSuccessChain；上表独立逐叶结论是补修前快照，不能自行改称 reviewer 已复审通过。
补充实跑：TestCapturePreflightProcessFacts 覆盖命令缺失、真实预检 stdout/stderr、exit 7 和超时；
custom valid_alias 覆盖可执行符号链接的 Red/Green 配对；
TestCaptureGoalSDDConnectedLedger 在同一 capture 树/同一 SDD 合同/run 中，
验证 verify/release evaluate 在账本未运行时拒绝，实际 implement tasks run 后验证、QA tick、complete 与 release 放行。
该链仍以 fixture 定位 QA 阶段与前置 gate，未声称覆盖规划/签名/准入的完整二进制全阶段流程。
TestCaptureSDDLedgerExecutedAndStale 另证实际 SDD TaskStatus 成功及候选改变后的拒绝。
connected fixture 又增加同一 run 的 goal tick/complete 未执行账本拒绝，
以及任务报告被改写后 verify/release evaluate、tick、complete 四入口的拒绝。
这里通过改写排除目录下的已登记任务报告隔离验证 ledger guard，不把 capture stale 当成账本拒绝证据。
因此新增验收测试与矩阵保留真实结果，九叶均不升级 verified。
下一步需先关闭剩余功能缺口，再确认冻结配置变更和 source commit 授权；
按新合同实际执行 Red/Green、逐叶独立验收后方可登记。
