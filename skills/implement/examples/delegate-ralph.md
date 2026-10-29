# 示例：外包给 ralph 循环

**用户说**：「这块让 ralph 一轮轮写」

**mode_id**：`im-ralph`

**做法**
1. `beacon implement delegate-ralph --run-id <id>`
2. ralph 每轮写一点、自检一次
3. `beacon implement receive-result` 收活：工件 + 证据按同一口径

**产出**：ralph 轮次记录 + 收获的工件

**不要做**：收活时跳过证据；把 ralph 的自检当成 QA 结论
