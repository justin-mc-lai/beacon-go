# 示例：实现跑偏了怎么发现

**用户说**：「实现好像和需求不一致」

**mode_id**：`tr-drift`

**做法**
1. `beacon truth drift --feature order-refund` → 列出移动的 AC
2. 若实现侧有并行变更：`beacon truth rebase --change <id> --accept-current`
3. 把结论写回 run 的 blocker，而不是口头说"我改一下"

**产出**：drift 清单（AC 编号 + 变化）

**不要做**：直接改实现让 drift 消失
