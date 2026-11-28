# 示例：实现跑偏了怎么发现

**用户说**：「实现好像和需求不一致」

**mode_id**：`tr-drift`

**做法**
1. `beacon truth drift --feature order-refund` → 列出移动的 AC
2. 树节点修改交 change；旧包并行变更先核对来源和当前内容，经批准后才使用 `beacon truth rebase --change <id> --accept-current`，不因发现 drift 就自动接受。
3. 把结论写回 run 的 blocker，而不是口头说"我改一下"

**产出**：旧包 drift 清单（AC 编号 + 变化），或树的当前/冻结指纹对照与 change 路由；hash 一致不证明实现正确。回执列 run_id/current_stage/existing_artifacts/next_action/owner。

**不要做**：直接改实现让 drift 消失
