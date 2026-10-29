# 示例：大海拆湖

**用户说**：「把整个结算系统重写」

**mode_id**：`pln-program`

**做法**
1. 判定这是 sea，不直接开跑
2. `beacon goal program plan --project . --feature settlement` — 建依赖图
3. 拆成可煮沸的湖：定价 → 优惠 → 对账 → 清分，标明顺序与依赖
4. 每个湖单独走 truth → implement

**产出**：依赖图 + 湖清单 + 每湖的入场条件

**不要做**：把"重写结算"当成一个 run 直接 tick
