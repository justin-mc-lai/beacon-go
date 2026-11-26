# 示例：冻结子树要改怎么办

**用户说**：「把订单退款子树的 AC-003 金额单位改成元」

**mode_id**：`ch-request`

**做法**
1. `beacon change request --project . --node <node-id> --summary "AC-003 单位改为元" --by <id> --json`
2. 记下返回的 change id 与 node id；祖先节点转为 `change_pending`
3. 编辑该子树中获授权的节点；不要直接改冻结的合同面
4. 自底向上重冻，直到 `change_pending` 清除；若触及 legacy 包，用 `change legacy` + `truth refreeze`
5. `beacon change list` — 确认变更记录与预期一致

**产出**：change id + node id + 重冻后的指纹

**不要做**：不经 request 直接改冻结合同；并发变更未用 `--base-fingerprint` 就覆盖
