# 示例：验收没过就如实报

**用户说**：「过了吗」

**mode_id**：`qa-gate`

**做法**
1. `beacon qa evaluate ...` → `verdict=block`
2. 逐条读 reason codes，指到具体测试/工件
3. 把失败写进 run 的 blocker，交回 implement

**产出**：block 裁决 + reason codes

**不要做**：重跑到过为止；改测试让它过
