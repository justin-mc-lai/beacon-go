# 示例：并行工位

**用户说**：「两个模块同时改」

**mode_id**：`im-team`

**做法**
1. `beacon goal implement parallel-plan --run-id <id> --lakes a,b`
2. `beacon implement worktree create --run-id <id> --lake a`（b 同理）
3. 各自提交，`beacon implement worktree merge` 只允许卫星 → 主星
4. 受保护路径（.env/密钥/支付/迁移）拒绝合并

**产出**：两条工位 + 合并记录

**不要做**：两个工位互相合并；把 .env 合进主线
