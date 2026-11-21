---
slug: kernel-qa-evidence-hardening
confirmed_by: justin
confirmed_at: 2026-11-21T10:11:49Z
hash: e76736063e2e727d1c43ed9eb90b1832415c52e4c6f1a99d840000a70bed01d9
confirmation_evidence: /Users/apple/Developer/Personal/products/portfolio/docs/decisions/2026-11-21-kernel-qa-hardening.md
---
## 1 交付形态
修复 Beacon Go 通用 QA 门：测试须实际运行，证据绑定同一候选，独立结论有效，所有推进入口采用同一判定。先完成模型路由与此修复，随后恢复第二湖。

## 2 界面与交互
沿用现有 truth、qa autotest、gate、goal、artifact 和 spine CLI。查询要显示被验对象、实际执行情况、独立检查者和拒绝原因。OpenMausBot 相关模型统一为 magpie 的 DS V4.1 Flash 路由组，真实工具任务验证后使用。

## 3 业务流转
开发前独立审查高风险验收场景；开发中持续自测；固定提交和二进制后由独立检查者验收。空跑、跳过、缺失用例、错 run、旧候选、自验、合同审查冒充产品 QA 均须拒绝；完整合法证据可以通过。最新失败不得被历史成功掩盖。手工、自动和外部回执推进必须一致。

## 4 边界与不做
不重写业务测试，不新增调度内核或平行交付数据库，不以固定覆盖率百分比取代业务断言，不重新解释已发布历史。保留旧未提交文件和审计；不 push、不 tag、不静默安装，不以本修复 PASS 解除六湖整机阻塞。

## 5 歧义与默认值
复用既有需求包、测试执行记录和 artifact 账本，缺可机读执行证据时明确阻塞，不从文件存在推定验收成功。先在隔离账本以真实 CLI 跑一条合法通过与一条拒绝，再扩大负控和回归。用户已授权本轮确切路径的需求、实现、证据本地提交；最终发布安装需对确切候选另行确认。
