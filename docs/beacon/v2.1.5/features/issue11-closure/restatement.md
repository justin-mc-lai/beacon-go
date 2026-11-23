---
slug: issue11-closure
confirmed_by: owner-via-chat-continuation
confirmed_at: 2026-11-23
hash: 65bb9eca6a61c9c0797061f17a6a01db43e712d067f6bb5f35c731a62482fb9b
---

## 1 拆解
确定性非 LLM 候选写入同一 SDD 草稿，不能覆写真实需求或宣称语义就绪。
## 2 提案
新合同校验真实候选 AC、状态适用性与图引用，选择产生 SDD 草稿。
## 3 迁移
dry-run、冲突拒绝、原文件保留和备份；冻结内容先 change；迁移后重新评审确认。
## 4 验收
真实入口和独立验收后才能关闭 issue；安全沙箱未明确前不能称全项完成。
## 5 非目标
不自动删旧证据，不把规则拆分变成内核 LLM，不自行豁免隔离边界。
