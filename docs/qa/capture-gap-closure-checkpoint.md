# Capture 缺口关闭 checkpoint

当前阶段：实现和回归补强；未变更冻结采集配置，未提交、未登记真实叶子 verified。

新增代码：custom-json 有效协议与绑定校验；预检真实进程事实 CapturePreflight；
预检失败建立 rejected attempt；custom 执行路径配对前规范化；DAO 对 capture_result 刷新 SQL 拒绝。

新增实跑证据入口：

- TestCapturePreflightFailureRetainsEvidence：预先取消仍保留登记。
- TestCapturePreflightProcessFacts：启动缺失/超时/exit 7，核对真实预检输出。
- TestCaptureConcurrentProcessesUseBarrier：两个实际 go test 进程同步屏障，不同 attempt/ID。
- TestCaptureCustomProtocolRedGreenAndRejections：含可执行符号链接配对，零 case/skip/错误调用绑定拒绝。
- TestCaptureGoalSDDConnectedLedger：同树、同 SDD 合同、同 run，真实任务执行及 capture；
  verify/release evaluate、goal tick/complete 的放行及未执行/报告改写拒绝。
- TestCaptureSDDLedgerExecutedAndStale：SDD 任务真实执行成功与输入改变拒绝。

独立 reviewer session ses_de387e012ffetaNYNWruZrtj5z 多轮只读复查，确认路径/预检修复、
custom 别名与 SDD 成功连通；最后指出的 tick/complete 账本拒绝又补测试并实跑通过，
该最后测试增量尚未由 reviewer 再复审，不宣称其已批准。

完整 build、test、vet 与版本/依赖/合同脚本曾通过；最后一轮全量测试首次出现
TestEvalConformanceMissingTC 的 qa_execution_stale，单独重跑和随后全量重跑通过。
此偶发失败根因未确认，不隐去。

仍需办理：冻结文字 Spec → 可执行采集 JSON 的 change/refreeze 授权；
候选 source commit 授权；最后增量独立复核；逐叶真实 Red/Green 与独立语义回执。
fixture 的合成 reviewer 文档不代替真实逐叶回执。
