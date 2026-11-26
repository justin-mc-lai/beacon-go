# 示例：旧需求包要变更

**用户说**：「order-refund 这个老包冻过了，AC-002 得改」

**mode_id**：`ch-legacy`

**做法**
1. 先判断：变更对象在意图树里吗？在则用 `beacon change request --node <id>`；只有 legacy 包才用 `change legacy`
2. `beacon change legacy --feature order-refund --summary "AC-002 规则调整" --by <id> --json`
3. 编辑 `docs/beacon/<version>/features/order-refund/` 内正文（现在已登记变更）
4. `beacon truth refreeze --feature order-refund` 关闭变更；并行变更落时用 `truth rebase --accept-current`
5. `beacon truth drift --feature order-refund` — 确认移动的 AC 就是预期项

**产出**：change id + 新指纹 + drift 报告

**不要做**：把 legacy 包当第二真源；未登记变更直接改冻结正文；reason-only 的空重冻
