# 示例：冻完要改怎么办

**用户说**：「AC-003 的金额单位要改成元」

**mode_id**：`tr-change`

**做法**
1. `beacon truth change --feature order-refund --summary "AC-003 单位改为元"`
2. 编辑包内正文（现在允许，因为已登记变更）
3. `beacon truth refreeze --feature order-refund`
4. `beacon truth drift --feature order-refund` — 确认移动的 AC 就是 AC-003

**产出**：变更记录 + 新指纹 + drift 报告

**不要做**：直接改冻结正文；改完不重冻
